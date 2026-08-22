#!/usr/bin/env bats
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

# Some lines pack several key=value pairs separated by spaces (e.g.
# `staged=1 unstaged=0 untracked=0 conflicted=0`) — split on whitespace too,
# not just newlines, before matching the key.
field() { printf '%s\n' "$output" | tr ' ' '\n' | sed -n "s/^$1=//p" | head -1; }

@test "reports a clean tree on a fresh checkout" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field clean)" = yes ]
	[ "$(field branch)" = main ]
	[ "$(field detached)" = no ]
}

@test "opens a run directory unless --no-run is passed" {
	run "$SCRIPTS/facts.sh" commit
	[ "$status" -eq 0 ]
	run_line="$(printf '%s\n' "$output" | sed -n 's/^run=//p')"
	[ -d "$run_line" ]
}

@test "--no-run prints no run= line" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[[ "$output" != *"run="* ]]
}

@test "distinguishes staged, unstaged, and untracked changes" {
	printf 'staged\n' >staged.txt
	git add staged.txt
	printf 'unstaged\n' >>seed.txt
	printf 'x\n' >untracked.txt
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field clean)" = no ]
	[ "$(field staged)" = 1 ]
	[ "$(field unstaged)" = 1 ]
	[ "$(field untracked)" = 1 ]
}

@test "a fully staged tree is not reported as clean (the shortstat blind spot)" {
	printf 'more\n' >>seed.txt
	git add seed.txt
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$(field clean)" = no ]
	[[ "$output" == *"staged_stat="* ]]
}

@test "untracked files are listed under their own key, not folded into the diff" {
	printf 'x\n' >new-file.txt
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$(field untracked_files)" = 1 ]
	[[ "$output" == *"untracked_file_list:"*"new-file.txt"* ]]
	[[ "$output" != *"unstaged_file_list:"*"new-file.txt"* ]]
}

@test "an unresolvable --base fails loudly and says so on stdout" {
	run "$SCRIPTS/facts.sh" commit --no-run --base does-not-exist
	[ "$status" -eq 1 ]
	[[ "$output" == *"base_state=unresolvable"* ]]
}

@test "--base with commits ahead reports the count and the log" {
	git checkout -q -b feature
	printf 'more\n' >>seed.txt
	git add seed.txt
	git commit -q -m 'feature commit'
	run "$SCRIPTS/facts.sh" commit --no-run --base main
	[ "$status" -eq 0 ]
	[ "$(field base_state)" = ok ]
	[ "$(field commits_ahead_of_base)" = 1 ]
	[[ "$output" == *"feature commit"* ]]
}

@test "rejects a skill name with a space" {
	run "$SCRIPTS/facts.sh" 'not a skill' --no-run
	[ "$status" -eq 2 ]
}

@test "usage error with no skill argument" {
	run "$SCRIPTS/facts.sh"
	[ "$status" -eq 2 ]
}

@test "fails outside a git repository" {
	cd "$MKIT_TMP/.."
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 1 ]
}

@test "detached HEAD is reported without a branch name" {
	git checkout -q --detach HEAD
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$(field detached)" = yes ]
}

@test "no gh on PATH reports pr=gh-missing under --gh" {
	binfake="$BATS_TEST_TMPDIR/no-gh-bin"
	mkdir -p "$binfake"
	for t in bash git sed awk grep cat head tail wc tr find sort mktemp touch \
		date basename dirname xargs jq; do
		p="$(command -v "$t" 2>/dev/null)" && ln -sf "$p" "$binfake/$t"
	done
	run env PATH="$binfake" "$SCRIPTS/facts.sh" pr --no-run --gh
	[ "$status" -eq 0 ]
	[ "$(field pr)" = gh-missing ]
}

# --- the commit journal ------------------------------------------------------------
# A tab-indented line inside the `journal:` block. Built with printf, not written into
# the pattern, so an editor stripping tabs from this file cannot silently pass the test.
journal_line() { printf '%s\n' "$output" | grep -qxF "$(printf '\t%s' "$1")"; }

