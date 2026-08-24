#!/usr/bin/env bats
#
# scripts/hooks/session-bootstrap.sh — the SessionStart hook.
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
	mkit_sandbox_home
	HOOK="$SCRIPTS/hooks/session-bootstrap.sh"
	USER_DIR="$MKIT_HOME"
	MARKER="$USER_DIR/journal.default"
	TOMBSTONE="$USER_DIR/bootstrap.disabled"
	STATE="$USER_DIR/bootstrap.state"
	WRAPPER="$MKIT_BIN/mkit-journal"
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

# --- setup and idempotence -------------------------------------------------------------

@test "a fresh user dir gets the marker, the wrapper and one notice" {
	hook
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
	[ -x "$WRAPPER" ]
	has_text "$output" '"systemMessage"'
	has_text "$output" 'on by default in every repo'
}

@test "a second run with everything correct emits zero bytes" {
	hook
	[ "$status" -eq 0 ]
	hook
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "the setup notice never repeats" {
	hook
	hook
	hook
	[ -z "$output" ]
}

@test "the payload is exactly one line of JSON" {
	hook
	[ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = 1 ]
	printf '%s' "$output" | jq -e . >/dev/null
}

@test "the marker is not re-truncated once it exists" {
	hook
	printf 'future content\n' >"$MARKER"
	hook
	[ "$(cat "$MARKER")" = 'future content' ]
}

@test "empty stdin is a silent exit 0" {
	run bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
}

@test "stdin that is not JSON is still handled — the hook must never grow a parser" {
	run bash -c "printf 'not json at all' | '$HOOK'"
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
}

@test "it runs to completion outside a git repository" {
	cd "$HOME"
	run bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
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

@test "the tombstone suppresses all setup and all output" {
	mkdir -p "$USER_DIR"
	printf 'pinned off\n' >"$TOMBSTONE"
	hook
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ ! -f "$MARKER" ]
	[ ! -e "$WRAPPER" ]
}

@test "the tombstone suppresses the prerequisite warning too" {
	mkdir -p "$USER_DIR"
	printf 'pinned off\n' >"$TOMBSTONE"
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "a tombstoned run does not recreate a hand-deleted marker" {
	hook
	[ -f "$MARKER" ]
	rm -f "$MARKER"
	printf 'pinned off\n' >"$TOMBSTONE"
	hook
	[ ! -f "$MARKER" ]
}

# --- the wrapper -----------------------------------------------------------------------

@test "the wrapper execs journal.sh and answers a real subcommand" {
	hook
	run "$WRAPPER" enabled --why
	[ "$status" -eq 0 ]
	has_text "$output" enabled
}

@test "the wrapper is mode 755, not an unreadable 711" {
	hook
	run bash -c "ls -l '$WRAPPER' | cut -c1-10"
	[ "$output" = '-rwxr-xr-x' ]
}

@test "no bin directory means no wrapper, and the steady state is still silent" {
	rmdir "$MKIT_BIN"
	run env MKIT_BIN="$MKIT_TMP/nope" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
	[ ! -d "$MKIT_TMP/nope" ]
	run env MKIT_BIN="$MKIT_TMP/nope" bash -c "'$HOOK' </dev/null"
	[ -z "$output" ]
}

@test "a wrapper baked with a stale plugin root is rewritten, silently" {
	hook
	printf '#!/usr/bin/env bash\n# mkit-generated wrapper — do not edit; re-generated on session start.\nexec "/gone/old/root/scripts/journal.sh" "$@"\n' >"$WRAPPER"
	chmod 755 "$WRAPPER"
	hook
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	run grep -c 'gone/old/root' "$WRAPPER"
	[ "$output" = 0 ]
	run "$WRAPPER" enabled
	[ "$status" -eq 0 ]
}

@test "a wrapper already pointing here is not rewritten" {
	hook
	touch -t 202601010000 "$MKIT_TMP/reference"
	touch "$WRAPPER"
	touch -t 202601010000 "$WRAPPER"
	hook
	# find -newer, not stat: BSD and GNU stat take different flags and macOS is the
	# supported platform, so the portable comparison is a file-vs-file mtime test.
	run find "$WRAPPER" -newer "$MKIT_TMP/reference"
	[ -z "$output" ]
}

@test "a foreign mkit-journal is never overwritten, and is reported once" {
	printf '#!/bin/sh\necho someone elses tool\n' >"$WRAPPER"
	chmod 755 "$WRAPPER"
	hook
	has_text "$output" 'mkit did not create it'
	run cat "$WRAPPER"
	has_text "$output" 'someone elses tool'
	hook
	[ -z "$output" ]
}

@test "a symlink at the wrapper path is never replaced" {
	ln -s /bin/echo "$WRAPPER"
	hook
	[ -L "$WRAPPER" ]
	[ "$(readlink "$WRAPPER")" = /bin/echo ]
}

@test "a directory at the wrapper path is left alone" {
	mkdir -p "$WRAPPER"
	hook
	[ "$status" -eq 0 ]
	[ -d "$WRAPPER" ]
}

@test "an unwritable bin directory is reported once and then never again" {
	chmod 500 "$MKIT_BIN"
	hook
	chmod 700 "$MKIT_BIN"
	has_text "$output" 'not writable'
	chmod 500 "$MKIT_BIN"
	hook
	chmod 700 "$MKIT_BIN"
	[ -z "$output" ]
}

@test "the wrapper write leaves no temp file beside it" {
	hook
	run bash -c "ls -a '$MKIT_BIN' | grep -c '^\.mkit-journal' || true"
	[ "$output" = 0 ]
}

# --- prerequisites and warn-once -------------------------------------------------------

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

@test "a missing hard prerequisite still writes the marker" {
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	[ -f "$MARKER" ]
}

@test "a hard prerequisite gap withholds the setup notice" {
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	run bash -c "printf '%s' '$output' | grep -c 'on by default' || true"
	[ "$output" = 0 ]
}

@test "the notice arrives the run after the hard prerequisite is satisfied" {
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	hook
	has_text "$output" 'on by default in every repo'
}

@test "a soft prerequisite does not withhold the notice" {
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	has_text "$output" 'on by default in every repo'
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
	hook
	printf 'notice/v1\nnotice/v1\nprereq/gone\n' >"$STATE"
	hook
	[ "$(wc -l <"$STATE" | tr -d ' ')" = 1 ]
}

@test "a user dir containing a quote and a backslash still yields valid JSON" {
	weird="$MKIT_TMP/we\"ird\\home"
	mkdir -p "$weird"
	run env MKIT_HOME="$weird" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	printf '%s' "$output" | jq -e . >/dev/null
	printf '%s' "$output" | jq -r .systemMessage | grep -qF 'we"ird\home'
}

# --- payload shape — the journal-nudge lesson ------------------------------------------

@test "additionalContext is nested under hookSpecificOutput, never top-level" {
	# The flat top-level form is accepted and silently ignored by the runtime, with no
	# diagnostic outside `claude --debug hooks`. journal-nudge.sh was bitten by exactly
	# this; the shape is a contract, not a preference.
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	run bash -c "printf '%s' '$output' | jq -r '.hookSpecificOutput.hookEventName'"
	[ "$output" = SessionStart ]
}

@test "the prereq payload carries systemMessage and additionalContext together" {
	run env PATH="$(mkit_fake_path jq)" bash -c "'$HOOK' </dev/null"
	out="$output"
	run bash -c "printf '%s' '$out' | jq -e 'has(\"systemMessage\")'"
	[ "$status" -eq 0 ]
	run bash -c "printf '%s' '$out' | jq -e '.hookSpecificOutput | has(\"additionalContext\")'"
	[ "$status" -eq 0 ]
}

@test "the setup notice carries systemMessage and no additionalContext" {
	hook
	out="$output"
	run bash -c "printf '%s' '$out' | jq -e 'has(\"systemMessage\")'"
	[ "$status" -eq 0 ]
	run bash -c "printf '%s' '$out' | jq -r '.hookSpecificOutput // \"absent\"'"
	[ "$output" = absent ]
}

@test "the silent path emits zero bytes, not an empty JSON object" {
	hook
	hook
	[ "${#output}" -eq 0 ]
}

# --- concurrency -----------------------------------------------------------------------

@test "six simultaneous first runs leave one marker, one wrapper and no temp files" {
	for i in 1 2 3 4 5 6; do
		bash -c "'$HOOK' </dev/null >/dev/null 2>&1" &
	done
	wait
	[ -f "$MARKER" ]
	[ -x "$WRAPPER" ]
	run bash -c "ls -a '$MKIT_BIN' | grep -c '^\.mkit-journal' || true"
	[ "$output" = 0 ]
	run "$WRAPPER" enabled
	[ "$status" -eq 0 ]
}

# --- the end-to-end path nothing covered before ----------------------------------------

@test "after the hook runs, a pristine repo reports enabled user" {
	hook
	run "$SCRIPTS/journal.sh" enabled --why
	[ "$output" = 'enabled user' ]
}

@test "journal.sh disable still beats the hook-written user default" {
	hook
	run "$SCRIPTS/journal.sh" disable
	[ "$status" -eq 0 ]
	run "$SCRIPTS/journal.sh" enabled --why
	[ "$output" = 'disabled repo' ]
}

@test "the hook does not resurrect repo state after journal.sh disable" {
	hook
	"$SCRIPTS/journal.sh" disable
	hook
	run "$SCRIPTS/journal.sh" enabled --why
	[ "$output" = 'disabled repo' ]
}
