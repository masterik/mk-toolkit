#!/usr/bin/env bats
# branch-status.sh: argv in, stdout and exit status out.
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

kv() { printf '%s\n' "$output" | sed -n "s/^$1=//p"; }

# Column <col> of the `branches:` row for <branch> (1 = the name).
branch_field() {
	printf '%s\n' "$output" | awk -v b="$1" -v col="$2" -F'\t' '
		/^branches:$/ { inblock = 1; next }
		/^worktrees:$/ { inblock = 0 }
		inblock && $1 == b { print $col }'
}

# Column <col> of the `worktrees:` row for <branch>.
worktree_field() {
	printf '%s\n' "$output" | awk -v b="$1" -v col="$2" -F'\t' '
		/^worktrees:$/ { inblock = 1; next }
		/^notes:$/ { inblock = 0 }
		inblock && $1 == b { print $col }'
}

# A branch with one commit of its own: a branch at main's HEAD is trivially its
# ancestor and would read as merged.
diverged_branch() {
	git checkout -q -b "$1"
	printf '%s\n' "$1" >>seed.txt
	commit_here "$1 work"
	git checkout -q main
}

# An upstream that existed and was deleted remotely.
gone_upstream() {
	git update-ref "refs/remotes/origin/$1" "$1"
	git branch -q --set-upstream-to="refs/remotes/origin/$1" "$1"
	git update-ref -d "refs/remotes/origin/$1"
}

# --- usage -----------------------------------------------------------------------------

@test "a --default with no local branch exits 2" {
	run branch_status --default no-such-branch --no-fetch --no-gh
	[ "$status" -eq 2 ]
}

@test "unknown options exit 2" {
	run branch_status --bogus
	[ "$status" -eq 2 ]
	run branch_status --keep
	[ "$status" -eq 2 ]
}

@test "outside a git repository exits 1" {
	cd "$TEST_TMP"
	run branch_status --default main --no-fetch --no-gh
	[ "$status" -eq 1 ]
}

# The default comes from facts.sh, never re-derived here.
@test "without --default, the default branch is facts.sh's" {
	run branch_status --no-fetch --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv default)" = main ]
}

@test "no resolvable default and no --default exits 2" {
	git checkout -q -b other
	git branch -q -D main
	run branch_status --no-fetch --no-gh
	[ "$status" -eq 2 ]
}

# --- protection ------------------------------------------------------------------------

@test "the default branch is protected" {
	run branch_status --default main --no-fetch --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv develop)" = none ]
	[ "$(kv protected)" = main ]
	[ "$(kv keep)" = none ]
	[ "$(kv keep_unknown)" = none ]
	[ "$(branch_field main 2)" = protected ]
}

@test "a local develop-like branch is protected beside the default" {
	git branch dev
	run branch_status --default main --no-fetch --no-gh
	[ "$(kv develop)" = dev ]
	git branch develop
	run branch_status --default main --no-fetch --no-gh
	[ "$(kv develop)" = develop ]
	[ "$(kv protected)" = main,develop ]
	[ "$(branch_field develop 2)" = protected ]
}

@test "a remote-only develop is not kept" {
	git update-ref refs/remotes/origin/develop HEAD
	run branch_status --default main --no-fetch --no-gh
	[ "$(kv develop)" = none ]
}

@test "--develop names the develop branch; one with no local branch is a note" {
	git branch staging
	run branch_status --default main --develop staging --no-fetch --no-gh
	[ "$(kv develop)" = staging ]
	[ "$(branch_field staging 2)" = protected ]
	run branch_status --default main --develop nope --no-fetch --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv develop)" = none ]
	[[ "$output" == *"notes:"*"--develop 'nope'"* ]]
}

@test "a kept branch is protected; a kept name with no local branch is reported, not refused" {
	diverged_branch staging
	run branch_status --default main --keep staging --keep ghost --no-fetch --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv protected)" = main,staging ]
	[ "$(kv keep)" = staging,ghost ]
	[ "$(kv keep_unknown)" = ghost ]
	[ "$(branch_field staging 2)" = protected ]
}

