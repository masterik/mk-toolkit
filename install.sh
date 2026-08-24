#!/usr/bin/env bash
#
# mkit user-scoped setup: make the commit journal the default in every repo, and report
# whether the gate ledger has what it needs to work.
#
#   usage: install.sh [--status] [--uninstall [--purge]] [--bin <dir>] [--no-bin] [--force]
#
# **You do not normally need to run this.** `scripts/hooks/session-bootstrap.sh` — the
# SessionStart hook — writes both files below on its own, idempotently, and re-points the
# wrapper when the plugin moves. This script stays for the three jobs a hook cannot do:
#
#   --status        report what is set up and which prerequisites are missing. The hook
#                   deliberately is not a diagnostic surface, so this is the one.
#   --uninstall     the only way to opt out. It writes a tombstone the hook honours;
#                   without one, an uninstall would last exactly until the next session.
#   --bin <dir>     a non-default wrapper location the hook will never guess.
#
# (Claude Code has no plugin-*installation* hook, and could not usefully have one:
# installing a plugin is user-scoped and happens once, while the journal's opt-in is per
# *git dir* — install time does not know which repos exist. SessionStart is a different
# thing entirely: it fires per session, so the hook simply re-asserts a user-scoped
# desired state every time and goes quiet once it holds.)
#
# What it actually installs, and why so little:
#
#   ~/.claude/mkit/journal.default   an empty marker file. journal.sh consults it only
#                                    when the repo itself has said nothing, so it turns
#                                    "off unless enabled" into "on unless disabled" for
#                                    this user, without touching a single repo.
#   <bin>/mkit-journal               a two-line wrapper resolving the installed plugin,
#                                    so `mkit-journal status` works from any repo.
#
# What it deliberately does NOT do:
#
#   - the gate ledger. There is nothing to enable: gate-run.sh writes gate.jsonl in
#     every repo already and `--no-ledger` is the opt-*out*. All it needs is jq and a
#     sha256 tool, so this script checks for them and reports — enabling an always-on
#     feature would be theatre, and a script that claims to have switched something on
#     is worse than one that says it was on all along.
#   - edit your shell rc. A tool that appends to a profile is a tool you cannot cleanly
#     uninstall. It prints the PATH line if <bin> is not on PATH; you decide.
#   - touch any repo. No walking ~/src enabling markers: the user-scoped default already
#     covers every repo, including ones cloned tomorrow, and per-repo state written by
#     an installer is state nobody remembers agreeing to.
#
# Reversible in full: `--uninstall` removes both files and writes
# ~/.claude/mkit/bootstrap.disabled, which is what makes the removal outlive the session.
# "Re-assert the desired state idempotently" and "respect a deliberate removal" cannot
# both be read off the same bytes — a deleted file has no provenance, so "never created"
# and "created then deleted" are identical on disk. A second file recording the *intent*
# is not one option among several; it is the only mechanism. `--uninstall --purge` drops
# the tombstone too, for someone who wants mkit's bytes gone and accepts that the next
# session sets it up again.
#
# Per-repo `journal.sh disable` still wins over the default, which is the whole reason
# journal.sh grew a tombstone — this one is that same idea a scope out.
#
# Exit: 0 ok, 1 a prerequisite is missing or a write failed, 2 bad usage.

set -euo pipefail

die() {
	printf 'install.sh: %s\n' "$1" >&2
	exit "${2:-1}"
}

mode=install
bin_dir=""
no_bin=no
force=no
purge=no

while [ $# -gt 0 ]; do
	case "$1" in
	--status) mode=status ;;
	--uninstall) mode=uninstall ;;
	--no-bin) no_bin=yes ;;
	--force) force=yes ;;
	--purge) purge=yes ;;
	--bin)
		shift
		[ $# -gt 0 ] || die '--bin needs a directory' 2
		bin_dir="$1"
		;;
	-h | --help)
		sed -n '2,40p' "$0" | sed 's/^#\{1,\} \{0,1\}//'
		exit 0
		;;
	*) die "unexpected argument: $1" 2 ;;
	esac
	shift
done

# This file sits at the plugin root, beside scripts/ — the same derive-from-my-own-
# location rule the scripts use, so a clone, a marketplace cache and a symlinked
# checkout all work without configuration.
plugin_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -x "$plugin_root/scripts/journal.sh" ] ||
	die "no scripts/journal.sh beside this file — is $plugin_root really the plugin root?"

