# Upstream Divergence Tracking

This file tracks modifications to files originating from [apache/incubator-devlake](https://github.com/apache/incubator-devlake)
that must be maintained during upstream syncs.

Owned plugins (`aireview`, `codecov`, `testregistry`, `agentready`, `langfuse`, `jira_snowflake`) are additions,
not modifications, and are not tracked here.

Shared internal packages under `backend/pkg/` (`gcshelper`, `oidchelper`) are also additions,
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

## gitlab: Map WorkInProgress to IsDraft in MR converter

**Files:**
- `backend/plugins/gitlab/tasks/mr_convertor.go`

**Reason:** `WorkInProgress` (from the GitLab API `work_in_progress` field) was extracted and
stored in `_tool_gitlab_merge_requests` but never forwarded to `code.PullRequest.IsDraft`
in the converter. This meant draft/WIP MRs were indistinguishable from non-draft MRs at the
domain layer. One-liner fix: `IsDraft: gitlabMr.WorkInProgress`.

**Upstream status:** Pending — should be contributed upstream as a bug fix.
**Upstream PR:** none yet
**Owner:** @fmuntean

**Rebase notes:** Change is isolated to the struct literal in `mr_convertor.go`'s `Convert` func.
No conflicts expected unless upstream touches the same field mapping block.

## server/api/auth: OIDC authentication

**Files:**
- `backend/server/api/auth/auth.go`
- `backend/server/api/auth/middleware.go`
- `backend/server/api/auth/handlers_test.go`
- `backend/server/api/auth/store.go`
- `backend/server/api/auth/cleanup.go`
- `backend/server/api/auth/revocation_cache.go`
- `backend/server/api/auth/revocation_cache_test.go`
- `backend/server/api/auth/auth_test.go`
- `env.example` (OIDC/auth env var documentation)

**Reason:** Upstream DevLake has no user authentication. This fork adds full OIDC login
(login/callback/logout, session JWT with server-side revocation, CSRF double-submit cookie)
for the internal Red Hat / Konflux deployment. Supports multiple OIDC providers (Microsoft
Entra ID, Google) and Azure Workload Identity federated code exchange. Auth is enabled by
default; `AUTH_ENABLED=false` is required to opt out (e.g. in local dev or CI).

The supporting library lives in `backend/pkg/oidchelper/` (an owned addition, not tracked here).

**Upstream status:** N/A — upstream Apache DevLake has no auth middleware.
**Upstream PR:** none — not applicable
**Owner:** @fmuntean

**Rebase notes:**
- `auth.go` / `middleware.go` wire into `backend/server/api/api.go` via `auth.Init()` and three
  `router.Use()` calls — watch for upstream changes to `api.go`'s middleware chain.
- `store.go` introduces the `auth_sessions` table; no upstream equivalent.
- `env.example` additions are at the end of the file and unlikely to conflict.

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

**Upstream status:** Submitted (awaiting merge)
**Upstream PR:** https://github.com/apache/devlake/pull/9032
**Owner:** @fmuntean

**Rebase notes:** If upstream changes `GenericModel`, check whether they still reference
`golang.org/x/exp/constraints` and reapply the inline if needed.

## docker-compose-dev: local UBI image builds

**Files:**
- `docker-compose-dev.yml`
- `Makefile` (root `build-config-ui-image`)
- `backend/Makefile` (`build-server-image`)
- `backend/Dockerfile`, `config-ui/Dockerfile` (DEPRECATED comments only)

**Reason:** Points `devlake` and `config-ui` builds at UBI Containerfiles
(`dockerfile: Containerfile`) and tags images as `localhost/devlake-*:local`
instead of personal Quay tags / Apache Scarf images. Makefile image targets use
`Containerfile`. Debian Dockerfiles are marked DEPRECATED pending removal.
Konflux Tekton builds `backend/Containerfile` and `config-ui/Containerfile`
(fork-only `backend/Dockerfile.konflux` removed).

**Upstream status:** N/A — Konflux local-dev / image-build customization.
**Upstream PR:** none — not applicable
**Owner:** @fmuntean

**Rebase notes:** If upstream regenerates `docker-compose-dev.yml`, re-add under
`devlake` / `config-ui`: `dockerfile: Containerfile` and
`image: localhost/devlake-backend:local` / `localhost/devlake-frontend:local`.
If upstream changes Makefile image targets or Dockerfiles, re-point builds at
`Containerfile` and keep DEPRECATED headers until the Debian files are deleted.

(`backend/Containerfile` and `config-ui/Containerfile` are fork additions, not
tracked here.)

## jira: cleanup stale board associations after tickets leave/change board

**Files:**
- `backend/plugins/jira/impl/impl.go`
- `backend/plugins/jira/tasks/stale_board_issue_cleaner.go`
- `backend/plugins/jira/tasks/stale_board_issue_cleaner_test.go`

**Reason:** Incremental Jira collection only adds board associations; it never removes
issues that left the board or moved to a different team. Stale rows in
`_tool_jira_board_issues` / domain `board_issues` keep those tickets on the old
board in metrics. Adds `cleanupStaleBoardIssues`, which batch-checks membership
via `agile/1.0/board/:id/issue` JQL (`issue IN (...)`, 100 per request),
re-fetches current issue state, and deletes the stale associations.

**Upstream status:** N/A — Konflux-specific subtask, not present in upstream Apache DevLake. Also depends on `collectAndExtractSingleIssue` in `parent_issue_collector.go` (Konflux-only).
**Upstream PR:** none — not applicable
**Owner:** @rsoaresd

**Rebase notes:** `stale_board_issue_cleaner.go` specific use case; no upstream equivalent.
`impl.go` registers `CleanupStaleBoardIssuesMeta` in `SubTaskMetas()` after
`ExtractEpicsMeta` — same conflict hotspot as `CollectParentIssuesMeta`.
Cleanup reuses `collectAndExtractSingleIssue` from `parent_issue_collector.go`
(also Konflux-only).

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

**Reason:** Both files previously diverged from upstream (missing zero-ID guard on
`AuthorId`/`MergedById`; missing `merged_by` → `_tool_github_repo_accounts`
emission). Both are now byte-identical to upstream's current code (verified via
diff against `apache/incubator-devlake`, both fixed by
[apache/devlake#8894](https://github.com/apache/devlake/pull/8894)). No entry
needed going forward — listed here only for traceability since the fix landed
alongside the `IsBot` work above.

**Upstream status:** N/A — matches upstream exactly, zero divergence.
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
