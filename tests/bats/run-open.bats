#!/usr/bin/env bats
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

@test "opens a run dir under <git-dir>/mkit/<skill>-..." {
	run "$SCRIPTS/run-open.sh" commit
	[ "$status" -eq 0 ]
	[ -d "$output" ]
	[[ "$output" == "$MKIT_TMP/.git/mkit/commit-"* ]]
}

@test "two runs of the same skill get distinct directories" {
	run "$SCRIPTS/run-open.sh" review
	first="$output"
	run "$SCRIPTS/run-open.sh" review
	second="$output"
	[ "$first" != "$second" ]
	[ -d "$first" ]
	[ -d "$second" ]
}

@test "rejects a skill name that is not a bare slug" {
	run "$SCRIPTS/run-open.sh" "not a skill"
	[ "$status" -eq 2 ]
}

@test "fails outside a git repository" {
	cd "$MKIT_TMP/.."
	run "$SCRIPTS/run-open.sh" commit
	[ "$status" -eq 1 ]
	[[ "$output" == *"not inside a git repository"* ]]
}

@test "usage error with no arguments" {
	run "$SCRIPTS/run-open.sh"
	[ "$status" -eq 2 ]
}

@test "--prune rejects a non-numeric count" {
	run "$SCRIPTS/run-open.sh" --prune abc
	[ "$status" -eq 2 ]
	[[ "$output" == *"takes a count"* ]]
}

@test "--prune rejects zero (would evict the caller's own run)" {
	run "$SCRIPTS/run-open.sh" --prune 0
	[ "$status" -eq 2 ]
	[[ "$output" == *"keep at least 1"* ]]
}

@test "--prune with no run directories yet is a no-op" {
	run "$SCRIPTS/run-open.sh" --prune
	[ "$status" -eq 0 ]
	[[ "$output" == "pruned 0 (no run directories)" ]]
}

@test "--prune removes only the oldest dirs beyond keep, per skill, once stale" {
	git_dir="$(git rev-parse --absolute-git-dir)"
	mkit_dir="$git_dir/mkit"
	mkdir -p "$mkit_dir"
	# Five commit run dirs, oldest to newest by name; back-date mtimes past the
	# 60-minute "still active" guard so prune is willing to remove any of them.
	for i in 1 2 3 4 5; do
		d="$mkit_dir/commit-2026010${i}T000000Z-aaaaa$i"
		mkdir -p "$d"
		touch -t 202601010000 "$d"
	done
	# One pr run dir, so pruning commit must not touch it.
	mkdir -p "$mkit_dir/pr-20260101T000000Z-zzzzzz"
	touch -t 202601010000 "$mkit_dir/pr-20260101T000000Z-zzzzzz"

	run "$SCRIPTS/run-open.sh" --prune 2
	[ "$status" -eq 0 ]
	[[ "$output" == "pruned 3 run dir(s), kept 3" ]]

	remaining=$(find "$mkit_dir" -maxdepth 1 -type d -name 'commit-*' | wc -l | tr -d ' ')
	[ "$remaining" -eq 2 ]
	[ -d "$mkit_dir/commit-20260105T000000Z-aaaaa5" ]
	[ -d "$mkit_dir/commit-20260104T000000Z-aaaaa4" ]
	[ ! -d "$mkit_dir/commit-20260101T000000Z-aaaaa1" ]
	[ -d "$mkit_dir/pr-20260101T000000Z-zzzzzz" ]
}

@test "--prune skips a dir beyond keep whose mtime is under an hour old" {
	git_dir="$(git rev-parse --absolute-git-dir)"
	mkit_dir="$git_dir/mkit"
	# rank 1 by name (newest): always kept, mtime irrelevant.
	mkdir -p "$mkit_dir/commit-20260103T000000Z-c"
	# rank 2: beyond keep=1 and backdated — eligible for removal.
	mkdir -p "$mkit_dir/commit-20260102T000000Z-b"
	touch -t 202601020000 "$mkit_dir/commit-20260102T000000Z-b"
	# rank 3: beyond keep=1 but freshly modified — must be skipped, not removed.
	mkdir -p "$mkit_dir/commit-20260101T000000Z-a"

	run "$SCRIPTS/run-open.sh" --prune 1
	[ "$status" -eq 0 ]
	[[ "$output" == "pruned 1 run dir(s), kept 1, skipped 1 still active (<60m)" ]]
	[ -d "$mkit_dir/commit-20260103T000000Z-c" ]
	[ -d "$mkit_dir/commit-20260101T000000Z-a" ]
	[ ! -d "$mkit_dir/commit-20260102T000000Z-b" ]
}

