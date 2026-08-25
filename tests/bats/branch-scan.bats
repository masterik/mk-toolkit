#!/usr/bin/env bats
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

# `branches:` and `worktrees:` are tab-separated rows under their own header line.
# Grab the row for one branch/path, tab-split, print column $2 (1 = the key itself).
branch_field() {
	local branch="$1" col="$2"
	printf '%s\n' "$output" |
		awk -v b="$branch" -v col="$col" -F'\t' '
			/^branches:$/ { inblock = 1; next }
			/^worktrees:$/ { inblock = 0 }
			inblock && $1 == b { print $col }
		'
}

worktree_field() {
	local branch="$1" col="$2"
	printf '%s\n' "$output" |
		awk -v b="$branch" -v col="$col" -F'\t' '
			/^worktrees:$/ { inblock = 1; next }
			inblock && $1 == b { print $col }
		'
}

kv() { printf '%s\n' "$output" | sed -n "s/^$1=//p"; }

# A branch with one commit of its own, not (yet) merged back into main — the shape every
# "unmerged" classification test needs, since a branch identical to main's HEAD is
# trivially its own ancestor and would misreport as `merged`.
make_diverged_branch() {
	git checkout -q -b "$1"
	printf 'diverged\n' >>seed.txt
	git add seed.txt
	git commit -q -m "$1 work"
	git checkout -q main
}

fake_gh() {
	local prs_json="$1" bindir="$BATS_TEST_TMPDIR/fake-gh-bin"
	mkdir -p "$bindir"
	for t in bash git sed awk grep cat head tail wc tr find sort mktemp touch \
		date basename dirname xargs jq rm cut mv chmod; do
		p="$(command -v "$t" 2>/dev/null)" && ln -sf "$p" "$bindir/$t"
	done
	cat >"$bindir/gh" <<EOF
#!/usr/bin/env bash
case "\$1 \$2" in
"auth status") exit 0 ;;
"pr list") cat <<'JSON'
$prs_json
JSON
;;
esac
EOF
	chmod +x "$bindir/gh"
	printf '%s\n' "$bindir"
}

@test "rejects a --default that does not exist locally" {
	run "$SCRIPTS/branch-scan.sh" --default no-such-branch --no-fetch --no-gh
	[ "$status" -eq 2 ]
}

@test "usage error with no --default" {
	run "$SCRIPTS/branch-scan.sh" --no-fetch --no-gh
	[ "$status" -eq 2 ]
}

@test "fails outside a git repository" {
	cd "$MKIT_TMP/.."
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$status" -eq 1 ]
}

@test "the default branch itself is protected" {
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$status" -eq 0 ]
	[ "$(kv default)" = main ]
	[ "$(kv develop)" = none ]
	[ "$(kv protected)" = main ]
	[ "$(branch_field main 2)" = protected ]
}

@test "a develop-like local branch is protected alongside the default" {
	git checkout -q -b develop
	git checkout -q main
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(kv develop)" = develop ]
	[ "$(kv protected)" = main,develop ]
	[ "$(branch_field develop 2)" = protected ]
}

@test "development and dev are recognized when develop is absent" {
	git checkout -q -b dev
	git checkout -q main
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(kv develop)" = dev ]
}

@test "a remote develop with no local branch is not kept" {
	git update-ref refs/remotes/origin/develop HEAD
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(kv develop)" = none ]
}

@test "a branch fully merged into main is classified merged" {
	git checkout -q -b feature
	printf 'more\n' >>seed.txt
	git add seed.txt
	git commit -q -m 'feature work'
	git checkout -q main
	git merge -q --no-ff feature
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(branch_field feature 2)" = merged ]
	[ "$(branch_field feature 4)" = main ]
}

@test "the current branch is never classified merged even when it is" {
	# feature == main's HEAD, so it is trivially an ancestor of main (merged) — but
	# it must still report `current`, not `merged`, since it cannot be deleted anyway.
	git checkout -q -b feature
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(branch_field feature 2)" = current ]
}

@test "a branch with no upstream is classified unpushed" {
	make_diverged_branch feature
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(branch_field feature 2)" = unpushed ]
	[ "$(branch_field feature 3)" = none ]
}

@test "a branch whose upstream was deleted remotely is classified gone" {
	make_diverged_branch feature
	git remote add origin https://example.invalid/repo.git
	git update-ref refs/remotes/origin/feature feature
	git branch --set-upstream-to=refs/remotes/origin/feature feature -q
	git update-ref -d refs/remotes/origin/feature
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(branch_field feature 2)" = gone ]
	[ "$(branch_field feature 3)" = gone ]
}

@test "a branch with a live upstream that is not merged is classified tracking" {
	make_diverged_branch feature
	git remote add origin https://example.invalid/repo.git
	git update-ref refs/remotes/origin/feature feature
	git branch --set-upstream-to=refs/remotes/origin/feature feature -q
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(branch_field feature 2)" = tracking ]
	[[ "$(branch_field feature 3)" == origin/feature ]]
}

@test "no remote at all reports remote=none and skips gh" {
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$(kv remote)" = none ]
	[ "$(kv gh)" = no-remote ]
}

@test "no gh on PATH reports gh=gh-missing" {
	git remote add origin https://example.invalid/repo.git
	binfake="$BATS_TEST_TMPDIR/no-gh-bin"
	mkdir -p "$binfake"
	for t in bash git sed awk grep cat head tail wc tr find sort mktemp touch \
		date basename dirname xargs jq; do
		p="$(command -v "$t" 2>/dev/null)" && ln -sf "$p" "$binfake/$t"
	done
	run env PATH="$binfake" "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$status" -eq 0 ]
	[ "$(kv gh)" = gh-missing ]
}

