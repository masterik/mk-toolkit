#!/usr/bin/env bash
#
# Every read-only fact a skill needs to start, in one call.
#
#   usage: facts.sh <skill> [--run] [--gh] [--base <ref>] [--range <a..b>] [--remote <name>]
#
#   facts.sh commit --run
#   facts.sh finish --base main --gh
#   facts.sh review --run --range HEAD~3..HEAD
#
# Reports; never acts. The only writes: the run dir `--run` creates under $TMPDIR, and
# the net-zero writability probe of the user dir. Never reads config: a pinned remote is
# applied by the skill re-running with `--remote <name>`.
#
# Output: `key=value` lines, some packing several pairs, then `<name>:` blocks, then a
# trailing `notes:` block. A cause that needs a sentence goes in `notes:`, never on a
# `key=value` line. The contract is internal: change it here and in every skill that
# parses it, in the same commit (ADR 0006 decision 4).
#
# Exit: 0 ok; 1 not a git repo, git failed, or --base/--range does not resolve
# (the `*_state=unresolvable` line is printed first); 2 bad usage.
#
# bash 3.2 (macOS /bin/bash) and Git Bash's bash 5: no associative arrays, no `mapfile`.

set -u

die() {
	printf 'facts.sh: %s\n' "$1" >&2
	exit "${2:-1}"
}

usage='usage: facts.sh <skill> [--run] [--gh] [--base <ref>] [--range <a..b>] [--remote <name>]'

[ $# -gt 0 ] || die "$usage" 2
skill="$1"
shift
case "$skill" in
'' | -*) die "first argument must be the skill name, got: $skill" 2 ;;
*[!a-zA-Z0-9_-]*) die "skill name may only contain [a-zA-Z0-9_-], got: $skill" 2 ;;
esac

want_run=no want_gh=no base='' range='' chosen=''
while [ $# -gt 0 ]; do
	case "$1" in
	--run) want_run=yes; shift ;;
	--gh) want_gh=yes; shift ;;
	--base | --range | --remote)
		[ $# -ge 2 ] && [ -n "$2" ] || die "$1 needs a value" 2
		# A value git would read as an option is refused before any git call sees it:
		# `--base=--output=PATH` reaches `git rev-parse` as an option, and git creates
		# PATH before failing on the rest.
		case "$2" in -*) die "$1 may not begin with '-': git would read '$2' as an option" 2 ;; esac
		case "$1" in
		--base) base="$2" ;;
		--range) range="$2" ;;
		--remote) chosen="$2" ;;
		esac
		shift 2
		;;
	*) die "unknown option: $1 ($usage)" 2 ;;
	esac
done

status_max=60
files_max=200
notes=''
note() {
	notes="$notes
  $1"
}

# Repo-wide from here on: called from a subdirectory, pathspec'd file lists covered
# only that subdirectory while the --shortstat beside them stayed repo-wide.
toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || die 'not inside a git work tree' 1
[ -n "$toplevel" ] || die 'not inside a git work tree' 1
cd "$toplevel" || die "cannot enter the work-tree top level: $toplevel" 1

if [ "$want_run" = yes ]; then
	tmp="${TMPDIR:-/tmp}"
	tmp="${tmp%/}"
	run_dir="$(mktemp -d "$tmp/mkit-$skill-XXXXXX")" || die "cannot create a run dir under $tmp" 1
	printf 'run_dir=%s\n' "$run_dir"
fi

git_dir="$(git rev-parse --absolute-git-dir)" || die 'git rev-parse --absolute-git-dir failed' 1
common_dir="$(cd "$(git rev-parse --git-common-dir)" && pwd -P)" || die 'cannot resolve the common git dir' 1
wt_list="$(git worktree list --porcelain)" || die 'git worktree list failed' 1
# The primary worktree is always listed first, from whichever worktree this runs.
primary="$(printf '%s\n' "$wt_list" | sed -n 's/^worktree //p' | head -1)"
worktrees="$(printf '%s\n' "$wt_list" | grep -c '^worktree ')"
linked=no
[ "$git_dir" != "$common_dir" ] && linked=yes
printf 'toplevel=%s\ngit_dir=%s\ncommon_dir=%s\nprimary=%s\n' "$toplevel" "$git_dir" "$common_dir" "${primary:-$toplevel}"
printf 'linked=%s\nworktrees=%s\n' "$linked" "$worktrees"

