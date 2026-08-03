# github_snowflake Plugin — Agent Context

Drop-in replacement for the GitHub API plugin per repository. Reads GitHub data
from a Snowflake replica (Fivetran, `GITHUB_DB.MARTS`) and writes into the
existing `_tool_github_*` tool-layer tables, then runs domain-layer convertors to
produce `code.*` domain records.

## Build & Test

```bash
cd backend
go build ./plugins/github_snowflake/...
go test  ./plugins/github_snowflake/... -v
golangci-lint run ./plugins/github_snowflake/...
```

Unit tests do **not** need MySQL. A real pipeline run does.

### Minimal real-run setup

1. Start MySQL: `podman compose -f docker-compose-dev.yml up -d mysql`
2. Ensure `.env` has `DB_URL`, `ENCRYPTION_SECRET`, and (for local API) `AUTH_ENABLED=false`.
   For native `make run`, use `localhost` (or `127.0.0.1`) in `DB_URL` — `env.example`'s
   `@mysql:3306` host only resolves inside the compose network.
3. Run the server (native desktop required for `externalbrowser` SSO):

```bash
cd backend
# Include github so its migrations create shared _tool_github_* tables (needed on a fresh DB).
DEVLAKE_PLUGINS=github,github_snowflake DISABLED_REMOTE_PLUGINS=true ENV_FILE=../.env make build-plugin run
```

4. Create a connection (`authType: externalbrowser` for local desktop) + run a pipeline for one repo (`githubId` + `fullName`)

Full step-by-step (Snowflake check, migrations, curl examples, result queries):
[docs/github-snowflake-local-testing.md](../../../docs/github-snowflake-local-testing.md)

## Layout

```
impl/impl.go              — plugin interfaces, SubTaskMetas, PrepareTaskData
api/connection_api.go     — connection CRUD (POST/GET/PATCH/DELETE)
models/connection.go      — SnowflakeGithubConnection (table: _tool_github_snowflake_connections)
models/migrationscripts/  — DB migrations
tasks/task_data.go        — GithubSnowflakeOptions, GithubSnowflakeTaskData, OpenSnowflakeDB
tasks/shared.go           — URL helpers
tasks/sync_*.go           — Snowflake SQL queries → _tool_github_* tool-layer tables
tasks/convert_*.go        — domain-layer convertors (adapted copies of github/tasks/*)
```

## Subtask pipeline order

1. `syncRepos`         — REPOSITORY → `_tool_github_repos`
2. `syncPullRequests`  — PULL_REQUEST ⨝ ISSUE ⨝ ISSUE_MERGED ⨝ USER → `_tool_github_pull_requests`
3. `syncPrCommits`     — COMMIT_PULL_REQUEST ⨝ COMMIT → `_tool_github_pull_request_commits`
4. `syncPrReviews`     — PULL_REQUEST_REVIEW ⨝ USER → `_tool_github_pull_request_reviews`
5. `syncReviewers`     — REQUESTED_REVIEWER_HISTORY → `_tool_github_reviewers`
6. `syncAccounts`      — USER (+ USER_EMAIL) → `_tool_github_accounts` + `_tool_github_repo_accounts`
7. `convertRepo` / `convertPullRequests` / `convertPrCommits` / `convertPrReviews` / `convertReviews` / `convertAccounts`

## Key conventions

- **Scope unit is `GithubRepo`** (numeric `githubId` + `fullName` owner/repo).
- **No raw-table layer**: writes directly to `_tool_github_*`. Convertors use
  `_raw_data_params`-scoped deletion for full sync (same pattern as jira_snowflake).
- **AuthType**: `"keypair"` (default, JWT) or `"externalbrowser"` (SSO, desktop only).
- **Connection defaults**: Database=`GITHUB_DB`, Schema=`MARTS`, Warehouse=`DEFAULT`.
- **PR line/comment counts** are unavailable in Snowflake — leave 0.
- **Actions jobs** table does not exist — never enable job sync/convertors.
- Do not log commit author emails in debug output.

## Snowflake schema notes (GITHUB_DB.MARTS — verified 2026-07-21)

| Table | Notes |
|---|---|
| `REPOSITORY` | No HTML/Clone URL — derive from FULL_NAME |
| `PULL_REQUEST` + `ISSUE` + `ISSUE_MERGED` | Fivetran splits PR fields across three tables |
| `COMMIT_PULL_REQUEST` | Orphan PR links exist — always INNER JOIN PULL_REQUEST |
| `PULL_REQUEST_REVIEW` | States: APPROVED, COMMENTED, DISMISSED, CHANGES_REQUESTED |
| `REQUESTED_REVIEWER_HISTORY` | Filter `REQUESTED_REVIEWER_TYPE = 'user'`; take latest non-removed |
| `"USER"` / `USER_EMAIL` | Identity present (not PII-stripped). Quote `"USER"` — reserved keyword in Snowflake. |

Pilot coverage today: **konflux-ci** org only in MARTS.

## GitHub plugin models dependency

Imports `plugins/github/models` for tool-layer structs. This is a shared schema
dependency, not a business-logic cross-import. Runtime API/business logic does
not require the github plugin, but a fresh DB still needs github loaded once so
its migrations create `_tool_github_*`. `impl.Init` registers a minimal
`githubPluginStub` for didgen.

## Don'ts

- Don't add models without a migration in `migrationscripts/register.go`
- Don't skip the Apache 2.0 license header on new `.go` files
- Don't configure the same repo in both a GitHub API connection and a
  github_snowflake connection simultaneously — this causes domain ID duplication
- Don't enable Actions job convertors (table missing in Snowflake)
