# metrics Plugin — Agent Context

Standalone HTTP API that queries DevLake's MySQL database and exposes pre-computed
metrics. Not part of the DevLake plugin framework — ships as an independent binary.

## Build & Run

```bash
# From backend/
make build-metrics-api

# Run locally
export MYSQL_HOST=<MYSQL_HOST>
export MYSQL_PORT=<MYSQL_PORT>
export MYSQL_USER=<MYSQL_USER>
export MYSQL_PASS=<MYSQL_PASS>
export MYSQL_DB=<MYSQL_DB>
export METRICS_ADDR=:<METRICS_ADDR>
export METRICS_ALLOWED_ORIGIN=<METRICS_ALLOWED_ORIGIN>
./bin/metrics-api

# Test
go test ./plugins/metrics/... -v
golangci-lint run ./plugins/metrics/...
```

## Layout

```
cmd/
  main.go
api/
  server.go
  middleware.go
  params.go
  routes/pr/
    register.go
    handlers/
      utils.go
      key_metrics.go
      stages.go
      cycle_time.go
      productivity.go
      flow.go
      zscore.go
      scatter.go
model/
  response.go
query/
  db.go
  pr/
    rows.go
    build.go
    queries.go
testsupport/
  time.go
transform/
  botfilter/
    botfilter.go
  pr/
    helpers.go
    key_metrics.go
    stages.go
    cycle_time.go
    flow.go
    zscore.go
    productivity.go
    scatter.go
    *_test.go
```

## URL Scheme

All routes: `POST /api/metrics/<category>/<name>`

| Category | Example path |
|---|---|
| PR | `/api/metrics/pr/key-metrics` |


## Request Body

All endpoints accept the same JSON body:

```json
{
  "owner": ["org"],
  "name": ["repo"],
  "from": 1700000000,
  "to": 1702000000,
  "blueprintid": "72",
  "connectionid": "1",
  "jiraproject": "PROJ",
  "projects": ["project-name"],
  "userwhitelist": [],
  "teamrepos": [],
  "projectname": "PRCT - Team"
}
```

## Response Shape

All endpoints return FORMAT.md JSON — see `model/response.go` for Go types.

## Conventions

- Transforms are pure functions — no I/O, no DB; takes rows → returns `model.MetricResponse`
- SQL lives in `query/pr/queries.go`; no SQL in handlers or transform files
- New SQL fragments use string concatenation (`+`) not `fmt.Sprintf` — see existing queries for the pattern
- Unit tests live next to source: `transform/pr/foo.go` → `transform/pr/foo_test.go`
- Use `testsupport.ParseTime()` for `*time.Time` fixtures in tests
- This binary is excluded from `build-plugins.sh` — build separately with `go build ./plugins/metrics/cmd/`
- Apache 2.0 license header required on all `.go` files


## Dont's

- Don't import from other plugins (plugins must be independent)
- Don't skip the Apache 2.0 license header on new `.go` files
