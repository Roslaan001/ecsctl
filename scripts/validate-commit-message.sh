#!/usr/bin/env bash
set -euo pipefail

message="${1:-}"
subject="${message%%$'\n'*}"

bash scripts/validate-commit-subject.sh "$subject"

# Match Prow's invalidcommitmsg rule: closing keywords followed by an issue
# reference would let GitHub close an issue when this commit is merged.
closing_issue_pattern='(clos(e[sd]?)|fix(es|ed)?|resolv(e[sd]?))[[:space:]:]+([[:alnum:]_]+/[[:alnum:]_]+)?#[0-9]+'
shopt -s nocasematch
if [[ "$message" =~ $closing_issue_pattern ]]; then
	printf 'Commit messages must not use GitHub issue-closing keywords with an issue reference.\n' >&2
	printf 'Move issue-closing references to the pull request description.\n' >&2
	exit 1
fi
