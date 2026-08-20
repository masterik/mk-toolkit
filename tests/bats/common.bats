#!/usr/bin/env bats
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

src() { printf '. "%s/lib/common.sh"; ' "$SCRIPTS"; }

@test "mkit_check_slug accepts a plain name" {
	run bash -c "$(src)mkit_check_slug commit"
	[ "$status" -eq 0 ]
}

@test "mkit_check_slug rejects a space" {
	run bash -c "$(src)mkit_check_slug 'bad name'"
	[ "$status" -eq 2 ]
	[[ "$output" == *"may only contain"* ]]
}

@test "mkit_check_slug rejects a path traversal" {
	run bash -c "$(src)mkit_check_slug '../etc'"
	[ "$status" -eq 2 ]
}

@test "mkit_check_slug rejects empty" {
	run bash -c "$(src)mkit_check_slug ''"
	[ "$status" -eq 2 ]
	[[ "$output" == *"empty name"* ]]
}

@test "mkit_require_repo passes inside a repo" {
	run bash -c "$(src)mkit_require_repo"
	[ "$status" -eq 0 ]
}

@test "mkit_require_repo fails outside a repo" {
	cd "$MKIT_TMP/.."
	run bash -c "$(src)mkit_require_repo"
	[ "$status" -eq 1 ]
	[[ "$output" == *"not inside a git repository"* ]]
}

@test "mkit_plugin_root resolves to the checkout root" {
	run bash -c "$(src)mkit_plugin_root"
	[ "$status" -eq 0 ]
	[ -f "$output/PREREQUISITES.md" ]
}

@test "mkit_refs_dir points at skills/_shared/references" {
	run bash -c "$(src)mkit_refs_dir"
	[ "$status" -eq 0 ]
	[[ "$output" == */skills/_shared/references ]]
	[ -d "$output" ]
}

@test "mkit_search finds a match with rg or grep fallback" {
	printf 'line one\nTARGET here\nline three\n' >haystack.txt
	run bash -c "$(src)mkit_search 5 TARGET haystack.txt"
	[ "$status" -eq 0 ]
	[[ "$output" == *"TARGET here"* ]]
}

@test "mkit_search respects the max-count cap" {
	printf 'TARGET\nTARGET\nTARGET\n' >haystack.txt
	run bash -c "$(src)mkit_search 1 TARGET haystack.txt"
	[ "$status" -eq 0 ]
	[ "$(printf '%s\n' "$output" | grep -c TARGET)" -eq 1 ]
}
