#!/usr/bin/env bash
set -euo pipefail

before="${1:-}"
after="${2:-HEAD}"
default_branch="${3:-main}"

if [[ -z "$before" || "$before" =~ ^0+$ ]]; then
	git fetch --no-tags origin "+refs/heads/$default_branch:refs/remotes/origin/$default_branch"
	commit_range="origin/$default_branch..$after"
else
	commit_range="$before..$after"
fi

commit_list="$(git rev-list --reverse --no-merges "$commit_range")"
if [[ -z "$commit_list" ]]; then
	printf 'No new non-merge commits to validate.\n'
	exit 0
fi
mapfile -t commits <<< "$commit_list"
failed=0
for commit in "${commits[@]}"; do
	subject="$(git show -s --format=%s "$commit")"
	if bash scripts/validate-commit-subject.sh "$subject"; then
		printf 'Valid: %s %s\n' "${commit:0:7}" "$subject"
	else
		printf 'Commit %s has an invalid subject.\n' "${commit:0:7}" >&2
		failed=1
	fi
done

exit "$failed"
