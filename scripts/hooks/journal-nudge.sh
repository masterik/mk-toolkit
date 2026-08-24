#!/usr/bin/env bash
#
# Stop / SubagentStop hook: name the changed paths this turn left without recorded
# intent, and hand that gap back to the model.
#
# Reads the event JSON on stdin. Emits nothing at all unless every gate below passes;
# when they do, emits one line of JSON:
#
#   {"hookSpecificOutput":{"hookEventName":"Stop","additionalContext":"..."},"suppressOutput":true}
#
# Registered by hooks/hooks.json at the plugin root, which is auto-discovered — so the
# hook is live the moment mkit is installed. That is exactly why the second gate is
# non-negotiable: journaling does nothing until `journal.sh enable` is run in a repo.
#
# The gates, cheapest first:
#
#   inside a git repo        a hook must be inert everywhere else
#   journaling enabled       journal.sh enabled — the repo-local opt-in marker
#   stop_hook_active false   the runtime's own same-turn guard; it arrives true on every
#                            round after the first, including rounds an exit-0
#                            additionalContext caused, so it is a real guard here
#   budget not spent         one nudge per prompt_id + agent_id, in a state file
#   journal_uncovered > 0    journal.sh uncovered — nothing to say otherwise
#
# Gate 5 is a whole-tree set difference, NOT a record of the turn: `uncovered` has no
# turn baseline, so the list can include paths the user changed before the agent ran.
# The nudge therefore names the gap and says so, and asks the agent to record only what
# it actually did — anything stronger would have the hook assert a provenance it cannot
# prove, and pressure the agent to invent intent for someone else's edit.
#
# Why the budget is load-bearing rather than belt-and-braces: an exit-0
# additionalContext *does* increment the runtime's consecutive-block counter, but the
# counter resets to 0 on any tool call — and this nudge asks for a tool call. Measured
# with CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=1: 5 hook invocations, 3 tool calls. The
# runtime cap (default 8, undocumented) will not bound a misbehaving journal hook. This
# state file is the only real bound.
#
# The key is prompt_id + agent_id, NOT prompt_id alone. SubagentStop carries the
# *parent's* prompt_id — a subagent's nudges and the parent's own later Stop share one
# prompt_id — so keying on prompt_id alone would let whichever fires first starve the
# other. Do not "simplify" this back to one field.
#
# Known, accepted gap: SubagentStop writes to the subagent's own git dir. A subagent
# under isolation: "worktree" journals into a worktree the parent never reads, and those
# entries die with the worktree. Same shape as the existing "entries die with the
# worktree" rule in journal.md — documented, not special-cased.
#
# Always exit 0, never exit 2. Exit 0 plus additionalContext already continues the turn
# so the model can act; exit 2 buys nothing and spends the block cap. A journaling hook
# must never fail or delay a user's edit.

set -euo pipefail

# set -euo pipefail and "must never fail" are reconciled here, deliberately, rather
# than by dropping either:
#   - the trap turns every exit status into 0, so an unbound variable, a failed git call
#     or a jq that chokes still ends the hook silently and successfully;
#   - errexit stays on so such a failure *stops* instead of continuing on half-computed
#     state, and every gate is written as an explicit `|| exit 0` so the expected
#     failures are visible rather than swept up by the trap;
#   - the nudge is buffered in a variable and printed by the last statement in the
#     script, so an abort part-way through cannot leak a truncated JSON object.
# `main || true` would have been the obvious alternative and is a trap: bash disables
# errexit inside a function invoked on the left of ||, so the whole body would run
# unguarded.
trap 'exit 0' EXIT

# shellcheck source=../lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

command -v jq >/dev/null 2>&1 || exit 0

plugin_root="$(mkit_plugin_root)"
journal_sh="$plugin_root/scripts/journal.sh"
[ -x "$journal_sh" ] || exit 0

# --- the event ---------------------------------------------------------------------
# One jq call for the whole payload. Malformed JSON, empty stdin and a payload that is
# valid JSON but not an object all land on the same silent exit.
event="$(cat)" || exit 0
[ -n "$event" ] || exit 0

