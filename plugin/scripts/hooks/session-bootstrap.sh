#!/usr/bin/env bash
#
# The SessionStart hook: say once, per missing tool, what mkit cannot do without it.
#
# Registered in hooks/hooks.json, so it needs no opt-in beyond installing the plugin.
#
# Why a hook and not just `install.sh --status`: a diagnostic you have to know about is a
# diagnostic nobody runs. A missing `jq` does not announce itself — it surfaces as
# `gate_cache=no-jq` on a gate report, or a `facts.sh` block that quietly came back
# thinner than it should have. The gap is cheap to detect and expensive to debug, so the
# session that starts with it says so, once, and then never again.
#
# What it is NOT: a setup step. It writes no marker, installs nothing, and creates no
# executable — there is nothing user-scoped left to set up. `install.sh --status` is the
# full diagnostic surface; this hook is only its one-time, unprompted subset.
#
# Three properties, in the order they matter:
#
#   1. **It always exits 0 and never blocks a session.** A SessionStart hook cannot
#      block one anyway, but nothing here may even look like it is trying to.
#   2. **Silent once everything it can report has been said.** Every session after the
#      first produces zero bytes on stdout and stderr, at a cost of ~4 `command -v`
#      lookups and one grep. Not `{}` — nothing at all.
#   3. **It respects a deliberate silencing.** `bootstrap.disabled` beats everything,
#      including the prerequisite report. See "the tombstone" below.
#
# It is user-scoped only: it never touches a repo, never calls git, and deliberately does
# not `cd` to the event's cwd. Whether you started the session inside a repo is none of
# its business — a tool missing from PATH is missing for every repo you will open.
#
# **It has no external prerequisite of its own** — not even jq. That is not incidental
# elegance: this is the hook whose job is reporting that jq is missing, and a reporter
# that needs the thing it reports on is unavailable in exactly the case that matters.
# Hence mkit_json_escape (awk) rather than jq for the payload.
#
# --- the tombstone ----------------------------------------------------------------------
#
# `install.sh --uninstall` writes `bootstrap.disabled`; this hook checks it first and
# exits silently forever while it is there. A user who has decided they do not want a
# tool mkit wants gets to say so once, rather than re-reading the same notice because
# self-healing keeps deciding the gap is new.
#
# It is a file rather than an absent state for the usual reason: a deleted file carries no
# provenance. "Never warned" and "warned and dismissed" are byte-identical on disk, so the
# intent needs its own record.
#
# --- what it will not do ----------------------------------------------------------------
#
#   - block, prompt, or install anything. A missing tool is the user's to install.
#   - warn about anything twice. bootstrap.state is the ledger of what has been said.
#   - stay quiet forever about a gap that went away and came back. A tool that is present
#     again loses its key, so a later removal warns again.
#
# Exit: always 0.

set -euo pipefail

# The trap normalizes every status to 0; errexit stays on so a half-computed state stops
# rather than continuing; every gate is an explicit `|| exit 0` so expected failures stay
# visible instead of being swept up by the trap; and the payload is buffered and printed
# by the last statement, so an abort part-way cannot leak truncated JSON.
trap 'exit 0' EXIT

# shellcheck source=../lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

# --- gate 1: drain stdin, and deliberately do not parse it -----------------------------
#
# This hook's action is user-scoped and event-independent: it would do exactly the same
# thing fired by any event, from any directory, so there is nothing in the payload to
# read. Parsing would mean requiring jq in the one hook that must work without it.
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

tombstone="$user_dir/bootstrap.disabled"
state="$user_dir/bootstrap.state"

# --- gate 3: the global opt-out --------------------------------------------------------
#
# One stat, and it must beat everything below it. An opt-out that still tells you about
# jq every session is not an opt-out.
[ ! -f "$tombstone" ] || exit 0

# --- gate 4: prerequisites -------------------------------------------------------------
# `command -v` is a builtin, so four lookups is sub-millisecond and forks nothing. It has
# to run on the silent path: you cannot know a tool is present without looking.
prereq_rows="$(mkit_prereq_rows --missing-only)" || true

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

# --- gate 5: nothing left to say -------------------------------------------------------
# The common path on every session after the first. Zero bytes, not an empty object.
if [ -z "$warn_keys" ]; then
	# Still heal the state file: a tool that came back should stop being remembered as
	# missing, even in a session with nothing else to do.
	for key in $(mkit_state_missing_keys "$state" "$prereq_rows"); do
		mkit_state_drop "$state" "$key" || true
	done
	exit 0
fi

# --- gate 6: spend the stamps, before emitting -----------------------------------------
#
# The asymmetry is the reason for the order: a stamp written for a message that never
# arrived costs one cosmetic line, while a message that arrives without its stamp is the
# repeating nag this whole file exists to prevent.
mkdir -p "$user_dir" 2>/dev/null || true
for key in $warn_keys; do
	mkit_state_add "$state" "$key" || true
done

# A tool that is present again loses its key, so a later removal warns again — a new gap
# deserves a new warning.
for key in $(mkit_state_missing_keys "$state" "$prereq_rows"); do
	mkit_state_drop "$state" "$key" || true
done

# --- gate 7: one payload ----------------------------------------------------------------
#
# systemMessage reaches the *user*; additionalContext reaches *Claude*. They are siblings
# in one object, so both audiences fit in a single emission — and a prereq gap genuinely
# needs both: only the user can `brew install jq`, and only the model will hit the
# confusing failures it causes and be able to offer to run it.
user_msg="mkit: setup is incomplete.$warn_lines"
agent_msg="mkit reports an incomplete setup:$warn_lines
Tell the user rather than working around it; a missing tool is theirs to install."

# additionalContext MUST be nested under hookSpecificOutput beside hookEventName. The
# flat top-level form is accepted and then silently ignored — no warning in stdout, in
# stream-json or in the transcript; the diagnostic shows up only under
# `claude --debug hooks`. The bats suite guards the shape.
out='{'
out="$out\"hookSpecificOutput\":{\"hookEventName\":\"SessionStart\","
out="$out\"additionalContext\":\"$(printf '%s' "$agent_msg" | mkit_json_escape)\"},"
out="$out\"systemMessage\":\"$(printf '%s' "$user_msg" | mkit_json_escape)\","
out="$out\"suppressOutput\":true}"

printf '%s\n' "$out"
