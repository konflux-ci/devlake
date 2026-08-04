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

## core: IsBot field on domain accounts model

**Files:**
- `backend/core/models/domainlayer/crossdomain/account.go`
- `backend/core/models/migrationscripts/20260715_add_is_bot_to_accounts.go` (new file)
- `backend/core/models/migrationscripts/register.go` (append-only)

**Reason:** Added `IsBot bool` to the domain `Account` struct so GitHub/GitLab
account convertors can flag bot identity once, instead of every downstream
consumer (n8n workflows, Grafana dashboards) re-deriving it via provider-specific
`author_name LIKE '%[bot]%'`-style patterns. See
`docs/research/bot-commit-identification.md` for the full analysis. Tracked in
DPROD-1342.

**Upstream status:** Pending — not proposed upstream. This is a Konflux-specific
domain-model addition; upstream has no equivalent field.
**Upstream PR:** none
**Owner:** @kpiwko

**Rebase notes:** Pure field addition, low conflict risk. Migration file is new
(no conflict); `register.go` only needs an append at the end of the `All()` slice.

## github: IsBot field in account convertor

**Files:**
- `backend/plugins/github/tasks/account_convertor.go`

**Reason:** Populate the new `IsBot` field via `isBotAccount()` (API
`Type == "Bot"` or `[bot]`/`-bot`/`-robot`/copilot/dependabot/github-actions
login patterns) plus `hasNoProfileData()` (an account with no `avatar_url`
ever collected — real GitHub users always get a default identicon — is almost
always a bot the login/type patterns missed).

**History:** this fork previously carried a much larger divergence here — a
two-pass design (`ConvertAccounts` + a separate `convertOrphanedRepoAccounts`
pass) introduced by `cd9928159` ("fix: populate bot account identity...",
2026-05-13, DPROD-1259) to handle accounts referenced only via
`_tool_github_repo_accounts` with no matching `_tool_github_accounts` row.
That divergence was never logged here. While implementing `IsBot`, we found
upstream had independently fixed the exact same problem
([apache/devlake#8894](https://github.com/apache/devlake/pull/8894), fixes
[#8886](https://github.com/apache/devlake/issues/8886), merged 2026-06-12) via
a more complete, unified-query design (also fixing `_raw_data` provenance,
which our two-pass patch had broken). We adopted upstream's version verbatim
instead of extending our own — `account_convertor.go` now matches upstream's
`ConvertAccounts` exactly except for `IsBot` population, and the fork's
`convertOrphanedRepoAccounts`/`buildOrphanDomainAccount` are gone.

**Upstream status:** Pending — `IsBot` itself is not proposed upstream (the
underlying query/struct now match upstream exactly, via #8894).
**Upstream PR:** none yet (fork proposal: konflux-ci/devlake#118)
**Owner:** @kpiwko

**Rebase notes:** Low conflict risk — the query/struct match upstream's #8894
version exactly (`Type` field + its `COALESCE(ga.type, '')` select and the
`IsBot` line in `buildDomainAccount` are the only additions). Future upstream
changes to `ConvertAccounts` should apply cleanly except around those two
insertion points.

## github: zero-ID guard and merged_by emission — adopted from upstream, no outstanding divergence

**Files:**
- `backend/plugins/github/tasks/pr_convertor.go`
- `backend/plugins/github/tasks/pr_extractor.go`

Both files previously diverged from upstream (missing zero-ID guard on
`AuthorId`/`MergedById`; missing `merged_by` → `_tool_github_repo_accounts`
emission), both fixed by
[apache/devlake#8894](https://github.com/apache/devlake/pull/8894).

- `pr_extractor.go` is byte-identical to upstream (verified via diff against
  `apache/incubator-devlake`) — no outstanding divergence.
- `pr_convertor.go` is behavior-identical (same zero-ID guard semantics) but
  not byte-identical: we extracted the check into a named `hasValidAccountId()`
  helper for unit-testability, which upstream doesn't have. This is a
  deliberate, cosmetic divergence, not a functional one — worth a quick glance
  on future upstream syncs to `pr_convertor.go`, but not a rebase risk.

No entry needed going forward for either file — listed here only for
traceability since the fix landed alongside the `IsBot` work above.

**Upstream status:** N/A — functionally matches upstream, zero behavioral divergence.
**Owner:** @kpiwko

## gitlab: Bot identity (IsBot) in account convertor

**Files:**
- `backend/plugins/gitlab/tasks/account_convertor.go`

**Reason:** Populate `IsBot` via `isGitlabBotAccount()`. GitLab accounts have no
API-reported type field (unlike GitHub), so detection is username/name-pattern
only: `project_<id>_bot...`/`group_<id>_bot...` access-token prefixes,
`-bot`/`-robot` suffixes, and "Service Account" in the display name.

**Upstream status:** Pending — not proposed upstream.
**Upstream PR:** none yet (fork proposal: konflux-ci/devlake#118)
**Owner:** @kpiwko

**Rebase notes:** Upstream's `account_convertor.go` for GitLab is currently
unchanged from the version this fork started from (single `Convert` closure,
no orphan handling) — low conflict risk, isolated to the `Convert` closure body.

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