# 0x1f, not a tab, as the field separator: a tab is IFS whitespace, so `IFS=$'\t' read`
# collapses runs of them — and agent_id is empty on the parent's own Stop, which would
# silently shift stop_hook_active into it. 0x1f cannot appear in any of these values.
SEP=$'\037'
# Reject, never normalise. Stripping control characters here aliased distinct events
# onto one identity: two prompt_ids differing only by an embedded newline shared a
# budget slot, and — verified, not theoretical — a session whose cwd was literally
# named $'repo\nevil' had the newline deleted, resolved to the *sibling* repo
# `repoevil`, nudged about that repo's uncovered paths and spent its budget there while
# its own repo went unnudged. A field carrying a control character or the separator is
# not a field we can identify, so the event is not ours to answer.
fields="$(printf '%s' "$event" | jq -r --arg s "$SEP" '
	if type != "object" then empty
	else
		[(.hook_event_name // ""), (.cwd // ""), (.prompt_id // ""),
		 (.agent_id // ""), (if .stop_hook_active == true then "1" else "0" end)]
		| if any(.[]; (type != "string") or test("[\t\n\r\u001f]")) then empty
		  else join($s) end
	end
' 2>/dev/null)" || exit 0
[ -n "$fields" ] || exit 0

IFS=$SEP read -r hook_event cwd prompt_id agent_id stop_active <<EOF
$fields
EOF

# Only these two events have a delivery channel back to the model; anything else that
# somehow reaches this script is not ours to answer.
case "$hook_event" in
Stop | SubagentStop) ;;
*) exit 0 ;;
esac

# No prompt_id means no budget, and no budget means no bound on the loop. Stay silent
# rather than nudge unbounded — the guard matters more than the nudge.
[ -n "$prompt_id" ] || exit 0

# --- gate 1: inside a git repo ------------------------------------------------------
# The event's cwd, not this process's: a hook is not started in the session's directory.
if [ -n "$cwd" ]; then
	cd "$cwd" 2>/dev/null || exit 0
fi
repo="$(git rev-parse --is-inside-work-tree --absolute-git-dir 2>/dev/null)" || exit 0
in_tree="$(printf '%s\n' "$repo" | sed -n 1p)"
git_dir="$(printf '%s\n' "$repo" | sed -n 2p)"
[ "$in_tree" = true ] || exit 0
[ -n "$git_dir" ] || exit 0

# --- gate 2: journaling enabled for this repo --------------------------------------
# Through journal.sh rather than by stat-ing the marker directly, so the marker's
# location stays one script's business.
[ "$("$journal_sh" enabled 2>/dev/null || true)" = enabled ] || exit 0

# --- gate 3: not already inside a hook-continued turn ------------------------------
[ "$stop_active" = 0 ] || exit 0

# --- gate 4: the per-prompt, per-agent budget --------------------------------------
mkit_dir="$git_dir/mkit"
state="$mkit_dir/journal-nudge.state"
# agent_id is absent for the parent's own Stop, which is a distinct slot from every
# subagent's, not a shared one.
key="$prompt_id/$agent_id"
if [ -f "$state" ] && grep -qxF -- "$key" "$state" 2>/dev/null; then
	exit 0
fi

# --- gate 5: is there anything uncovered ------------------------------------------
unc_out="$("$journal_sh" uncovered 2>/dev/null)" || exit 0
n_uncovered="$(printf '%s\n' "$unc_out" | sed -n 's/^journal_uncovered=\([0-9]\{1,\}\)$/\1/p' | sed -n 1p)"
[ -n "$n_uncovered" ] || exit 0
[ "$n_uncovered" -gt 0 ] || exit 0
uncovered="$(printf '%s\n' "$unc_out" | sed -n '/^uncovered:$/,$p' | sed '1d' | awk 'NF')"
[ -n "$uncovered" ] || exit 0

# --- the nudge ---------------------------------------------------------------------
# journal.sh is named by its resolved absolute path: the model must not be handed an
# unexpanded ${CLAUDE_PLUGIN_ROOT} to resolve itself. mkit_plugin_root derives it from
# this file's own location, so it is right whatever the install looks like — and note
# that the runtime's own ${CLAUDE_PLUGIN_ROOT} carries a trailing slash, which is
# another reason not to build this path from the environment.
case "$hook_event" in
SubagentStop) source_tag=subagent-stop ;;
*) source_tag=stop ;;
esac

