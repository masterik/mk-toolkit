#!/usr/bin/env bash
#
# Open a fresh mkit run directory and print its absolute path.
#
#   usage: run-open.sh <skill>
#   e.g.:  run-open.sh review  ->  /repo/.git/mkit/review-20260819T111347Z-RPfCbj
#
# Why this is a script and not three lines in a SKILL.md: the invariants below are
# mechanical, they run at the start of every skill, and getting any of them wrong fails
# silently or destructively.
#
#   - mktemp -d, not mkdir -p     the timestamp is second-resolution, so two runs in one
#                                 checkout can pick the same name; mkdir -p would merge them
#                                 and let each clobber the other's logs and findings
#   - --absolute-git-dir          the path is handed to subagents and reused across shells,
#                                 where a relative .git/... would resolve somewhere else
#   - inside the git dir          never committed, never shows up in git status, and a
#                                 linked worktree gets its own
#
# Prints the path on stdout and nothing else, so it is safe in a command substitution.
# Errors go to stderr. Exit: 0 ok, 1 not a git repo, 2 bad usage.

set -euo pipefail

skill="${1:-}"

if [ -z "$skill" ]; then
	printf 'run-open.sh: usage: run-open.sh <skill>\n' >&2
	exit 2
fi

# The name becomes a path component; keep it boring so it cannot traverse or glob.
case "$skill" in
*[!a-zA-Z0-9_-]*)
	printf 'run-open.sh: skill name may only contain [a-zA-Z0-9_-], got: %s\n' "$skill" >&2
	exit 2
	;;
esac

if ! git_dir="$(git rev-parse --absolute-git-dir 2>/dev/null)"; then
	printf 'run-open.sh: not inside a git repository\n' >&2
	exit 1
fi

mkit_dir="$git_dir/mkit"
mkdir -p "$mkit_dir"

run_dir="$(mktemp -d "$mkit_dir/$skill-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")"
printf '%s\n' "$run_dir"