@test "--prune never touches journal.jsonl" {
	git_dir="$(git rev-parse --absolute-git-dir)"
	mkit_dir="$git_dir/mkit"
	mkdir -p "$mkit_dir"
	printf '{"kind":"unit","seq":1}\n' >"$mkit_dir/journal.jsonl"
	# A decoy *file* whose name matches prune's `-name "<skill>-*"` glob. It is what
	# makes this test fail if prune's find ever loses `-type d` — the journal itself
	# would survive that change on its name alone, and the guarantee is too
	# destructive to rest on one filter.
	decoy="$mkit_dir/commit-20260101T000000Z-journal.jsonl"
	printf 'decoy\n' >"$decoy"
	# Three stale run dirs so prune with keep=1 actually removes something.
	for i in 1 2 3; do
		d="$mkit_dir/commit-2026010${i}T000000Z-aaaaa$i"
		mkdir -p "$d"
		touch -t 202601010000 "$d"
	done
	touch -t 202601010000 "$mkit_dir/journal.jsonl" "$decoy"

	run "$SCRIPTS/run-open.sh" --prune 1
	[ "$status" -eq 0 ]
	[[ "$output" == "pruned 2 run dir(s), kept 1" ]]
	[ -f "$mkit_dir/journal.jsonl" ]
	[ "$(cat "$mkit_dir/journal.jsonl")" = '{"kind":"unit","seq":1}' ]
	[ -f "$decoy" ]
}

# Naming the individual files prune must spare (journal.jsonl, gate.jsonl, ...) protects
# only the files that existed when the test was written; the next thing dropped in the
# mkit directory would be unguarded. So the assertion is the general one: prune removes
# `<skill>-*` DIRECTORIES and nothing else, whatever else happens to be in there.
@test "--prune removes only <skill>-* directories and leaves every other entry alone" {
	git_dir="$(git rev-parse --absolute-git-dir)"
	mkit_dir="$git_dir/mkit"
	mkdir -p "$mkit_dir/scratch" "$mkit_dir/random-dir"
	printf '{"kind":"unit","seq":1}\n' >"$mkit_dir/journal.jsonl"
	printf '{"kind":"gate","step":"lint"}\n' >"$mkit_dir/gate.jsonl"
	: >"$mkit_dir/journal.enabled"
	printf 'stray\n' >"$mkit_dir/notes.txt"
	printf 'kept\n' >"$mkit_dir/scratch/inner.txt"
	printf 'kept\n' >"$mkit_dir/random-dir/inner.txt"
	# Three stale run dirs, so prune with keep=1 actually removes something and the
	# survival of everything else is not just prune declining to run.
	for i in 1 2 3; do
		d="$mkit_dir/commit-2026010${i}T000000Z-aaaaa$i"
		mkdir -p "$d"
		touch -t 202601010000 "$d"
	done
	find "$mkit_dir" -maxdepth 1 ! -name mkit -exec touch -t 202601010000 {} +

	run "$SCRIPTS/run-open.sh" --prune 1
	[ "$status" -eq 0 ]
	[[ "$output" == "pruned 2 run dir(s), kept 1" ]]

	# Every entry that is not a <skill>-* directory, still there and still intact.
	[ "$(cat "$mkit_dir/journal.jsonl")" = '{"kind":"unit","seq":1}' ]
	[ "$(cat "$mkit_dir/gate.jsonl")" = '{"kind":"gate","step":"lint"}' ]
	[ -f "$mkit_dir/journal.enabled" ]
	[ "$(cat "$mkit_dir/notes.txt")" = stray ]
	[ "$(cat "$mkit_dir/scratch/inner.txt")" = kept ]
	[ "$(cat "$mkit_dir/random-dir/inner.txt")" = kept ]

	# ...and the general form: nothing outside the `commit-*` set was removed.
	survivors="$(find "$mkit_dir" -maxdepth 1 -mindepth 1 ! -name 'commit-*' | sort)"
	[ "$(printf '%s\n' "$survivors" | wc -l | tr -d ' ')" -eq 6 ]
	[ -d "$mkit_dir/commit-20260103T000000Z-aaaaa3" ]
	[ ! -d "$mkit_dir/commit-20260101T000000Z-aaaaa1" ]
	[ ! -d "$mkit_dir/commit-20260102T000000Z-aaaaa2" ]
}
