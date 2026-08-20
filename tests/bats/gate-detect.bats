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
