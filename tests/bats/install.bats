#!/usr/bin/env bats
#
# install.sh — the by-hand diagnostic, and the only way to silence the SessionStart hook.
# It installs nothing; these tests exist mostly to hold that line, plus the anti-drift
# check at the end: the hook shares its prerequisite table with this script, so the two
# reporting different sentences for the same gap is a real failure mode.
#
# `[[ ... ]]` is avoided throughout: on bash 3.2 errexit does not fire for a failing
# `[[ ... ]]`, so such an assertion only aborts a test as its last command.

load helpers.bash

setup() {
	mkit_setup_repo
	INSTALL="$(cd "$SCRIPTS/.." && pwd)/install.sh"
	USER_DIR="$MKIT_HOME"
	TOMBSTONE="$USER_DIR/bootstrap.disabled"
	STATE="$USER_DIR/bootstrap.state"
	HOOK="$SCRIPTS/hooks/session-bootstrap.sh"
}

teardown() {
	mkit_teardown_repo
}

has_text() { printf '%s\n' "$1" | grep -qF -- "$2"; }

@test "no arguments reports status rather than installing something" {
	run bash "$INSTALL"
	[ "$status" -eq 0 ]
	has_text "$output" 'prerequisites:'
	has_text "$output" 'session hook:'
	has_text "$output" 'gate ledger:'
}

@test "a status run writes nothing at all into the user dir" {
	run bash "$INSTALL" --status
	[ "$status" -eq 0 ]
	# The whole point of "installs nothing": a diagnostic that leaves state behind is
	# an installer wearing a different name.
	[ ! -f "$TOMBSTONE" ]
	[ ! -f "$STATE" ]
}

@test "--status lists every prerequisite row, including the ok ones" {
	run bash "$INSTALL" --status
	has_text "$output" 'prerequisites:'
	# Assert against the prerequisites BLOCK alone. report_ledger also prints
	# "gate.jsonl" and "usable (jq + sha256 present)", so grepping the whole output for
	# git/jq/sha256 passes even when the table itself printed nothing — the assertion
	# would survive report_prereqs being deleted outright.
	table="$(printf '%s\n' "$output" | sed -n '/^prerequisites:/,/^$/p')"
	has_text "$table" 'git'
	has_text "$table" 'jq'
	has_text "$table" 'node'
	has_text "$table" 'sha256'
	# ...and at least one row actually reads `ok`, which is the claim in the name. Without
	# this the test passes on a table that is all MISSING rows.
	printf '%s\n' "$table" | grep -qE '^  [a-z0-9]+ +ok'
}

@test "--status reports the hook as active before it has been silenced" {
	run bash "$INSTALL" --status
	has_text "$output" 'active'
}

@test "the exit status of a status run is the prerequisite verdict" {
	run bash "$INSTALL" --status
	[ "$status" -eq 0 ]
	# A caller scripting "is this machine ready" branches on exactly this.
	run env PATH="$(mkit_fake_path jq)" bash "$INSTALL" --status
	[ "$status" -eq 1 ]
}

@test "a missing soft prerequisite does not fail the run" {
	run env PATH="$(mkit_fake_path node)" bash "$INSTALL" --status
	[ "$status" -eq 0 ]
	has_text "$output" 'node'
}

@test "--uninstall writes the tombstone" {
	run bash "$INSTALL" --uninstall
	[ "$status" -eq 0 ]
	[ -f "$TOMBSTONE" ]
	has_text "$output" 'silenced'
}

@test "the tombstone is non-empty and says how to undo itself" {
	bash "$INSTALL" --uninstall >/dev/null
	run cat "$TOMBSTONE"
	has_text "$output" 'install.sh'
}

@test "--uninstall drops the record of what has already been said" {
	# Let the hook say its piece and stamp the key, then uninstall.
	env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null" >/dev/null
	[ -f "$STATE" ]
	bash "$INSTALL" --uninstall >/dev/null
	[ ! -f "$STATE" ]
}

@test "--uninstall makes the silence outlive the next session" {
	bash "$INSTALL" --uninstall >/dev/null
	# Even with a genuine gap to report, the tombstone wins.
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "--uninstall --purge removes the tombstone and frees the hook again" {
	bash "$INSTALL" --uninstall >/dev/null
	[ -f "$TOMBSTONE" ]
	run bash "$INSTALL" --uninstall --purge
	[ "$status" -eq 0 ]
	[ ! -f "$TOMBSTONE" ]
	has_text "$output" 'warn again'
	# And it means it: the hook is free to speak.
	run env PATH="$(mkit_fake_path node)" bash -c "'$HOOK' </dev/null"
	[ -n "$output" ]
}

@test "--status reports a silenced hook distinctly from an active one" {
	bash "$INSTALL" --uninstall >/dev/null
	run bash "$INSTALL" --status
	has_text "$output" 'silenced'
}

@test "--status reports the tombstone path so it can be found" {
	bash "$INSTALL" --uninstall >/dev/null
	run bash "$INSTALL" --status
	has_text "$output" 'bootstrap.disabled'
}

@test "the gate ledger is reported as always on, never as something to enable" {
	run bash "$INSTALL" --status
	has_text "$output" 'always on'
	has_text "$output" 'nothing to install'
}

@test "the gate ledger reports degraded when its hash tool is gone" {
	run env PATH="$(mkit_fake_path shasum)" bash "$INSTALL" --status
	has_text "$output" 'degraded'
}

@test "an unknown flag exits 2" {
	run bash "$INSTALL" --nonsense
	[ "$status" -eq 2 ]
}

@test "--help prints the whole header, including the exit contract" {
	run bash "$INSTALL" --help
	[ "$status" -eq 0 ]
	has_text "$output" 'usage: install.sh'
	# The last line of the header is the only statement of the exit contract, and a
	# hard-coded sed range used to drop exactly it. Pin the end of the block, not just
	# the start.
	has_text "$output" 'Exit: 0 ok'
	has_text "$output" 'not make the machine unready'
	[ ! -f "$TOMBSTONE" ]
}

@test "install.sh and session-bootstrap.sh report a gap in the same words" {
	# The anti-drift test, and the reason the prerequisite table lives in lib/common.sh
	# rather than in either caller. Two producers of one sentence is how a diagnostic
	# and the hook that points at it start disagreeing about what is wrong.
	fake="$(mkit_fake_path node)"
	run env PATH="$fake" bash "$INSTALL" --status
	sentence='only findings.mjs (the review skill) needs it'
	has_text "$output" "$sentence"
	run env PATH="$fake" bash -c "'$HOOK' </dev/null"
	has_text "$output" "$sentence"
}
