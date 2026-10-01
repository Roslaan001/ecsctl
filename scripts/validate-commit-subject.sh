#!/usr/bin/env bash
set -euo pipefail

subject="${1:-}"
pattern='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([[:alnum:]][[:alnum:]./_-]*\))?(!)?: .+'

if [[ ! "$subject" =~ $pattern ]]; then
	printf 'Invalid commit subject: %s\n' "$subject" >&2
	printf 'Use Conventional Commits, for example: fix(state): prevent stale lock removal\n' >&2
	exit 1
fi

if (( ${#subject} > 72 )); then
	printf 'Commit subject is %d characters; keep it to 72 or fewer.\n' "${#subject}" >&2
	exit 1
fi
