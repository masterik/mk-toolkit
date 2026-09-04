#!/usr/bin/env bats
load helpers.bash

HOOK="$SCRIPTS/hooks/journal-nudge.sh"

setup() {
	mkit_setup_repo
	# The event files live OUTSIDE the repo on purpose: written inside it they would be
	# untracked, and every coverage assertion below would be counting them.
	EVD="$(mktemp -d "${TMPDIR:-/tmp}/mkit-ev.XXXXXX")"
	EVD="$(cd "$EVD" && pwd -P)"
}

teardown() {
	mkit_teardown_repo
	[ -n "${EVD:-}" ] && [ -d "$EVD" ] && rm -rf -- "$EVD"
}

# mkev <event> <prompt_id> <agent_id|""> <stop_hook_active true|false> [cwd]
# Mirrors the real payload, including the undocumented extras, and omits agent_id
# entirely when empty — which is how the parent's own Stop actually arrives.
mkev() {
	local f="$EVD/ev-$RANDOM$RANDOM.json"
	jq -cn --arg e "$1" --arg p "$2" --arg a "$3" --argjson s "$4" \
		--arg c "${5:-$MKIT_TMP}" \
		'{session_id: "s1", transcript_path: "/dev/null", hook_event_name: $e,
		  prompt_id: $p, cwd: $c, stop_hook_active: $s,
		  last_assistant_message: "done", permission_mode: "default"}
		 + (if $a == "" then {} else {agent_id: $a, agent_type: "general-purpose"} end)' >"$f"
	printf '%s\n' "$f"
}

# The hook reads stdin, so feed it a file. `run` folds stderr into $output, which makes
# every "silent" assertion below cover stderr too.
run_hook() { run bash -c "'$HOOK' <'$1'"; }

raw_stdin() { run bash -c "printf '%s' '$1' | '$HOOK'"; }

enable_and_dirty() {
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'edit\n' >>seed.txt
}

state_file() { printf '%s\n' "$MKIT_TMP/.git/mkit/journal-nudge.state"; }

# --- silent no-ops: every gate, plus the malformed input -----------------------------

@test "empty stdin is a silent exit 0" {
	enable_and_dirty
	run bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ ! -f "$(state_file)" ]
}

@test "malformed stdin is a silent exit 0" {
	enable_and_dirty
	raw_stdin 'not json at all {{{'
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ ! -f "$(state_file)" ]
}

@test "valid JSON that is not an object is a silent exit 0" {
	enable_and_dirty
	raw_stdin '[1,2,3]'
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "outside a git repo it is a silent exit 0" {
	enable_and_dirty
	local outside
	outside="$(mktemp -d "$EVD/notrepo.XXXXXX")"
	run_hook "$(mkev Stop p1 '' false "$outside")"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "journaling disabled is a silent exit 0, and writes no state" {
	printf 'edit\n' >>seed.txt
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ ! -f "$(state_file)" ]
}

@test "stop_hook_active true is a silent exit 0" {
	enable_and_dirty
	run_hook "$(mkev Stop p1 '' true)"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ ! -f "$(state_file)" ]
}

