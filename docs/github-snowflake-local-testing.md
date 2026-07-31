# github_snowflake Plugin — Local Testing Guide

## Prerequisites

- Go 1.21+
- podman + podman-compose
- Access to the Snowflake account (`GITHUB_DB.MARTS`)
- Snowflake role with `SELECT` on that schema (e.g. `GITHUB_GROUP`)
- A konflux-ci repo to pilot (`githubId` + `owner/repo` full name)

---

## Step 1 — Verify Snowflake access

Before starting DevLake, confirm you can reach Snowflake from your machine.

Create a throwaway Go file (outside the repo):

```go
// /tmp/check_github_snowflake.go
package main

import (
	"database/sql"
	"fmt"
	"log"

	sf "github.com/snowflakedb/gosnowflake"
)

func main() {
	cfg := &sf.Config{
		Account:       "YOUR_ACCOUNT", // e.g. "myorg-myaccount" (not the full URL)
		User:          "YOUR_USER",
		Role:          "GITHUB_GROUP",
		Warehouse:     "DEFAULT",
		Database:      "GITHUB_DB",
		Schema:        "MARTS",
		Authenticator: sf.AuthTypeExternalBrowser, // opens browser for SSO
	}
	dsn, err := sf.DSN(cfg)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var n int
	row := db.QueryRow("SELECT COUNT(*) FROM REPOSITORY")
	if err := row.Scan(&n); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("OK — %d repositories visible\n", n)
}
```

Run it from `backend/` where `gosnowflake` is already in `go.mod`:

```bash
cd backend
go run /tmp/check_github_snowflake.go
# A browser window opens for SSO — log in once, then the script prints:
# OK — NNN repositories visible
```

Pick a pilot repo ID and full name:

```sql
SELECT ID, FULL_NAME FROM REPOSITORY ORDER BY FULL_NAME LIMIT 20;
```

**Common errors:**

| Error | Fix |
|---|---|
| `account is empty` | Use the account identifier (e.g. `myorg-myaccount`), not the full `*.snowflakecomputing.com` URL |
| `Object does not exist or not authorized` | Your role lacks `SELECT` on `GITHUB_DB.MARTS` — ask a Snowflake admin |
| Browser window never opens | You may be running inside a container or headless SSH session — run locally |

---

## Step 2 — Start MySQL

DevLake needs MySQL for connections, tool-layer tables, and domain tables. Unit tests do **not** need MySQL; a real pipeline run does.

```bash
podman machine start   # if not already running
podman compose -f docker-compose-dev.yml up -d mysql
```

Confirm MySQL is ready:

```bash
podman compose -f docker-compose-dev.yml exec mysql \
  mysql -umerico -pmerico lake -e "SELECT 1;"
```

---

## Step 3 — Configure `.env`

Create or edit `.env` in the repo root:

```env
# Disable auth for local testing (skip bearer token on every API call)
AUTH_ENABLED=false

# Required: any 32+ char string works for local dev — replace with your own value
ENCRYPTION_SECRET=<generate-a-random-32-char-string-here>

DB_URL=mysql://merico:merico@127.0.0.1:3306/lake?charset=utf8mb4&parseTime=True

# Generate ENCRYPTION_SECRET with:
# openssl rand -hex 16
```

---

## Step 4 — Build and run DevLake (plugin only)

Browser-based SSO (`externalbrowser` auth) only works when DevLake runs **natively on your desktop**, not inside a container (the browser pop-up cannot reach a container process).

```bash
cd backend
DEVLAKE_PLUGINS=github_snowflake DISABLED_REMOTE_PLUGINS=true ENV_FILE=../.env make build-plugin run
```

- `DEVLAKE_PLUGINS=github_snowflake` — only compile this plugin (much faster than building all plugins)
- `DISABLED_REMOTE_PLUGINS=true` — skip loading remote/dynamic plugins
- `ENV_FILE=../.env` — point to the `.env` in the repo root

Verify the plugin loaded:

```bash
curl -s http://localhost:8080/plugins | jq '.[] | select(.plugin == "github_snowflake")'
```

Trigger DB migrations (creates `_tool_github_snowflake_connections` table):

```bash
curl -s http://localhost:8080/proceed-db-migration | jq .
```

Verify the table exists:

```bash
podman compose -f docker-compose-dev.yml exec mysql \
  mysql -umerico -pmerico lake -e "SHOW TABLES LIKE '_tool_github_snowflake%';"
```

