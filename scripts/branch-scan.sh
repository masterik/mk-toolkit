#!/usr/bin/env bash
#
# Classify every local branch and worktree for a repo-wide cleanup: which branches are
# merged (locally, or via a PR git's own merge-base can't see because of a squash merge),
# which still have an open PR, which were never pushed, and which worktree each one owns.
# Reports candidates; it never deletes a branch, removes a worktree, or touches a remote.
#
#   usage: branch-scan.sh --default <branch> [--no-fetch] [--no-gh]
#
#   --default <branch>   the branch already resolved by `facts.sh` (usually its own
#                         default_branch=) — this script does not re-derive it, so there
#                         is exactly one place that logic lives
#   --no-fetch            skip `git fetch --prune`; upstream=gone then reflects whatever
#                         the remote-tracking refs already were, not the current remote
#   --no-gh               skip the GitHub lookup even when `gh` is present and authenticated
#
# Output: key=value lines, then a `branches:` block (one tab-separated row per local
# branch) and a `worktrees:` block (one row per worktree, primary included).
#
#   default=<branch>            echoed back, so a later step's report matches this run
#   develop=<branch>|none         first of develop/development/dev that exists LOCALLY
#   protected=<branch>[,<branch>] the branches this run will never suggest deleting
#   remote=<name>|none
#   fetch=ok|skipped|no-remote|failed
#   gh=ok|skipped|no-remote|gh-missing|jq-missing|gh-unauthenticated|gh-error
#
#   branches:
#     <name>\t<class>\t<upstream>\t<merged_into>\t<pr>
#       class: protected · current · merged · merged-pr · open-pr · closed-pr · gone ·
#              unpushed · tracking          (the skill maps this to an action; see below)
#       upstream: none · gone · <remote>/<name>
#       merged_into: -  or a comma list of protected branches it is an ancestor of
#       pr: -  or open#<n> / merged#<n> / closed#<n>   (only populated when gh=ok; a
#           `merged` match is trusted only when the PR's own head commit is this branch's
#           current tip or an ancestor of it — a reused branch name otherwise falls through
#           to whatever class the branch's own git/upstream state supports)
#
#   worktrees:
#     <branch-or-HEAD>\t<path>\t<origin>\t<clean>
#       origin: primary · claude-code · linked   (../_shared/references/worktree.md)
#       clean: yes · no · missing (path is gone — `git worktree prune` would fix it) ·
#              error (`git status` itself failed there — treat as never automatic, same
#              as `no`: an unreadable worktree is not a proven-clean one)
#
# `class` is a fact, not a verdict — this script does not decide what counts as "safe to
# delete", the same restraint `gate-detect.sh` takes with `fast=`/`full=`. As a guide to
# what each class usually means for a cleanup skill:
#
#   protected / current    never a candidate
#   merged                  git itself proves it: safe to delete outright
#   merged-pr               not an ancestor locally (a squash or rebase merge changes the
#                           commit SHAs), but GitHub says the PR merged — safe, but say so
#   open-pr / closed-pr     unmerged work with a PR trail — ask before deleting
#   gone                    upstream existed and was deleted remotely, with no PR trail
#                           found (gh unavailable, or none ever opened) — ask
#   unpushed                never had an upstream at all — ask, and mention that deleting
#                           an unmerged branch is only recoverable via `git reflog` for a
#                           while
#   tracking                still has a live upstream and is not merged — probably still
#                           in use elsewhere; ask, don't guess
#
# `git fetch --prune` is the one mutation this script makes, and the reason it exists:
# without it `upstream=gone` reflects a stale local view. It only ever updates this
# repo's own remote-tracking refs (`refs/remotes/<remote>/*`) — it cannot delete, rename
# or otherwise touch a branch on the remote itself, which is the whole of what "local
# only" means here.
#
# Exit: 0 ok, 1 not a git repo, 2 bad usage.

set -euo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

default=""
no_fetch=no
no_gh=no

while [ $# -gt 0 ]; do
	case "$1" in
	--default)
		default="${2:-}"
		[ -n "$default" ] || mkit_die '--default needs a branch name' 2
		shift 2
		;;
	--no-fetch)
		no_fetch=yes
		shift
		;;
	--no-gh)
		no_gh=yes
		shift
		;;
	*) mkit_die "unknown option: $1" 2 ;;
	esac
done
[ -n "$default" ] || mkit_die 'usage: branch-scan.sh --default <branch> [--no-fetch] [--no-gh]' 2

