#!/usr/bin/env bash
#
# Open a fresh mkit run directory and print its absolute path.
#
#   usage: run-open.sh <skill>            open a run directory, print its path
#          run-open.sh --prune [keep]     remove all but the newest <keep> run
#                                         directories per skill (default 5)
#
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
#
# Pruning belongs here for the same reason: it must never remove a directory another run
# is still reading, so it keeps the newest few per skill and is only ever run at the end.

set -euo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

prune() {
	local keep="${1:-5}" mkit_dir skill d n kept=0 removed=0 live=0
	case "$keep" in *[!0-9]* | '') mkit_die "--prune takes a count, got: $keep" 2 ;; esac
	# `0` is all digits, so it passed validation and then evicted every run directory —
	# including the live one belonging to the caller doing the pruning.
	[ "$keep" -ge 1 ] || mkit_die '--prune must keep at least 1 run directory' 2
	mkit_dir="$(mkit_dir_or_die)"
	if [ ! -d "$mkit_dir" ]; then
		printf 'pruned 0 (no run directories)\n'
		return 0
	fi

	# One pass per skill, so a busy `review` never evicts the only `pr` run.
	for skill in commit review finish pr cleanup; do
		n=0
		# Newest first: the name's UTC timestamp sorts lexicographically.
		while IFS= read -r d; do
			[ -n "$d" ] || continue
			n=$((n + 1))
			if [ "$n" -le "$keep" ]; then
				kept=$((kept + 1))
			elif [ -n "$(find "$d" -maxdepth 0 -mmin -60 2>/dev/null)" ]; then
				# Age-ranked eviction alone cannot see a run still being written: a long
				# review holding an older directory would be rm -rf'd underneath itself.
				live=$((live + 1))
			else
				rm -rf -- "$d"
				removed=$((removed + 1))
			fi
		done <<-EOF
			$(find "$mkit_dir" -maxdepth 1 -type d -name "$skill-*" 2>/dev/null | sort -r)
		EOF
	done
	printf 'pruned %d run dir(s), kept %d' "$removed" "$kept"
	[ "$live" -gt 0 ] && printf ', skipped %d still active (<60m)' "$live"
	printf '\n'
}

case "${1:-}" in
'') mkit_die 'usage: run-open.sh <skill> | run-open.sh --prune [keep]' 2 ;;
--prune)
	prune "${2:-5}"
	exit 0
	;;
esac

skill="$1"
mkit_check_slug "$skill"

mkit_dir="$(mkit_dir_or_die)"
mkdir -p "$mkit_dir"

run_dir="$(mktemp -d "$mkit_dir/$skill-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")"
printf '%s\n' "$run_dir"