---

## Step 5 — Create a connection

```bash
curl -s -X POST http://localhost:8080/plugins/github_snowflake/connections \
  -H 'Content-Type: application/json' \
  -d '{
    "name":      "my-snowflake-github",
    "account":   "YOUR_ACCOUNT",
    "user":      "YOUR_USER",
    "authType":  "externalbrowser",
    "database":  "GITHUB_DB",
    "schema":    "MARTS",
    "warehouse": "DEFAULT",
    "role":      "GITHUB_GROUP"
  }' | jq .
```

Note the `id` in the response — that is your `connectionId`.

> For **key-pair auth** (production / CI), use `"authType": "keypair"` and
> add `"privateKey": "-----BEGIN PRIVATE KEY-----\n..."`.
> Key-pair auth works inside containers and does not require a browser.

---

## Step 6 — Run a pipeline

You need:
- `connectionId` from Step 5
- `githubId`: numeric GitHub repo ID from Snowflake `REPOSITORY.ID`
- `name` / `fullName`: `owner/repo` (e.g. `konflux-ci/build-service`)

```bash
curl -s -X POST http://localhost:8080/pipelines \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "github-snowflake-test",
    "plan": [[{
      "plugin":  "github_snowflake",
      "options": {
        "connectionId": 1,
        "githubId":     123456789,
        "name":         "konflux-ci/build-service",
        "fullName":     "konflux-ci/build-service"
      }
    }]]
  }' | jq '{id, status}'
```

Poll the pipeline until it completes:

```bash
PIPELINE_ID=<id from above>
while true; do
  curl -s http://localhost:8080/pipelines/$PIPELINE_ID | jq '{status, message}'
  sleep 5
done
```

A successful run ends with `"status": "TASK_COMPLETED"`.

On the first `externalbrowser` sync, Snowflake opens a browser window for SSO — complete login once.

---

## Step 7 — Verify results in MySQL

```bash
MYSQL="podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e"

# Row counts — tool layer and domain layer
$MYSQL "SELECT
  (SELECT COUNT(*) FROM _tool_github_repos WHERE connection_id = 1)              AS tool_repos,
  (SELECT COUNT(*) FROM _tool_github_pull_requests WHERE connection_id = 1)      AS tool_prs,
  (SELECT COUNT(*) FROM pull_requests)                                           AS domain_prs,
  (SELECT COUNT(*) FROM _tool_github_pull_request_reviews WHERE connection_id = 1) AS tool_reviews,
  (SELECT COUNT(*) FROM _tool_github_accounts WHERE connection_id = 1)           AS tool_accounts\G"

# Sample PRs
$MYSQL "SELECT number, state, title, author_name, merged
        FROM _tool_github_pull_requests
        WHERE connection_id = 1
        ORDER BY github_updated_at DESC
        LIMIT 10;"
```

**Expected results:**
- At least one row in `_tool_github_repos` for the pilot repo
- `tool_prs` > 0 for an active konflux-ci repo; `domain_prs` should match after convertors run
- Author/reviewer logins populated (unlike Jira Snowflake PII stripping)

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Plugin not listed at `/plugins` | Build step skipped or failed | Re-run `make build-plugin run` and check for compile errors |
| `ENCRYPTION_SECRET` error on startup | `.env` missing or key too short | Set a 32+ char hex string in `.env` |
| `401 Unauthorized` on API calls | Auth enabled | Set `AUTH_ENABLED=false` in `.env` |
| MySQL connection refused | MySQL not running | `podman compose -f docker-compose-dev.yml up -d mysql` |
| `invalid identifier 'X'` in pipeline logs | Column/table name mismatch | `DESCRIBE TABLE GITHUB_DB.MARTS.<TABLE>;` in Snowflake |
| `Object 'USER' does not exist` | Unquoted reserved keyword | Queries must use `"USER"` (already done in sync tasks) |
| Browser pop-up doesn't open | Running inside a container | Run DevLake natively with `make run`, not via `podman compose` |
| Pipeline ends with `TASK_FAILED` | Check `message` field | `curl -s http://localhost:8080/pipelines/$ID \| jq .message` |
| `tool_prs` populated but `domain_prs` = 0 | Convertor subtask failed | Check pipeline subtask logs; ensure convertors are enabled |
| Same repo also on GitHub API connection | Domain ID duplication risk | Remove the repo from one of the two connections |