# Sourced, not reimplemented. The prerequisite table, the bin-dir rule, the wrapper's
# body and the one-time state helpers are all shared with the SessionStart hook — and
# the wrapper especially must have exactly one producer, or the hook's "is this file
# mine?" test starts failing to recognize this script's output. `die` stays local: it
# prefixes `install.sh:` where mkit_die prefixes `mkit:`.
# shellcheck source=scripts/lib/common.sh
. "$plugin_root/scripts/lib/common.sh"

user_dir="$(mkit_user_dir)"
default_marker="$user_dir/journal.default"
tombstone="$user_dir/bootstrap.disabled"
state_file="$user_dir/bootstrap.state"

[ -n "$bin_dir" ] || bin_dir="$(mkit_bin_dir || true)"
wrapper="${bin_dir:+$bin_dir/mkit-journal}"

# --- prerequisites -------------------------------------------------------------------
# Reported per feature, not as one pass/fail: jq missing takes the journal down entirely
# but only costs the gate ledger its cache, and telling someone "mkit is broken" when
# one optimization is unavailable is the misdiagnosis `gate_cache=no-hash` exists to
# avoid.
#
# The rows come from `mkit_prereq_rows` in common.sh so the hook reports the same
# sentences. This script prints every row, including the `ok`s — a human watching an
# installer wants the whole picture, where the hook shows only what is wrong.
report_prereqs() {
	local rows status=0 tool state text

	rows="$(mkit_prereq_rows)" || status=$?

	printf 'prerequisites:\n'
	while IFS="$(printf '\t')" read -r tool state text; do
		[ -n "$tool" ] || continue
		if [ -n "$text" ]; then
			printf '  %-8s %s — %s\n' "$tool" "$state" "$text"
		else
			printf '  %-8s %s\n' "$tool" "$state"
		fi
	done <<EOF
$rows
EOF

	return "$status"
}

# --- the gate ledger: report, never enable --------------------------------------------
report_ledger() {
	printf 'gate ledger:\n'
	printf '  always on — gate-run.sh writes <git-dir>/mkit/gate.jsonl in every repo.\n'
	printf '  nothing to install; --no-ledger / --no-cache are the opt-outs.\n'
	if mkit_have jq && mkit_have_hash; then
		printf '  status: usable (jq + sha256 present)\n'
	else
		printf '  status: degraded — records still written, cache annotation unavailable\n'
	fi
}

report_journal() {
	printf 'commit journal:\n'
	if [ -f "$default_marker" ]; then
		printf '  user default: ON  (%s)\n' "$default_marker"
		printf '  every repo journals unless it runs journal.sh disable\n'
	elif [ -f "$tombstone" ]; then
		printf '  user default: off, and pinned off (%s)\n' "$tombstone"
		printf '  the SessionStart hook honours the tombstone; re-run this script to undo\n'
	else
		printf '  user default: off (no %s)\n' "$default_marker"
		printf '  the SessionStart hook will turn it on next session\n'
	fi
	# Only meaningful inside a repo, and being outside one is not an error here.
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		printf '  this repo:    %s\n' "$("$plugin_root/scripts/journal.sh" enabled --why)"
	fi
}

report_wrapper() {
	printf 'wrapper:\n'
	if [ -n "$wrapper" ] && [ -f "$wrapper" ]; then
		printf '  installed: %s\n' "$wrapper"
	elif [ -z "$bin_dir" ]; then
		printf '  none — no ~/.local/bin or ~/bin found; pass --bin <dir> to install one\n'
	else
		printf '  not installed (%s)\n' "$wrapper"
	fi
}

case "$mode" in
status)
	report_prereqs || true
	printf '\n'
	report_journal
	printf '\n'
	report_wrapper
	printf '\n'
	report_ledger
	exit 0
	;;
