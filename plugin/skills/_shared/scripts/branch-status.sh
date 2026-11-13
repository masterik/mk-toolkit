#!/usr/bin/env bash
#
# Classify every local branch and worktree for a repo-wide cleanup, with one batched
# `gh` call.
#
#   usage: branch-status.sh [--default <name>] [--develop <name>] [--keep <name>]...
#                           [--remote <name>] [--no-fetch] [--no-gh]
#
# Reports candidates; never deletes, removes or pushes. `git fetch --prune` is the one
# mutation: it updates only this repo's remote-tracking refs, and makes `upstream=gone`
# mean something. Never reads config: the default branch, remote and remote repo come
# from `facts.sh` beside this file (one resolver), `--default`/`--remote` override, and
# `--keep` names what the repo's cleanup config keeps.
#
# Output: `key=value` lines, then the `branches:` and `worktrees:` tables (tab-separated),
# then `notes:` when there is something to say.
#
#   branches:  name  class  upstream  merged_into  pr
#     class    protected | current | merged | merged-pr | open-pr | closed-pr | gone |
#              unpushed | tracking — first match wins, in that order
#     upstream none (never pushed) | gone (deleted remotely) | <remote>/<branch>
#     merged_into  default/develop it is an ancestor of, comma-joined, or `-`
#     pr       open#12 | merged#12 | closed#12 (the newest PR for that head), or `-`
#   worktrees: branch (HEAD when detached)  path  origin  clean
#     origin   primary | claude-code | linked
#     clean    yes | no | missing (path gone) | error (status failed; never collapsed into yes)
#
# `protected=`, `keep=`, `keep_unknown=` are comma-joined for reading; a branch name may
# contain a comma, so the `class` column is the exact answer per branch.
#
# Exit: 0 ok; 1 not a git repo or facts.sh failed; 2 bad usage or a --default with no
# local branch. bash 3.2 compatible.

set -u

die() {
	printf 'branch-status.sh: %s\n' "$1" >&2
	exit "${2:-1}"
}

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" || die 'cannot locate this script' 1

def='' develop='' keep='' chosen='' no_fetch=no no_gh=no
nl='
'
while [ $# -gt 0 ]; do
	case "$1" in
	--default | --develop | --keep | --remote)
		[ $# -ge 2 ] && [ -n "$2" ] || die "$1 needs a value" 2
		case "$2" in -*) die "$1 may not begin with '-': got '$2'" 2 ;; esac
		case "$1" in
		--default) def="$2" ;;
		--develop) develop="$2" ;;
		--keep) keep="$keep$2$nl" ;;
		--remote) chosen="$2" ;;
		esac
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
	*) die "unknown option: $1" 2 ;;
	esac
done

toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || die 'not inside a git work tree' 1
[ -n "$toplevel" ] || die 'not inside a git work tree' 1
cd "$toplevel" || die "cannot enter $toplevel" 1

# The remote and the default branch, resolved once, by facts.sh.
if [ -n "$chosen" ]; then
	facts="$(bash "$here/facts.sh" cleanup --remote "$chosen")" || die 'facts.sh failed' 1
else
	facts="$(bash "$here/facts.sh" cleanup)" || die 'facts.sh failed' 1
fi
fact() { printf '%s\n' "$facts" | sed -n "s/^$1=//p" | head -1; }
remote="$(fact remote)"
[ "$remote" = none ] && remote=''
remote_repo="$(fact remote_repo)"
[ "$remote_repo" = none ] && remote_repo=''
[ -n "$def" ] || def="$(fact default_branch)"

has_local() { git show-ref --verify --quiet "refs/heads/$1"; }
in_list() { printf '%s' "$2" | grep -qxF -- "$1"; } # <name> <newline list>
join() { local j; j="$(printf '%s' "$1" | paste -sd, -)"; printf '%s' "${j:-none}"; }

if [ -z "$def" ] || [ "$def" = unknown ]; then
	die 'no default branch could be resolved here; pass --default <name>' 2
fi
has_local "$def" || die "--default branch does not exist locally: $def" 2

notes=''
# Only a local branch counts: a develop that exists only as origin/develop is not one of
# this repo's branches to keep.
if [ -n "$develop" ]; then
	if ! has_local "$develop"; then
		notes="$notes- develop: --develop '$develop' is not a local branch here; nothing extra is protected$nl"
		develop=none
	fi
else
	develop=none
	for cand in develop development dev; do
		[ "$cand" = "$def" ] && continue
		has_local "$cand" && {
			develop="$cand"
			break
		}
	done
fi

# Protected: the default first and unconditionally, then develop, then each kept name that
# exists locally. A kept name with no local branch is reported, never refused: a keep
# list travels with the repo.
protected="$def$nl"
[ "$develop" != none ] && ! in_list "$develop" "$protected" && protected="$protected$develop$nl"
keep_unknown=''
while IFS= read -r k; do
	[ -n "$k" ] || continue
	if has_local "$k"; then
		in_list "$k" "$protected" || protected="$protected$k$nl"
	else
		in_list "$k" "$keep_unknown" || keep_unknown="$keep_unknown$k$nl"
	fi
done <<EOF
$keep
EOF

printf 'default=%s\ndevelop=%s\n' "$def" "$develop"
printf 'protected=%s\nkeep=%s\nkeep_unknown=%s\n' "$(join "$protected")" "$(join "$keep")" "$(join "$keep_unknown")"
printf 'remote=%s\n' "${remote:-none}"
if [ -n "$chosen" ] && [ "$chosen" != "$remote" ]; then
	notes="$notes- remote: --remote '$chosen' is not a remote of this checkout; ${remote:-no remote} is used instead$nl"
