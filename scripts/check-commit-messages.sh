#!/usr/bin/env bash
set -euo pipefail

before="${1:-}"
after="${2:-HEAD}"
default_branch="${3:-main}"

git fetch --no-tags origin "+refs/heads/$default_branch:refs/remotes/origin/$default_branch"

if [[ -z "$before" || "$before" =~ ^0+$ ]]; then
	commit_range="origin/$default_branch..$after"
elif git merge-base --is-ancestor "$before" "$after"; then
	commit_range="$before..$after"
else
	# After a force-push, the previous tip may not be in the new history.
	# Validate the new branch commits against the default branch instead.
	printf 'Previous branch tip is not an ancestor; validating against %s.\n' "$default_branch"
	commit_range="origin/$default_branch..$after"
fi

# A branch push can include a merge from the default branch. Don't revalidate
# commits already present there; only check commits unique to this branch.
commit_list="$(git rev-list --reverse --no-merges "$commit_range" --not "origin/$default_branch")"
if [[ -z "$commit_list" ]]; then
	printf 'No new non-merge commits to validate.\n'
	exit 0
fi
mapfile -t commits <<< "$commit_list"
failed=0
for commit in "${commits[@]}"; do
	subject="$(git show -s --format=%s "$commit")"
	message="$(git show -s --format=%B "$commit")"
	if bash scripts/validate-commit-message.sh "$message"; then
		printf 'Valid: %s %s\n' "${commit:0:7}" "$subject"
	else
		printf 'Commit %s has an invalid message.\n' "${commit:0:7}" >&2
		failed=1
	fi
done

exit "$failed"
