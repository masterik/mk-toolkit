#!/usr/bin/env bats
load helpers.bash

setup() {
	mkit_setup_repo
	RUN="$MKIT_TMP/.git/mkit/run1"
	mkdir -p "$RUN"
}
teardown() { mkit_teardown_repo; }

@test "a passing step prints ok and exits 0" {
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	[[ "$output" == *"lint ok"* ]]
	[[ "$output" == *"gate=ok steps=lint"* ]]
}

@test "a failing step exits with the command's own code and logs full output" {
	run "$SCRIPTS/gate-run.sh" "$RUN" test -- bash -c 'echo boom; exit 7'
	[ "$status" -eq 7 ]
	[[ "$output" == *"test FAIL exit=7"* ]]
	[[ "$output" == *"gate=FAILED step=test"* ]]
	[ -f "$RUN/gate-test.log" ]
	grep -q boom "$RUN/gate-test.log"
}

@test "a failure line is surfaced by the FAIL-pattern grep" {
	run "$SCRIPTS/gate-run.sh" "$RUN" test -- bash -c 'echo "AssertionError: nope"; exit 1'
	[ "$status" -eq 1 ]
	[[ "$output" == *"AssertionError: nope"* ]]
}

@test "an empty command is rejected rather than passing by running nothing" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --chain 'lint='
	[ "$status" -eq 2 ]
	[[ "$output" == *"empty command"* ]]
}

@test "--chain stops at the first failure and does not run later steps" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --chain 'a=true' 'b=exit 1' 'c=touch should-not-run'
	[ "$status" -eq 1 ]
	[[ "$output" == *"a ok"* ]]
	[[ "$output" == *"b FAIL"* ]]
	[[ "$output" != *"c "* ]]
	[ ! -f "$RUN/should-not-run" ]
}

@test "--chain --keep-going runs every step and reports the first failure" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --keep-going --chain 'a=true' 'b=exit 3' 'c=true'
	[ "$status" -eq 3 ]
	[[ "$output" == *"a ok"* ]]
	[[ "$output" == *"b FAIL"* ]]
	[[ "$output" == *"c ok"* ]]
	[[ "$output" == *"gate=FAILED step=b exit=3"* ]]
}

@test "a command argument with an embedded space survives quoting" {
	run "$SCRIPTS/gate-run.sh" "$RUN" step -- printf '[%s]\n' 'foo bar'
	[ "$status" -eq 0 ]
	grep -qF '[foo bar]' "$RUN/gate-step.log"
}

@test "a command argument with an embedded single quote survives quoting" {
	run "$SCRIPTS/gate-run.sh" "$RUN" step -- printf '[%s]\n' "it's fine"
	[ "$status" -eq 0 ]
	grep -qF "[it's fine]" "$RUN/gate-step.log"
}

@test "--tail bounds the printed tail on failure" {
	cmd='for i in $(seq 1 50); do echo "line $i"; done; exit 1'
	run "$SCRIPTS/gate-run.sh" "$RUN" --tail 3 test -- bash -c "$cmd"
	[ "$status" -eq 1 ]
	[[ "$output" == *"tail -3:"* ]]
	[[ "$output" == *"line 50"* ]]
	[[ "$output" != *"line 47"* ]]
}

@test "rejects a run directory that does not exist" {
	run "$SCRIPTS/gate-run.sh" "$MKIT_TMP/no-such-dir" lint -- true
	[ "$status" -eq 2 ]
	[[ "$output" == *"open it with run-open.sh"* ]]
}

@test "rejects a step name that is not a bare slug" {
	run "$SCRIPTS/gate-run.sh" "$RUN" 'bad step' -- true
	[ "$status" -eq 2 ]
}
