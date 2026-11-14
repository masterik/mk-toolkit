#!/usr/bin/env bats
# facts.sh: argv in, stdout and exit status out.
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

# --- the working tree ----------------------------------------------------------------

@test "a fresh checkout is clean, on its branch, with no run dir" {
	run facts commit
	[ "$status" -eq 0 ]
	[ "$(field clean)" = yes ]
	[ "$(field branch)" = main ]
	[ "$(field detached)" = no ]
	[ "$(field toplevel)" = "$REPO" ]
	[[ "$output" != *"run_dir="* ]]
	[[ "$output" != *"status:"* ]]
}

@test "staged, unstaged and untracked are counted apart" {
	printf 'staged\n' >staged.txt
	git add staged.txt
	printf 'unstaged\n' >>seed.txt
	printf 'x\n' >untracked.txt
	run facts commit
	[ "$status" -eq 0 ]
	[ "$(field clean)" = no ]
	[ "$(field staged)" = 1 ]
	[ "$(field unstaged)" = 1 ]
	[ "$(field untracked)" = 1 ]
	[ "$(field conflicted)" = 0 ]
}

# A bare `git diff --shortstat` is empty on a fully staged tree, which reads exactly
# like a clean one.
@test "a fully staged tree is not clean, and its staged stat says so" {
	printf 'more\n' >>seed.txt
	git add seed.txt
	run facts commit
	[ "$(field clean)" = no ]
	[ "$(field unstaged_stat)" = none ]
	[[ "$(field staged_stat)" == 1 ]]
	[[ "$output" == *"staged_stat=1 file changed"* ]]
	[ "$(field staged_files)" = 1 ]
}

# `git diff` never lists an untracked file: a dirty tree of new files reported
# `untracked=6` beside a file list naming none of them.
@test "untracked files only: listed under their own key, not in the diff lists" {
	printf 'x\n' >new-file.txt
	run facts commit
	[ "$(field clean)" = no ]
	[ "$(field untracked_files)" = 1 ]
	[ "$(field unstaged_files)" = 0 ]
	[ "$(field staged_files)" = 0 ]
	[[ "$output" == *$'untracked_file_list:\nnew-file.txt'* ]]
}

@test "lock and snapshot files stay out of the scopes" {
	printf 'x\n' >deps.lock
	printf 'x\n' >real.txt
	run facts commit
	[ "$(field untracked_files)" = 1 ]
	[[ "$(facts_only)" != *"deps.lock"$'\n'"real.txt"* ]]
	[[ "$output" == *$'untracked_file_list:\nreal.txt'* ]]
}

# Called from a subdirectory, the pathspec'd lists covered only that subdirectory while
# the --shortstat beside them stayed repo-wide.
@test "facts are computed from the top level, wherever it is called" {
	mkdir -p sub
	printf 'x\n' >sub/inside.txt
	commit_here sub
	printf 'more\n' >>seed.txt
	cd sub
	run facts commit
	[ "$status" -eq 0 ]
	[ "$(field toplevel)" = "$REPO" ]
	[ "$(field unstaged_files)" = 1 ]
	[[ "$output" == *$'unstaged_file_list:\nseed.txt'* ]]
}

# A failing `git status` must not read as a clean tree: `clean=yes` is what a skill
# skips work on.
@test "a failing git status is an error, never clean=yes" {
	printf 'dirty\n' >dirty.txt
	printf 'not an index at all' >.git/index
	run facts commit
	[ "$status" -ne 0 ]
	[ "$(field clean)" != yes ]
}

# --- .mkit is no special case ---------------------------------------------------------

# `.mkit/` left the design (#50): an old one is untracked like any other directory, so
# the user sees it (doctor names the remedy) rather than facts hiding it.
@test "an old .mkit dir is reported like any other untracked path" {
	mkdir -p .mkit/review-x
	printf 'log\n' >.mkit/review-x/step.log
	run facts commit
	[ "$(field clean)" = no ]
	[ "$(field untracked)" = 1 ]
	[[ "$(facts_only)" == *".mkit/"* ]]
}

# --- the run dir -----------------------------------------------------------------------

