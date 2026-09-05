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
	[ -f "$output/.claude-plugin/plugin.json" ]
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

# --- the gate ledger -----------------------------------------------------------------

fp() { bash -c "$(src)mkit_tree_fingerprint"; }

@test "mkit_tree_fingerprint is invariant across committing the same content" {
	printf 'seed changed\n' >seed.txt
	printf 'brand new\n' >added.txt
	dirty="$(fp)"
	[ -n "$dirty" ]
	git add -A
	git commit -q -m 'land it'
	clean="$(fp)"
	[ "$dirty" = "$clean" ]
}

@test "mkit_tree_fingerprint is invariant across staging" {
	printf 'seed changed\n' >seed.txt
	printf 'brand new\n' >added.txt
	unstaged="$(fp)"
	git add -A
	staged="$(fp)"
	[ "$unstaged" = "$staged" ]
}

@test "mkit_tree_fingerprint changes when file content changes" {
	before="$(fp)"
	printf 'seed changed\n' >seed.txt
	after="$(fp)"
	[ "$before" != "$after" ]
}

@test "mkit_tree_fingerprint sees a deletion and survives committing it" {
	printf 'extra\n' >extra.txt
	git add extra.txt
	git commit -q -m 'extra'
	before="$(fp)"
	rm seed.txt
	deleted="$(fp)"
	[ "$before" != "$deleted" ]
	git add -A
	git commit -q -m 'drop seed'
	committed="$(fp)"
	[ "$deleted" = "$committed" ]
}

@test "mkit_tree_fingerprint reads a rename without desynchronizing" {
	printf 'movable\n' >a.txt
	git add a.txt
	git commit -q -m 'a'
	before="$(fp)"
	git mv a.txt b.txt
	renamed="$(fp)"
	[ "$before" != "$renamed" ]
	git commit -q -m 'rename a to b'
	committed="$(fp)"
	[ "$renamed" = "$committed" ]
}

@test "mkit_tree_fingerprint counts an untracked file and survives committing it" {
	before="$(fp)"
	printf 'untracked\n' >new.txt
	untracked="$(fp)"
	[ "$before" != "$untracked" ]
	git add -A
	git commit -q -m 'track it'
	committed="$(fp)"
	[ "$untracked" = "$committed" ]
}

@test "mkit_tree_fingerprint never sees an ignored file" {
	printf 'ignored.txt\n' >.gitignore
	git add .gitignore
	git commit -q -m 'ignore'
	before="$(fp)"
	printf 'noise\n' >ignored.txt
	created="$(fp)"
	[ "$before" = "$created" ]
	printf 'more noise\n' >ignored.txt
	modified="$(fp)"
	[ "$before" = "$modified" ]
}

@test "mkit_tree_fingerprint handles a path containing spaces" {
	printf 'spaced\n' >"a file.txt"
	git add "a file.txt"
	git commit -q -m 'spaced path'
	before="$(fp)"
	printf 'spaced again\n' >"a file.txt"
	dirty="$(fp)"
	[ "$before" != "$dirty" ]
	git add -A
	git commit -q -m 'land spaced path'
	committed="$(fp)"
	[ "$dirty" = "$committed" ]
}

@test "mkit_tree_fingerprint works in a repo with no HEAD" {
	empty="$(mktemp -d "${TMPDIR:-/tmp}/mkit-empty.XXXXXX")"
	empty="$(cd "$empty" && pwd -P)"
	cd "$empty"
	git init -q -b main .
	printf 'only file\n' >only.txt
	run bash -c "$(src)mkit_tree_fingerprint"
	[ "$status" -eq 0 ]
	[[ "$output" =~ ^[0-9a-f]{16}$ ]]
	first="$output"
	printf 'only file changed\n' >only.txt
	second="$(fp)"
	[ "$first" != "$second" ]
	cd "$MKIT_TMP"
	rm -rf -- "$empty"
}

@test "mkit_tree_fingerprint cannot see a mode change (documented gap)" {
	before="$(fp)"
	chmod +x seed.txt
	after="$(fp)"
	[ "$before" = "$after" ]
}

@test "mkit_tree_fingerprint prints 16 lowercase hex characters" {
	run bash -c "$(src)mkit_tree_fingerprint"
	[ "$status" -eq 0 ]
	[[ "$output" =~ ^[0-9a-f]{16}$ ]]
}

@test "mkit_tree_fingerprint keeps two contents apart when a tracked file became a directory" {
	# `git hash-object --stdin-paths` aborts on the first path it cannot hash, and the
	# `paste` that pairs shas back onto paths does not notice — every alphabetically-later
	# path took the wrong sha, so two different trees hashed alike. A false `fresh` is the
	# one direction a gate ledger may never be wrong in, so this is the load-bearing case.
	printf 'x\n' >foo
	printf 'A\n' >z.txt
	git add -A
	git commit -q -m 'foo and z'
	base="$(fp)"
	rm foo
	mkdir foo
	printf 'inner\n' >foo/inner
	printf 'CONTENT-A\n' >z.txt
	a="$(fp)"
	printf 'CONTENT-B\n' >z.txt
	b="$(fp)"
	[ -n "$a" ]
	[ "$a" != "$b" ]
	[ "$a" != "$base" ]
}