# --- the user dir: ~/.mkit, or MKIT_HOME --------------------------------------------
# Probed by writing, not by a mode check: a sandbox denies the write while the mode bits
# say yes. Net-zero: the probe file goes, and every directory this created goes again,
# deepest first, stopping at the first that gained content since.
user_dir="${MKIT_HOME:-}"
[ -z "$user_dir" ] && [ -n "${HOME:-}" ] && user_dir="$HOME/.mkit"
user_dir_writable=no
if [ -n "$user_dir" ]; then
	created='' d="$user_dir"
	while [ ! -e "$d" ]; do
		created="$created$d
"
		p="$(dirname "$d")"
		[ "$p" = "$d" ] && break
		d="$p"
	done
	if [ -z "$created" ] || mkdir -p "$user_dir" 2>/dev/null; then
		if probe="$(mktemp "$user_dir/.writable-XXXXXX" 2>/dev/null)"; then
			printf 'probe\n' >"$probe" 2>/dev/null && user_dir_writable=yes
			rm -f "$probe"
		fi
	fi
	printf '%s' "$created" | while IFS= read -r d; do
		[ -n "$d" ] && { rmdir "$d" 2>/dev/null || break; }
	done
fi
printf 'user_dir=%s\nuser_dir_writable=%s\n' "${user_dir:-none}" "$user_dir_writable"
if [ "$user_dir_writable" = no ]; then
	if [ -z "$user_dir" ]; then
		note "user_dir_writable=no — \$HOME is unset, so there is no user-scoped directory. Remedy:
  set MKIT_HOME to an absolute path, or run with HOME set. Tell the user; do not retry the write."
	else
		note "user_dir_writable=no — nothing needs that directory today, so nothing is failing
  yet; a later user-scoped write would. Remedy: run \`! mkdir -p '$user_dir'\` (a sandboxed
  session cannot create it), then add $user_dir to permissions.additionalDirectories
  (sandbox.filesystem.allowWrite grants the sandbox only). Tell the user; do not retry the write."
	fi
fi

# --- worktree origin: the lookup table in worktree.md ----------------------------------
#   primary checkout          -> nothing to tear down
#   the harness's worktree    -> hand back with ExitWorktree, never rm it
#   any other linked worktree -> git worktree remove (or `wt`, when the skill has it)
origin=primary cleanup=none
if [ "$linked" = yes ]; then
	case "$toplevel" in
	*/.claude/worktrees/*) origin=claude-code cleanup=exit-worktree ;;
	*) origin=linked cleanup=git-worktree ;;
	esac
fi
printf 'worktree_origin=%s\ncleanup_path=%s\n' "$origin" "$cleanup"

# --- branch, upstream ------------------------------------------------------------------
branch="$(git branch --show-current)" || die 'git branch --show-current failed' 1
if [ -n "$branch" ]; then
	detached=no
else
	branch="$(git rev-parse --short HEAD 2>/dev/null)" || branch=none
	detached=yes
fi
printf 'branch=%s\ndetached=%s\n' "$branch" "$detached"
upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null)" || upstream=''
if [ -n "$upstream" ]; then
	printf 'upstream=%s\npushed=yes\n' "$upstream"
else
	printf 'upstream=none\npushed=no\n'
fi

# --- remote (ADR 0005, amended by ADR 0006) ----------------------------------------------
# --remote -> origin -> the lone remote -> git's first. git lists remotes alphabetically,
# so "first" alone would pick `fork` over `origin`. Several remotes and no --remote is
# `remote_choice=ask`: the skill asks, then re-runs with --remote.
remotes="$(git remote)" || die 'git remote failed' 1
has_remote() { printf '%s\n' "$remotes" | grep -qxF -- "$1"; }
n_remotes="$(printf '%s\n' "$remotes" | grep -c .)"
remote='' source=none
if [ -n "$chosen" ] && has_remote "$chosen"; then
	remote="$chosen" source=chosen
else
	if [ -n "$chosen" ]; then
		chosen_missing="$chosen"
	fi
	if [ "$n_remotes" -eq 0 ]; then
		:
	elif has_remote origin; then
		remote=origin source=origin
	elif [ "$n_remotes" -eq 1 ]; then
		remote="$remotes" source=only
	else
		remote="$(printf '%s\n' "$remotes" | head -1)" source=first
	fi
	if [ -n "${chosen_missing:-}" ]; then
		note "remote_source=$source — --remote '$chosen_missing' is not a remote of this checkout (have: $(printf '%s\n' "$remotes" | paste -sd, - | sed 's/^$/none/')); ${remote:-no remote} is used instead."
	fi
fi
choice=no
[ "$n_remotes" -gt 1 ] && [ "$source" != chosen ] && choice=ask

# `owner/repo` of a GitHub remote, else empty: the repo `gh` is told to use, since left to
# itself it picks a set default or its own ranking. The host is anchored on what can
# precede it in a URL, so `evilgithub.com` and `github.com.evil.example` are not GitHub.
remote_repo=''
if [ -n "$remote" ]; then
	url="$(git remote get-url "$remote" 2>/dev/null)" || url=''
	url="${url%/}"
	url="${url%.git}"
	gh_re='(^|[/@])github\.com[:/]+([^/]+/[^/]+)$'
	if [[ "$url" =~ $gh_re ]]; then
		remote_repo="${BASH_REMATCH[2]}"
	fi
fi
printf 'remote=%s\nremote_source=%s\nremote_repo=%s\n' "${remote:-none}" "$source" "${remote_repo:-none}"
printf 'remotes=%s\nremote_choice=%s\n' "$(printf '%s\n' "$remotes" | paste -sd, - | sed 's/^$/none/')" "$choice"

# Always a local branch, or `unknown`: the remote's HEAD wins only when this checkout has
# that branch — a stale or remote-only name would stop cleanup on a branch never checked
# out here.
has_local() { git show-ref --verify --quiet "refs/heads/$1"; }
default_branch=''
if [ -n "$remote" ]; then
	head="$(git symbolic-ref --short "refs/remotes/$remote/HEAD" 2>/dev/null)" || head=''
	head="${head#"$remote"/}"
	[ -n "$head" ] && has_local "$head" && default_branch="$head"
fi
if [ -z "$default_branch" ]; then
	for b in main master trunk; do
		has_local "$b" && {
			default_branch="$b"
			break
		}
	done
fi
printf 'default_branch=%s\n' "${default_branch:-unknown}"
[ -n "$base" ] && printf 'base=%s\n' "$base"
if [ -n "$upstream" ]; then
	counts="$(git rev-list --left-right --count "HEAD...$upstream" 2>/dev/null)" || counts='0	0'
	set -- $counts
	printf 'ahead=%s behind=%s\n' "${1:-0}" "${2:-0}"
fi

# --- working tree ------------------------------------------------------------------------
# `.mkit/` run dirs and the old gate ledger are excluded until #50 retires `.mkit/`;
# `.mkit/config.toml` is the user's own file and stays visible.
# A failing `git status` is an error, never an empty answer: `clean=yes` is what a skill
# skips work on.
porcelain="$(git status --porcelain -- . ':(exclude,glob).mkit/*/**' ':(exclude).mkit/gate.jsonl')" ||
	die 'git status failed, so there is no answer about the working tree (an empty one would read as clean)' 1
count() { printf '%s' "$1" | grep -c . || true; }
staged=0 unstaged=0 untracked=0 total=0
if [ -n "$porcelain" ]; then
	total="$(count "$porcelain")"
	untracked="$(printf '%s\n' "$porcelain" | grep -c '^??' || true)"
	staged="$(printf '%s\n' "$porcelain" | grep -v '^??' | grep -c '^[^ ]' || true)"
	unstaged="$(printf '%s\n' "$porcelain" | grep -v '^??' | grep -c '^.[^ ]' || true)"
fi
conflicted="$(git diff --name-only --diff-filter=U)" || die 'git diff --diff-filter=U failed' 1
clean=no
[ "$total" -eq 0 ] && clean=yes
printf 'clean=%s\n' "$clean"
printf 'staged=%s unstaged=%s untracked=%s conflicted=%s\n' "$staged" "$unstaged" "$untracked" "$(count "$conflicted")"
if [ "$total" -gt 0 ]; then
	printf 'status:\n'
	printf '%s\n' "$porcelain" | head -n "$status_max"
	[ "$total" -gt "$status_max" ] &&
		printf '... %d more entries not shown (status_max=%d)\n' "$((total - status_max))" "$status_max"
fi

# One diff scope: a stat line, a count, and the list. The same pathspec on both, or the
# stat counts a file the list drops. A git failure is fatal: `stat=none files=0` is the
# honest answer for an empty scope, so a failure printing it is a wrong answer that reads
# exactly like a right one.
noise=(. ':(exclude)*.lock' ':(exclude)*.snap')
print_list() { # <label> <list>
	local n
	n="$(count "$2")"
	printf '%s_files=%s\n' "$1" "$n"
	[ "$n" -gt 0 ] || return 0
	printf '%s_file_list:\n' "$1"
	printf '%s\n' "$2" | head -n "$files_max"
	[ "$n" -gt "$files_max" ] &&
		printf '... %d more files not shown (files_max=%d)\n' "$((n - files_max))" "$files_max"
	return 0
}
scope() { # <label> [git diff options and --end-of-options <rev>]
	local label="$1" stat list
	shift
	stat="$(git diff --shortstat "$@" -- "${noise[@]}")" || die "git diff --shortstat $* failed" 1
	list="$(git diff --name-only "$@" -- "${noise[@]}")" || die "git diff --name-only $* failed" 1
	stat="$(printf '%s' "$stat" | sed 's/^ *//')"
	printf '%s_stat=%s\n' "$label" "${stat:-none}"
	print_list "$label" "$list"
}

if [ -n "$range" ]; then
	# An unresolvable range would otherwise print `range_stat=none range_files=0` — a
	# range with nothing in it. Say so, and fail.
	if ! git rev-list --count --end-of-options "$range" -- >/dev/null 2>&1; then
		printf 'range_state=unresolvable\n'
		die "--range does not resolve: $range" 1
	fi
	scope range --end-of-options "$range"
	printf 'range_state=ok\n'
elif [ "$clean" = no ]; then
	# Both stats, always: a bare `git diff --shortstat` is empty on a fully staged tree,
	# which reads exactly like a clean one.
	scope unstaged
	scope staged --cached
	# `git diff` never lists an untracked file: its own block, or a commit or review scope
	# silently omits every new file.
	list="$(git ls-files --others --exclude-standard -- "${noise[@]}" ':(exclude).mkit')" ||
		die 'git ls-files --others failed' 1
	print_list untracked "$list"
fi

if [ -n "$base" ]; then
	# Unresolvable used to fall through and exit 0: `finish`/`pr` could not tell it from a
	# base with nothing on it.
	if ! git rev-parse --verify --quiet --end-of-options "$base" >/dev/null 2>&1; then
		printf 'base_state=unresolvable\n'
		die "--base does not resolve to a commit: $base" 1
	fi
	printf 'base_state=ok\n'
	ahead_of_base="$(git rev-list --count "$base..HEAD")" || die "git rev-list $base..HEAD failed" 1
	printf 'commits_ahead_of_base=%s\n' "$ahead_of_base"
	if [ "$ahead_of_base" -gt 0 ]; then
		printf 'commits:\n'
		git log --oneline "$base..HEAD"
	fi
	# Unconditionally: `pr` reads base_stat after `commit` ran, when the count can be 0
	# while the work is real.
	scope base --end-of-options "$base"
	if git merge-base --is-ancestor "$base" HEAD 2>/dev/null; then
		printf 'ff_from_base=yes\n'
	else
		printf 'ff_from_base=no\n'
	fi
fi

codeowners=none
for c in .github/CODEOWNERS CODEOWNERS docs/CODEOWNERS; do
	[ -f "$toplevel/$c" ] && {
		codeowners="$toplevel/$c"
		break
	}
done
printf 'codeowners=%s\n' "$codeowners"

# --- the PR for this branch (--gh) ----------------------------------------------------------
# One value per cause: only `pr=none` justifies opening one. `gh` is told the resolved
# remote's repo; a detached HEAD has no branch to have a PR, so it is not looked up.
if [ "$want_gh" = yes ]; then
	if ! command -v gh >/dev/null 2>&1; then
		printf 'pr=gh-missing\n'
	else
		pr=''
		if [ "$detached" = no ]; then
			if [ -n "$remote_repo" ]; then
				set -- pr view "$branch" --repo "$remote_repo"
			else
				set -- pr view
			fi
			pr="$(gh "$@" --json url,state,isDraft --jq '[.url, .state, (.isDraft|tostring)] | @tsv' 2>/dev/null)" || pr=''
		fi
		if [ -n "$pr" ]; then
			printf '%s\n' "$pr" | head -1 | awk -F'\t' '{ printf "pr=%s pr_state=%s pr_draft=%s\n", $1, $2, $3 }'
		elif ! gh auth status >/dev/null 2>&1; then
			printf 'pr=gh-unauthenticated\n'
		elif [ -z "$remote" ]; then
			printf 'pr=no-remote\n'
		else
			printf 'pr=none\n'
		fi
	fi
fi

if [ -n "$notes" ]; then
	printf 'notes:%s\n' "$notes"
fi
exit 0
