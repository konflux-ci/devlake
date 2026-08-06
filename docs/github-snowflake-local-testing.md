# github_snowflake Plugin — Local Testing Guide

## Prerequisites

- Go 1.21+
- podman + podman-compose
- Access to the Snowflake account (`GITHUB_DB.MARTS`)
- Snowflake role `GITHUB_GROUP` **granted to your user** (SSO login alone is not enough —
  ask a Snowflake admin if `USE ROLE GITHUB_GROUP` fails)
- Warehouse access (e.g. `DEFAULT`) for that role
- A konflux-ci repo to pilot (`githubId` + `owner/repo` full name)

---

## Step 1 — Verify Snowflake access

Before starting DevLake, confirm you can reach Snowflake from your machine.

### 1a — Check role access in the Snowflake console

In a Snowflake worksheet (use your real username from `SELECT CURRENT_USER()`, not the
literal string `CURRENT_USER`):

```sql
SELECT CURRENT_USER(), CURRENT_ROLE();

-- Replace with the username returned above (quoted string, not CURRENT_USER as an identifier):
-- SHOW GRANTS TO USER "your.username";

USE ROLE GITHUB_GROUP;
USE WAREHOUSE DEFAULT;
USE DATABASE GITHUB_DB;
USE SCHEMA MARTS;

SELECT COUNT(*) FROM REPOSITORY;
SELECT ID, FULL_NAME FROM REPOSITORY ORDER BY FULL_NAME LIMIT 20;
```

If `USE ROLE GITHUB_GROUP` fails with “not granted to this user”, stop and request the
role from a Snowflake admin before continuing.


### 1b — Check from Go (same auth path as the plugin)

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
| `Role 'GITHUB_GROUP' ... is not granted to this user` | Ask a Snowflake admin to grant `GITHUB_GROUP` to your user; verify with `USE ROLE GITHUB_GROUP` in a Snowflake worksheet |
| `Object does not exist or not authorized` | Your role lacks `SELECT` on `GITHUB_DB.MARTS` — ask a Snowflake admin |
| Browser window never opens | You may be running inside a container or headless SSH session — run locally |
| `User 'CURRENT_USER' does not exist` | `SHOW GRANTS TO USER` needs a real username string from `SELECT CURRENT_USER()`, not `CURRENT_USER` as an identifier |

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

## Step 4 — Build and run DevLake

Browser-based SSO (`externalbrowser` auth) only works when DevLake runs **natively on your desktop**, not inside a container (the browser pop-up cannot reach a container process).

`github_snowflake` writes into the shared `_tool_github_*` tables owned by the **github** plugin. On a fresh database those tables only exist after github's migrations run, so include `github` in `DEVLAKE_PLUGINS` at least for the first boot:

```bash
cd backend
DEVLAKE_PLUGINS=github,github_snowflake DISABLED_REMOTE_PLUGINS=true ENV_FILE=../.env make build-plugin run
```

- `DEVLAKE_PLUGINS=github,github_snowflake` — load github (for `_tool_github_*` schema) + this plugin
- `DISABLED_REMOTE_PLUGINS=true` — skip loading remote/dynamic plugins
- `ENV_FILE=../.env` — point to the `.env` in the repo root

After the tool tables exist, you can drop back to `DEVLAKE_PLUGINS=github_snowflake` for faster rebuilds if you prefer.

Verify the plugins loaded:

```bash
curl -s http://localhost:8080/plugins | jq '.[] | select(.plugin == "github_snowflake" or .plugin == "github")'
```

Trigger DB migrations (creates `_tool_github_snowflake_connections` and, via github, `_tool_github_*`):

```bash
curl -s http://localhost:8080/proceed-db-migration | jq .
```

Verify the tables exist:

```bash
podman compose -f docker-compose-dev.yml exec mysql \
  mysql -umerico -pmerico lake -e "
    SHOW TABLES LIKE '_tool_github_snowflake%';
    SHOW TABLES LIKE '_tool_github_repos';
  "
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
| `Table 'lake._tool_github_repos' doesn't exist` | github plugin migrations never ran | Restart with `DEVLAKE_PLUGINS=github,github_snowflake` and re-run `/proceed-db-migration` |
| `failed to decode PEM block from private key` | Connection is on `keypair` with empty/invalid key | Recreate or PATCH connection with `"authType": "externalbrowser"` (local) or a valid PKCS#8 `privateKey` |

| `tool_prs` populated but `domain_prs` = 0 | Convertor subtask failed | Check pipeline subtask logs; ensure convertors are enabled |
| Same repo also on GitHub API connection | Domain ID duplication risk | Remove the repo from one of the two connections |