@test "--run creates a run dir under TMPDIR and writes nothing into the work tree" {
	before="$(git status --porcelain --ignored)"
	exclude_before="$(cat .git/info/exclude)"
	export TMPDIR="$TEST_TMP/tmp/"
	mkdir -p "$TMPDIR"
	run facts commit --run
	[ "$status" -eq 0 ]
	dir="$(field run_dir)"
	[ -d "$dir" ]
	[[ "$dir" == "$TEST_TMP/tmp/mkit-commit-"* ]]
	[ "$(git status --porcelain --ignored)" = "$before" ]
	[ "$(cat .git/info/exclude)" = "$exclude_before" ]
	[ ! -e .mkit ]
}

# With TMPDIR unset the run dir is mktemp's own default, never a /tmp this script names.
@test "--run without TMPDIR still creates a run dir" {
	run env -u TMPDIR "$TEST_BASH" "$SCRIPTS/facts.sh" commit --run
	[ "$status" -eq 0 ]
	dir="$(field run_dir)"
	[ -d "$dir" ]
	rmdir "$dir"
	! grep -n '/tmp' "$SCRIPTS/facts.sh"
}

# Run from a subdirectory with a relative TMPDIR: resolved after the cd to the top
# level, it put the run dir inside the checkout.
@test "a relative TMPDIR resolves against the caller's directory, not the work tree" {
	mkdir sub
	before="$(git status --porcelain --ignored)"
	rel="$(basename "$REPO")-tmp"
	mkdir "$(dirname "$REPO")/$rel"
	cd sub
	run env TMPDIR="../../$rel" "$TEST_BASH" "$SCRIPTS/facts.sh" commit --run
	cd ..
	dir="$(field run_dir)"
	made=no
	[ -n "$dir" ] && [ -d "$dir" ] && made=yes
	rm -rf -- "$(dirname "$REPO")/$rel"
	[ "$status" -eq 0 ]
	[ "$made" = yes ]
	[ "$dir" = "$(dirname "$REPO")/$rel/${dir##*/}" ]
	[ "$(git status --porcelain --ignored)" = "$before" ]
}

# --- usage -----------------------------------------------------------------------------

@test "usage errors exit 2" {
	run facts
	[ "$status" -eq 2 ]
	run facts --run
	[ "$status" -eq 2 ]
	run facts 'not a skill'
	[ "$status" -eq 2 ]
	run facts commit --bogus
	[ "$status" -eq 2 ]
	run facts commit --base
	[ "$status" -eq 2 ]
}

# `--base=--output=PATH` reached `git rev-parse` as an option, and git created PATH
# before failing on the rest.
@test "a --base or --range that git would read as an option is refused before git sees it" {
	run facts commit --base --output=leak.txt
	[ "$status" -eq 2 ]
	run facts commit --range --output=leak.txt
	[ "$status" -eq 2 ]
	[ ! -e leak.txt ]
}

@test "outside a git repository exits 1" {
	cd "$TEST_TMP"
	run facts commit
	[ "$status" -eq 1 ]
}

# --- base and range --------------------------------------------------------------------

# An unresolvable --base used to fall through and exit 0, so finish/pr got a fact set
# they could not tell from a base with nothing on it.
@test "an unresolvable --base says so on stdout and exits 1" {
	run facts finish --base does-not-exist
	[ "$status" -eq 1 ]
	[[ "$output" == *"base_state=unresolvable"* ]]
	[[ "$output" != *"commits_ahead_of_base="* ]]
}

# A tree or blob resolves too, and gave counts that read like a real base.
@test "a --base that is not a commit is unresolvable" {
	run facts finish --base 'HEAD^{tree}'
	[ "$status" -eq 1 ]
	[[ "$output" == *"base_state=unresolvable"* ]]
	run facts finish --base 'HEAD:seed.txt'
	[ "$status" -eq 1 ]
	[[ "$output" == *"base_state=unresolvable"* ]]
}

@test "--base with commits ahead reports the count, the log and the diff" {
	git checkout -q -b feature
	printf 'more\n' >>seed.txt
	commit_here 'feature commit'
	run facts finish --base main
	[ "$status" -eq 0 ]
	[ "$(field base)" = main ]
	[ "$(field base_state)" = ok ]
	[ "$(field commits_ahead_of_base)" = 1 ]
	[[ "$output" == *"feature commit"* ]]
	[ "$(field base_files)" = 1 ]
	[ "$(field ff_from_base)" = yes ]
}