fi

if [ "$no_fetch" = yes ]; then
	fetch=skipped
elif [ -z "$remote" ]; then
	fetch=no-remote
elif git fetch "$remote" --prune -q >/dev/null 2>&1; then
	fetch=ok
else
	fetch=failed
fi
printf 'fetch=%s\n' "$fetch"

# One batched `gh` call, never one per branch. The newest PR (by number) per head wins:
# a branch PR'd, closed, and PR'd again is judged by the latest.
prs=''
if [ "$no_gh" = yes ]; then
	gh_state=skipped
elif [ -z "$remote" ]; then
	gh_state=no-remote
elif ! command -v gh >/dev/null 2>&1; then
	gh_state=gh-missing
elif ! gh auth status >/dev/null 2>&1; then
	gh_state=gh-unauthenticated
else
	if [ -n "$remote_repo" ]; then
		set -- --repo "$remote_repo"
	else
		set --
	fi
	if raw="$(gh pr list --state all --limit 500 ${1+"$@"} \
		--json headRefName,number,state,headRefOid \
		--jq '.[] | [.headRefName, (.number|tostring), .state, .headRefOid] | @tsv' 2>/dev/null)"; then
		gh_state=ok
		prs="$(printf '%s\n' "$raw" | awk -F'\t' 'NF >= 3 {
			if (!($1 in n) || $2 + 0 > n[$1] + 0) { n[$1] = $2; s[$1] = $3; o[$1] = $4 }
		} END { for (b in n) printf "%s\t%s\t%s\t%s\n", b, n[b], tolower(s[b]), o[b] }')"
	else
		gh_state=gh-error
	fi
fi
printf 'gh=%s\n' "$gh_state"

wt_raw="$(git worktree list --porcelain 2>/dev/null)" && worktrees_state=ok || worktrees_state=unreadable
printf 'worktrees_state=%s\n' "$worktrees_state"

# A `merged` PR is trusted only when its head commit is this branch's tip or an ancestor
# of it: a reused branch name must never delete new work. A missing oid falls back to the
# name match.
oid_is_local() { # <branch> <oid>
	local tip
	[ -n "$2" ] || return 0
	tip="$(git rev-parse -q --verify "refs/heads/$1")" || return 1
	[ "$2" = "$tip" ] && return 0
	git cat-file -e "$2^{commit}" 2>/dev/null || return 1
	git merge-base --is-ancestor "refs/heads/$1" "$2"
}

current="$(git branch --show-current 2>/dev/null)" || current=''
printf 'branches:\n'
git for-each-ref refs/heads --format='%(refname:lstrip=2)%09%(upstream)%09%(upstream:track)' |
	while IFS='	' read -r name up track; do
		[ -n "$name" ] || continue
		if [ -z "$up" ]; then
			upstream=none
		else
			case "$track" in
			*gone*) upstream=gone ;;
			*) upstream="${up#refs/remotes/}" ;;
			esac
		fi
		# Default and develop only, each tested by name: a kept branch is never a merge
		# target (that would turn "delete less" into "delete more"), and a comma-joined
		# list would test fragments of a name containing a comma.
		merged_into=''
		for p in "$def" "$develop"; do
			[ "$p" = none ] && continue
			if git merge-base --is-ancestor "refs/heads/$name" "refs/heads/$p" 2>/dev/null; then
				merged_into="${merged_into:+$merged_into,}$p"
			fi
		done
		pr='' pr_state='' pr_oid=''
		if [ -n "$prs" ]; then
			row="$(printf '%s\n' "$prs" | awk -F'\t' -v b="$name" '$1 == b { print; exit }')"
			if [ -n "$row" ]; then
				pr_state="$(printf '%s' "$row" | cut -f3)"
				pr_oid="$(printf '%s' "$row" | cut -f4)"
				pr="$pr_state#$(printf '%s' "$row" | cut -f2)"
			fi
		fi
		if in_list "$name" "$protected"; then
			class=protected
		elif [ "$name" = "$current" ]; then
			class=current
		elif [ -n "$merged_into" ]; then
			class=merged
		elif [ "$pr_state" = merged ] && oid_is_local "$name" "$pr_oid"; then
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
		printf '%s\t%s\t%s\t%s\t%s\n' "$name" "$class" "$upstream" "${merged_into:--}" "${pr:--}"
	done

printf 'worktrees:\n'
if [ "$worktrees_state" = ok ]; then
	primary=''
	printf '%s\n' "$wt_raw" | while IFS= read -r line; do
		case "$line" in
		'worktree '*)
			path="${line#worktree }"
			[ -n "$primary" ] || primary="$path"
			continue
			;;
		'branch '*) wbranch="${line#branch refs/heads/}" ;;
		detached) wbranch=HEAD ;;
		*) continue ;;
		esac
		origin=linked
		if [ "$path" = "$primary" ]; then
			origin=primary
		else
			case "$path" in */.claude/worktrees/*) origin=claude-code ;; esac
		fi
		# A failed status and an empty one never collapse into the same answer.
		if [ ! -d "$path" ]; then
			clean=missing
		elif st="$(git -C "$path" status --porcelain 2>/dev/null)"; then
			if [ -z "$st" ]; then clean=yes; else clean=no; fi
		else
			clean=error
		fi
		printf '%s\t%s\t%s\t%s\n' "$wbranch" "$path" "$origin" "$clean"
	done
fi

if [ -n "$notes" ]; then
	printf 'notes:\n%s' "$notes"
fi
exit 0
