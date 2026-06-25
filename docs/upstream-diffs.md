# Upstream Divergence Tracking

This file tracks modifications to files originating from [apache/devlake](https://github.com/apache/devlake)
that must be maintained during upstream syncs.

Owned plugins (`aireview`, `codecov`, `testregistry`, `agentready`, `langfuse`) are additions,
not modifications, and are not tracked here.

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

## github: Bot identity — GraphQL Actor support & `IsBot` domain field (Planned)

**Files (currently diverged):**
- `backend/plugins/github/tasks/account_convertor.go`
- `backend/plugins/github/tasks/pr_convertor.go` — *will revert to upstream (restore zero-ID guard)*
- `backend/plugins/github/tasks/pr_extractor.go` — *will revert to upstream (restore MergedBy repo_account emission)*
- `backend/plugins/github_graphql/tasks/account_graphql_pre_extractor.go`
- `backend/plugins/github_graphql/tasks/pr_collector.go`
- `backend/plugins/github_graphql/tasks/pr_extractor.go`
- `backend/plugins/github_graphql/tasks/issue_collector.go`
- `backend/plugins/github_graphql/tasks/issue_extractor.go`
- `backend/core/models/domainlayer/crossdomain/account.go`

**Reason:** Upstream only handles `User` type in GitHub GraphQL actor fields. Fork
adds `Bot` type support via `GraphqlInlineActorQuery` (PR #84) so bot identities
(`dependabot[bot]`, `renovate[bot]`) are preserved in the domain layer instead of
appearing as empty/unknown authors.

The domain `accounts` model gains an `IsBot` boolean field, populated during account
conversion using provider-specific signals:
- GitHub: `_tool_github_accounts.Type == "Bot"` or login matches `*[bot]`
- GitLab: username matches `project_*_bot*`, `group_*_bot*`, `*-bot`, or name
  contains "Service Account"

`account_convertor.go` adds a small second pass (`convertOrphanedRepoAccounts()`) to create
domain accounts for bots that return 404 from `/users/<login>` REST API.

**Upstream status:** Pending — GraphQL Bot fragment and `IsBot` field are candidates
for upstream contribution.
**Upstream PR:** none yet
**Owner:** @kpiwko

**Rebase notes:** GraphQL files (`account_graphql_pre_extractor.go`, collectors,
extractors) add new types and functions — watch for upstream actor handling changes.
`account_convertor.go` adds the orphan pass function at the end — low conflict risk
since the main converter matches upstream's structure. `account.go` adds one field —
trivial merge.

For reduction analysis from the current state, see `docs/research/bot-commit-identification.md`.