# `pr` reads base_stat after `commit` ran: the count can be 0 while the work is real.
@test "--base with nothing ahead still reports the base scope" {
	run facts pr --base main
	[ "$status" -eq 0 ]
	[ "$(field commits_ahead_of_base)" = 0 ]
	[ "$(field base_stat)" = none ]
	[ "$(field base_files)" = 0 ]
}

# A typo'd range printed `range_stat=none range_files=0` and exited 0 — a clean review
# of nothing.
@test "an unresolvable --range says so and exits 1, never an empty range" {
	run facts review --range nonexistentA..nonexistentB
	[ "$status" -eq 1 ]
	[ "$(field range_state)" = unresolvable ]
	[[ "$output" != *"range_stat=none"* ]]
}

@test "a resolvable --range replaces the working-tree scopes" {
	printf 'b\n' >b.txt
	commit_here second
	printf 'dirty\n' >>seed.txt
	run facts review --range HEAD~1..HEAD
	[ "$status" -eq 0 ]
	[ "$(field range_state)" = ok ]
	[ "$(field range_files)" = 1 ]
	[[ "$output" != *"unstaged_files="* ]]
}

# --- branch and worktree ---------------------------------------------------------------

@test "a detached HEAD is reported as such" {
	git checkout -q --detach HEAD
	run facts commit
	[ "$status" -eq 0 ]
	[ "$(field detached)" = yes ]
	[ "$(field branch)" = "$(git rev-parse --short HEAD)" ]
}

@test "the primary checkout has nothing to tear down" {
	run facts finish
	[ "$(field linked)" = no ]
	[ "$(field worktrees)" = 1 ]
	[ "$(field primary)" = "$REPO" ]
	[ "$(field worktree_origin)" = primary ]
	[ "$(field cleanup_path)" = none ]
}

@test "a linked worktree is reported with the primary and its cleanup path" {
	git worktree add -q -b feature "$TEST_TMP/wt" main
	cd "$TEST_TMP/wt"
	run facts finish
	[ "$status" -eq 0 ]
	[ "$(field linked)" = yes ]
	[ "$(field worktrees)" = 2 ]
	[ "$(field primary)" = "$REPO" ]
	[ "$(field common_dir)" = "$REPO/.git" ]
	[ "$(field worktree_origin)" = linked ]
	[ "$(field cleanup_path)" = git-worktree ]
	[ "$(field branch)" = feature ]
}

@test "a worktree under .claude/worktrees/ is handed back, never removed" {
	mkdir -p "$TEST_TMP/.claude/worktrees"
	git worktree add -q -b feature "$TEST_TMP/.claude/worktrees/feature" main
	cd "$TEST_TMP/.claude/worktrees/feature"
	run facts finish
	[ "$(field worktree_origin)" = claude-code ]
	[ "$(field cleanup_path)" = exit-worktree ]
}

# branch-status.sh reads every worktree's origin from here: one classifier.
@test "--worktrees lists every worktree's origin" {
	mkdir -p "$TEST_TMP/.claude/worktrees"
	git worktree add -q -b a "$TEST_TMP/.claude/worktrees/a" main
	git worktree add -q -b b "$TEST_TMP/wt b" main
	run facts cleanup
	[[ "$output" != *"worktree_origins:"* ]]
	run facts cleanup --worktrees
	[ "$status" -eq 0 ]
	block="$(printf '%s\n' "$output" | sed -n '/^worktree_origins:$/,/^[a-z_]*[:=]/p')"
	[[ "$block" == *"primary	$REPO"* ]]
	[[ "$block" == *"claude-code	$TEST_TMP/.claude/worktrees/a"* ]]
	[[ "$block" == *"linked	$TEST_TMP/wt b"* ]]
}

@test "upstream, ahead and behind" {
	run facts pr
	[ "$(field upstream)" = none ]
	[ "$(field pushed)" = no ]
	[[ "$output" != *"ahead="* ]]
	git init -q --bare "$TEST_TMP/remote.git"
	git remote add origin "$TEST_TMP/remote.git"
	git push -q -u origin main 2>/dev/null
	printf 'more\n' >>seed.txt
	commit_here ahead
	run facts pr
	[ "$(field upstream)" = origin/main ]
	[ "$(field pushed)" = yes ]
	[ "$(field ahead)" = 1 ]
	[ "$(field behind)" = 0 ]
}