uninstall)
	removed=0
	if [ -f "$default_marker" ]; then
		rm -f "$default_marker" && removed=1
		printf 'removed %s\n' "$default_marker"
	fi
	# A wrapper we did not generate is somebody else's file that happens to share the
	# name. Removing it would be the same overreach the hook refuses.
	if [ -n "$wrapper" ] && [ ! -L "$wrapper" ] && mkit_wrapper_is_ours "$wrapper"; then
		rm -f "$wrapper" && removed=1
		printf 'removed %s\n' "$wrapper"
	elif [ -n "$wrapper" ] && [ -e "$wrapper" ]; then
		printf 'left alone (not mkit-generated): %s\n' "$wrapper"
	fi
	if [ -f "$state_file" ]; then
		rm -f "$state_file" && removed=1
	fi

	if [ "$purge" = yes ]; then
		rm -f "$tombstone" 2>/dev/null || true
		# rmdir, not rm -r: the directory may hold state a later version put there, and
		# an uninstaller that deletes a directory it did not create is how people lose
		# data. With the tombstone gone this can now actually succeed.
		rmdir "$user_dir" 2>/dev/null || true
		[ "$removed" -eq 1 ] || printf 'nothing to remove\n'
		printf '\nPurged — no tombstone left behind. The SessionStart hook will therefore\n'
		printf 'set this up again on your next session. Drop --purge to make it stick.\n'
	else
		# The tombstone is the entire reason an uninstall outlives the session: the hook
		# re-asserts the desired state every time it runs, and cannot tell "never set up"
		# from "deliberately removed" by looking at absent files. This records the intent.
		mkdir -p "$user_dir" 2>/dev/null || die "cannot create $user_dir"
		{
			printf 'mkit bootstrap disabled by install.sh --uninstall.\n'
			printf 'While this file exists, the SessionStart hook writes nothing and stays silent.\n'
			printf 'Undo: re-run install.sh. Remove this file too: install.sh --uninstall --purge.\n'
		} >"$tombstone" || die "cannot write $tombstone"
		[ "$removed" -eq 1 ] || printf 'nothing to remove\n'
		printf 'pinned off: %s\n' "$tombstone"
		printf '\nThe SessionStart hook will now stay silent instead of setting this up\n'
		printf 'again next session. Re-run install.sh to undo.\n'
	fi

	printf '\nPer-repo markers are untouched: journal.sh disable tombstones and\n'
	printf 'journal.enabled markers stay where they are, inside each git dir.\n'
	exit 0
	;;
esac

# --- install ---------------------------------------------------------------------------
if ! report_prereqs && [ "$force" = no ]; then
	die 'refusing to install with a hard prerequisite missing (--force overrides)' 1
fi

printf '\n'

mkdir -p "$user_dir" || die "cannot create $user_dir"

# Clear the tombstone first, the same way `journal.sh enable` clears the repo one: a
# re-run must be able to undo an `--uninstall`, and a directory left holding both files
# reads as off.
if [ -f "$tombstone" ]; then
	rm -f "$tombstone" || die "cannot remove $tombstone"
	printf 'un-pinned: removed %s\n' "$tombstone"
fi

if [ -f "$default_marker" ] && [ "$force" = no ]; then
	printf 'user default already on: %s\n' "$default_marker"
else
	: >"$default_marker" || die "cannot write $default_marker"
	printf 'user default ON: %s\n' "$default_marker"
fi

# Spend the hook's one-time notice here. This script is about to say the same thing out
# loud, in more detail; without this the hook would repeat it on the next session.
mkit_state_add "$state_file" 'notice/v1' || true

if [ "$no_bin" = yes ]; then
	printf 'wrapper skipped (--no-bin)\n'
elif [ -z "$bin_dir" ]; then
	printf 'wrapper skipped: no ~/.local/bin or ~/bin — pass --bin <dir> to install one\n'
elif [ ! -w "$bin_dir" ]; then
	printf 'wrapper skipped: %s is not writable\n' "$bin_dir"
elif [ -L "$wrapper" ] || { [ -e "$wrapper" ] && ! mkit_wrapper_is_ours "$wrapper"; }; then
	# A symlink, a directory, or a regular file we did not generate. Never clobber it —
	# `[ -f ]` follows symlinks, so the check has to test -L first or a deliberate
	# symlink gets silently replaced by a regular file.
	printf 'wrapper skipped: %s exists and is not mkit-generated\n' "$wrapper"
else
	# The plugin root is baked in at write time rather than resolved at run time. The
	# alternative — parsing ~/.claude/plugins/installed_plugins.json on every call —
	# needs jq for a path lookup and breaks the moment the marketplace name changes.
	# Staleness is no longer the user's problem to fix by hand: the SessionStart hook
	# runs from the *new* plugin root after an upgrade and re-points this itself.
	mkit_write_wrapper "$wrapper" "$plugin_root" || die "cannot write $wrapper"
	printf 'wrapper: %s -> scripts/journal.sh\n' "$wrapper"
	case ":$PATH:" in
	*":$bin_dir:"*) ;;
	*) printf '  note: %s is not on PATH. Add it yourself:\n    export PATH="%s:$PATH"\n' "$bin_dir" "$bin_dir" ;;
	esac
fi

printf '\n'
report_ledger

printf '\nDone. Journaling is now the default for every repo you open.\n'
printf 'Per repo:   journal.sh disable   (opt out here, beats the default)\n'
printf 'Globally:   %s --uninstall\n' "$0"
printf 'Check:      %s --status\n' "$0"
printf '\nThe SessionStart hook keeps this in place, so you should not need to run\n'
printf 'this script again.\n'