mkit_require_repo
toplevel="$(git rev-parse --show-toplevel)"
cd "$toplevel"

git show-ref --verify --quiet "refs/heads/$default" ||
	mkit_die "--default branch does not exist locally: $default" 2

# --- protected branches: the default, plus a develop-like branch if one exists LOCALLY -
# Only a local branch counts. A `develop` that exists only as origin/develop is not one of
# this repo's local branches to keep — nothing here creates one, and this script only
# ever reports on what is already checked out somewhere.
develop=none
for cand in develop development dev; do
	[ "$cand" = "$default" ] && continue
	if git show-ref --verify --quiet "refs/heads/$cand"; then
		develop="$cand"
		break
	fi
done
protected="$default"
[ "$develop" != none ] && protected="$protected,$develop"

printf 'default=%s\n' "$default"
printf 'develop=%s\n' "$develop"
printf 'protected=%s\n' "$protected"

current="$(git branch --show-current)"

# --- fetch: updates only this repo's remote-tracking refs, never the remote itself -----
remote="$(git remote | head -1)"
printf 'remote=%s\n' "${remote:-none}"

fetch_state=skipped
if [ "$no_fetch" = yes ]; then
	fetch_state=skipped
elif [ -z "$remote" ]; then
	fetch_state=no-remote
elif git fetch "$remote" --prune -q 2>/dev/null; then
	fetch_state=ok
else
	fetch_state=failed
fi
printf 'fetch=%s\n' "$fetch_state"

# --- gh: one batched call, cached to a file, looked up per branch with jq (no network --
# per branch). One value per distinct cause, the `pr=gh-missing`/`pr=jq-missing` lesson
# from facts.sh: only some of these mean "there was nothing to look up".
gh_state=ok
pr_cache=""
if [ "$no_gh" = yes ]; then
	gh_state=skipped
elif [ -z "$remote" ]; then
	gh_state=no-remote
elif ! command -v gh >/dev/null 2>&1; then
	gh_state=gh-missing
elif ! command -v jq >/dev/null 2>&1; then
	gh_state=jq-missing
elif ! gh auth status >/dev/null 2>&1; then
	gh_state=gh-unauthenticated
else
	pr_cache="$(mktemp -t mkit-pr-cache)"
	trap 'rm -f "$pr_cache"' EXIT
	if ! gh pr list --state all --json headRefName,number,state,headRefOid --limit 500 \
		>"$pr_cache" 2>/dev/null; then
		gh_state=gh-error
	fi
fi
printf 'gh=%s\n' "$gh_state"