@test "mkit_tree_fingerprint hashes a symlink as its target path, not the target's content" {
	printf 'target\n' >t.txt
	ln -s t.txt link
	git add -A
	git commit -q -m 'link'
	before="$(fp)"
	# Editing what the link points AT moves the fingerprint through t.txt, and committing
	# that leaves the link itself untouched — which is only true if the link was hashed as
	# its target path. `hash-object` would have followed it and read the new content, so a
	# repo with any tracked symlink would have read `drifted` forever.
	printf 'edited\n' >t.txt
	git add -A
	git commit -q -m 'edit target'
	after_edit="$(fp)"
	[ "$before" != "$after_edit" ]
	ln -sfn other.txt link
	retargeted="$(fp)"
	[ "$after_edit" != "$retargeted" ]
}

@test "mkit_tree_fingerprint survives an unhashable path in the dirty set" {
	printf 'a\n' >a.txt
	git add -A
	git commit -q -m 'a'
	mkfifo pipe
	printf 'more\n' >>a.txt
	out="$(fp)"
	[ -n "$out" ]
	[[ "$out" =~ ^[0-9a-f]{16}$ ]]
}

@test "mkit_age_human renders seconds, minutes, hours and days" {
	run bash -c "$(src)mkit_age_human 45"
	[ "$output" = "45s" ]
	run bash -c "$(src)mkit_age_human 360"
	[ "$output" = "6m" ]
	run bash -c "$(src)mkit_age_human 7200"
	[ "$output" = "2h" ]
	run bash -c "$(src)mkit_age_human 200000"
	[ "$output" = "2d" ]
}

@test "mkit_age_human renders a non-numeric age as a question mark" {
	run bash -c "$(src)mkit_age_human abc"
	[ "$status" -eq 0 ]
	[ "$output" = "?" ]
}

@test "mkit_gate_ledger_path points at gate.jsonl in the repo mkit dir" {
	run bash -c "$(src)mkit_gate_ledger_path"
	[ "$status" -eq 0 ]
	[ "$output" = "$MKIT_TMP/.git/mkit/gate.jsonl" ]
}

@test "mkit_gate_ledger_path fails quietly outside a repo" {
	cd "$MKIT_TMP/.."
	run bash -c "$(src)mkit_gate_ledger_path"
	[ "$status" -eq 1 ]
	[ -z "$output" ]
}

# --- mkit_json_escape ------------------------------------------------------------------
#
# Direct unit tests, because nothing else covers this any more. It used to be exercised
# end-to-end by the SessionStart hook's "weird user dir" test, back when the payload
# interpolated $MKIT_HOME; the payload now carries only fixed sentences, so that test can
# no longer reach the escape. The function is still the hook's only route to valid JSON
# without jq, so it needs coverage of its own rather than coverage by side effect.

@test "mkit_json_escape escapes a double quote" {
	run bash -c "$(src)printf '%s' 'a\"b' | mkit_json_escape"
	[ "$status" -eq 0 ]
	[ "$output" = 'a\"b' ]
}

@test "mkit_json_escape escapes a backslash" {
	run bash -c "$(src)printf '%s' 'a\\b' | mkit_json_escape"
	[ "$status" -eq 0 ]
	[ "$output" = 'a\\b' ]
}

@test "mkit_json_escape turns a newline into an escape, not a raw break" {
	run bash -c "$(src)printf 'a\nb' | mkit_json_escape"
	[ "$status" -eq 0 ]
	# One line out: a raw newline inside a JSON string is a parse error, and the hook
	# emits its whole payload on a single line.
	[ "$(printf '%s\n' "$output" | wc -l | tr -d ' ')" = 1 ]
	[ "$output" = 'a\nb' ]
}

@test "mkit_json_escape escapes a tab" {
	run bash -c "$(src)printf 'a\tb' | mkit_json_escape"
	[ "$output" = 'a\tb' ]
}

@test "mkit_json_escape output is usable as a JSON string body" {
	# The property that actually matters, and the one the hook depends on: whatever goes
	# in, wrapping the result in quotes yields a document jq parses and reads back byte
	# for byte.
	run bash -c "$(src)printf '%s' 'quote \" back \\ end' | mkit_json_escape"
	[ "$status" -eq 0 ]
	esc="$output"
	run bash -c "printf '{\"m\":\"%s\"}' '$esc' | jq -r .m"
	[ "$status" -eq 0 ]
	[ "$output" = 'quote " back \ end' ]
}

@test "mkit_have_hash finds a sha256 tool" {
	run bash -c "$(src)mkit_have_hash"
	[ "$status" -eq 0 ]
}
