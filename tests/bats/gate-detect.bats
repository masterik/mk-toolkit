#!/usr/bin/env bats
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

field() { printf '%s\n' "$output" | sed -n "s/^$1=//p" | head -1; }

@test "no recognized manifest reports ecosystem=none fast=none" {
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	[ "$(field ecosystem)" = none ]
	[ "$(field fast)" = none ]
}

@test "node: npm is the default package manager with no lockfile" {
	printf '{"scripts":{"test":"echo t"}}' >package.json
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	[[ "$output" == *"pm=npm"* ]]
	[ "$(field ecosystem)" = node ]
	[ "$(field fast)" = "npm run test" ]
}

@test "node: a lockfile picks the package manager" {
	printf '{"scripts":{"test":"echo t"}}' >package.json
	: >pnpm-lock.yaml
	run "$SCRIPTS/gate-detect.sh"
	[[ "$output" == *"pm=pnpm"* ]]
	[ "$(field fast)" = "pnpm run test" ]
}

@test "node: typecheck outranks test for the fast tier, full runs lint then test then build" {
	printf '{"scripts":{"lint":"x","typecheck":"x","test":"x","build":"x"}}' >package.json
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field fast)" = "npm run typecheck" ]
	[ "$(field full)" = "npm run lint|npm run typecheck|npm run test|npm run build" ]
	[[ "$(field alt_fast)" == *"npm run lint"* ]]
	[[ "$(field alt_fast)" == *"npm run test"* ]]
}

@test "node: a missing jq is reported, not confused with no scripts" {
	printf '{"scripts":{"test":"x"}}' >package.json
	# Some systems keep jq in the same directory as tools the script genuinely
	# needs (e.g. macOS ships /usr/bin/jq next to dirname and sed), so excluding
	# whole PATH directories can take out more than jq. Build a bin/ containing
	# only what gate-detect.sh + common.sh actually call, minus jq.
	binfake="$BATS_TEST_TMPDIR/no-jq-bin"
	mkdir -p "$binfake"
	for t in bash git sed awk grep cat head tail wc tr find sort mktemp touch \
		date basename dirname xargs; do
		p="$(command -v "$t" 2>/dev/null)" && ln -sf "$p" "$binfake/$t"
	done
	run env PATH="$binfake" "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	[ "$(field scripts)" = none ]
	[ "$(field scripts_state)" = no-jq ]
}

@test "node: workspaces are detected" {
	printf '{"scripts":{"test":"x"},"workspaces":["packages/*"]}' >package.json
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field workspaces)" = yes ]
}

@test "rust: Cargo.toml gives clippy as fast and the clippy/test/build full chain" {
	printf '[package]\nname="x"\n' >Cargo.toml
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field ecosystem)" = rust ]
	[ "$(field fast)" = "cargo clippy --all-targets -- -D warnings" ]
	[ "$(field full)" = "cargo clippy --all-targets -- -D warnings|cargo test|cargo build" ]
}

@test "go: go.mod gives go vet as fast" {
	printf 'module x\n' >go.mod
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field ecosystem)" = go ]
	[ "$(field fast)" = "go vet ./..." ]
}

@test "python: pyproject.toml with mypy config adds mypy to the full chain" {
	printf '[tool.mypy]\nstrict = true\n' >pyproject.toml
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field ecosystem)" = python ]
	[[ "$(field full)" == *"mypy ."* ]]
	[[ "$(field full)" == *"pytest -q"* ]]
}

@test "multiple ecosystems in one repo are all reported" {
	printf '{"scripts":{"test":"x"}}' >package.json
	printf 'module x\n' >go.mod
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field ecosystem)" = "node,go" ]
}

@test "make: a documented 'check:' target wins over the inferred fast command" {
	printf '{"scripts":{"test":"x"}}' >package.json
	printf 'check:\n\techo ok\n' >Makefile
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field fast)" = "make check" ]
	[[ "$(field alt_fast)" == *"npm run test"* ]]
}

