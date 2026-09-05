#!/usr/bin/env bash
#
# One-time cleanup of the state the commit journal left behind.
#
#   usage: tools/purge-journal-state.sh [--apply] [--bin <dir>]
#
# Journaling was removed in full, but a machine that ran mkit <= 0.12.1 still holds the
# files that version created. Nothing in the plugin removes them any more, because nothing
# in the plugin knows about them: `install.sh --uninstall` only manages state the *current*
# version writes, and teaching it about deleted machinery would mean shipping — and
# maintaining — a wrapper-ownership test for a file no version writes.
#
# So this is a script you run once and delete, not a feature. It lives in `tools/`, outside
# `plugin/`, and is never part of the payload.
#
# What it removes:
#
#   ~/.claude/mkit/journal.default   the user-scoped marker that turned journaling on in
#                                    every repo. Inert now — nothing reads it.
#   <bin>/mkit-journal               the wrapper. NOT inert: its body is
#                                    `exec "<old plugin root>/scripts/journal.sh"`, which
#                                    keeps working while that version-pinned checkout
#                                    survives and becomes a dangling executable on your
#                                    PATH once Homebrew or the marketplace cache prunes it.
#   ~/.claude/mkit/bootstrap.state   the `notice/v1` line only. That key announced the
#                                    journal setup; the current hook never writes it and
#                                    its self-heal only ever drops `prereq/` keys, so it
#                                    would sit there forever. Every other line is live
#                                    state and is left alone.
#   <git-dir>/mkit/journal*          journal.jsonl, the enable/disable markers, and the
#                                    deleted Stop hook's journal-nudge.state, for the repo you
#                                    run this in ONLY. See "one repo" below. journal.lock/ is
#                                    reported and left alone — it's a directory.
#
# Safety, in the order it matters:
#
#   - **Dry run by default.** It prints what it would do and changes nothing until you
#     pass `--apply`. Same shape as `mkit storage prune`.
#   - **It never removes a wrapper it did not write.** A symlink, a directory, or a regular
#     file lacking mkit's generation marker is reported and left exactly where it is —
#     `mkit-journal` is a plausible enough name that someone else's script could own it.
#     `-L` is tested before `-f`, because `[ -f ]` follows a symlink and would otherwise
#     let a deliberate link read as a regular file.
#   - **It never deletes a directory** — only the named files. An uninstaller that removes
#     a directory it did not create is how people lose data.
#   - **One repo, never a scan.** Per-repo journal state lives in `<git-dir>/mkit/`, is
#     never committed, and dies with the clone. Walking your disk looking for git dirs to
#     write to is not something a cleanup script gets to do uninvited, so it handles the
#     repo you run it in and names the rest as your call.
#
# Exit: 0 whatever happened (including nothing to do), 1 a removal failed, 2 bad usage.

set -euo pipefail

apply=no
bin_override=""

while [ $# -gt 0 ]; do
	case "$1" in
	--apply) apply=yes ;;
	--bin)
		shift
		[ $# -gt 0 ] || {
			printf 'purge-journal-state.sh: --bin needs a directory\n' >&2
			exit 2
		}
		bin_override="$1"
		;;
	-h | --help)
		awk 'NR == 1 { next } /^#/ { sub(/^#[[:space:]]?/, ""); print; next } { exit }' "$0"
		exit 0
		;;
	*)
		printf 'purge-journal-state.sh: unexpected argument: %s\n' "$1" >&2
		exit 2
		;;
	esac
	shift
done

# The marker install.sh and the SessionStart hook both wrote into every wrapper they
# generated. Copied here rather than sourced: lib/common.sh no longer defines it, and this
# script must keep working after the plugin it is cleaning up after has moved on.
WRAPPER_MARK='mkit-generated wrapper'

user_dir="${MKIT_HOME:-$HOME/.claude/mkit}"
found=0
removed=0
failed=0

say() { printf '%s\n' "$1"; }

# Report one item, and remove it only under --apply.
#   drop <path> <description>
drop() {
	local path="$1" what="$2"
	found=$((found + 1))
	if [ "$apply" = no ]; then
		printf '  would remove  %s\n                %s\n' "$path" "$what"
		return 0
	fi
	if rm -f -- "$path" 2>/dev/null; then
		printf '  removed       %s\n' "$path"
		removed=$((removed + 1))
	else
		printf '  FAILED        %s\n' "$path" >&2
		failed=$((failed + 1))
	fi
}

keep() {
	printf '  left alone    %s\n                %s\n' "$1" "$2"
}

say 'mkit — one-time removal of leftover commit-journal state'
say ''
[ "$apply" = yes ] || say 'DRY RUN — nothing will be changed. Re-run with --apply to do it.'
[ "$apply" = yes ] || say ''

# --- user-scoped: the marker -----------------------------------------------------------
say "user state ($user_dir):"
marker="$user_dir/journal.default"
if [ -f "$marker" ]; then
	drop "$marker" 'the marker that turned journaling on in every repo'
else
	say '  nothing     no journal.default'
fi

