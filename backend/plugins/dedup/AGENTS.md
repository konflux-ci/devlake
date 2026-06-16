# dedup Plugin — Agent Context

Metric plugin that eliminates duplicate domain-layer records caused by the same physical GitHub (or GitLab) repository being collected via multiple connections. It populates a `_tool_dedup_canonical_scopes` mapping table and creates four MySQL views (`deduped_repos`, `deduped_pull_requests`, `deduped_issues`, `deduped_repo_commits`) that Grafana dashboards query instead of the raw domain tables.

## Build & Test

```bash
cd backend
go test ./plugins/dedup/... -v
golangci-lint run ./plugins/dedup/...
```

Single-file verification: `go vet ./plugins/dedup/...`

## Layout

- `impl/impl.go` — plugin interfaces (PluginMeta, PluginTask, PluginMetric, MetricPluginBlueprintV200)
- `models/canonical_scope.go` — `_tool_dedup_canonical_scopes` model
- `models/migrationscripts/init_schema.go` — creates the mapping table + 4 MySQL views
- `models/migrationscripts/register.go` — `All()` returning all migration scripts
- `tasks/task_data.go` — `DedupOptions` and `DedupTaskData`
- `tasks/collect_canonical_scopes.go` — subtask: queries `_tool_github_repos`, picks MIN(connection_id) per html_url, upserts canonical mappings

## How It Works

1. The `collectCanonicalScopes` subtask runs after GitHub collection (metric plugin stage).
2. It queries `_tool_github_repos` grouped by `html_url` and picks the numerically lowest `connection_id` as the canonical connection for each physical repository.
3. It constructs the canonical domain ID (e.g. `github:GithubRepo:1:498260751`) and upserts it into `_tool_dedup_canonical_scopes`.
4. The four MySQL views filter domain tables through this mapping, so only canonical records are returned.
5. Grafana dashboards use these views instead of the raw domain tables.

## Problem Being Solved

When a repository appears in DevLake under N connections (e.g. connections 1, 38, 55, 57), every entity (PR, issue, repo_commit) is stored N times with N different domain IDs. Metrics are inflated by factor N. This plugin resolves it without touching any upstream DevLake code or modifying the raw data.

See `docs/upstream-diffs.md` for the rationale and upstream issue link.

## Conventions

- This is a **metric plugin** (not data-source): no Connection or Scope models
- Implements `MetricPluginBlueprintV200`; runs after github/gitlab
- The canonical scope computation is **global** (not per-project); running it N times per project is idempotent
- Currently supports GitHub only; GitLab support can be added by extending `collectCanonicalScopes` to also query `_tool_gitlab_projects`

## Don'ts

- Don't add models to `GetTablesInfo()` without a migration script in `migrationscripts/register.go`
- Don't import from other plugins (plugins must be independent)
- Don't skip the Apache 2.0 license header on new files
- Don't use string `MIN(id)` for canonical selection — connection IDs must be compared numerically

## Pattern References

| Change Type | Example File |
|---|---|
| Add GitLab support | extend `tasks/collect_canonical_scopes.go` to also query `_tool_gitlab_projects` |
| Add a new view | new migration script in `models/migrationscripts/` + update `register.go` |
| Update view SQL | new migration script with `CREATE OR REPLACE VIEW ...` |