# `ahead=0 behind=0` is what `finish` reads as "nothing to push": a failed count must
# never print it.
@test "a failing ahead/behind count is an error, never 0/0" {
	git init -q --bare "$TEST_TMP/remote.git"
	git remote add origin "$TEST_TMP/remote.git"
	git push -q -u origin main 2>/dev/null
	stub_git_fail 'rev-list --left-right'
	run facts pr
	[ "$status" -eq 1 ]
	[[ "$output" != *"ahead="* ]]
}

# An empty URL reads as "not a GitHub remote", and `gh` then guesses the repo.
@test "a failing remote URL lookup is an error, never remote_repo=none" {
	git remote add origin https://github.com/me/repo.git
	stub_git_fail 'remote get-url'
	run facts pr
	[ "$status" -eq 1 ]
	[[ "$output" != *"remote_repo="* ]]
}

# --- remote resolution (ADR 0005, amended by ADR 0006) ---------------------------------

@test "remote: none, then origin, then ask when there are several" {
	run facts pr
	[ "$(field remote)" = none ]
	[ "$(field remote_source)" = none ]
	[ "$(field remotes)" = none ]
	[ "$(field remote_choice)" = no ]

	git remote add origin https://example.invalid/o.git
	run facts pr
	[ "$(field remote)" = origin ]
	[ "$(field remote_source)" = origin ]
	[ "$(field remote_choice)" = no ]

	# git lists remotes alphabetically: `fork` is first, `origin` still wins.
	git remote add fork https://example.invalid/f.git
	run facts pr
	[ "$(field remote)" = origin ]
	[ "$(field remote_source)" = origin ]
	[ "$(field remotes)" = fork,origin ]
	[ "$(field remote_choice)" = ask ]
}

@test "remote: a lone remote not named origin, and git's first of several" {
	git remote add upstream https://example.invalid/u.git
	run facts pr
	[ "$(field remote)" = upstream ]
	[ "$(field remote_source)" = only ]
	[ "$(field remote_choice)" = no ]

	git remote add fork https://example.invalid/f.git
	run facts pr
	[ "$(field remote)" = fork ]
	[ "$(field remote_source)" = first ]
	[ "$(field remote_choice)" = ask ]
}

@test "--remote chooses, and settles the question" {
	git remote add origin https://example.invalid/o.git
	git remote add fork https://example.invalid/f.git
	run facts pr --remote fork
	[ "$status" -eq 0 ]
	[ "$(field remote)" = fork ]
	[ "$(field remote_source)" = chosen ]
	[ "$(field remote_choice)" = no ]
}

