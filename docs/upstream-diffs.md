# Upstream Divergence Tracking

This file tracks modifications to files originating from [apache/incubator-devlake](https://github.com/apache/incubator-devlake)
that must be maintained during upstream syncs.

Owned plugins (`aireview`, `codecov`, `testregistry`, `agentready`, `langfuse`, `jira_snowflake`) are additions,
not modifications, and are not tracked here.

`jira_snowflake/tasks/convert_*.go` are adapted copies of `jira/tasks/` convertors — see the
[jira_snowflake AGENTS.md](../backend/plugins/jira_snowflake/AGENTS.md) for the diff details.

## gitextractor: ForceFullClone / FORCE_FULL_GIT_HISTORY

**Files:**
- `backend/plugins/gitextractor/impl/impl.go`
- `backend/plugins/gitextractor/parser/clone_gitcli.go`
- `backend/plugins/gitextractor/parser/taskdata.go`
- `env.example`

**Reason:** Upstream gitextractor incremental syncs miss commits via `--shallow-since`.
Adds a separate `ForceFullClone` strategy that bypasses shallow cloning entirely,
controlled by the `FORCE_FULL_GIT_HISTORY` environment variable. Also fixes a temp
directory leak in `doubleClone()`.

**Upstream status:** Pending
**Upstream PR:** none yet
**Owner:** @kpiwko

**Rebase notes:** Touches clone strategy selection in `clone_gitcli.go`.
Watch for upstream changes to `CloneRepo()`, `shallowClone()`, or `doubleClone()`.

## archived/base.go: inline Unsigned constraint

**Files:**
- `backend/core/models/migrationscripts/archived/base.go`

**Reason:** `golang.org/x/exp/constraints` was imported only for `constraints.Unsigned` in
`GenericModel`. Recent versions of `golang.org/x/exp` require Go 1.23+ (they import the
standard `cmp` package added in Go 1.21). The CI environment runs an older Go, so the
transitive dependency chain through `core/runner` → `archived/base.go` caused a `typecheck`
failure in golangci-lint for any PR that introduces a new plugin main package.

Replaced `constraints.Unsigned` with a locally-defined `unsignedInteger` interface that has
identical semantics, eliminating the `golang.org/x/exp` import entirely.

**Upstream status:** Pending submission upstream (trivial/safe change)
**Upstream PR:** none yet
**Owner:** @fmuntean

**Rebase notes:** If upstream changes `GenericModel`, check whether they still reference
`golang.org/x/exp/constraints` and reapply the inline if needed.

 ## jira: Scope collectParentIssues to current board
  
  **Files:**
  - `backend/plugins/jira/tasks/parent_issue_collector.go` 
  - `backend/plugins/jira/impl/impl.go`
  
  **Reason:** collectParentIssues queries all issues on the Jira connection for epic keys
  (filtering by connection_id only). Scoped the epic key query to the current board via board_id filter.
  
  **Upstream status:** N/A — collectParentIssues is Konflux-specific (commit f1c634d), not present in upstream Apache DevLake.
  **Upstream PR:** none — not applicable
  **Owner:** @cmulliga
  
  **Rebase notes:** `parent_issue_collector.go` is Konflux-only, no upstream conflicts expected.
  `impl.go` has a Konflux addition (`CollectParentIssuesMeta` in `SubTaskMetas()`) — watch for upstream changes to the subtask registration list.

## github: PR convertor incremental filtering by updated_at

**Files:**
- `backend/plugins/github/tasks/pr_convertor.go`
- `backend/plugins/github/tasks/pr_comment_convertor.go`
- `backend/plugins/github/tasks/pr_commit_convertor.go`
- `backend/plugins/github/tasks/pr_issue_convertor.go`
- `backend/plugins/github/tasks/pr_issue_enricher.go`
- `backend/plugins/github/tasks/pr_label_convertor.go`
- `backend/plugins/github/tasks/pr_review_convertor.go`
- `backend/plugins/github/tasks/review_convertor.go`

**Reason:** The convertor had no incremental filter. Filtering on `github_updated_at`
(when GitHub last modified the PR) causes silent data loss: a PR merged during a long
pipeline run gets its `github_updated_at` set before the watermark, so the next
incremental run skips it even though the extractor correctly upserted it as `MERGED`.
`lake.pull_requests` then permanently retains the stale `OPEN` status until a full re-sync.

Observed on `konflux-ci/build-definitions` (June 29): a ~6h partial pipeline pushed
the watermark to ~17:00; PRs closed between 15:00–17:00 were skipped by the convertor
on the next nightly run.

Fix: filter on `updated_at` (DB upsert timestamp), gated on `IsIncremental()`.

**Upstream status:** TO DO
**Upstream PR:** none yet
**Owner:** @rsoaresd

**Rebase notes:** Watch for upstream changes to `ConvertPullRequests()` input clause logic.
