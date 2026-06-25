# Bot Commit & PR Identification in DevLake

Research findings from production database analysis (June 2026).

## Problem Statement

Production dashboards cannot reliably differentiate bot commits/PRs from real users.
The n8n workflows use pattern-based heuristics (`author_name LIKE '%[bot]%'`, empty
`author_name`, commit message patterns) which are fragile and inconsistent across
commits vs PRs. The problem is compounded by needing to work across multiple
providers (GitHub and GitLab), each with different bot conventions.

There are two distinct concerns:

1. **Historical data cleanup** — phantom PRs with empty `author_name` caused by
   a removed zero-ID guard, addressed via manual backfill (June 2026)
2. **Going-forward bot identification** — adding a cross-provider `IsBot` field
   to the domain model so downstream queries don't need provider-specific heuristics

## Key Findings

### Commits: Bot detection works but patterns differ by provider

| Category | Count | Detection Method |
|----------|-------|-----------------|
| Bot-pattern authors (`[bot]` suffix, etc.) | 365,088 | `author_name LIKE '%[bot]%'` |
| Empty `author_name` | 53 | All are MkDocs CI deployments (`ci-bot@example.com`) |
| Total commits | 772,379 | — |

The `[bot]` suffix in `author_name` is a reliable signal for GitHub commits. The 53
empty-author commits are MkDocs CI deployment commits (empty author_name and
author_email) — unrelated to the GitHub/GitLab bot identity problem.

### Pull Requests: Phantom author problem (GitHub)

Current state (283,242 total PRs: 275,217 GitHub, 8,025 GitLab):

| Category | Count | Root Cause |
|----------|-------|-----------|
| Empty `author_name` | 135,066 | Phantom `:0` author_id from removed zero-ID guard |
| Bot-pattern `author_name` | 30,755 | `dependabot[bot]`, `renovate[bot]`, etc. (GraphQL-collected) |
| Normal `author_name` | 117,421 | Human authors with proper identity |
| Tool-layer `author_id=0` | 116,033 | GitHub REST API returns `author: null` for bot PRs |

All 135K empty-author PRs are GitHub-only and have phantom `github:GithubAccount:X:0`
domain IDs (perfect 1:1 correlation). These are caused by the fork's removal of the
upstream zero-ID guard in `pr_convertor.go`. When the GitHub REST API returns
`author: null` for a PR, `AuthorId=0`, and the fork generates phantom IDs that
match no `accounts` row.

See "Historical Data Cleanup" section below for backfill details.

### Tool-Layer Account Types

**GitHub** (`_tool_github_accounts`):
```
Type          Count
User          23,850
Bot              37
Organization     11
```

The `Type` field is barely populated — REST API `/users/<login>` returns 404 for most
bots, so no `GithubAccount` row is created. Only GraphQL collection (PR #84's
`... on Bot` fragment) correctly populates bot type.

**GitLab** (`_tool_gitlab_accounts`): 237,367 total accounts, no `Type` field at all.

### GitLab Bot Patterns (from prod data)

GitLab has distinct bot naming conventions, none of which overlap with GitHub's:

| Pattern | Count | Example |
|---------|-------|---------|
| `project_*_bot*` (project access tokens) | 25,169 | `project_78877_bot_ec63972b...` |
| `group_*_bot*` (group access tokens) | 5,955 | `group_80889_bot_73ed0037...` |
| Name contains "Service Account" | 2,852 | `rh-renovate-bot` / `renovate service account` |
| Username ends in `-bot` | 1,291 | `pnc-stage-bot`, `snyk-broker-bot` |

GitLab bots in commits also differ from GitHub: `Release Service Bot`, `ci-robot`,
`OpenShift Cherrypick Robot` — no `[bot]` suffix convention.

## Historical Data Cleanup (completed June 2026)

### Root cause: the zero-ID guard bug

Upstream apache/devlake has this guard in `pr_convertor.go`:

```go
if pr.AuthorId != 0 {
    domainPr.AuthorId = accountIdGen.Generate(data.Options.ConnectionId, pr.AuthorId)
}
```

Our fork removed it, unconditionally generating `AuthorId`. When `pr.AuthorId == 0`
(null author from GitHub API), this produces `github:GithubAccount:X:0` — a phantom
ID that matches no `accounts` row, resulting in empty `author_name` after JOIN.