# A keep list that omits the default is a mistake, not an instruction.
@test "the default stays protected when the keep list omits it" {
	git branch staging
	run branch_status --default main --keep staging --no-fetch --no-gh
	[ "$(branch_field main 2)" = protected ]
	[[ "$(kv protected)" == main,* ]]
}

# Treating "merged into staging" as merged would turn a key meant to delete less into
# one that deletes more.
@test "a kept branch is not a merge target" {
	git branch staging
	diverged_branch feature
	git checkout -q staging
	git merge -q --no-ff feature -m merge
	git checkout -q main
	run branch_status --default main --keep staging --no-fetch --no-gh
	[ "$(branch_field feature 2)" = unpushed ]
	[ "$(branch_field feature 4)" = - ]
}

# Splitting a comma-joined list would test fragments of the name.
@test "a default branch whose name contains a comma is tested by name" {
	git branch "release,2026"
	diverged_branch feature
	git checkout -q "release,2026"
	git merge -q --no-ff feature -m merge
	git checkout -q main
	run branch_status --default "release,2026" --no-fetch --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv default)" = "release,2026" ]
	[ "$(branch_field "release,2026" 2)" = protected ]
	[ "$(branch_field feature 2)" = merged ]
	[ "$(branch_field feature 4)" = "release,2026" ]
}

# --- classes ---------------------------------------------------------------------------

@test "a branch merged into the default is merged, and names its target" {
	diverged_branch feature
	git merge -q --no-ff feature -m merge
	run branch_status --default main --no-fetch --no-gh
	[ "$(branch_field feature 2)" = merged ]
	[ "$(branch_field feature 4)" = main ]
}

@test "the current branch is current even when merged" {
	git checkout -q -b feature
	run branch_status --default main --no-fetch --no-gh
	[ "$(branch_field feature 2)" = current ]
	[ "$(branch_field feature 4)" = main ]
}

@test "unpushed, gone and tracking" {
	diverged_branch never
	diverged_branch deleted
	diverged_branch live
	git remote add origin "$TEST_TMP/nowhere.git"
	gone_upstream deleted
	git update-ref refs/remotes/origin/live live
	git branch -q --set-upstream-to=refs/remotes/origin/live live
	run branch_status --default main --no-fetch --no-gh
	[ "$(branch_field never 2)" = unpushed ]
	[ "$(branch_field never 3)" = none ]
	[ "$(branch_field deleted 2)" = gone ]
	[ "$(branch_field deleted 3)" = gone ]
	[ "$(branch_field live 2)" = tracking ]
	[ "$(branch_field live 3)" = origin/live ]
}

# --- fetch -----------------------------------------------------------------------------

@test "fetch: no-remote, skipped, ok, failed" {
	run branch_status --default main --no-gh
	[ "$(kv remote)" = none ]
	[ "$(kv fetch)" = no-remote ]
	git init -q --bare "$TEST_TMP/remote.git"
	git remote add origin "$TEST_TMP/remote.git"
	run branch_status --default main --no-fetch --no-gh
	[ "$(kv fetch)" = skipped ]
	run branch_status --default main --no-gh
	[ "$(kv remote)" = origin ]
	[ "$(kv fetch)" = ok ]
	git remote set-url origin "$TEST_TMP/missing.git"
	run branch_status --default main --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv fetch)" = failed ]
	[[ "$output" == *"branches:"* ]]
}

# Without the fetch, `upstream=gone` reflects a stale local view.
@test "the fetch prunes a deleted upstream into gone" {
	git init -q --bare "$TEST_TMP/remote.git"
	git remote add origin "$TEST_TMP/remote.git"
	diverged_branch feature
	git push -q -u origin feature 2>/dev/null
	git push -q origin --delete feature 2>/dev/null
	git update-ref refs/remotes/origin/feature feature
	run branch_status --default main --no-gh
	[ "$(branch_field feature 2)" = gone ]
}

@test "--remote picks the remote; one that is not configured falls back with a note" {
	git remote add origin "$TEST_TMP/o.git"
	git remote add fork "$TEST_TMP/f.git"
	run branch_status --default main --remote fork --no-fetch --no-gh
	[ "$(kv remote)" = fork ]
	run branch_status --default main --remote nope --no-fetch --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv remote)" = origin ]
	[[ "$output" == *"notes:"*"- remote: --remote 'nope'"* ]]
}