@test "without --journal there is no journal= line at all" {
	"$SCRIPTS/journal.sh" enable
	printf 'more\n' >>seed.txt
	"$SCRIPTS/journal.sh" add --paths seed.txt --type feat --scope core \
		--subject 'extend the seed' --why 'the test needs a record'
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[[ "$output" != *"journal="* ]]
	[[ "$output" != *"journal:"* ]]
}

@test "--journal reports journal=off when journaling was never enabled" {
	run "$SCRIPTS/facts.sh" commit --no-run --journal
	[ "$status" -eq 0 ]
	[ "$(field journal)" = off ]
	[[ "$output" != *"journal:"* ]]
}

@test "--journal reports journal=off after disable, which removes the marker" {
	"$SCRIPTS/journal.sh" enable
	"$SCRIPTS/journal.sh" disable
	run "$SCRIPTS/facts.sh" commit --no-run --journal
	[ "$status" -eq 0 ]
	[ "$(field journal)" = off ]
}

@test "--journal reports journal=empty when enabled with no records" {
	"$SCRIPTS/journal.sh" enable
	printf 'more\n' >>seed.txt
	run "$SCRIPTS/facts.sh" commit --no-run --journal
	[ "$status" -eq 0 ]
	[ "$(field journal)" = empty ]
	[[ "$output" != *"journal:"* ]]
}

@test "--journal reports journal=on and a nested block once a record exists" {
	"$SCRIPTS/journal.sh" enable
	printf 'more\n' >>seed.txt
	"$SCRIPTS/journal.sh" add --paths seed.txt --type feat --scope core \
		--subject 'extend the seed' --why 'coverage arithmetic needs a claimed path'
	run "$SCRIPTS/facts.sh" commit --no-run --journal
	[ "$status" -eq 0 ]
	[ "$(field journal)" = on ]
	[[ "$output" == *"journal:"* ]]
	journal_line 'journal_entries=1'
	journal_line 'journal_covered=1 journal_uncovered=0'
	[[ "$output" == *"why=coverage arithmetic needs a claimed path"* ]]
}

@test "the journal block is indented, so its keys are not top-level facts" {
	"$SCRIPTS/journal.sh" enable
	printf 'more\n' >>seed.txt
	"$SCRIPTS/journal.sh" add --paths seed.txt --type feat --scope core \
		--subject 'extend the seed' --why 'indentation guard'
	run "$SCRIPTS/facts.sh" commit --no-run --journal
	# `field` only sees column 0: a consumer parsing key=value lines must not pick up
	# a key that belongs to the nested block.
	[ -z "$(field journal_entries)" ]
	! printf '%s\n' "$output" | grep -q '^journal_entries='
}

@test "--journal reports journal=jq-missing when records exist but jq does not" {
	"$SCRIPTS/journal.sh" enable
	printf 'more\n' >>seed.txt
	"$SCRIPTS/journal.sh" add --paths seed.txt --type feat --scope core \
		--subject 'extend the seed' --why 'records exist and cannot be read'
	binfake="$BATS_TEST_TMPDIR/no-jq-bin"
	mkdir -p "$binfake"
	for t in bash git sed awk grep cat head tail wc tr find sort mktemp touch \
		date basename dirname xargs; do
		p="$(command -v "$t" 2>/dev/null)" && ln -sf "$p" "$binfake/$t"
	done
	run env PATH="$binfake" "$SCRIPTS/facts.sh" commit --no-run --journal
	# Distinct from `empty`: emptiness is a stat, so it still answers with no jq, and
	# only a journal that has something in it can report this.
	[ "$status" -eq 0 ]
	[ "$(field journal)" = jq-missing ]
	[[ "$output" != *"journal:"* ]]
}

@test "a journal jq cannot parse reports journal=unreadable, not a failed fact set" {
	"$SCRIPTS/journal.sh" enable
	printf 'more\n' >>seed.txt
	printf 'not json at all\n' >"$(git rev-parse --absolute-git-dir)/mkit/journal.jsonl"
	run "$SCRIPTS/facts.sh" commit --no-run --journal
	# The journal is an optimization; every other key here is not. A corrupt file must
	# not take `commit`'s one starting call down with it.
	[ "$status" -eq 0 ]
	[ "$(field journal)" = unreadable ]
	[ "$(field clean)" = no ]
}