### Two distinct data paths (GitHub)

1. **GraphQL collection (PR #84):** Correctly identifies bots with proper login
   (`dependabot[bot]`), ID, and Type. These PRs have valid `author_id` and `author_name`.
   30,755 PRs currently have proper bot names via this path.
2. **REST API collection:** Returns `author: null` for bot PRs. With the removed
   guard, these get phantom IDs and empty names.

### Manual backfill (partially effective)

A manual backfill was performed using `backfill-pr-authors.py` which generated
`backfill-authors-update.sql` — a 4.3 MB script that resolved `author_name` for
15,107 PRs by looking up the actual author login from the GitHub API. This covered
both human users and bots (e.g. `rh-tap-build-team[bot]`, `seanconroy2021`).

However, subsequent data collection cycles appear to have regenerated the phantom
IDs, overwriting the backfill. As of June 2026, 135,066 PRs still have empty
`author_name` with phantom `:0` author_id. The backfill approach is not durable
without also restoring the zero-ID guard to prevent re-generation.

### Going forward

The root fix requires two parts:

1. **Restore the zero-ID guard** (Step 1 in Implementation Plan) — prevents new
   phantom IDs from being generated during collection
2. **Re-run backfill or re-collect** — fixes the 135K existing phantom PRs

New data collected via GraphQL (PR #84) correctly identifies bot PRs with proper
names and IDs. The remaining challenge beyond data cleanup is the separate concern
of **bot identification** — how to reliably distinguish bot accounts from human
accounts across providers without per-query pattern heuristics.

## Cross-Provider Bot Identification (going forward)

### Cross-provider pattern fragility

Pattern-based detection requires maintaining provider-specific heuristics that
differ fundamentally between GitHub and GitLab:

| Signal | GitHub | GitLab |
|--------|--------|--------|
| Login/username pattern | `*[bot]` | `project_*_bot*`, `group_*_bot*`, `*-bot` |
| API-level type field | `Type` (User/Bot) — poorly populated | None |
| Service account marker | N/A | Name contains "Service Account" |
| Null author | `author_id=0` (REST API) | N/A |

These patterns will keep evolving as providers add new bot/service account concepts.
Encoding them in every n8n workflow query is brittle and error-prone.

## Recommendation: Add `is_bot` to Domain `accounts` Model

### Alternatives considered

**Pattern-based detection in queries** — Each downstream query (n8n workflow, Grafana
dashboard) applies provider-specific heuristics (`author_name LIKE '%[bot]%'` for
GitHub, `user_name LIKE 'project_%_bot%'` for GitLab, etc.). This works for a single
provider but breaks down across GitHub and GitLab because their bot conventions differ
fundamentally. Every consumer must duplicate all patterns, and new bot conventions from
either provider require updating every query.

**Centralized SQL view** — A `v_account_bot_status` view would centralize the pattern
logic in one place. This avoids duplication across queries but still requires maintaining
provider-specific heuristics as SQL. It doesn't solve the fundamental problem — the
domain layer has no concept of bot identity — and adds DDL management overhead without
improving the data model.

### Why `IsBot` on the domain model

With `is_bot` on the `accounts` model, each plugin's convertor applies provider-specific
logic once during data conversion, and all downstream consumers check a single boolean
field. This is the right layer for this concern — bot identity is a property of the
account, not a query-time derivation.

### Upstream divergence reduction

An additional benefit: `IsBot` reduces divergence from `apache/devlake`. The fork
currently diverges on 8 files (~320 changed lines) for bot account handling.
After implementation, 2 files revert to match upstream exactly, 1 file shrinks
significantly, and the domain model gains one new field:

| File | Current divergence | After IsBot implementation |
|------|-------------------|--------------------------|
| `account_convertor.go` | Reversed join, custom struct, rewritten query, orphan pass (135 lines) | Revert join to upstream direction, keep only additive orphan pass (~40 lines) with `IsBot=true` |
| `pr_convertor.go` | Removed zero-ID guard (13 lines) | **Reverted to upstream** (0 lines) |
| `pr_extractor.go` | Removed MergedBy repo_account emission¹ (10 lines) | **Reverted to upstream** (0 lines) |
| 4 GraphQL files | `... on Bot` fragment support (~160 lines) | Unchanged — candidate for upstream contribution |
| `account.go` (domain model) | Matches upstream | +1 field: `IsBot bool` (~1 line) |

¹ The fork removed the repo_account emission for MergedBy users (the `results = append(results, mergedByUser)` block in the Extract function). The MergedBy *field extraction* in `convertGithubPullRequest()` (lines 209-212) is unchanged from upstream.

**Net result:** 8 files / ~320 lines → 6 files / ~200 lines. `pr_convertor.go` fully reverts; `pr_extractor.go` fully reverts.
Rebase risk drops significantly — the largest conflict source (`account_convertor.go`
reversed query) is removed.

For the full target-state divergence, see `docs/upstream-diffs.md`.

### Implementation Plan

**Step 1: Restore the zero-ID guard** (fix the bug, prerequisite)

In `backend/plugins/github/tasks/pr_convertor.go`, restore upstream's guard:

```go
if pr.AuthorId != 0 {
    domainPr.AuthorId = accountIdGen.Generate(data.Options.ConnectionId, pr.AuthorId)
}
if pr.MergedById != 0 {
    domainPr.MergedById = accountIdGen.Generate(data.Options.ConnectionId, pr.MergedById)
}
```

This prevents phantom `github:GithubAccount:X:0` IDs from being generated for
any future REST API collection. Without this guard, any backfill of historical
data will be overwritten by the next collection cycle (see "Historical Data
Cleanup" above). After restoring the guard, re-run the backfill or trigger
a full re-collection to fix the 135K existing phantom PRs.

**Step 2: Add `IsBot` field to domain `accounts` model**

In `backend/core/models/domainlayer/crossdomain/account.go`:

```go
type Account struct {
    domainlayer.DomainEntity
    Email        string     `gorm:"type:varchar(255)"`
    FullName     string     `gorm:"type:varchar(255)"`
    UserName     string     `gorm:"type:varchar(255)"`
    AvatarUrl    string     `gorm:"type:varchar(255)"`
    Organization string     `gorm:"type:varchar(255)"`
    CreatedDate  *time.Time
    Status       int
    IsBot        bool       `gorm:"type:boolean;default:false"`
}
```

Add a migration script in `backend/core/models/migrationscripts/`.

**Step 3: Populate `IsBot` in GitHub account convertor**

In `backend/plugins/github/tasks/account_convertor.go`, set `IsBot` based on:
- `_tool_github_accounts.Type == "Bot"`
- Login matches `*[bot]`

In `convertOrphanedRepoAccounts()`, set `IsBot = true` (orphans are almost always bots).

**Step 4: Populate `IsBot` in GitLab account convertor**

In `backend/plugins/gitlab/tasks/account_convertor.go`, set `IsBot` based on:
- Username matches `project_*_bot*` or `group_*_bot*`
- Username ends in `-bot`
- Name contains "Service Account" (case-insensitive)

**Step 5: Backfill `IsBot` for existing accounts**

```sql
-- GitHub bots
UPDATE accounts SET is_bot = true
WHERE user_name LIKE '%[bot]';

-- GitLab project/group access tokens
UPDATE accounts SET is_bot = true
WHERE user_name LIKE 'project\_%\_bot%' ESCAPE '\'
   OR user_name LIKE 'group\_%\_bot%' ESCAPE '\';

-- GitLab named bots and service accounts
UPDATE accounts SET is_bot = true
WHERE user_name LIKE '%-bot'
   OR full_name LIKE '%Service Account%';
```

Note: the phantom `author_id LIKE '%:0'` cleanup should be re-run after
restoring the zero-ID guard (Step 1), since the earlier manual backfill was
overwritten by subsequent collection cycles. The cleanup SQL:

```sql
UPDATE pull_requests SET author_id = '', author_name = '' WHERE author_id LIKE '%:0';
```

Alternatively, trigger a full re-collection which will correctly skip
`AuthorId=0` with the guard restored.

**Limitation:** After restoring the zero-ID guard, the 135K phantom PRs will have
*empty* `author_id` — no `accounts` row to join on, so `IsBot` filtering cannot
help them. These PRs require either re-collection via GraphQL (which correctly
identifies bot authors with proper IDs) or a targeted backfill that resolves
author identity from the GitHub API.

**Step 6: Simplify n8n workflow queries**

Replace all provider-specific bot patterns with:

```sql
-- Exclude bots from metrics
WHERE a.is_bot = false

-- Or for tables without account join
WHERE author_name NOT IN (
    SELECT user_name FROM accounts WHERE is_bot = true
)
```

## Appendix A: N8n Workflow Bot Detection Patterns (Current)

Patterns found in `../n8n-workflows/` that would be replaced by `is_bot`:

| Workflow | Pattern | Purpose |
|----------|---------|---------|
| `8Oi1vXcAdLbL2owl.json` | `author_name LIKE '%[bot]%'` | Exclude bots from contributor metrics |
| `ajIm9fOvAjEUbUuj.json` | `author_email = 'noreply@github.com'` | Bot email detection |
| `4Ffddz0D5jELpsIf.json` | `message LIKE 'Merge pull request #%'` | Merge commit detection |

## Appendix B: Verification Queries

Queries used to produce the datapoints in this document. Run against the prod
database (`lake`). Data verified 2026-06-24.

```sql
-- Commits: bot-pattern authors
SELECT COUNT(*) FROM commits WHERE author_name LIKE '%[bot]%';
-- Result: 365,088

-- Commits: empty author_name (all are MkDocs CI deployment commits with
-- both author_name and author_email empty, message "Deployed <sha> with MkDocs version: 1.6.1")
SELECT COUNT(*) FROM commits WHERE author_name = '' OR author_name IS NULL;
-- Result: 53

-- Commits: total
SELECT COUNT(*) FROM commits;
-- Result: 772,379

-- PRs: total and breakdown
SELECT COUNT(*) FROM pull_requests;
-- Result: 283,242

SELECT
  CASE
    WHEN author_name = '' OR author_name IS NULL THEN 'empty'
    WHEN author_name LIKE '%[bot]%' THEN 'bot_pattern'
    ELSE 'normal'
  END AS category,
  COUNT(*) AS cnt
FROM pull_requests GROUP BY category;
-- Result: empty=135,066  bot_pattern=30,755  normal=117,421

-- PRs: by provider
SELECT
  CASE
    WHEN id LIKE 'github:%' THEN 'github'
    WHEN id LIKE 'gitlab:%' THEN 'gitlab'
    ELSE 'other'
  END AS provider,
  COUNT(*) AS cnt
FROM pull_requests GROUP BY provider;
-- Result: github=275,217  gitlab=8,025

-- PRs: phantom author_id correlation
SELECT COUNT(*) FROM pull_requests WHERE author_id LIKE '%:0';
-- Result: 135,066 (all empty author_name PRs have phantom IDs — perfect correlation)

SELECT COUNT(*) FROM pull_requests
WHERE (author_name = '' OR author_name IS NULL) AND author_id NOT LIKE '%:0';
-- Result: 0 (no empty names without phantom IDs)

-- PRs: empty author_name by provider (all GitHub)
SELECT COUNT(*) FROM pull_requests
WHERE (author_name = '' OR author_name IS NULL) AND id LIKE 'github:%';
-- Result: 135,066

-- Tool-layer: author_id=0
SELECT COUNT(*) FROM _tool_github_pull_requests WHERE author_id = 0;
-- Result: 116,033

-- GitHub accounts by type
SELECT COALESCE(type,'(null)') AS type, COUNT(*)
FROM _tool_github_accounts GROUP BY type;
-- Result: User=23,850  Bot=37  Organization=11

-- GitLab accounts total
SELECT COUNT(*) FROM _tool_gitlab_accounts;
-- Result: 237,367

-- GitLab bot patterns
SELECT COUNT(*) FROM _tool_gitlab_accounts WHERE username LIKE 'project\_%\_bot%';
-- Result: 25,169

SELECT COUNT(*) FROM _tool_gitlab_accounts WHERE username LIKE 'group\_%\_bot%';
-- Result: 5,955

SELECT COUNT(*) FROM _tool_gitlab_accounts WHERE name LIKE '%Service Account%';
-- Result: 2,852

SELECT COUNT(*) FROM _tool_gitlab_accounts WHERE username LIKE '%-bot';
-- Result: 1,291
```