# The contract is inlined, not linked, on purpose: it keeps compliance to one Bash call
# with no skill load. The reference is for the rare case only. Kept to one short
# paragraph after the path list, not a multi-line spec: every phrase a test pins
# (grep the .bats file before trimming further) still has to appear verbatim.
context="$(printf '%s\n' \
	"mkit journal: $n_uncovered uncovered path(s)." \
	"" \
	"uncovered:" \
	"$(printf '%s\n' "$uncovered" | sed 's/^/  /')" \
	"" \
	"  $journal_sh add --paths <a,b> --type <feat|fix|docs|refactor|test|chore|perf> --scope <s> --subject \"...\" --why \"...\" --source $source_tag" \
	"" \
	"One unit = one reason. Record only the paths you changed and know the reason for. Never invent a reason to cover a path. Details: $plugin_root/skills/_shared/references/journal.md")"

# Built by jq, never by string interpolation: the uncovered paths go inside a JSON
# string, and a path with a quote or a backslash in it must land as data.
#
# additionalContext MUST be nested under hookSpecificOutput alongside hookEventName.
# The flat top-level form is silently ignored - no warning in stdout, in stream-json or
# in the transcript; the diagnostic only appears under `claude --debug hooks`. The
# published docs describe the flat form. tests/bats/journal-nudge.bats guards this.
out="$(jq -cn --arg ev "$hook_event" --arg ctx "$context" \
	'{hookSpecificOutput: {hookEventName: $ev, additionalContext: $ctx},
	  suppressOutput: true}')" || exit 0

# Spend the budget before emitting, never after: a nudge that was delivered but not
# recorded is the unbounded loop this gate exists to prevent.
#
# One `>>` append, deliberately not a lock and no longer a read-modify-write. Two stop
# events for *different* agents of one prompt can land in the same millisecond — a
# parallel fan-out finishing together — and the filter-and-replace this replaces then lost
# 5 of 6 keys (measured; at a 5ms stagger, 0 of 12). A lock would serialize the turn's
# critical path, which this hook must never delay; a single short line appended with `>>`
# is atomic. The same-key race needs no handling at all: one agent has one stop event at
# a time.
#
# The file still self-cleans, one pass later instead of on every write: an append cannot
# prune, so once it has outgrown state_max_lines it is rewritten to the prompt in flight
# — which always includes the key just appended. That rewrite has the same lost-update
# window as before, and there it does not matter: a line lost at prune time costs one
# extra nudge later in that prompt, never an unbounded loop.
#
# Every write can fail on a read-only or full git dir — mktemp with "mkstemp failed:
# Permission denied", mv with "rename: Operation not permitted", `>>` with "Permission
# denied" — and the EXIT trap only fixes the *status*, not what already reached stderr.
# The silence contract covers stderr too (journal-nudge.bats asserts empty output), so
# every write is muted. Note the braces around the append: redirections are set up left
# to right, so a bare `>>"$state" 2>/dev/null` reports its own failure *before* stderr is
# redirected, and the diagnostic escapes.
state_max_lines=64
mkdir -p "$mkit_dir" 2>/dev/null || exit 0
{ printf '%s\n' "$key" >>"$state"; } 2>/dev/null || exit 0

lines="$(wc -l <"$state" 2>/dev/null | tr -d ' ')" || lines=0
[ -n "$lines" ] || lines=0
if [ "$lines" -gt "$state_max_lines" ]; then
	# None of this is allowed to swallow the nudge, unlike the write above: the budget is
	# already spent, so an aborted prune would cost a nudge and gain nothing. A prune that
	# cannot run just leaves the file longer than we would like.
	if tmp="$(mktemp "$state.XXXXXX" 2>/dev/null)"; then
		# From here the temp file exists, so every path out of this block removes it
		# rather than leave it beside the state file forever.
		trap 'rm -f -- "$tmp" 2>/dev/null; exit 0' EXIT
		if awk -v p="$prompt_id/" 'index($0, p) == 1' "$state" >"$tmp" 2>/dev/null; then
			mv -f "$tmp" "$state" 2>/dev/null || true
		fi
		rm -f -- "$tmp" 2>/dev/null || true
		trap 'exit 0' EXIT
	fi
fi

# suppressOutput keeps the raw stdout out of the user's transcript; the model still
# receives additionalContext. The nudge is the agent's business, not the user's.
printf '%s\n' "$out"
