#
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

#!/bin/bash
# Opens (or updates) a single PR that syncs this fork to an Apache DevLake tag.
# Never merges. If GitHub reports conflicts, a human resolves them.

set -euo pipefail

TAG_REGEX='^v[0-9]+\.[0-9]+\.[0-9]+(-beta[0-9]+)?$'
UPSTREAM_REMOTE_URL="${UPSTREAM_REMOTE_URL:-https://github.com/apache/devlake.git}"
UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-apache}"
SYNC_BRANCH="${SYNC_BRANCH:-chore/upstream-sync}"
BASE_BRANCH="${BASE_BRANCH:-main}"
LABEL="${LABEL:-upstream-sync}"
REPO="${REPO:-${GITHUB_REPOSITORY:-konflux-ci/devlake}}"

die() {
	echo "error: $*" >&2
	exit 1
}

ensure_upstream_remote() {
	if git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1; then
		git remote set-url "$UPSTREAM_REMOTE" "$UPSTREAM_REMOTE_URL"
	else
		git remote add "$UPSTREAM_REMOTE" "$UPSTREAM_REMOTE_URL"
	fi
}

list_matching_tags() {
	git ls-remote --tags "$UPSTREAM_REMOTE" \
		| awk '{print $2}' \
		| sed 's#^refs/tags/##' \
		| grep -v '\^{}' \
		| { grep -E "$TAG_REGEX" || true; } \
		| sort -V
}

newest_matching_tag() {
	list_matching_tags | tail -1
}

tag_commit() {
	git rev-parse "$1^{commit}"
}

is_ancestor_of_base() {
	git merge-base --is-ancestor "$1" "origin/${BASE_BRANCH}"
}

# True when SHA is the commit an Apache tag points at (bot-pushed PR head).
is_upstream_tag_commit() {
	local sha="$1"
	git ls-remote --tags "$UPSTREAM_REMOTE" | awk '{print $1}' | grep -Fxq "$sha"
}

predict_conflict_paths() {
	local sha="$1"
	local out
	out=$(git merge-tree --write-tree --name-only "origin/${BASE_BRANCH}" "$sha") || true
	printf '%s\n' "$out" | awk '
		NR == 1 && /^[0-9a-f]{40,}$/ { next }
		/^$/ { exit }
		/^Auto-merging / { exit }
		/^CONFLICT / { exit }
		NF { print }
	'
}

ensure_label() {
	gh label create "$LABEL" \
		--repo "$REPO" \
		--description "Automated Apache DevLake upstream sync" \
		--color "1D76DB" \
		--force >/dev/null
}