@test "a --remote that names no remote falls back, says so in notes:, and is not an error" {
	git remote add origin https://example.invalid/o.git
	git remote add fork https://example.invalid/f.git
	run facts pr --remote gone
	[ "$status" -eq 0 ]
	[ "$(field remote)" = origin ]
	[ "$(field remote_source)" = origin ]
	[ "$(field remote_choice)" = ask ]
	[[ "$output" == *"notes:
- remote_source=origin — --remote 'gone' is not a remote of this checkout"* ]]
}

@test "remote_repo is owner/repo for GitHub remotes only" {
	git remote add origin https://github.com/me/fork.git
	run facts pr
	[ "$(field remote_repo)" = me/fork ]
	git remote set-url origin git@github.com:org/project.git
	run facts pr
	[ "$(field remote_repo)" = org/project ]
	git remote set-url origin ssh://git@github.com/org/project/
	run facts pr
	[ "$(field remote_repo)" = org/project ]
	git remote set-url origin git@github.com:org/bare
	run facts pr
	[ "$(field remote_repo)" = org/bare ]
	git remote set-url origin https://user@github.com/org/tok.git
	run facts pr
	[ "$(field remote_repo)" = org/tok ]
	# The host is not a substring match: neither of these is GitHub.
	git remote set-url origin https://evilgithub.com/a/b.git
	run facts pr
	[ "$(field remote_repo)" = none ]
	git remote set-url origin https://github.com.evil.example/a/b.git
	run facts pr
	[ "$(field remote_repo)" = none ]
	git remote set-url origin https://gitlab.com/a/b.git
	run facts pr
	[ "$(field remote_repo)" = none ]
}

# The remote's HEAD wins only when this checkout has that branch: a remote-only name
# would stop cleanup on a branch never checked out here.
@test "default_branch is always a local branch, or unknown" {
	run facts cleanup
	[ "$(field default_branch)" = main ]

	git branch develop
	git remote add origin https://example.invalid/o.git
	git update-ref refs/remotes/origin/develop HEAD
	git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/develop
	run facts cleanup
	[ "$(field default_branch)" = develop ]

	git update-ref refs/remotes/origin/trunk-only HEAD
	git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/trunk-only
	run facts cleanup
	[ "$(field default_branch)" = main ]

	git checkout -q -b other
	git branch -D main -q
	git branch -D develop -q
	run facts cleanup
	[ "$(field default_branch)" = unknown ]
}

# --- codeowners ------------------------------------------------------------------------

@test "codeowners names the file, or none" {
	run facts pr
	[ "$(field codeowners)" = none ]
	mkdir -p .github
	printf '* @someone\n' >.github/CODEOWNERS
	run facts pr
	[ "$(field codeowners)" = "$REPO/.github/CODEOWNERS" ]
}

# --- the PR (--gh) ---------------------------------------------------------------------

@test "no PR lookup without --gh" {
	run facts pr
	[[ "$output" != *"pr="* ]]
}

@test "no gh on PATH is pr=gh-missing" {
	run env PATH="$(fake_path gh)" "$TEST_BASH" "$SCRIPTS/facts.sh" pr --gh
	[ "$status" -eq 0 ]
	[ "$(field pr)" = gh-missing ]
}

@test "a PR is looked up in the resolved remote's repo" {
	git remote add origin https://github.com/me/fork.git
	git remote add upstream git@github.com:org/project.git
	stub_gh
	export GH_VIEW='{"url":"https://github.com/org/project/pull/7","state":"OPEN","isDraft":true}'
	run facts pr --gh --remote upstream
	[ "$status" -eq 0 ]
	[ "$(field pr)" = https://github.com/org/project/pull/7 ]
	[ "$(field pr_state)" = OPEN ]
	[ "$(field pr_draft)" = true ]
	grep -q '^pr view main --repo org/project ' "$GH_LOG"
}

@test "pr= names why there is no PR: none, unauthenticated, no remote" {
	stub_gh
	run facts pr --gh
	[ "$(field pr)" = no-remote ]
	git remote add origin https://github.com/me/fork.git
	run facts pr --gh
	[ "$(field pr)" = none ]
	GH_AUTH=fail run facts pr --gh
	[ "$(field pr)" = gh-unauthenticated ]
}

# `gh pr view --repo` needs a branch, and a detached HEAD has none to have a PR.
@test "a detached HEAD skips the PR lookup" {
	git remote add origin https://github.com/me/fork.git
	git checkout -q --detach
	stub_gh
	export GH_VIEW='{"url":"u","state":"OPEN","isDraft":false}'
	run facts pr --gh
	[ "$(field pr)" = none ]
	! grep -q 'pr view' "$GH_LOG"
}

# --- the user dir ----------------------------------------------------------------------

@test "user_dir is MKIT_HOME, and probing it creates nothing" {
	export MKIT_HOME="$REPO/.mkit-home/a/b"
	run facts commit
	[ "$status" -eq 0 ]
	[ "$(field user_dir)" = "$MKIT_HOME" ]
	[ "$(field user_dir_writable)" = yes ]
	[ ! -e "$REPO/.mkit-home" ]
}

@test "an unwritable user dir is a named fact with a remedy that works" {
	mkdir -p "$MKIT_HOME"
	chmod 500 "$MKIT_HOME"
	run facts commit
	chmod 700 "$MKIT_HOME"
	[ "$status" -eq 0 ]
	[ "$(field user_dir_writable)" = no ]
	[[ "$output" == *"notes:"*"- user_dir_writable=no — run \`! mkdir -p $MKIT_HOME\`"*"permissions.additionalDirectories"* ]]
	[[ "$output" != *".claude/mkit"* ]]
	[ -z "$(ls -A "$MKIT_HOME")" ]
}

# The remedy is userdir.Remedy()'s sentence (a Go test compares them); a path a shell
# would split is quoted the same way.
@test "the user-dir remedy quotes a path the shell would split" {
	export MKIT_HOME="$REPO/seed.txt/my state"
	run facts commit
	[ "$(field user_dir_writable)" = no ]
	[[ "$output" == *"run \`! mkdir -p '$MKIT_HOME'\`"* ]]
}