@test "docs_candidates surfaces a check command named in AGENTS.md" {
	printf '{"scripts":{"test":"x"}}' >package.json
	printf 'Run `npm run test` before every commit.\n' >AGENTS.md
	run "$SCRIPTS/gate-detect.sh"
	[[ "$output" == *"docs_candidates:"* ]]
	[[ "$output" == *"AGENTS.md:"*"npm run test"* ]]
}

@test "docs_candidates=none when nothing in the repo's docs names a check command" {
	run "$SCRIPTS/gate-detect.sh"
	[[ "$output" == *"docs_candidates=none"* ]]
}

@test "--dir only locates the repo; detection still runs from the toplevel" {
	mkdir -p sub
	printf '{"scripts":{"test":"x"}}' >package.json
	# A manifest that exists only inside sub/ must NOT be picked up — gate-detect.sh
	# cds back to the repo root before it looks for anything (facts.sh relies on the
	# same repo-wide behavior for its own diff scope).
	printf '{"scripts":{"build":"x"}}' >sub/package.json
	run "$SCRIPTS/gate-detect.sh" --dir sub
	[ "$(field ecosystem)" = node ]
	[ "$(field fast)" = "npm run test" ]
}

@test "--dir errors on a path that does not exist" {
	run "$SCRIPTS/gate-detect.sh" --dir does-not-exist
	[ "$status" -eq 2 ]
	[[ "$output" == *"cannot enter"* ]]
}

@test "rejects an unknown option" {
	run "$SCRIPTS/gate-detect.sh" --bogus
	[ "$status" -eq 2 ]
}

@test "fails outside a git repository" {
	cd "$MKIT_TMP/.."
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 1 ]
}

# --- the gate ledger ---------------------------------------------------------------
#
# gate-detect.sh annotates the commands it just proposed with what `gate-run.sh` already
# proved over the current content. The two halves must agree on one key — the exact
# command string — so wherever a test only needs a record to exist, it gets one by
# actually running gate-run.sh rather than by writing the file.

# The throwaway repo has no package.json, so every proposal is `none`. This gives the
# reader something to look up: fast=`npm run lint`, full=`npm run lint|npm run test|npm
# run build`, with the lockfile pinning pm=npm.
#   node_repo [lint-script] [build-script]
node_repo() {
	local lint="${1:-true}" build="${2:-true}"
	printf '{"name":"t","scripts":{"lint":"%s","test":"true","build":"%s"}}\n' "$lint" "$build" >package.json
	: >package-lock.json
	git add package.json package-lock.json
	git commit -q -m 'node repo'
}

ledger() { printf '%s\n' "$MKIT_TMP/.git/mkit/gate.jsonl"; }

# The fingerprint a forged record has to carry to classify as anything other than
# `drifted`. Taken from the shared helper rather than from gate-detect's output, because
# gate-detect only prints one once the ledger is non-empty — and the first forged record
# has to be written before there is a ledger at all. That the two agree is what every
# `fresh` below asserts.
fingerprint_now() { (. "$SCRIPTS/lib/common.sh" && mkit_tree_fingerprint); }

# Open a run dir and run one gate step through the real writer.
gate_run() {
	local rd
	rd="$("$SCRIPTS/run-open.sh" gate)"
	"$SCRIPTS/gate-run.sh" "$rd" "$@"
}

