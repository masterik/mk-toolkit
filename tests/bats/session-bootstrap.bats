#!/usr/bin/env bats
#
# scripts/hooks/session-bootstrap.sh — the SessionStart hook.
#
# It installs nothing. Its whole contract is: name a missing prerequisite once, stay
# silent otherwise, never fail a session. Most of what follows is the "stay silent" half,
# because that is the half a regression makes intolerable rather than merely wrong.
#
# `[[ ... ]]` is avoided throughout, deliberately. On bash 3.2 — the bash these scripts
# target, and the one bats runs under on macOS — errexit does not fire for a failing
# `[[ ... ]]` compound, so such an assertion aborts a test only when it happens to be the
# test's last command. Every assertion below is a simple command or a `[ ... ]`, both of
# which do fail the test.

load helpers.bash

HOOK=""

setup() {
	mkit_setup_repo
	HOOK="$SCRIPTS/hooks/session-bootstrap.sh"
	USER_DIR="$MKIT_HOME"
	TOMBSTONE="$USER_DIR/bootstrap.disabled"
	STATE="$USER_DIR/bootstrap.state"
}

teardown() {
	mkit_teardown_repo
}

# Run the hook the way Claude Code does: a JSON event on stdin, stdout captured.
# `run` folds stderr into $output, so a "silent" assertion covers both streams.
hook() {
	run bash -c "printf '%s' '{\"hook_event_name\":\"SessionStart\",\"cwd\":\"$MKIT_TMP\",\"source\":\"startup\"}' | '$HOOK'"
}

has_text() { printf '%s\n' "$1" | grep -qF -- "$2"; }

# --- silence, the common path -----------------------------------------------------------

@test "a machine with every prerequisite emits zero bytes, not an empty JSON object" {
	hook
	[ "$status" -eq 0 ]
	[ "${#output}" -eq 0 ]
}

@test "it writes nothing when there is nothing to report" {
	hook
	[ ! -f "$STATE" ]
}

# The next three run with a tool missing on purpose. With every prerequisite present the
# hook is silent whatever it does, so asserting silence there proves only that the
# environment had nothing to report — the test would still pass if stdin handling, the
# cwd-independence or the whole payload path were broken. Given something to say, the
# message IS the evidence it ran to completion.

@test "empty stdin is handled — the hook still runs to completion" {
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	has_text "$output" 'findings.mjs'
}

@test "stdin that is not JSON is still handled — the hook must never grow a parser" {
	run env PATH="$(mkit_fake_path node)" bash -c "printf 'not json at all' | '$HOOK'"
	[ "$status" -eq 0 ]
	has_text "$output" 'findings.mjs'
}

@test "it runs to completion outside a git repository" {
	# It never calls git and deliberately does not cd to the event's cwd, so being outside
	# a repo must change nothing at all about what it reports.
	cd "$MKIT_TMP/.."
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	has_text "$output" 'findings.mjs'
}

@test "HOME and MKIT_HOME both unset is a silent exit 0" {
	run env -i PATH="$PATH" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a relative MKIT_HOME is a silent exit 0 and writes nothing" {
	run env MKIT_HOME='relative/path' bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ ! -e 'relative/path' ]
}

@test "an unwritable user dir is a silent exit 0" {
	mkdir -p "$MKIT_TMP/ro"
	chmod 500 "$MKIT_TMP/ro"
	run env MKIT_HOME="$MKIT_TMP/ro/mkit" bash -c "'$HOOK' </dev/null"
	chmod 700 "$MKIT_TMP/ro"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

# --- the opt-out tombstone -------------------------------------------------------------

@test "the tombstone suppresses the prerequisite warning" {
	mkdir -p "$USER_DIR"
	printf 'silenced\n' >"$TOMBSTONE"
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "the tombstone beats even a hard prerequisite gap" {
	mkdir -p "$USER_DIR"
	printf 'silenced\n' >"$TOMBSTONE"
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a tombstoned run records nothing either" {
	mkdir -p "$USER_DIR"
	printf 'silenced\n' >"$TOMBSTONE"
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ ! -f "$STATE" ]
}

# --- reporting a gap, exactly once ------------------------------------------------------

@test "a missing tool is reported once and never again" {
	fake="$(mkit_fake_path jq)"
	run env PATH="$fake" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	has_text "$output" 'jq'
	has_text "$output" 'setup is incomplete'
	run env PATH="$fake" bash -c "'$HOOK' </dev/null"
	[ -z "$output" ]
}

@test "the report names the feature a missing tool costs, not just the tool" {
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	has_text "$output" 'findings.mjs'
}

@test "a missing jq still produces valid JSON" {
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	# jq is on the *test's* PATH even though it was off the hook's — that is the point.
	printf '%s' "$output" | jq -e . >/dev/null
}

@test "the payload is exactly one line of JSON" {
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = 1 ]
	printf '%s' "$output" | jq -e . >/dev/null
}