# --- user-scoped: the stale one-time-notice key -----------------------------------------
#
# A rewrite, not a delete: bootstrap.state is live state for the current hook. Only the
# `notice/v1` line goes, and only if it is actually there.
state="$user_dir/bootstrap.state"
if [ -f "$state" ] && grep -qxF 'notice/v1' "$state" 2>/dev/null; then
	found=$((found + 1))
	if [ "$apply" = no ]; then
		printf '  would edit    %s\n                drop the stale `notice/v1` line, keep the rest\n' "$state"
	else
		tmp=""
		if tmp="$(mktemp "$state.XXXXXX" 2>/dev/null)"; then
			grep_rc=0
			grep -vxF 'notice/v1' "$state" >"$tmp" 2>/dev/null || grep_rc=$?
			# grep exits 1 when every line matched — bootstrap.state held only
			# notice/v1, so the empty file just written is the correct result, not
			# a failure. Only a status above 1 is a real read/write error.
			if [ "$grep_rc" -le 1 ] && mv -f -- "$tmp" "$state" 2>/dev/null; then
				printf '  edited        %s (dropped notice/v1)\n' "$state"
				removed=$((removed + 1))
			else
				rm -f -- "$tmp" 2>/dev/null
				printf '  FAILED        %s\n' "$state" >&2
				failed=$((failed + 1))
			fi
		else
			printf '  FAILED        %s\n' "$state" >&2
			failed=$((failed + 1))
		fi
	fi
fi

# --- the wrapper -------------------------------------------------------------------------
say ''
say 'wrapper:'
bin_dirs=()
add_bin_dir() {
	local d="$1" seen
	[ -n "$d" ] || return 0
	for seen in "${bin_dirs[@]+"${bin_dirs[@]}"}"; do
		[ "$seen" = "$d" ] && return 0
	done
	bin_dirs+=("$d")
}
if [ -n "$bin_override" ]; then
	add_bin_dir "$bin_override"
else
	# Both defaults the old bin-dir rule could have chosen, plus MKIT_BIN if it is set —
	# the wrapper may have been written before a directory was created or removed, so
	# check every candidate rather than only the first that exists today.
	for d in "${MKIT_BIN:-}" "$HOME/.local/bin" "$HOME/bin"; do
		add_bin_dir "$d"
	done
fi

wrapper_seen=0
for d in "${bin_dirs[@]+"${bin_dirs[@]}"}"; do
	w="$d/mkit-journal"
	if [ -L "$w" ]; then
		# -L first: [ -f ] follows the link.
		wrapper_seen=1
		keep "$w" 'a symlink — mkit never created one, so it is not ours to remove'
	elif [ -d "$w" ]; then
		wrapper_seen=1
		keep "$w" 'a directory — left untouched'
	elif [ -e "$w" ]; then
		wrapper_seen=1
		if head -n 5 -- "$w" 2>/dev/null | grep -qF -- "$WRAPPER_MARK" 2>/dev/null; then
			drop "$w" 'the mkit-generated journal wrapper (execs a journal.sh that is gone)'
		else
			keep "$w" 'exists but carries no mkit generation marker — someone else owns it'
		fi
	fi
done
[ "$wrapper_seen" -eq 1 ] || say '  nothing     no mkit-journal in any candidate bin directory'

# --- this repo ---------------------------------------------------------------------------
say ''
if git_dir="$(git rev-parse --absolute-git-dir 2>/dev/null)"; then
	say "this repo ($git_dir/mkit):"
	repo_seen=0
	for f in journal.jsonl journal.enabled journal.disabled journal-nudge.state; do
		p="$git_dir/mkit/$f"
		if [ -e "$p" ]; then
			repo_seen=1
			drop "$p" "per-repo journal state ($f)"
		fi
	done
	if [ -d "$git_dir/mkit/journal.lock" ]; then
		repo_seen=1
		keep "$git_dir/mkit/journal.lock" 'a directory — left untouched'
	fi
	[ "$repo_seen" -eq 1 ] || say '  nothing     no journal state in this repo'
else
	say 'this repo:'
	say '  skipped     not inside a git repository'
fi

# --- what is left ------------------------------------------------------------------------
say ''
if [ "$found" -eq 0 ]; then
	say 'Nothing to clean up — this machine has no leftover journal state.'
	exit 0
fi

if [ "$apply" = no ]; then
	printf '%d item(s) to clean up. Re-run with --apply to do it:\n' "$found"
	if [ -n "$bin_override" ]; then
		printf '  %s --apply --bin %q\n' "$0" "$bin_override"
	else
		printf '  %s --apply\n' "$0"
	fi
	exit 0
fi

printf '%d removed' "$removed"
[ "$failed" -eq 0 ] || printf ', %d FAILED' "$failed"
printf '.\n'
say ''
say 'Other clones keep their own <git-dir>/mkit/journal.jsonl until you delete them or the'
say 'clone. It is never committed and nothing reads it, so it costs only disk.'
say 'This script has done its job and can be deleted.'

[ "$failed" -eq 0 ] || exit 1
exit 0