@test "--no-gh reports gh=skipped even with gh available" {
	git remote add origin https://example.invalid/repo.git
	bindir="$(fake_gh '[]')"
	run env PATH="$bindir:$PATH" "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(kv gh)" = skipped ]
}

@test "a branch with a merged PR is classified merged-pr, not gone" {
	make_diverged_branch feature
	git remote add origin https://example.invalid/repo.git
	git update-ref refs/remotes/origin/feature feature
	git branch --set-upstream-to=refs/remotes/origin/feature feature -q
	git update-ref -d refs/remotes/origin/feature
	bindir="$(fake_gh '[{"headRefName":"feature","number":42,"state":"MERGED"}]')"
	run env PATH="$bindir:$PATH" "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$(kv gh)" = ok ]
	[ "$(branch_field feature 2)" = merged-pr ]
	[ "$(branch_field feature 5)" = merged#42 ]
}

@test "a branch with an open PR is classified open-pr" {
	make_diverged_branch feature
	git remote add origin https://example.invalid/repo.git
	git update-ref refs/remotes/origin/feature feature
	git branch --set-upstream-to=refs/remotes/origin/feature feature -q
	bindir="$(fake_gh '[{"headRefName":"feature","number":7,"state":"OPEN"}]')"
	run env PATH="$bindir:$PATH" "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$(branch_field feature 2)" = open-pr ]
	[ "$(branch_field feature 5)" = open#7 ]
}

@test "the newest PR wins when a branch has more than one" {
	make_diverged_branch feature
	git remote add origin https://example.invalid/repo.git
	bindir="$(fake_gh '[{"headRefName":"feature","number":3,"state":"CLOSED"},{"headRefName":"feature","number":9,"state":"OPEN"}]')"
	run env PATH="$bindir:$PATH" "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$(branch_field feature 5)" = open#9 ]
}

@test "a worktree's branch and cleanliness are reported" {
	wt_dir="$BATS_TEST_TMPDIR/wt-feature"
	git worktree add -q -b feature "$wt_dir" main
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(worktree_field feature 3)" = linked ]
	[ "$(worktree_field feature 4)" = yes ]
	printf 'dirty\n' >"$wt_dir/dirty.txt"
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(worktree_field feature 4)" = no ]
	git worktree remove --force "$wt_dir"
}

@test "the primary worktree is classified origin=primary" {
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(worktree_field main 3)" = primary ]
}

@test "a worktree under .claude/worktrees/ is classified origin=claude-code" {
	wt_dir="$BATS_TEST_TMPDIR/.claude/worktrees/feature"
	mkdir -p "$(dirname "$wt_dir")"
	git worktree add -q -b feature "$wt_dir" main
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(worktree_field feature 3)" = claude-code ]
	git worktree remove --force "$wt_dir"
}

@test "a worktree whose git metadata is broken reports clean=error, not clean=yes" {
	wt_dir="$BATS_TEST_TMPDIR/wt-broken"
	git worktree add -q -b feature "$wt_dir" main
	rm -f "$wt_dir/.git" # the linked worktree's gitdir-pointer file
	run "$SCRIPTS/branch-scan.sh" --default main --no-fetch --no-gh
	[ "$(worktree_field feature 4)" = error ]
	rm -rf "$wt_dir"
	git worktree prune
}

@test "a standalone closed PR with no open or merged PR classifies as closed-pr" {
	make_diverged_branch feature
	git remote add origin https://example.invalid/repo.git
	bindir="$(fake_gh '[{"headRefName":"feature","number":5,"state":"CLOSED"}]')"
	run env PATH="$bindir:$PATH" "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$(branch_field feature 2)" = closed-pr ]
	[ "$(branch_field feature 5)" = closed#5 ]
}

@test "a protected branch name containing a comma is still recognized, not corrupted by CSV joining" {
	git branch "release,2026"
	make_diverged_branch feature
	git checkout -q "release,2026"
	git merge -q --no-ff feature
	git checkout -q main
	run "$SCRIPTS/branch-scan.sh" --default "release,2026" --no-fetch --no-gh
	[ "$(kv default)" = "release,2026" ]
	[ "$(branch_field feature 2)" = merged ]
	[ "$(branch_field feature 4)" = "release,2026" ]
}

@test "a merged-PR match with a mismatched head oid does not classify as merged-pr" {
	make_diverged_branch feature
	bad_oid="$(git rev-parse main)"
	git remote add origin https://example.invalid/repo.git
	git update-ref refs/remotes/origin/feature feature
	git branch --set-upstream-to=refs/remotes/origin/feature feature -q
	git update-ref -d refs/remotes/origin/feature
	bindir="$(fake_gh "[{\"headRefName\":\"feature\",\"number\":42,\"state\":\"MERGED\",\"headRefOid\":\"$bad_oid\"}]")"
	run env PATH="$bindir:$PATH" "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$(kv gh)" = ok ]
	[ "$(branch_field feature 2)" != merged-pr ]
	[ "$(branch_field feature 2)" = gone ]
}

@test "a merged-PR match with the branch's own current tip as head oid classifies as merged-pr" {
	make_diverged_branch feature
	git remote add origin https://example.invalid/repo.git
	tip="$(git rev-parse feature)"
	bindir="$(fake_gh "[{\"headRefName\":\"feature\",\"number\":42,\"state\":\"MERGED\",\"headRefOid\":\"$tip\"}]")"
	run env PATH="$bindir:$PATH" "$SCRIPTS/branch-scan.sh" --default main --no-fetch
	[ "$(branch_field feature 2)" = merged-pr ]
}