# --- gh --------------------------------------------------------------------------------

@test "gh: no-remote, skipped, gh-missing, gh-unauthenticated, gh-error" {
	run branch_status --default main --no-fetch
	[ "$(kv gh)" = no-remote ]
	git remote add origin https://github.com/me/repo.git
	stub_gh
	run branch_status --default main --no-fetch --no-gh
	[ "$(kv gh)" = skipped ]
	GH_AUTH=fail run branch_status --default main --no-fetch
	[ "$(kv gh)" = gh-unauthenticated ]
	run branch_status --default main --no-fetch
	[ "$(kv gh)" = gh-error ]
	[ "$(branch_field main 2)" = protected ]
	run env PATH="$(fake_path gh)" "$TEST_BASH" "$SCRIPTS/branch-status.sh" --default main --no-fetch
	[ "$status" -eq 0 ]
	[ "$(kv gh)" = gh-missing ]
}

@test "one batched PR lookup, in the resolved remote's repo" {
	diverged_branch a
	diverged_branch b
	git remote add origin https://github.com/me/fork.git
	git remote add upstream git@github.com:org/project.git
	stub_gh
	export GH_LIST='[]'
	run branch_status --default main --remote upstream --no-fetch
	[ "$(kv gh)" = ok ]
	[ "$(grep -c '^pr ' "$GH_LOG")" = 1 ]
	grep -q -- '--repo org/project' "$GH_LOG"
}

