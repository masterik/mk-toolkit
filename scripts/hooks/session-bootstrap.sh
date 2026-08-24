#!/usr/bin/env bash
#
# The SessionStart hook: make mkit's user-scoped setup happen by itself.
#
# Registered in hooks/hooks.json, so it needs no opt-in beyond installing the plugin.
# It writes the two files `install.sh` writes — `~/.claude/mkit/journal.default` (which
# turns the commit journal from "off unless enabled" into "on unless disabled" in every
# repo) and `<bin>/mkit-journal` — and then goes quiet forever.
#
# Why a hook and not just the script: `install.sh` is a command you have to know about.
# Every session that starts without it having been run is a session where `commit`
# re-derives intent from the diff for want of one empty marker file. The whole point of
# the journal is that it costs nothing at the moment the intent is still known, and a
# setup step nobody runs costs exactly the feature.
#
# What it is NOT: a diagnostic surface. `install.sh --status` is that, and it exists so
# this hook never has to be. Anything this hook cannot fix, it says once and drops.
#
# Three properties, in the order they matter:
#
#   1. **It always exits 0 and never blocks a session.** A SessionStart hook cannot
#      block one anyway, but nothing here may even look like it is trying to.
#   2. **Silent once the state is correct.** Every session after the first produces zero
#      bytes on stdout and stderr, at a cost of ~4 stats and one grep. Not `{}` — nothing
#      at all. Every message it can ever emit is one-time, keyed in bootstrap.state.
#   3. **It respects a deliberate removal.** `bootstrap.disabled` beats everything,
#      including the prerequisite report. See "the tombstone" below.
#
# It is user-scoped only: it never touches a repo, never calls git, and deliberately does
# not `cd` to the event's cwd. Whether you started the session inside a repo is none of
# its business — the marker it writes applies to every repo you will ever open, which is
# precisely why it can be written from anywhere.
#
# **It has no external prerequisite of its own** — not even jq. That is not incidental
# elegance: this is the hook whose job includes reporting that jq is missing, and a
# reporter that needs the thing it reports on is unavailable in exactly the case that
# matters. Hence mkit_json_escape (awk) rather than jq for the payload.
#
# --- the tombstone, and why one is necessary ------------------------------------------
#
# "Idempotently re-assert the desired state" and "respect a deliberate removal" cannot
# both be decided from the same bytes. A deleted file carries no provenance: "never
# created" and "created and then deliberately deleted" are byte-identical on disk. So a
# second file recording the *intent* is not one design option among several — it is the
# only mechanism. Everything else (a version stamp, a run-once ledger, firing only on
# source == startup) is a heuristic that fails the first time the user's disk state and
# their intent disagree.
#
# `install.sh --uninstall` writes `bootstrap.disabled`; this hook checks it first and
# exits silently forever while it is there. That is exactly the shape journal.sh already
# uses one scope in: its `journal.disabled` tombstone exists *only* to outvote a
# user-scoped default, because without it a `disable` in a repo covered by
# journal.default would be a no-op that reported success. Substitute user/hook for
# repo/default and it is the same sentence. Precedence never resolves toward writing, at
# either level.
#
# Residue we accept: someone who deletes `journal.default` by hand rather than through
# `--uninstall` gets it back next session. The one-time notice names the supported
# opt-outs, so that is a path they were told about.
#
# --- what it will not do --------------------------------------------------------------
#
#   - create a bin directory. A directory mkit invented would not be on PATH anyway and
#     would outlive the uninstall.
#   - touch a `mkit-journal` it did not generate — a symlink, a directory, or anyone
#     else's file of that name. It says so once and leaves it.
#   - refuse to write the marker when a prerequisite is missing. install.sh refuses
#     because a human is watching a script that would otherwise appear to succeed and
#     then not work; this hook has no watcher, the marker is inert without jq (see
#     journal-nudge.sh's own first gate), and refusing means re-deciding every session
#     forever instead of closing the delta once. What it *does* withhold is the notice:
#     "journaling is now on" is false while jq is missing.
#   - warn about anything twice. bootstrap.state is the ledger of what has been said.
#
# Exit: always 0.

set -euo pipefail

# Same reconciliation of `set -e` with "must never fail" as journal-nudge.sh: the trap
# normalizes every status to 0; errexit stays on so a half-computed state stops rather
# than continuing; every gate is an explicit `|| exit 0` so expected failures stay
# visible instead of being swept up by the trap; and the payload is buffered and printed
# by the last statement, so an abort part-way cannot leak truncated JSON.
trap 'exit 0' EXIT

# shellcheck source=../lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

# --- gate 1: drain stdin, and deliberately do not parse it -----------------------------
#
# The divergence from journal-nudge.sh is intentional; do not "fix" it. That hook parses
# because its *fields* drive behaviour — the budget key comes from prompt_id, the repo
# from cwd. This hook's action is user-scoped, event-independent and idempotent: it would
# do exactly the same thing fired by any event, from any directory, so there is nothing
# in the payload to read. Parsing would mean requiring jq in the one hook that must work
# without it.
#
# A bash builtin rather than `cat`: zero forks on the session-start path. The read is
# EPIPE politeness toward the writer, and the timeout is the backstop for a runtime that
# leaves stdin open rather than closing it.
read -r -d '' -t 2 _ <&0 2>/dev/null || true

