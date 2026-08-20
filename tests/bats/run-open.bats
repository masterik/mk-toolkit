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