# $1 = branch name; prints "<state>#<number>#<headRefOid>" or "-". The newest PR (by
# number) wins when a branch was opened, closed, reopened and PR'd again — history a
# cleanup decision should see, not just whichever the API returned first. The oid travels
# with the match so the caller can refuse a `merged` result that does not actually belong
# to this branch's current content (see `pr_oid_is_local`, below) — internal only, never
# printed as-is: the public `pr` column stays `<state>#<number>`.
pr_lookup() {
	if [ "$gh_state" != ok ] || [ ! -s "$pr_cache" ]; then
		printf -- '-\n'
		return 0
	fi
	jq -r --arg b "$1" '
		[.[] | select(.headRefName == $b)] | sort_by(.number) | last
		| if . == null then "-"
		  else ((.state | ascii_downcase) + "#" + (.number | tostring) + "#" + (.headRefOid // ""))
		  end
	' "$pr_cache" 2>/dev/null || printf -- '-\n'
}

# Guards against a reused or coincidentally-matching branch name (f01 in review): a
# `merged` PR match is only trusted when its recorded head commit is this branch's own
# current tip, or an ancestor of it (the branch has not gained commits the PR never saw).
# A missing oid (jq's `// ""` fallback, or an API that omitted the field) degrades to the
# pre-existing name-only behavior rather than refusing outright — rare, and not this fix's
# job to eliminate.
pr_oid_is_local() {
	local branch="$1" oid="$2" tip
	[ -n "$oid" ] || return 0
	tip="$(git rev-parse -q --verify "refs/heads/$branch" 2>/dev/null)" || return 1
	[ "$oid" = "$tip" ] && return 0
	git cat-file -e "${oid}^{commit}" 2>/dev/null || return 1
	git merge-base --is-ancestor "$branch" "$oid" 2>/dev/null
}

# --- one row per local branch -----------------------------------------------------------
printf 'branches:\n'
git for-each-ref refs/heads --format='%(refname:short)' | while IFS= read -r b; do
	[ -n "$b" ] || continue

	upstream_ref="$(git for-each-ref "refs/heads/$b" --format='%(upstream)')"
	track="$(git for-each-ref "refs/heads/$b" --format='%(upstream:track)')"
	if [ -z "$upstream_ref" ]; then
		upstream=none
	elif printf '%s' "$track" | grep -q gone; then
		upstream=gone
	else
		upstream="${upstream_ref#refs/remotes/}"
	fi

	# Test $default and $develop directly rather than splitting the comma-joined
	# $protected: a branch name may legally contain a comma (git's ref-name rules do not
	# forbid it), and splitting on one would silently test a nonexistent fragment instead
	# of the real branch.
	merged_into=""
	for p in "$default" "$develop"; do
		[ "$p" = none ] && continue
		if git merge-base --is-ancestor "$b" "$p" 2>/dev/null; then
			merged_into="${merged_into:+$merged_into,}$p"
		fi
	done
	[ -n "$merged_into" ] || merged_into=-

	pr_raw="$(pr_lookup "$b")"
	pr=-
	pr_state=none
	pr_oid=""
	if [ "$pr_raw" != - ]; then
		pr_state="${pr_raw%%#*}"
		pr_rest="${pr_raw#*#}"
		pr_number="${pr_rest%%#*}"
		pr_oid="${pr_rest#*#}"
		pr="${pr_state}#${pr_number}"
	fi

	if [ "$b" = "$default" ] || { [ "$develop" != none ] && [ "$b" = "$develop" ]; }; then
		class=protected
	elif [ "$b" = "$current" ]; then
		# Ahead of "merged": you cannot delete the branch you are standing on,
		# however merged it already is.
		class=current
	elif [ "$merged_into" != - ]; then
		class=merged
	elif [ "$pr_state" = merged ] && pr_oid_is_local "$b" "$pr_oid"; then
		class=merged-pr
	elif [ "$pr_state" = open ]; then
		class=open-pr
	elif [ "$pr_state" = closed ]; then
		class=closed-pr
	elif [ "$upstream" = gone ]; then
		class=gone
	elif [ "$upstream" = none ]; then
		class=unpushed
	else
		class=tracking
	fi

	printf '%s\t%s\t%s\t%s\t%s\n' "$b" "$class" "$upstream" "$merged_into" "$pr"
done

# --- one row per worktree, primary included --------------------------------------------
# Same lookup table `facts.sh` uses for the worktree it is currently inside, run here over
# every worktree in the repo. The classification *loop* below is duplicated rather than
# shared — `facts.sh` answers for "the one this session is in", this answers for "every
# one that exists", and forcing a single function to serve both would make neither easy to
# change alone — but the one-line primary lookup itself has no such reason to differ
# between the two files, so it is `mkit_primary_worktree` (`lib/common.sh`) in both.
primary="$(mkit_primary_worktree)"
printf 'worktrees:\n'
git worktree list --porcelain | awk '
	/^worktree / { path = substr($0, 10) }
	/^branch /   { print path "\t" substr($0, 8); path = "" }
	/^detached/  { print path "\tHEAD"; path = "" }
' | while IFS="$(printf '\t')" read -r path refline; do
	[ -n "$path" ] || continue
	branch="${refline#refs/heads/}"
	case "$path" in
	"$primary") origin=primary ;;
	*/.claude/worktrees/*) origin=claude-code ;;
	*) origin=linked ;;
	esac
	if [ ! -d "$path" ]; then
		clean=missing
	else
		# `|| status_rc=$?`, not a bare command substitution: under `set -e`, a plain
		# `x="$(cmd)"` assignment aborts the whole script the instant `cmd` exits
		# nonzero, which would silently take down the rest of this scan over one bad
		# worktree instead of reporting it. A failed `status` and stdout suppressed to
		# `/dev/null` both read as empty, so they must not collapse into the same
		# `clean=yes` a real clean worktree gets.
		status_rc=0
		status_out="$(git -C "$path" status --porcelain 2>/dev/null)" || status_rc=$?
		if [ "$status_rc" -ne 0 ]; then
			clean=error
		elif [ -z "$status_out" ]; then
			clean=yes
		else
			clean=no
		fi
	fi
	printf '%s\t%s\t%s\t%s\n' "$branch" "$path" "$origin" "$clean"
done