@test "full coverage is a silent exit 0" {
	enable_and_dirty
	"$SCRIPTS/journal.sh" add --paths seed.txt --type feat --scope core \
		--subject 'change the seed' --why 'the caller asked' >/dev/null
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a clean tree is a silent exit 0" {
	"$SCRIPTS/journal.sh" enable >/dev/null
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "an unknown hook_event_name is a silent exit 0" {
	enable_and_dirty
	run_hook "$(mkev PostToolUse p1 '' false)"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a missing prompt_id is a silent exit 0 - no budget means no bound" {
	enable_and_dirty
	raw_stdin "$(jq -cn --arg c "$MKIT_TMP" \
		'{hook_event_name: "Stop", cwd: $c, stop_hook_active: false}')"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

# --- the one case that nudges, for both events ---------------------------------------

@test "Stop nudges once, with hookEventName Stop" {
	enable_and_dirty
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	[ -n "$output" ]
	printf '%s' "$output" | jq -e '.hookSpecificOutput.hookEventName == "Stop"' >/dev/null
	printf '%s' "$output" | jq -e '.suppressOutput == true' >/dev/null
	[ -f "$(state_file)" ]
}

@test "SubagentStop nudges with hookEventName SubagentStop" {
	enable_and_dirty
	run_hook "$(mkev SubagentStop p1 a1b2c3d4e5f60718 false)"
	[ "$status" -eq 0 ]
	printf '%s' "$output" | jq -e '.hookSpecificOutput.hookEventName == "SubagentStop"' >/dev/null
}

@test "the emitted object is exactly one line of JSON" {
	enable_and_dirty
	run_hook "$(mkev Stop p1 '' false)"
	[ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" -eq 1 ]
}

# --- the mandatory JSON-shape guard --------------------------------------------------
#
# The published docs describe a flat top-level `additionalContext`. That form is SILENTLY
# ignored by the runtime - no warning in stdout, in stream-json or in the transcript; the
# diagnostic only shows under `claude --debug hooks`. So a regression to the documented
# shape would produce a hook that runs, exits 0, looks healthy and never nudges. This
# test is the only thing that would catch it.

@test "additionalContext is nested under hookSpecificOutput, never top-level" {
	enable_and_dirty
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	# nested, non-empty, and accompanied by hookEventName
	printf '%s' "$output" | jq -e '.hookSpecificOutput | has("additionalContext")' >/dev/null
	printf '%s' "$output" | jq -e '.hookSpecificOutput | has("hookEventName")' >/dev/null
	printf '%s' "$output" | jq -e '.hookSpecificOutput.additionalContext | type == "string"' >/dev/null
	printf '%s' "$output" | jq -e '(.hookSpecificOutput.additionalContext | length) > 0' >/dev/null
	# and NOT the documented-but-broken flat form
	printf '%s' "$output" | jq -e 'has("additionalContext") == false' >/dev/null
}

# --- the budget key is prompt_id + agent_id ------------------------------------------
#
# SubagentStop carries the PARENT's prompt_id, so a budget keyed on prompt_id alone would
# let whichever nudge fires first starve the other.

@test "the same prompt_id and agent_id nudges only once" {
	enable_and_dirty
	run_hook "$(mkev Stop p1 '' false)"
	[ -n "$output" ]
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "the same prompt_id with different agent_ids nudges each of them" {
	enable_and_dirty
	run_hook "$(mkev SubagentStop p1 aaaa1111 false)"
	printf '%s' "$output" | jq -e '.hookSpecificOutput.hookEventName == "SubagentStop"' >/dev/null
	# a second subagent under the same parent prompt
	run_hook "$(mkev SubagentStop p1 bbbb2222 false)"
	[ -n "$output" ]
	printf '%s' "$output" | jq -e '.hookSpecificOutput.hookEventName == "SubagentStop"' >/dev/null
	# and the parent's own Stop, which carries no agent_id, is a third slot
	run_hook "$(mkev Stop p1 '' false)"
	[ -n "$output" ]
	printf '%s' "$output" | jq -e '.hookSpecificOutput.hookEventName == "Stop"' >/dev/null
	# each of the three is now spent
	run_hook "$(mkev SubagentStop p1 aaaa1111 false)"
	[ -z "$output" ]
	run_hook "$(mkev Stop p1 '' false)"
	[ -z "$output" ]
}

@test "a new prompt_id gets a fresh budget" {
	enable_and_dirty
	run_hook "$(mkev Stop p1 '' false)"
	[ -n "$output" ]
	run_hook "$(mkev Stop p2 '' false)"
	[ -n "$output" ]
	# Keys are appended, not filtered-and-replaced, so p1's line is still here. The
	# assertion used to be `= 1`, which pinned the read-modify-write that lost keys;
	# what bounds the file now is the prune pass below.
	run grep -c . "$(state_file)"
	[ "$output" = 2 ]
}

@test "simultaneous nudges for different agents each record their key" {
	# The reproduced defect: 6 stop events inside one millisecond left 1 of 6 keys in
	# the state file, so 5 agents were nudged a second time. An append cannot lose a
	# concurrent one.
	enable_and_dirty
	for i in 1 2 3 4 5 6; do
		ev="$(mkev SubagentStop p1 "agent$i" false)"
		bash -c "'$HOOK' <'$ev' >/dev/null" &
	done
	wait
	[ "$(grep -c . "$(state_file)")" -eq 6 ]
	for i in 1 2 3 4 5 6; do
		grep -qxF "p1/agent$i" "$(state_file)"
	done
}

@test "the state file is pruned to the prompt in flight once it outgrows the cap" {
	# Appending cannot prune, so the bound is a later pass rather than every write.
	# Pre-seeded past the cap instead of running 65 hooks: the mechanism under test is
	# the rewrite, not the counting.
	enable_and_dirty
	mkdir -p "$MKIT_TMP/.git/mkit"
	for i in $(seq 1 70); do printf 'oldprompt%s/\n' "$i"; done >"$(state_file)"
	run_hook "$(mkev Stop p1 '' false)"
	[ -n "$output" ]
	[ "$(grep -c . "$(state_file)")" -eq 1 ]
	grep -qxF 'p1/' "$(state_file)"
	# and the prune leaves no temp file beside the state file
	[ -z "$(find "$MKIT_TMP/.git/mkit" -maxdepth 1 -name 'journal-nudge.state.*')" ]
}

# --- the payload ---------------------------------------------------------------------

@test "the nudge names the uncovered count and the journal.sh add invocation, never the paths themselves" {
	# The one-line budget itself is pinned by the test below; this one covers content.
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'edit\n' >>seed.txt
	printf 'new\n' >added.txt
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	local ctx
	ctx="$(printf '%s' "$output" | jq -r '.hookSpecificOutput.additionalContext')"
	# Simple commands, not `[[ ... ]]`: on bash 3.2 errexit does not fire for a failing
	# `[[ ... ]]`, so such an assertion aborts a bats test only as its last command —
	# and every check here has to be able to fail.
	printf '%s' "$ctx" | grep -qF -- '2 uncovered path(s)'
	printf '%s' "$ctx" | grep -qF -- "$SCRIPTS/journal.sh uncovered"
	printf '%s' "$ctx" | grep -qF -- "$SCRIPTS/journal.sh add "
	printf '%s' "$ctx" | grep -qF -- '--source stop'
	# the add flags are journal.md's job now, not the transcript's
	[ -z "$(printf '%s' "$ctx" | grep -F -- '--subject' || true)" ]
	[ -z "$(printf '%s' "$ctx" | grep -F -- '--why' || true)" ]
	# the resolved absolute path, never an unexpanded plugin-root placeholder
	[ -z "$(printf '%s' "$ctx" | grep -F -- '${CLAUDE_PLUGIN_ROOT}' || true)" ]
	[ -z "$(printf '%s' "$ctx" | grep -F -- 'CLAUDE_PLUGIN_ROOT' || true)" ]
	# collapsed: the individual paths are not named — the agent fetches them itself
	[ -z "$(printf '%s' "$ctx" | grep -F -- 'seed.txt' || true)" ]
	[ -z "$(printf '%s' "$ctx" | grep -F -- 'added.txt' || true)" ]
	# the contract is inlined, and the reference is a pointer for the rare case
	printf '%s' "$ctx" | grep -qF -- 'One unit = one reason'
	printf '%s' "$ctx" | grep -qF -- '_shared/references/journal.md'
}

# --- the transcript budget -----------------------------------------------------------
#
# additionalContext is rendered verbatim in the user's transcript, prefixed "Stop hook
# feedback:", and `suppressOutput` does not hide it — there is no display setting that
# does. So every line the nudge spends is a line of the user's own answer pushed off
# screen, on every turn of every journaling repo. The nudge is therefore one line, and
# this test is the only thing keeping it one: nothing else fails when a helpful second
# sentence becomes a helpful second paragraph.

@test "the nudge is a single line" {
	enable_and_dirty
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	local ctx
	ctx="$(printf '%s' "$output" | jq -r '.hookSpecificOutput.additionalContext')"
	# grep -c '' counts lines; the context is printed without a trailing newline, so a
	# one-line nudge is 1 and any embedded newline — blank line or not — is 2 or more.
	[ "$(printf '%s' "$ctx" | grep -c '')" -eq 1 ]
	# and stdout stays the single JSON line it has always been
	[ "$(printf '%s\n' "$output" | grep -c .)" -eq 1 ]
}

@test "a SubagentStop nudge asks for --source subagent-stop" {
	enable_and_dirty
	run_hook "$(mkev SubagentStop p1 aaaa1111 false)"
	local ctx
	ctx="$(printf '%s' "$output" | jq -r '.hookSpecificOutput.additionalContext')"
	[[ "$ctx" == *"--source subagent-stop"* ]]
}

@test "the nudge points at journal.sh uncovered instead of naming an unusual path, and covering it silences the next one" {
	# Quoting an unusual filename correctly is journal.sh's job (tests/bats/journal.bats
	# covers café.txt / spaces / quotes there); this test only needs to confirm the hook
	# no longer re-renders the path itself, and that the pointer it gives instead
	# (`journal.sh uncovered`) leads to a real fix for the gap.
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'x\n' >'café.txt'
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	ctx="$(printf '%s' "$output" | jq -r '.hookSpecificOutput.additionalContext')"
	printf '%s' "$ctx" | grep -qF -- "$SCRIPTS/journal.sh uncovered"
	[ -z "$(printf '%s' "$ctx" | grep -F -- 'café.txt' || true)" ]

	run "$SCRIPTS/journal.sh" add --paths 'café.txt' --type feat --scope core \
		--subject s --why w
	[ "$status" -eq 0 ]
	# covering the path the hook pointed at is now recorded, so the next prompt is silent
	run_hook "$(mkev Stop p2 '' false)"
	[ -z "$output" ]
}

@test "a dirty tree of only *.lock / *.snap paths never nudges" {
	# Real names that really match the glob — `package-lock.lock` did not exist and
	# matched by construction, so it could not tell a working exclusion from a
	# fabricated one. Cargo.lock is the case a user actually hits.
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'lock\n' >Cargo.lock
	printf 'snap\n' >ui.snap
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a lockfile that does not match the glob still nudges" {
	# The exclusion is a glob, not an intent: package-lock.json is a lockfile and is
	# nudged. Pins the pair so the comment beside the pathspec cannot drift back to
	# "lockfile churn never nudges". The hook no longer names the path itself, so the
	# count is what proves it was counted as uncovered.
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf '{}\n' >package-lock.json
	run_hook "$(mkev Stop p1 '' false)"
	[ "$status" -eq 0 ]
	[ -n "$output" ]
	ctx="$(printf '%s' "$output" | jq -r .hookSpecificOutput.additionalContext)"
	printf '%s' "$ctx" | grep -qF -- '1 uncovered path(s)'
}

# --- control characters are rejected, never normalised away ------------------------

@test "a cwd containing a newline is rejected rather than resolved to another repo" {
	# Verified failure, not a theoretical one: stripping the newline turned a session
	# in $'repo\nevil' into the sibling repo `repoevil`, so the hook nudged about that
	# repo's paths and spent its budget there while this repo went unnudged.
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'dirty\n' >a.txt
	run bash -c 'printf "{\"hook_event_name\":\"Stop\",\"cwd\":\"%s\\nevil\",\"prompt_id\":\"p1\",\"stop_hook_active\":false}" "$1" | "$2"' \
		_ "$MKIT_TMP" "$SCRIPTS/hooks/journal-nudge.sh"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a prompt_id containing a newline is rejected, so budget keys cannot alias" {
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'dirty\n' >a.txt
	run bash -c 'printf "{\"hook_event_name\":\"Stop\",\"cwd\":\"%s\",\"prompt_id\":\"p\\n1\",\"stop_hook_active\":false}" "$1" | "$2"' \
		_ "$MKIT_TMP" "$SCRIPTS/hooks/journal-nudge.sh"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a non-string identifier is rejected" {
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'dirty\n' >a.txt
	run bash -c 'printf "{\"hook_event_name\":\"Stop\",\"cwd\":\"%s\",\"prompt_id\":42,\"stop_hook_active\":false}" "$1" | "$2"' \
		_ "$MKIT_TMP" "$SCRIPTS/hooks/journal-nudge.sh"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

# --- the silence contract covers stderr too ---------------------------------------

@test "a state write that cannot succeed stays silent and leaves no temp file" {
	# mktemp and mv print real diagnostics on a read-only git dir ("mkstemp failed",
	# "rename ... Operation not permitted"); the EXIT trap only fixes the status.
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'dirty\n' >a.txt
	mkit_dir="$MKIT_TMP/.git/mkit"
	chmod 500 "$mkit_dir"
	run bash -c 'printf "{\"hook_event_name\":\"Stop\",\"cwd\":\"%s\",\"prompt_id\":\"p1\",\"stop_hook_active\":false}" "$1" | "$2" 2>&1' \
		_ "$MKIT_TMP" "$SCRIPTS/hooks/journal-nudge.sh"
	chmod 700 "$mkit_dir"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ -z "$(find "$mkit_dir" -maxdepth 1 -name 'journal-nudge.state.*' 2>/dev/null)" ]
}

# --- the nudge must not claim a provenance it cannot prove ------------------------

@test "the nudge does not claim the uncovered paths came from this turn" {
	# `uncovered` is a whole-tree set difference with no turn baseline, so it cannot
	# tell the agent's edits from work already in the tree. Asserting otherwise had
	# the hook author a judgement (rule 2) and pressured the agent to invent intent.
	"$SCRIPTS/journal.sh" enable >/dev/null
	printf 'dirty\n' >a.txt
	run bash -c 'printf "{\"hook_event_name\":\"Stop\",\"cwd\":\"%s\",\"prompt_id\":\"p1\",\"stop_hook_active\":false}" "$1" | "$2"' \
		_ "$MKIT_TMP" "$SCRIPTS/hooks/journal-nudge.sh"
	[ "$status" -eq 0 ]
	ctx="$(printf '%s' "$output" | jq -r .hookSpecificOutput.additionalContext)"
	[[ "$ctx" != *"from this turn"* ]]
	[[ "$ctx" == *"record only the paths you changed"* ]]
	[[ "$ctx" == *"Never invent a reason"* ]]
	# and it tells the agent what to do when none of them are its own: nothing, silently
	[[ "$ctx" == *"record nothing and stay silent"* ]]
}

@test "a prune that cannot run still delivers the nudge and leaves no temp file" {
	# The budget is spent by the append, so a failing prune must not swallow the nudge.
	# A read-only mkit dir with the state file already in it is exactly that case:
	# appending to an existing file still works, mktemp in the directory does not.
	enable_and_dirty
	mkit_dir="$MKIT_TMP/.git/mkit"
	for i in $(seq 1 70); do printf 'oldprompt%s/\n' "$i"; done >"$(state_file)"
	chmod 500 "$mkit_dir"
	run bash -c "'$HOOK' <'$(mkev Stop p1 '' false)' 2>&1"
	chmod 700 "$mkit_dir"
	[ "$status" -eq 0 ]
	printf '%s' "$output" | jq -e '.hookSpecificOutput.hookEventName == "Stop"' >/dev/null
	# the key was still recorded, and the unpruned file is the only cost
	grep -qxF 'p1/' "$(state_file)"
	[ -z "$(find "$mkit_dir" -maxdepth 1 -name 'journal-nudge.state.*')" ]
}