# Append one hand-written record. Only for the fields gate-run.sh will never write: a
# head that no longer resolves, a proof from another tree, an epoch hours in the past.
#   forge <cmd> [exit] [epoch] [fingerprint] [head]
forge() {
	local cmd="$1" ex="${2:-0}" epoch="${3:-$(date -u +%s)}"
	local fp="${4:-$(fingerprint_now)}" head="${5:-$(git rev-parse HEAD)}"
	mkdir -p "$(dirname "$(ledger)")"
	jq -cn --arg cmd "$cmd" --argjson exit "$ex" --argjson epoch "$epoch" \
		--arg fingerprint "$fp" --arg head "$head" \
		'{kind:"gate", ts:"forged", epoch:$epoch, fingerprint:$fingerprint, step:"forged",
		  cmd:$cmd, exit:$exit, secs:0, head:$head, branch:"main", skill:"test", log:"-"}' \
		>>"$(ledger)"
}

@test "gate ledger: with no ledger at all the annotation is dropped whole, as gate_cache=empty" {
	node_repo
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	[ "$(field gate_cache)" = empty ]
	# Not one class, and no fingerprint either: there is nothing to compare against, so
	# reporting a fingerprint would invite a caller to compare it itself.
	! printf '%s\n' "$output" | grep -q '^fast_cache='
	! printf '%s\n' "$output" | grep -q '^full_cache'
	! printf '%s\n' "$output" | grep -q '^gate_fingerprint='
}

@test "gate ledger: the age bound is reported beside the fingerprint, not left implicit" {
	node_repo
	gate_run lint -- npm run lint
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	[ "$(field gate_max_age_min)" = 60 ]
	# Not pinned to `age=0s`: under a full-suite run the step can land a second later.
	[[ "$(field fast_cache)" == "fresh exit=0 age="* ]]
}

@test "gate ledger: no ledger reports no age bound either" {
	node_repo
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	[ "$(field gate_cache)" = empty ]
	[ -z "$(field gate_max_age_min)" ]
}

@test "gate ledger: --no-cache reports gate_cache=off and emits no classes" {
	node_repo
	gate_run lint -- npm run lint
	run "$SCRIPTS/gate-detect.sh" --no-cache
	[ "$status" -eq 0 ]
	[ "$(field gate_cache)" = off ]
	! printf '%s\n' "$output" | grep -q '^fast_cache='
	! printf '%s\n' "$output" | grep -q '^gate_fingerprint='
}

@test "gate ledger: --no-cache still reports every non-cache fact" {
	node_repo
	gate_run lint -- npm run lint
	run "$SCRIPTS/gate-detect.sh" --no-cache
	[ "$(field ecosystem)" = node ]
	[[ "$output" == *"pm=npm"* ]]
	[ "$(field fast)" = "npm run lint" ]
	[ "$(field full)" = "npm run lint|npm run test|npm run build" ]
	[ "$(field scripts)" = "build,lint,test" ]
	[ "$(field scripts_state)" = ok ]
}

@test "gate ledger: a step just run through gate-run.sh comes back fresh with its exit and age" {
	node_repo
	gate_run lint -- npm run lint
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	[[ "$(field fast_cache)" == "fresh exit=0 age="* ]]
	[[ "$(field gate_fingerprint)" =~ ^[0-9a-f]{16}$ ]]
}

@test "gate ledger: a proof recorded over a dirty tree survives committing that tree unchanged" {
	# The flagship property, end to end. `review` gates a dirty tree, `finish` commits and
	# gates again; if committing moved the key, the cache would miss exactly when it is
	# meant to pay off, and the feature would save nothing while appearing to work.
	node_repo
	printf 'work in progress\n' >feature.txt
	printf 'edited\n' >>seed.txt
	gate_run lint -- npm run lint
	run "$SCRIPTS/gate-detect.sh"
	[[ "$(field fast_cache)" == "fresh exit=0 age="* ]]
	before="$(field gate_fingerprint)"

	git add -A
	git commit -q -m 'commit exactly the content the gate ran over'

	run "$SCRIPTS/gate-detect.sh"
	[[ "$(field fast_cache)" == "fresh exit=0 age="* ]]
	[ "$(field gate_fingerprint)" = "$before" ]
}

