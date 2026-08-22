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

# --- the gate ledger ---------------------------------------------------------------

ledger() { printf '%s\n' "$MKIT_TMP/.git/mkit/gate.jsonl"; }

# Append <count> synthetic records carrying <head>, cheaply — a jq call per line would
# dominate the runtime of the rotation tests.
seed_ledger() {
	local i
	mkdir -p "$(dirname -- "$(ledger)")"
	for ((i = 0; i < $1; i++)); do
		printf '{"kind":"gate","ts":"2026-01-01T00:00:00Z","epoch":1767225600,'
		printf '"fingerprint":"seed%s","step":"seed","cmd":"true","exit":0,"secs":0,' "$i"
		printf '"head":"%s","branch":"main","skill":"seed","log":"/dev/null"}\n' "$2"
	done >>"$(ledger)"
}

@test "a finished step appends one ledger record" {
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	[ -f "$(ledger)" ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 1 ]
	[ "$(jq -r '.kind' "$(ledger)")" = "gate" ]
	[ "$(jq -r '.step' "$(ledger)")" = "lint" ]
	[ "$(jq -r '.exit' "$(ledger)")" = "0" ]
	[ -n "$(jq -r '.fingerprint' "$(ledger)")" ]
	[ "$(jq -r '.fingerprint' "$(ledger)")" != "null" ]
}

@test "a failing step records its exit code" {
	run "$SCRIPTS/gate-run.sh" "$RUN" test -- bash -c 'exit 7'
	[ "$status" -eq 7 ]
	[ "$(jq -r '.exit' "$(ledger)")" = "7" ]
	[ "$(jq -r '.step' "$(ledger)")" = "test" ]
}

@test "a chain that stops leaves no record for the steps it never ran" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --chain 'a=true' 'b=false' 'c=true'
	[ "$status" -ne 0 ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 2 ]
	[ "$(jq -r '.step' "$(ledger)" | tr '\n' ' ')" = "a b " ]
	! jq -r '.step' "$(ledger)" | grep -qx c
}

@test "--keep-going records every step" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --keep-going --chain 'a=true' 'b=exit 3' 'c=true'
	[ "$status" -eq 3 ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 3 ]
	[ "$(jq -r '.step' "$(ledger)" | tr '\n' ' ')" = "a b c " ]
}

# The flagship cross-skill lookup is keyed on `cmd`: `commit`/`review` call the single-step
# form and `finish`/`pr` call --chain, so if the two forms disagree the cache never hits.
@test "both call forms record the same normalized command" {
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- echo hello world
	[ "$status" -eq 0 ]
	run "$SCRIPTS/gate-run.sh" "$RUN" --chain 'lint=echo hello world'
	[ "$status" -eq 0 ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 2 ]
	single="$(jq -r '.cmd' "$(ledger)" | sed -n 1p)"
	chained="$(jq -r '.cmd' "$(ledger)" | sed -n 2p)"
	[ "$single" = "echo hello world" ]
	[ "$chained" = "echo hello world" ]
	[ "$single" = "$chained" ]
}

@test "an argument containing a space records the quoted form, not the ambiguous join" {
	# Joining argv on single spaces is lossy: these two commands execute differently but
	# would join to the same string, and the second could then be served the first's
	# proof. The join is what makes the cross-skill lookup hit, so it stays — but only
	# where every argument survives the round trip.
	"$SCRIPTS/gate-run.sh" "$RUN" a -- printf '[%s]' 'foo bar'
	"$SCRIPTS/gate-run.sh" "$RUN" b -- printf '[%s]' foo bar
	with_space="$(jq -r 'select(.step == "a") | .cmd' "$(ledger)")"
	without="$(jq -r 'select(.step == "b") | .cmd' "$(ledger)")"
	[ "$with_space" != "$without" ]
	[ "$without" = "printf [%s] foo bar" ]
	[[ "$with_space" == *"'foo bar'"* ]]
}

@test "the ledger changes neither the exit code nor a single byte of the output" {
	run "$SCRIPTS/gate-run.sh" "$RUN" test -- bash -c 'echo boom; exit 7'
	ledger_status="$status"
	ledger_output="$output"
	run "$SCRIPTS/gate-run.sh" "$RUN" --no-ledger test -- bash -c 'echo boom; exit 7'
	[ "$status" -eq "$ledger_status" ]
	[ "$output" = "$ledger_output" ]
	# The argv-flattening guarantee is unaffected by the normalized copy of the command.
	run "$SCRIPTS/gate-run.sh" "$RUN" step -- printf '[%s]\n' 'foo bar'
	[ "$status" -eq 0 ]
	[ "$(wc -l <"$RUN/gate-step.log" | tr -d ' ')" -eq 1 ]
	grep -qF '[foo bar]' "$RUN/gate-step.log"
}

