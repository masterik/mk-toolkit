#!/usr/bin/env bash
#
# mkit diagnostics: report whether this machine has what the plugin's scripts need, and
# provide the one opt-out the SessionStart hook honours.
#
#   usage: install.sh [--status] [--uninstall [--purge]]
#
# **There is nothing to install.** No marker, no wrapper, no per-repo state: every
# feature the plugin ships is on as soon as the plugin is, and the only thing that can
# stop one working is a missing tool — which this script reports and refuses to install
# on your behalf. The name is kept because it is the path `--uninstall` is documented at.
#
# Two jobs, both of which a hook cannot do:
#
#   --status        report what is present and what is missing, `ok` rows included. The
#                   SessionStart hook deliberately reports only gaps, once each; a human
#                   who ran a diagnostic on purpose wants the whole picture.
#   --uninstall     silence that hook for good. It writes a tombstone the hook honours;
#                   without one, a dismissal would last exactly until the next session.
#
# With no arguments it does what `--status` does, since there is no other mode left.
#
# What it deliberately does NOT do:
#
#   - install, or offer to install, a missing tool. `brew install jq` is one command and
#     it is the user's to run; a plugin that installs things into someone's PATH on their
#     behalf is a plugin they cannot cleanly remove.
#   - enable anything. The gate ledger is always on — gate-run.sh writes gate.jsonl in
#     every repo already and `--no-ledger` is the opt-*out*. A script that claims to have
#     switched something on is worse than one that says it was on all along.
#   - edit your shell rc. A tool that appends to a profile is a tool you cannot cleanly
#     uninstall.
#   - touch any repo. Per-repo state written by an installer is state nobody remembers
#     agreeing to.
#
# `--uninstall --purge` drops the tombstone too, for someone who wants mkit's bytes gone
# and accepts that the next session may warn them again.
#
# Exit: 0 ok, 1 a *hard* prerequisite (git, jq) is missing or a write failed, 2 bad usage.
# A soft gap (node, sha256) is reported and still exits 0 — it degrades a feature, it does
# not make the machine unready.

set -euo pipefail

die() {
	printf 'install.sh: %s\n' "$1" >&2
	exit "${2:-1}"
}

mode=status
purge=no

while [ $# -gt 0 ]; do
	case "$1" in
	--status) mode=status ;;
	--uninstall) mode=uninstall ;;
	--purge) purge=yes ;;
	-h | --help)
		# Print the header block by *shape* — every comment line after the shebang, stopping
		# at the first line that is not one. A hard-coded range silently drops the last line
		# the moment the header grows or shrinks, and the exit contract lives on that line.
		awk 'NR == 1 { next } /^#/ { sub(/^#[[:space:]]?/, ""); print; next } { exit }' "$0"
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
[ -d "$plugin_root/scripts" ] ||
	die "no scripts/ beside this file — is $plugin_root really the plugin root?"

# Sourced, not reimplemented: the prerequisite table and the one-time state helpers are
# shared with the SessionStart hook, and the degradation sentences are only one source of
# truth if neither caller writes its own. `die` stays local: it prefixes `install.sh:`
# where mkit_die prefixes `mkit:`.
# shellcheck source=scripts/lib/common.sh
. "$plugin_root/scripts/lib/common.sh"

user_dir="$(mkit_user_dir)"
tombstone="$user_dir/bootstrap.disabled"
state_file="$user_dir/bootstrap.state"

# --- prerequisites -------------------------------------------------------------------
# Reported per feature, not as one pass/fail: telling someone "mkit is broken" when one
# optimization is unavailable is the misdiagnosis `gate_cache=no-hash` exists to avoid.
#
# The rows come from `mkit_prereq_rows` in common.sh so the hook reports the same
# sentences. This script prints every row, including the `ok`s.
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

# --- the SessionStart hook's own state -------------------------------------------------
report_hook() {
	printf 'session hook:\n'
	if [ -f "$tombstone" ]; then
		printf '  silenced (%s)\n' "$tombstone"
		printf '  it reports nothing while that file exists; remove it to undo\n'
	else
		printf '  active — reports a missing tool once, then stays silent\n'
		printf '  silence it: %s --uninstall\n' "$0"
	fi
}

case "$mode" in
uninstall)
	removed=0
	if [ -f "$state_file" ]; then
		rm -f "$state_file" && removed=1
		printf 'removed %s\n' "$state_file"
	fi

	if [ "$purge" = yes ]; then
		if [ -f "$tombstone" ]; then
			rm -f "$tombstone" && removed=1
			printf 'removed %s\n' "$tombstone"
		fi
		# rmdir, not rm -r: the directory may hold state a later version put there, and
		# an uninstaller that deletes a directory it did not create is how people lose
		# data. With the tombstone gone this can now actually succeed.
		rmdir "$user_dir" 2>/dev/null || true
		[ "$removed" -eq 1 ] || printf 'nothing to remove\n'
		printf '\nPurged — no tombstone left behind. The SessionStart hook will therefore\n'
		printf 'warn again about anything still missing. Drop --purge to make it stick.\n'
	else
		# The tombstone is the entire reason a dismissal outlives the session: the hook
		# re-checks every time it runs, and cannot tell "never warned" from "warned and
		# dismissed" by looking at absent files. This records the intent.
		mkdir -p "$user_dir" 2>/dev/null || die "cannot create $user_dir"
		{
			printf 'mkit bootstrap disabled by install.sh --uninstall.\n'
			printf 'While this file exists, the SessionStart hook stays silent.\n'
			printf 'Undo: remove this file. Remove it now: install.sh --uninstall --purge.\n'
		} >"$tombstone" || die "cannot write $tombstone"
		[ "$removed" -eq 1 ] || printf 'nothing to remove\n'
		printf 'silenced: %s\n' "$tombstone"
		printf '\nThe SessionStart hook will now stay quiet instead of reporting a missing\n'
		printf 'tool next session. Remove that file to undo.\n'
	fi
	exit 0
	;;
esac

# --- status ----------------------------------------------------------------------------
# The prerequisite status is the exit status: a caller scripting this wants to branch on
# "is this machine ready", and that is exactly what a MISSING row means.
prereq_status=0
report_prereqs || prereq_status=$?
printf '\n'
report_hook
printf '\n'
report_ledger
exit "$prereq_status"