@test "gate ledger: editing a file after a passing run classifies it drifted" {
	node_repo
	gate_run lint -- npm run lint
	printf 'changed after the gate ran\n' >>seed.txt
	run "$SCRIPTS/gate-detect.sh"
	[[ "$(field fast_cache)" == "drifted exit=0 age="* ]]
}

@test "gate ledger: a non-zero step classifies failed, and stays failed past the age bound" {
	node_repo 'exit 1'
	run gate_run lint -- npm run lint
	[ "$status" -eq 1 ]
	run "$SCRIPTS/gate-detect.sh"
	[[ "$(field fast_cache)" == "failed exit=1 age="* ]]

	# `failed` is classified before the age bound on purpose: a tree proven red is worth
	# saying however old the proof is. Only a *pass* expires.
	rm -f "$(ledger)"
	forge 'npm run lint' 1 "$(($(date -u +%s) - 7200))"
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field fast_cache)" = "failed exit=1 age=2h" ]
}

@test "gate ledger: a pass older than the 60-minute bound classifies stale" {
	node_repo
	forge 'npm run lint' 0 "$(($(date -u +%s) - 7200))"
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field fast_cache)" = "stale exit=0 age=2h" ]
}

@test "gate ledger: a record whose head no longer resolves classifies unknown-head" {
	node_repo
	# Fingerprint matches, so only the head can disqualify this record — which is the
	# point: content alone cannot tell you the proof came from a history you still have.
	forge 'npm run lint' 0 "$(date -u +%s)" "$(fingerprint_now)" \
		0000000000000000000000000000000000000dead
	run "$SCRIPTS/gate-detect.sh"
	[[ "$(field fast_cache)" == "unknown-head exit=0 age="* ]]
}

@test "gate ledger: a command with no record classifies none, with '-' in its exit and age slots" {
	node_repo
	gate_run lint -- npm run lint
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field full_cache)" = "fresh|none|none" ]
	[[ "$(field full_cache_exit)" == "0|-|-" ]]
	[[ "$(field full_cache_age)" == *"|-|-" ]]
}

@test "gate ledger: classes line up positionally with full=, they are not sorted or collapsed" {
	node_repo true 'exit 1'
	# The three full steps get three different answers, and only their positions say which
	# is which. A reader that sorted, reversed, deduped or packed them would still print
	# one of each — in the wrong slot. `lint` is never run at all, so slot 1 is `none`.
	gate_run test -- npm run test
	run gate_run build -- npm run build
	[ "$status" -eq 1 ]
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field full)" = "npm run lint|npm run test|npm run build" ]
	[ "$(field full_cache)" = "none|fresh|failed" ]
	[ "$(field full_cache_exit)" = "-|0|1" ]
	[[ "$(field full_cache_age)" == "-|"*"|"* ]]
	[ "$(field fast_cache)" = none ]
}

@test "gate ledger: the key is the exact command, not the step name" {
	node_repo
	forge 'npm run lint'
	# A different check that happens to share a step name. `npm run test --coverage` proves
	# nothing about `npm run test`, so the record must not be found.
	forge 'npm run test --coverage'
	run "$SCRIPTS/gate-detect.sh"
	[ "$(field full_cache)" = "fresh|none|none" ]
}

@test "gate ledger: the newest record for a command wins over an older one" {
	node_repo
	forge 'npm run lint' 0
	forge 'npm run lint' 1
	run "$SCRIPTS/gate-detect.sh"
	[[ "$(field fast_cache)" == "failed exit=1 age="* ]]
}

@test "gate ledger: a malformed line in the ledger does not break detection" {
	node_repo
	gate_run lint -- npm run lint
	printf 'not json at all\n' >>"$(ledger)"
	run "$SCRIPTS/gate-detect.sh"
	[ "$status" -eq 0 ]
	# Detection itself is unaffected; the annotation is what degrades.
	[ "$(field fast)" = "npm run lint" ]
	[ "$(field ecosystem)" = node ]
}