render_pr_body() {
	local tag="$1"
	local sha="$2"
	local conflicts="$3"
	local conflict_section

	if [ -z "$conflicts" ]; then
		conflict_section="GitHub may still report conflicts; predicted none via \`git merge-tree\`."
	else
		conflict_section="Predicted conflict paths (from \`git merge-tree\`; not a substitute for the GitHub conflict UI):"
		conflict_section+=$'\n\n'
		while IFS= read -r path; do
			[ -z "$path" ] && continue
			conflict_section+="- \`${path}\`"$'\n'
		done <<<"$conflicts"
	fi

	cat <<EOF
## Summary

Sync this fork to Apache DevLake **${tag}** (\`${sha}\`).

- Release: https://github.com/apache/devlake/releases/tag/${tag}
- This PR is opened automatically. **Do not auto-merge. Do not squash.**
- Land with a **merge commit** so upstream history stays intact for the next sync.

The PR head is the upstream tag (not a pre-resolved merge). GitHub will block the merge button when this fork and upstream both changed the same files. Owned plugins and other fork-only paths are kept; they are not in the upstream tag.

## Resolve conflicts

1. \`gh pr checkout\` (or fetch \`${SYNC_BRANCH}\`).
2. \`git merge origin/${BASE_BRANCH}\`.
3. Resolve using [docs/upstream-diffs.md](https://github.com/${REPO}/blob/${BASE_BRANCH}/docs/upstream-diffs.md).
4. Push to \`${SYNC_BRANCH}\`. CI then runs on the combined tree.

Until step 4, CI (if it runs) tests **upstream's tree only**, not this fork.

## Predicted conflicts

${conflict_section}

## After merge

The next scheduled run skips this tag once it is an ancestor of \`${BASE_BRANCH}\`.
After merging (merge commit, not squash), delete \`${SYNC_BRANCH}\` if GitHub did not. The next scheduled run recreates it.
EOF
}

already_commented_newer_tag() {
	local number="$1"
	local tag="$2"
	gh api "repos/${REPO}/issues/${number}/comments" --paginate \
		--jq '.[].body' \
		| grep -Fq "<!-- upstream-sync-newer-tag:${tag} -->"
}

comment_newer_tag() {
	local number="$1"
	local tag="$2"
	if already_commented_newer_tag "$number" "$tag"; then
		echo "Already commented on PR #${number} about ${tag}; leaving in-progress work alone."
		return
	fi
	gh pr comment "$number" --repo "$REPO" --body "$(cat <<EOF
<!-- upstream-sync-newer-tag:${tag} -->
A newer upstream tag \`${tag}\` is available, but this PR has commits that are not an Apache tag (likely conflict resolution in progress).

Not force-pushing, to avoid wiping that work. Merge or close this PR so the next run can sync \`${tag}\`.
EOF
)"
}

[ -n "${GH_TOKEN:-}" ] || die "GH_TOKEN is required"
command -v gh >/dev/null || die "gh is required"

ensure_upstream_remote
git fetch origin "${BASE_BRANCH}"
git fetch "$UPSTREAM_REMOTE" "${BASE_BRANCH}" || true

tag="${INPUT_TAG:-}"
if [ -z "$tag" ]; then
	tag=$(newest_matching_tag)
	[ -n "$tag" ] || die "no upstream tags matching ${TAG_REGEX}"
	echo "Newest matching upstream tag: ${tag}"
else
	echo "${tag}" | grep -Eq "$TAG_REGEX" || die "tag ${tag} does not match ${TAG_REGEX}"
	echo "Using requested tag: ${tag}"
fi

git fetch "$UPSTREAM_REMOTE" "refs/tags/${tag}:refs/tags/${tag}" --force
sha=$(tag_commit "$tag")
echo "Tag ${tag} -> ${sha}"

if is_ancestor_of_base "$sha"; then
	echo "${tag} is already an ancestor of origin/${BASE_BRANCH}; nothing to do."
	exit 0
fi

pr_number=""
pr_head=""
pr_json=$(gh pr list --repo "$REPO" --head "$SYNC_BRANCH" --base "$BASE_BRANCH" --state open --json number,headRefOid)
if [ "$pr_json" != "[]" ]; then
	pr_number=$(printf '%s' "$pr_json" | jq -r '.[0].number')
	pr_head=$(printf '%s' "$pr_json" | jq -r '.[0].headRefOid')
	echo "Open PR #${pr_number} head=${pr_head}"
fi

if [ -n "$pr_number" ] && [ "$pr_head" = "$sha" ]; then
	echo "PR #${pr_number} already points at ${tag}; nothing to do."
	exit 0
fi

if [ -n "$pr_number" ] && [ -n "$pr_head" ] && ! is_upstream_tag_commit "$pr_head"; then
	echo "PR #${pr_number} has human (non-tag) commits; not force-pushing."
	comment_newer_tag "$pr_number" "$tag"
	exit 0
fi

conflicts=$(predict_conflict_paths "$sha")
body=$(render_pr_body "$tag" "$sha" "$conflicts")
title="chore: sync upstream Apache DevLake ${tag}"

git push --force origin "${sha}:refs/heads/${SYNC_BRANCH}"
ensure_label

if [ -n "$pr_number" ]; then
	gh pr edit "$pr_number" --repo "$REPO" --title "$title" --body "$body"
	echo "Updated PR #${pr_number} to ${tag}."
else
	gh pr create --repo "$REPO" \
		--base "$BASE_BRANCH" \
		--head "$SYNC_BRANCH" \
		--title "$title" \
		--body "$body" \
		--label "$LABEL"
	echo "Opened PR for ${tag}."
fi