# --- gate 2: a resolvable, absolute user dir -------------------------------------------
#
# Explicit, rather than letting `set -u` trip inside a command substitution later. A
# relative MKIT_HOME has to die here too: a hook process has no defined working
# directory, so a relative path would resolve somewhere unpredictable.
user_dir="$(mkit_user_dir 2>/dev/null)" || exit 0
case "$user_dir" in
/*) ;;
*) exit 0 ;;
esac

marker="$user_dir/journal.default"
tombstone="$user_dir/bootstrap.disabled"
state="$user_dir/bootstrap.state"

# --- gate 3: the global opt-out --------------------------------------------------------
#
# One stat, and it must beat everything below it — including the prerequisite report. An
# opt-out that still tells you about jq every session is not an opt-out.
[ ! -f "$tombstone" ] || exit 0

# --- gate 4: the desired-state delta ---------------------------------------------------
# Stats only, no forks.
plugin_root="$(mkit_plugin_root)"
journal_sh="$plugin_root/scripts/journal.sh"

marker_missing=no
[ -f "$marker" ] || marker_missing=yes

# na | none | ours-current | ours-stale | foreign
bin_dir="$(mkit_bin_dir 2>/dev/null || true)"
wrapper=""
wrapper_state=na
if [ -n "$bin_dir" ]; then
	wrapper="$bin_dir/mkit-journal"
	if [ -L "$wrapper" ]; then
		# -L before -f, always: `[ -f ]` follows symlinks, so without this a deliberate
		# symlink would read as a regular file and get replaced by one.
		wrapper_state=foreign
	elif [ ! -e "$wrapper" ]; then
		wrapper_state=none
	elif ! mkit_wrapper_is_ours "$wrapper"; then
		wrapper_state=foreign
	elif mkit_wrapper_is_current "$wrapper" "$plugin_root"; then
		wrapper_state=ours-current
	else
		wrapper_state=ours-stale
	fi
fi

# --- gate 5: prerequisites -------------------------------------------------------------
# `command -v` is a builtin, so four lookups is sub-millisecond and forks nothing. It has
# to run on the silent path: you cannot know a tool is present without looking.
prereq_rows="$(mkit_prereq_rows --missing-only)" && hard_missing=no || hard_missing=yes

# --- gate 6: is there anything to do, or anything left to say? -------------------------
pending_write=no
case "$marker_missing:$wrapper_state" in
yes:*) pending_write=yes ;;
*:none | *:ours-stale) pending_write=yes ;;
esac

# The notice predicate is deliberately "the marker exists AND the notice is unspent AND
# no hard prerequisite is missing" — *not* "we wrote something just now". That covers a
# crash between the write and the stamp for free, and it delivers the notice the session
# after someone installs jq rather than never.
notice_due=no
if [ "$hard_missing" = no ] &&
	{ [ "$marker_missing" = no ] || [ "$pending_write" = yes ]; } &&
	! mkit_state_has "$state" 'notice/v1'; then
	notice_due=yes
fi

# Which one-time warnings are unspent. Keys, not sentences, so the wording can change
# without re-nagging everyone.
warn_keys=""
warn_lines=""
if [ -n "$prereq_rows" ]; then
	while IFS="$(printf '\t')" read -r tool st text; do
		[ -n "$tool" ] || continue
		mkit_state_has "$state" "prereq/$tool" && continue
		warn_keys="$warn_keys prereq/$tool"
		warn_lines="$warn_lines
  $tool $st — $text"
	done <<EOF
$prereq_rows
EOF
fi

case "$wrapper_state" in
foreign)
	if ! mkit_state_has "$state" 'wrapper/foreign'; then
		warn_keys="$warn_keys wrapper/foreign"
		warn_lines="$warn_lines
  $wrapper already exists and mkit did not create it — left untouched."
	fi
	;;
esac
if [ "$pending_write" = yes ] && [ -n "$bin_dir" ] && [ ! -w "$bin_dir" ] &&
	! mkit_state_has "$state" 'wrapper/unwritable'; then
	warn_keys="$warn_keys wrapper/unwritable"
	warn_lines="$warn_lines
  $bin_dir is not writable, so mkit-journal was not installed."
fi

# --- gate 7: nothing to do and nothing to say -----------------------------------------
# The common path on every session after the first. Zero bytes, not an empty object.
if [ "$pending_write" = no ] && [ "$notice_due" = no ] && [ -z "$warn_keys" ]; then
	# Still heal the state file: a tool that came back should stop being remembered as
	# missing, even in a session with nothing else to do.
	for key in $(mkit_state_missing_keys "$state" "$prereq_rows"); do
		mkit_state_drop "$state" "$key" || true
	done
	exit 0
fi

# --- gate 8: the writes ---------------------------------------------------------------
# Each independently guarded. None may abort the script — a failed write means one
# message goes unsent, never a failed session.
wrote_marker=no
wrote_wrapper=no

if [ "$pending_write" = yes ]; then
	if mkdir -p "$user_dir" 2>/dev/null; then
		if [ "$marker_missing" = yes ]; then
			# Create-only, never re-truncate: no mtime churn, and a future version that
			# gives the marker content would not have it clobbered here.
			if { : >"$marker"; } 2>/dev/null; then
				wrote_marker=yes
				marker_missing=no
			fi
		fi
	fi

	case "$wrapper_state" in
	none | ours-stale)
		if [ -n "$bin_dir" ] && [ -w "$bin_dir" ] && [ -x "$journal_sh" ]; then
			# Never bake a wrapper pointing at a journal.sh that is not there.
			if mkit_write_wrapper "$wrapper" "$plugin_root"; then
				# A stale rewrite is silent on purpose. Someone who alternates between a
				# dev clone and a marketplace cache would otherwise be told twice a day
				# about a wrapper that works fine either way.
				[ "$wrapper_state" = none ] && wrote_wrapper=yes
				wrapper_state=ours-current
			fi
		fi
		;;
	esac
fi

# The notice is only true if the marker is actually in place now.
[ "$marker_missing" = no ] || notice_due=no

# --- gate 9: spend the stamps, before emitting ----------------------------------------
#
# journal-nudge.sh's rule, and the asymmetry is the reason: a stamp written for a message
# that never arrived costs one cosmetic line, while a message that arrives without its
# stamp is the repeating nag this whole file exists to prevent.
[ "$notice_due" = no ] || mkit_state_add "$state" 'notice/v1' || true
for key in $warn_keys; do
	mkit_state_add "$state" "$key" || true
done

# --- gate 9b: self-heal ----------------------------------------------------------------
# A tool that is present again loses its key, so a later removal warns again — a new gap
# deserves a new warning.
for key in $(mkit_state_missing_keys "$state" "$prereq_rows"); do
	mkit_state_drop "$state" "$key" || true
done

# --- gate 10: one payload, at most --------------------------------------------------
#
# systemMessage reaches the *user*; additionalContext reaches *Claude*. They are siblings
# in one object, so both audiences fit in a single emission — and which audiences a
# message needs is a real distinction, not symmetry for its own sake:
#
#   the setup notice  -> user only. It announces files created in their home directory.
#                        The agent gets its own nudge at Stop and does not need this;
#                        context injected here is unpaid tokens.
#   a prereq gap      -> both. Only the user can `brew install jq`; only the model will
#                        hit the confusing failures, and it can offer to run it.
user_msg=""
agent_msg=""

if [ "$notice_due" = yes ]; then
	user_msg="mkit: the commit journal is now on by default in every repo."
	# Name the files as *state*, not as actions taken just now. The notice can be
	# deferred by a session or more (a hard prerequisite withholds it), by which point
	# these were written in an earlier session and "created" would be false.
	user_msg="$user_msg
  marker   $marker"
	# Name the executable whenever one of ours is on disk, whether this run put it there
	# or not. Having placed an executable in a directory on someone's PATH and never
	# mentioned it is the thing that turns "helpful" into "creepy".
	if [ "$wrapper_state" = ours-current ] || [ "$wrote_wrapper" = yes ]; then
		user_msg="$user_msg
  wrapper  $wrapper"
	fi
	user_msg="$user_msg

It records why each unit of work exists, so commit does not have to infer intent
from the diff later. Opt out any time:
  this repo:  $journal_sh disable
  everywhere: $plugin_root/install.sh --uninstall
(those paths are this version of the plugin; install.sh --status explains the rest)"
fi

if [ -n "$warn_lines" ]; then
	[ -z "$user_msg" ] || user_msg="$user_msg

"
	user_msg="$user_msg""mkit: setup is incomplete.$warn_lines"
	agent_msg="mkit reports an incomplete setup:$warn_lines
Tell the user rather than working around it; a missing tool is theirs to install."
fi

[ -n "$user_msg" ] || [ -n "$agent_msg" ] || exit 0

out='{'
if [ -n "$agent_msg" ]; then
	# additionalContext MUST be nested under hookSpecificOutput beside hookEventName.
	# The flat top-level form is accepted and then silently ignored — no warning in
	# stdout, in stream-json or in the transcript; the diagnostic shows up only under
	# `claude --debug hooks`. journal-nudge.sh learned this the hard way and its bats
	# suite guards the shape; so does this one.
	out="$out\"hookSpecificOutput\":{\"hookEventName\":\"SessionStart\","
	out="$out\"additionalContext\":\"$(printf '%s' "$agent_msg" | mkit_json_escape)\"},"
fi
if [ -n "$user_msg" ]; then
	out="$out\"systemMessage\":\"$(printf '%s' "$user_msg" | mkit_json_escape)\","
fi
out="$out\"suppressOutput\":true}"

printf '%s\n' "$out"