@test "PR classes: merged-pr, open-pr, closed-pr, and the newest PR wins" {
	diverged_branch squashed
	diverged_branch open
	diverged_branch closed
	diverged_branch reopened
	git remote add origin https://github.com/me/repo.git
	gone_upstream squashed
	stub_gh
	export GH_LIST='[
		{"headRefName":"squashed","number":42,"state":"MERGED","headRefOid":""},
		{"headRefName":"open","number":7,"state":"OPEN","headRefOid":""},
		{"headRefName":"closed","number":5,"state":"CLOSED","headRefOid":""},
		{"headRefName":"reopened","number":3,"state":"CLOSED","headRefOid":""},
		{"headRefName":"reopened","number":9,"state":"OPEN","headRefOid":""},
		{"headRefName":"reopened","number":4,"state":"CLOSED","headRefOid":""}]'
	run branch_status --default main --no-fetch
	[ "$(kv gh)" = ok ]
	[ "$(branch_field squashed 2)" = merged-pr ]
	[ "$(branch_field squashed 5)" = merged#42 ]
	[ "$(branch_field open 2)" = open-pr ]
	[ "$(branch_field open 5)" = open#7 ]
	[ "$(branch_field closed 2)" = closed-pr ]
	[ "$(branch_field reopened 5)" = open#9 ]
	[ "$(branch_field main 5)" = - ]
}

# A reused branch name must never delete new work.
@test "a merged PR whose head is not the tip or an ancestor of it is not merged-pr" {
	diverged_branch feature
	git remote add origin https://github.com/me/repo.git
	gone_upstream feature
	stub_gh
	git checkout -q -b unrelated
	printf 'x\n' >unrelated.txt
	commit_here unrelated
	foreign="$(git rev-parse HEAD)"
	git checkout -q main
	export GH_LIST="[{\"headRefName\":\"feature\",\"number\":42,\"state\":\"MERGED\",\"headRefOid\":\"$foreign\"}]"
	run branch_status --default main --no-fetch
	[ "$(kv gh)" = ok ]
	[ "$(branch_field feature 2)" = gone ]
	[ "$(branch_field feature 5)" = merged#42 ]
	# An oid this clone has never seen is not trusted either.
	export GH_LIST='[{"headRefName":"feature","number":42,"state":"MERGED","headRefOid":"0123456789abcdef0123456789abcdef01234567"}]'
	run branch_status --default main --no-fetch
	[ "$(branch_field feature 2)" = gone ]
}

# Trusted when the PR saw everything the branch holds: its head is the tip, or the tip
# is an ancestor of its head. A branch that gained commits since is new work.
@test "a merged PR is trusted only when it saw every commit on the branch" {
	diverged_branch feature
	git remote add origin https://github.com/me/repo.git
	stub_gh
	pr_list() { export GH_LIST="[{\"headRefName\":\"feature\",\"number\":42,\"state\":\"MERGED\",\"headRefOid\":\"$1\"}]"; }
	pr_list "$(git rev-parse feature)"
	run branch_status --default main --no-fetch
	[ "$(branch_field feature 2)" = merged-pr ]

	# The PR got a commit this clone's branch never had: still everything is in it.
	git checkout -q -b pr-head feature
	printf 'pushed elsewhere\n' >elsewhere.txt
	commit_here elsewhere
	pr_list "$(git rev-parse HEAD)"
	git checkout -q main
	git branch -q -D pr-head
	run branch_status --default main --no-fetch
	[ "$(branch_field feature 2)" = merged-pr ]

	# The branch gained a commit after the PR merged.
	pr_list "$(git rev-parse feature)"
	git checkout -q feature
	printf 'later\n' >later.txt
	commit_here later
	git checkout -q main
	run branch_status --default main --no-fetch
	[ "$(branch_field feature 2)" = unpushed ]
}

# --- worktrees -------------------------------------------------------------------------

@test "worktrees: primary, linked, clean or dirty" {
	git worktree add -q -b feature "$TEST_TMP/wt" main
	run branch_status --default main --no-fetch --no-gh
	[ "$(kv worktrees_state)" = ok ]
	[ "$(worktree_field main 2)" = "$REPO" ]
	[ "$(worktree_field main 3)" = primary ]
	[ "$(worktree_field feature 2)" = "$TEST_TMP/wt" ]
	[ "$(worktree_field feature 3)" = linked ]
	[ "$(worktree_field feature 4)" = yes ]
	printf 'dirty\n' >"$TEST_TMP/wt/dirty.txt"
	run branch_status --default main --no-fetch --no-gh
	[ "$(worktree_field feature 4)" = no ]
}

@test "a detached worktree is reported as HEAD" {
	git worktree add -q --detach "$TEST_TMP/wt" main
	run branch_status --default main --no-fetch --no-gh
	[ "$(worktree_field HEAD 3)" = linked ]
}

@test "a worktree under .claude/worktrees/ is origin=claude-code" {
	mkdir -p "$TEST_TMP/.claude/worktrees"
	git worktree add -q -b feature "$TEST_TMP/.claude/worktrees/feature" main
	run branch_status --default main --no-fetch --no-gh
	[ "$(worktree_field feature 3)" = claude-code ]
}

@test "a worktree whose directory is gone is clean=missing" {
	git worktree add -q -b feature "$TEST_TMP/wt" main
	rm -rf "$TEST_TMP/wt"
	run branch_status --default main --no-fetch --no-gh
	[ "$(worktree_field feature 4)" = missing ]
}

# An unreadable worktree is not a proven-clean one.
@test "a worktree whose status fails is clean=error, never yes" {
	git worktree add -q -b feature "$TEST_TMP/wt" main
	rm -f "$TEST_TMP/wt/.git"
	run branch_status --default main --no-fetch --no-gh
	[ "$(worktree_field feature 4)" = error ]
}

# cleanup demotes a merged branch to ask on clean=no, then offers --force with a
# "discards uncommitted work" sentence: a log mkit wrote must never trigger that.
@test "a worktree holding only an old .mkit run dir is clean; a real file is not" {
	git worktree add -q -b feature "$TEST_TMP/wt" main
	mkdir -p "$TEST_TMP/wt/.mkit/review-x"
	printf 'log\n' >"$TEST_TMP/wt/.mkit/review-x/step.log"
	run branch_status --default main --no-fetch --no-gh
	[ "$(worktree_field feature 4)" = yes ]
	printf 'dirty\n' >"$TEST_TMP/wt/dirty.txt"
	run branch_status --default main --no-fetch --no-gh
	[ "$(worktree_field feature 4)" = no ]
}