@test "--no-ledger writes no ledger at all" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --no-ledger lint -- true
	[ "$status" -eq 0 ]
	[ ! -f "$(ledger)" ]
}

@test "an unwritable ledger does not fail the gate" {
	[ "$(id -u)" -eq 0 ] && skip "root ignores mode bits"
	chmod 500 "$MKIT_TMP/.git/mkit"
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	chmod 700 "$MKIT_TMP/.git/mkit"
	[ "$status" -eq 0 ]
	[[ "$output" == *"lint ok"* ]]
	[[ "$output" == *"gate=ok steps=lint"* ]]
	[ ! -f "$(ledger)" ]
}

@test "the skill is derived from the run-dir basename" {
	REVIEW_RUN="$MKIT_TMP/.git/mkit/review-20260101T000000Z-aaaaaa"
	mkdir -p "$REVIEW_RUN"
	run "$SCRIPTS/gate-run.sh" "$REVIEW_RUN" lint -- true
	[ "$status" -eq 0 ]
	[ "$(jq -r '.skill' "$(ledger)")" = "review" ]
}

@test "the record names the log it wrote" {
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	log="$(jq -r '.log' "$(ledger)")"
	[ "$log" = "$RUN/gate-lint.log" ]
	[ -f "$log" ]
}

# Otherwise the second record claims proof over content that step never read.
@test "the fingerprint is computed once per invocation, not once per step" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --chain 'a=bash -c "echo x > newfile.txt"' 'b=true'
	[ "$status" -eq 0 ]
	[ -f "$MKIT_TMP/newfile.txt" ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 2 ]
	first="$(jq -r '.fingerprint' "$(ledger)" | sed -n 1p)"
	second="$(jq -r '.fingerprint' "$(ledger)" | sed -n 2p)"
	[ -n "$first" ]
	[ "$first" = "$second" ]
}

@test "rotation bounds the ledger to exactly the newest 200 and keeps the record just written" {
	# An upper bound alone would also pass if trim emptied the file, which is the failure
	# worth catching here: 450 seeded + 1 appended trims to exactly 200, newest kept.
	seed_ledger 450 "$(git rev-parse HEAD)"
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 200 ]
	jq -e . "$(ledger)" >/dev/null
	[ "$(jq -r 'select(.step == "lint" and .cmd == "true") | .step' "$(ledger)")" = lint ]
}

@test "rotation leaves the ledger alone when jq cannot read every line" {
	# One malformed line used to make jq stop there; paste then paired the heads it had
	# printed with the wrong records and the rewrite kept a handful of the oldest.
	seed_ledger 450 "$(git rev-parse HEAD)"
	printf 'not json at all\n' >>"$(ledger)"
	before="$(wc -l <"$(ledger)" | tr -d ' ')"
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq "$((before + 1))" ]
}

@test "rotation breaks a lock left behind by a killed trim" {
	seed_ledger 450 "$(git rev-parse HEAD)"
	mkdir "$(ledger).lock"
	touch -t 202601010000 "$(ledger).lock"
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 200 ]
	[ ! -d "$(ledger).lock" ]
}

@test "rotation defers to a lock a live trim could still be holding" {
	seed_ledger 450 "$(git rev-parse HEAD)"
	mkdir "$(ledger).lock"
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	[ "$(wc -l <"$(ledger)" | tr -d ' ')" -eq 451 ]
	[ -d "$(ledger).lock" ]
	rmdir "$(ledger).lock"
}

@test "rotation drops records whose head no longer resolves first" {
	bogus=0123456789abcdef0123456789abcdef01234567
	! git cat-file -e "$bogus^{commit}" 2>/dev/null
	seed_ledger 450 "$bogus"
	seed_ledger 2 "$(git rev-parse HEAD)"
	run "$SCRIPTS/gate-run.sh" "$RUN" lint -- true
	[ "$status" -eq 0 ]
	! grep -q "$bogus" "$(ledger)"
	[ "$(grep -c '"step":"seed"' "$(ledger)")" -eq 2 ]
	[ "$(jq -r 'select(.step == "lint") | .step' "$(ledger)")" = "lint" ]
}

@test "the ledger is JSONL" {
	run "$SCRIPTS/gate-run.sh" "$RUN" --keep-going --chain 'a=true' 'b=exit 1' 'c=echo "quo\"ted"'
	[ -s "$(ledger)" ]
	jq -e . "$(ledger)" >/dev/null
	while IFS= read -r line; do
		printf '%s' "$line" | jq -e . >/dev/null
	done <"$(ledger)"
}
