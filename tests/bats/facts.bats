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

# --- where a write may land ---------------------------------------------------------
# Three boundaries can refuse a write on the author's machine — the OS sandbox, the
# auto-mode classifier, the worktree-isolation guard — and none announces itself. These
# keys turn a mid-run `Operation not permitted` into a starting fact.

@test "tmp= names the ephemeral scratch root" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field tmp)" = "${TMPDIR:-/tmp}" ]
}

@test "run_ignored=no when .mkit/ is not ignored, with its remedy in notes:" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field run_ignored)" = no ]
	[[ "$output" == *"notes:"* ]]
	[[ "$output" == *"run_ignored=no"*"Do not stage while"* ]]
	[[ "$output" == *"info/exclude"* ]]
}

@test "run_ignored=yes once the common-dir exclude carries the rule" {
	mkdir -p "$(git rev-parse --git-common-dir)/info"
	printf '.mkit/*\n!.mkit/config.toml\n' >>"$(git rev-parse --git-common-dir)/info/exclude"
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field run_ignored)" = yes ]
	[[ "$output" != *"Do not stage while"* ]]
}

@test "opening a run directory is what makes .mkit/ ignored" {
	run "$SCRIPTS/facts.sh" commit
	[ "$status" -eq 0 ]
	[ "$(field run_ignored)" = yes ]
	run git status --porcelain
	[ -z "$output" ]
}

# --- the committed config -----------------------------------------------------------
#
# `config_state=absent` is a normal state and never a note: config is an input, never a
# permission (ADR 0001 decision 3). The state worth a sentence is `shadowed`.

@test "config= names the path, and absent is silent" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field config)" = "$MKIT_TMP/.mkit/config.toml" ]
	[ "$(field config_state)" = absent ]
	[[ "$output" != *"config_state=shadowed"* ]]
}

@test "config_state=untracked once the file exists but is not committed" {
	mkdir -p .mkit
	printf 'version = 1\n' >.mkit/config.toml
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field config_state)" = untracked ]
}

@test "config_state=tracked once it is committed" {
	mkdir -p .mkit
	printf 'version = 1\n' >.mkit/config.toml
	git add -f .mkit/config.toml
	git commit -q -m 'mkit config'
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field config_state)" = tracked ]
}

# A repo carrying the legacy directory-only rule: `mkit init` would write a file that
# never reaches a fresh clone, so it is named at the first call with a remedy.
@test "config_state=shadowed under a legacy directory-only rule, with a remedy" {
	printf '.mkit/\n' >.gitignore
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field config_state)" = shadowed ]
	[[ "$output" == *"notes:"* ]]
	[[ "$output" == *"config_state=shadowed"* ]]
	[[ "$output" == *'!.mkit/config.toml'* ]]
	[[ "$output" == *".gitignore"* ]]
}

@test "user_dir is reported, and its writability with it" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field user_dir)" = "$MKIT_HOME" ]
	[ "$(field user_dir_writable)" = yes ]
}

@test "an unwritable user dir is a named fact with a remedy that works" {
	mkdir -p "$MKIT_HOME"
	chmod 500 "$MKIT_HOME"
	run "$SCRIPTS/facts.sh" commit --no-run
	chmod 700 "$MKIT_HOME"
	[ "$status" -eq 0 ]
	[ "$(field user_dir_writable)" = no ]
	[[ "$output" == *"user_dir_writable=no"* ]]
	[[ "$output" == *"permissions.additionalDirectories"* ]]
	# Never the unreachable remedy: a protected path cannot be allowlisted.
	[[ "$output" != *".claude/mkit"* ]]
}

@test "reporting on the user dir does not create it" {
	[ ! -d "$MKIT_HOME" ]
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$(field user_dir_writable)" = yes ]
	[ ! -d "$MKIT_HOME" ]
}

@test "git_bin is an absolute path to the real binary" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	bin="$(field git_bin)"
	[[ "$bin" == /* ]]
	[ -x "$bin" ]
}

@test "mkit is reported, never compared" {
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	# Both keys are always present: a skill reads a starting fact, not a lookup that
	# may be missing. Values are raw — no minimum is declared on either side, so there
	# is nothing here that says "too old".
	[ -n "$(field mkit_bin)" ]
	[ -n "$(field mkit)" ]
	# One token, no spaces: several key=value lines pack more than one pair.
	[ "$(field mkit | wc -w | tr -d ' ')" = 1 ]
}

@test "mkit off PATH is a starting fact, not a failure" {
	PATH="$(mkit_fake_path mkit)" run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field mkit_bin)" = none ]
	[ "$(field mkit)" = none ]
}

# --- mkit's own scratch is not the user's work ------------------------------------------
#
# The run directory moved *inside* the working directory (ADR 0002), which put mkit's logs
# in range of every enumeration facts.sh performs. `run_ignored=no` — an isolated session,
# which cannot write the exclude file — is exactly where that bites, and the result is a
# commit or review scope built from mkit's own output.
#
# The `notes:` block legitimately names `.mkit/` when reporting run_ignored=no, so these
# assertions look at the facts above it, never at the whole output.
facts_only() { printf '%s\n' "$output" | sed '/^notes:/,$d'; }

@test "an unignored .mkit/ is not reported as the user's work" {
	mkdir -p .mkit/review-x
	printf 'step output\n' >.mkit/review-x/step.log
	printf '{"step":"test"}\n' >.mkit/gate.jsonl
	# Nothing ignores it here, or the test proves nothing: no .gitignore, no exclude entry.
	run git check-ignore -q .mkit/gate.jsonl
	[ "$status" -ne 0 ]
	[ -n "$(git status --porcelain)" ]

	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	# A tree holding nothing but mkit's scratch is a clean tree, so the scope keys are
	# absent for the same reason they are on a fresh checkout — not merely empty.
	[ "$(field clean)" = yes ]
	[ "$(field untracked)" = 0 ]
	[[ "$output" != *"status:"* ]]
	[[ "$output" != *"untracked_file_list"* ]]
	case "$(facts_only)" in *.mkit/review-x* | *gate.jsonl*) return 1 ;; esac
}

@test "excluding .mkit/ does not hide the user's own untracked files" {
	# The narrowness matters as much as the exclusion: the bug this guards against would
	# be re-introduced just as badly by dropping untracked reporting altogether.
	mkdir -p .mkit/review-x
	printf 'step output\n' >.mkit/review-x/step.log
	printf 'real work\n' >new-feature.txt
	run "$SCRIPTS/facts.sh" commit --no-run
	[ "$status" -eq 0 ]
	[ "$(field clean)" = no ]
	[ "$(field untracked)" = 1 ]
	[ "$(field untracked_files)" = 1 ]
	[[ "$output" == *"new-feature.txt"* ]]
	case "$(facts_only)" in *.mkit/review-x* | *gate.jsonl*) return 1 ;; esac
}