@test "two tools missing are reported in one payload" {
	run env PATH="$(mkit_fake_path jq node)" bash -c "'$HOOK' </dev/null"
	[ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = 1 ]
	has_text "$output" 'jq'
	has_text "$output" 'findings.mjs'
}

@test "a tool installed later has its key dropped from the state file" {
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	run grep -c 'prereq/jq' "$STATE"
	[ "$output" = 1 ]
	hook
	run bash -c "grep -c 'prereq/jq' '$STATE' || true"
	[ "$output" = 0 ]
}

@test "a tool removed again is reported a second time" {
	fake="$(mkit_fake_path jq)"
	run env PATH="$fake" bash -c "'$HOOK' </dev/null"
	hook # jq back: key dropped
	run env PATH="$fake" bash -c "'$HOOK' </dev/null"
	has_text "$output" 'jq'
}

@test "the state file rewrite dedupes benign duplicates" {
	mkdir -p "$USER_DIR"
	# `prereq/gone` names a tool no longer in the table, so it is stale and triggers the
	# rewrite; the duplicate `prereq/node` rows are what the rewrite must collapse.
	printf 'prereq/gone\nprereq/gone\nprereq/node\nprereq/node\n' >"$STATE"
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ "$(wc -l <"$STATE" | tr -d ' ')" = 1 ]
	run cat "$STATE"
	[ "$output" = 'prereq/node' ]
}

@test "a user dir containing a quote and a backslash does not break the run" {
	# Note what this does NOT prove: the payload interpolates no path any more, so the weird
	# bytes never reach mkit_json_escape and this cannot stand in for a test of the escape
	# itself. What it does hold is that an awkward MKIT_HOME still yields a clean exit and a
	# parseable document. The escape's own correctness is covered directly in common.bats.
	weird="$MKIT_TMP/we\"ird\\home"
	mkdir -p "$weird"
	run env MKIT_HOME="$weird" PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	printf '%s' "$output" | jq -e . >/dev/null
}

# --- payload shape ----------------------------------------------------------------------

@test "additionalContext is nested under hookSpecificOutput, never top-level" {
	# The flat top-level form is accepted and silently ignored by the runtime, with no
	# diagnostic outside `claude --debug hooks`. The shape is a contract, not a preference.
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	run bash -c "printf '%s' '$output' | jq -r '.hookSpecificOutput.hookEventName'"
	[ "$output" = SessionStart ]
}

@test "the payload carries systemMessage and additionalContext together" {
	# Both audiences, one emission: only the user can install the tool, only the model
	# will hit the failures it causes.
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	out="$output"
	run bash -c "printf '%s' '$out' | jq -e 'has(\"systemMessage\")'"
	[ "$status" -eq 0 ]
	run bash -c "printf '%s' '$out' | jq -e '.hookSpecificOutput | has(\"additionalContext\")'"
	[ "$status" -eq 0 ]
}

@test "the payload sets suppressOutput" {
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	run bash -c "printf '%s' '$output' | jq -e '.suppressOutput'"
	[ "$status" -eq 0 ]
}

# --- concurrency -----------------------------------------------------------------------

@test "six simultaneous first runs stay consistent and leave no temp files" {
	fake="$(mkit_fake_path node)"
	for i in 1 2 3 4 5 6; do
		env PATH="$fake" bash -c "'$HOOK' </dev/null >/dev/null 2>&1" &
	done
	wait
	# Racing appends may write the key more than once — harmless for membership, and the
	# next rewrite dedupes. What must hold is that the key is there and nothing is left
	# half-written beside it.
	run grep -qxF 'prereq/node' "$STATE"
	[ "$status" -eq 0 ]
	run bash -c "ls -a '$USER_DIR' | grep -c '^bootstrap.state\.' || true"
	[ "$output" = 0 ]
	# And the steady state is still silence.
	run env PATH="$fake" bash -c "'$HOOK' </dev/null"
	[ -z "$output" ]
}
