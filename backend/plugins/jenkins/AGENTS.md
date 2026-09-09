# jenkins Plugin — Agent Context

Upstream Apache DevLake plugin extended by this fork with build-parameter collection, scope-configurable metadata extraction (`fieldExtractors`), and UNSTABLE result mapping. See `docs/upstream-diffs.md` for the full list of divergences.

## Build & Test

```bash
cd backend
go test ./plugins/jenkins/... -v              # unit tests
go test ./plugins/jenkins/e2e/... -v           # e2e tests (needs MySQL + lake_test)
golangci-lint run ./plugins/jenkins/...        # lint
```

## Layout

- `impl/impl.go` — plugin interfaces; `EnrichOptions` loads scope config when only `scopeConfigId` is present
- `models/` — tool-layer models + `migrationscripts/register.go`
- `models/field_extractor_rule.go` — `FieldExtractorRule` type stored as JSON in scope config
- `models/build_parameter.go` — `_tool_jenkins_build_parameters` table
- `tasks/field_extractor.go` — `FieldExtractor`: compiles rules, applies them to `JenkinsBuild.Metadata`
- `tasks/build_extractor.go` — calls `NewFieldExtractor` and `ExtractBuildParameters`
- `tasks/build_cicd_convertor.go` — maps UNSTABLE → FAILURE; preserves `OriginalResult`
- `tasks/stage_convertor.go` — applies same result rule (including UNSTABLE) to CICD tasks
- `e2e/snapshot_tables/` — CSV snapshots for e2e tests

## FieldExtractor Design

`FieldExtractorRule` (JSON in `_tool_jenkins_scope_configs.field_extractors`) lets teams define metadata dimensions without modifying plugin code:

```json
{
  "key": "ocp_version",
  "sources": ["parameter:OCP_VERSION", "full_name"],
  "pattern": "ocp-([0-9]+\\.[0-9]+)",
  "group": 1,
  "default": "unknown",
  "onlyIfEmpty": true
}
```

- **sources**: tried in order; first non-empty match wins. Built-ins: `full_name`, `job_name`, `triggered_by`, `parameter:<name>`.
- **pattern**: optional Go regex. If present, applies to the resolved source value. `group` selects a capture group (0 = full match).
- **default**: used when all sources produce empty/no-match.
- **onlyIfEmpty**: skip rule if `Metadata[key]` is already set.
- `NewFieldExtractor` returns `nil, nil` when no rules are configured; the nil guard in `build_extractor.go` is intentional.

## UNSTABLE Result Mapping

Jenkins' `UNSTABLE` result maps to `FAILURE` in both `cicd_pipelines.result` and `cicd_tasks.result` (stages). The raw Jenkins value is preserved in `cicd_pipelines.original_result`. Both `build_cicd_convertor.go` and `stage_convertor.go` must include `UNSTABLE` in their Failure lists to keep the domain layer consistent.

## Conventions

- **Parameters last-wins**: `ExtractBuildParameters` iterates build actions in order; if the same parameter name appears in multiple actions, the last value is used.
- New scope-config fields require a migration script in `models/migrationscripts/` and a registration in `register.go:All()`.
- Don't add models to the tool layer without a migration.
- Don't import from other plugins.
- Don't skip the Apache 2.0 license header on new files.
- Use `strings.EqualFold()` for case-insensitive string comparisons.

## Pattern References

| Change Type | Example File |
|---|---|
| Add metadata dimension | `models/field_extractor_rule.go`, `tasks/field_extractor.go` |
| Add build collection field | `models/response.go`, `tasks/build_extractor.go` |
| Add migration | `models/migrationscripts/20260902_add_build_parameters_and_metadata.go` |
| Update CICD result mapping | `tasks/build_cicd_convertor.go` + `tasks/stage_convertor.go` (both must stay in sync) |
| Add e2e snapshot | `e2e/snapshot_tables/` CSV files |
