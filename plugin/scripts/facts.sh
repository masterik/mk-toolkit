#!/usr/bin/env bash
#
# Gather every read-only fact an mkit skill needs to start, in one call, and open the
# run directory while we are here.
#
#   usage: facts.sh <skill> [--base <branch>] [--range <range>] [--gh]
#                           [--no-run] [--status-max N] [--files-max N]
#
#   facts.sh commit
#   facts.sh finish --base main
#   facts.sh review --range HEAD
#   facts.sh pr --base main --gh
#
# Why a script: every fact below is mechanical, several are easy to get subtly wrong
# (an unresolved reference path, a bare `git diff --stat` that reads as a clean tree
# when the work is fully staged, `git checkout <base>` inside a linked worktree), and
# the classification at the end — which kind of worktree this is — is a lookup table,
# not a judgement.
#
# It reports. It never acts: no staging, no merging, no `wt` invocation, and the only
# things it writes are the run directory it opens and the one `.mkit/` line `run-open.sh`
# adds to the common-dir exclude so that directory does not show up in `git status`. The
# user-dir writability probe is net-zero by construction — see `mkit_user_dir_writable`.
#
# Output is `key=value` lines, then optional `block:` sections. Stable and greppable.
# Exit: 0 ok, 1 not a git repo, 2 bad usage.

set -euo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

skill=""
base=""
range=""
want_gh=no
want_run=yes
status_max=60
files_max=200

[ $# -gt 0 ] || mkit_die 'usage: facts.sh <skill> [--base <branch>] [--range <range>] [--gh] [--no-run]' 2
skill="$1"
shift
case "$skill" in
--*) mkit_die "first argument must be the skill name, got: $skill" 2 ;;
esac
mkit_check_slug "$skill"

while [ $# -gt 0 ]; do
	case "$1" in
	--base)
		base="${2:-}"
		[ -n "$base" ] || mkit_die '--base needs a branch name' 2
		shift 2
		;;
	--range)
		range="${2:-}"
		[ -n "$range" ] || mkit_die '--range needs a git range' 2
		shift 2
		;;
	--gh)
		want_gh=yes
		shift
		;;
	--no-run)
		want_run=no
		shift
		;;
	--status-max)
		status_max="${2:-60}"
		shift 2
		;;
	--files-max)
		files_max="${2:-200}"
		shift 2
		;;
	*) mkit_die "unknown option: $1" 2 ;;
	esac
done

mkit_require_repo

# --- where things are -------------------------------------------------------------
# The reference paths come from this script's own location, which is what retires
# "resolve ${CLAUDE_PLUGIN_ROOT} before you put a path in a brief" from the skills.
plugin_root="$(mkit_plugin_root)"
if [ "$want_run" = yes ]; then
	run_dir="$("$plugin_root/scripts/run-open.sh" "$skill")"
	printf 'run=%s\n' "$run_dir"
fi
printf 'plugin=%s\nrefs=%s\n' "$plugin_root" "$(mkit_refs_dir)"

toplevel="$(git rev-parse --show-toplevel)"
git_dir="$(git rev-parse --absolute-git-dir)"
# Run repo-wide from here on, as gate-detect.sh does and quality-gate.md requires. Called
# from a subdirectory, the pathspec'd file lists below covered only that subdirectory while
# the --shortstat beside them stayed repo-wide, so the two keys contradicted each other and
# a skill deciding commit boundaries from the list silently lost files.
cd "$toplevel" || mkit_die "cannot enter repo root: $toplevel" 1
common_dir="$(cd "$(git rev-parse --git-common-dir)" && pwd)"
primary="$(mkit_primary_worktree)"
printf 'toplevel=%s\ngit_dir=%s\ncommon_dir=%s\nprimary=%s\n' \
	"$toplevel" "$git_dir" "$common_dir" "$primary"

linked=no
[ "$git_dir" != "$common_dir" ] && linked=yes
printf 'linked=%s\nworktrees=%s\n' "$linked" \
	"$(git worktree list --porcelain | grep -c '^worktree ')"

# --- where a write is allowed to land ---------------------------------------------
# Three boundaries can refuse a write on this machine and none of them announce
# themselves: the OS sandbox, the auto-mode classifier, and the worktree-isolation guard.
# Reported here, at the first call, so a refusal is a starting fact instead of a mid-run
# `Operation not permitted`. `../_shared/references/output-discipline.md` states the rule
# these three keys support.
#
# `tmp=` is the home for anything that dies with the command; `run=` (above) for anything
# a later step or session reads. Nothing of mkit's ever reaches the user's own content: the
# single in-tree write is the ignored `.mkit/` that holds `run=` itself.
printf 'tmp=%s\n' "${TMPDIR:-/tmp}"

# `.mkit/` unignored is not a cosmetic problem: `git worktree remove` refuses, `git add -A`
# would commit run artefacts, and the gate fingerprint sees a directory that changes while
# the gate runs. `run-open.sh` tries to add the rule to the common-dir exclude on every
# call; a worktree-isolated session cannot reach that file, so the answer is a fact.
notes=""
if mkit_run_ignored; then
	printf 'run_ignored=yes\n'
else
	printf 'run_ignored=no\n'
	notes="$notes
  run_ignored=no — .mkit/ is not ignored here, so a staging step would sweep run
  artefacts into a commit and \`git worktree remove\` would refuse. Do not stage while
  this says no. Remedy, from the main checkout: add \`.mkit/\` to
  $common_dir/info/exclude, or to .gitignore."
fi

# The repo's committed config, if any. `mkit init` writes it, `mkit repo profile` reads
# it, and every skill runs fine without it — config is an input, never a permission
# (ADR 0001 decision 3), so `absent` is a normal state and never a note.
#
# The state worth a sentence is `shadowed`: a repo set up before the config existed
# carries a directory-only `.mkit/` rule, under which the file can be written and then
# silently never travels to a fresh clone — the one property it exists for. Reported here
# because it is a fact about this checkout, discovered at the first call, and because the
# remedy differs by which file carries the rule.
printf 'config=%s\n' "$(mkit_config_path)"
config_file="$(mkit_config_path)"
if git ls-files --error-unmatch .mkit/config.toml >/dev/null 2>&1; then
	printf 'config_state=tracked\n'
elif ! mkit_config_committable; then
	printf 'config_state=shadowed\n'
	notes="$notes
  config_state=shadowed — .mkit/config.toml is ignored in this checkout, so \`mkit init\`
  would write a file that never reaches a fresh clone. Remedy: $(mkit_config_ignored_remedy)."
elif [ -f "$config_file" ]; then
	printf 'config_state=untracked\n'
else
	printf 'config_state=absent\n'
fi

# The user-scoped state directory. Nothing writes to it today — it emptied when the
# `SessionStart` hook went — but it is still where user-scoped state will land, and an
# unwritable one is reported with the one remedy that works: `~/.claude/mkit` was
# unfixable by configuration, which is why the directory moved (docs/adr/0002).
printf 'user_dir=%s\n' "$(mkit_user_dir)"
if mkit_user_dir_writable; then
	printf 'user_dir_writable=yes\n'
else
	printf 'user_dir_writable=no\n'
	notes="$notes
  user_dir_writable=no — nothing needs that directory today, so nothing is failing
  yet; a later user-scoped write would. Remedy: $(mkit_user_dir_remedy).
  Tell the user; do not retry the write."
fi

# --- the git invocation a skill parses --------------------------------------------
# One key, absolute, no spaces — the invocation is `$git_bin -C $toplevel`, spelled out in
# `../_shared/references/git-safety.md`. Emitted as a fact because a skill must not
# assemble it from a guess, and for two measured reasons:
#
#   - a PreToolUse hook on this machine rewrites `git status --short` into a wrapper that
#     reshapes output for reading, so a summarized status can reach a skill looking
#     exactly like the tree it is judging;
#   - the worktree-isolation guard refuses any launcher it cannot read a git target
#     through, while an absolute binary path is not rewritten at all — so the command text
#     reaches the guard as plain git.
git_bin="$(command -v git 2>/dev/null || true)"
case "$git_bin" in
/*) ;;
'') git_bin=git ;;
*) git_bin="$(cd "$(dirname "$git_bin")" 2>/dev/null && pwd)/git" ;;
esac
printf 'git_bin=%s\n' "$git_bin"

# --- worktree origin: the lookup table from worktree.md, run here ------------------
wt_config=none
for c in "$toplevel/.config/wt.toml" "${XDG_CONFIG_HOME:-$HOME/.config}/worktrunk/config.toml"; do
	[ -f "$c" ] && {
		wt_config="$c"
		break
	}
done

# Worktree classification, and the cleanup path it implies.
#
# Tested, not assumed: `wt list` enumerates *every* git worktree in the repo, not only
# the ones worktrunk created — so its output cannot tell a worktrunk worktree from a
# hand-made one, and neither can the path (worktrunk's `worktree-path` template is
# fully configurable). What actually decides cleanup is narrower:
#
#   the harness's own worktree  -> hand back with the ExitWorktree tool, never rm it
#   any other linked worktree, wt present -> `wt merge` (honors the user's hooks/config)
#   any other linked worktree, no wt      -> plain `git worktree remove`
#   the primary checkout                  -> nothing to tear down
#
# `wt list` is still worth one call inside a linked worktree — it proves wt can see
# this repo at all — and costs ~350 ms against git's ~20 ms, so it runs only there.
origin=primary
cleanup=none
wt_lists_this=unknown
if [ "$linked" = yes ]; then
	case "$toplevel" in
	*/.claude/worktrees/*)
		origin=claude-code
		cleanup=exit-worktree
		;;
	*)
		origin=linked
		cleanup=git-worktree
		if items="$(mkit_wt_items)" && [ -n "$items" ]; then
			if printf '%s' "$items" |
				jq -e --arg p "$toplevel" 'any(.[]; (.path // .worktree.path) == $p)' >/dev/null 2>&1; then
				wt_lists_this=yes
				cleanup=wt
			else
				wt_lists_this=no
			fi
		fi
		;;
	esac
fi
printf 'worktree_origin=%s\ncleanup_path=%s\nwt_lists_this=%s\nwt_config=%s\n' \
	"$origin" "$cleanup" "$wt_lists_this" "$wt_config"
# Advisory: `wt` is usually also a shell function, and the agent's own shell may have it
# even when this script does not. `wt_bin=none` is not proof the agent cannot call wt.
printf 'wt_bin=%s\n' "$(mkit_wt_bin || echo none)"

# --- branch, upstream, remote -----------------------------------------------------
branch="$(git branch --show-current)"
if [ -n "$branch" ]; then
	printf 'branch=%s\ndetached=no\n' "$branch"
else
	printf 'branch=%s\ndetached=yes\n' "$(git rev-parse --short HEAD)"
fi

upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
printf 'upstream=%s\npushed=%s\n' "${upstream:-none}" "$([ -n "$upstream" ] && echo yes || echo no)"

remote="$(git remote | head -1)"
printf 'remote=%s\n' "${remote:-none}"

default_branch=""
if [ -n "$remote" ]; then
	default_branch="$(git symbolic-ref --short "refs/remotes/$remote/HEAD" 2>/dev/null | sed "s|^$remote/||" || true)"
fi
if [ -z "$default_branch" ]; then
	for b in main master trunk; do
		git show-ref --verify --quiet "refs/heads/$b" && {
			default_branch="$b"
			break
		}
	done
fi
printf 'default_branch=%s\n' "${default_branch:-unknown}"
[ -n "$base" ] && printf 'base=%s\n' "$base"

if [ -n "$upstream" ]; then
	set -- $(git rev-list --left-right --count "HEAD...$upstream" 2>/dev/null || echo "0 0")
	printf 'ahead=%s behind=%s\n' "${1:-0}" "${2:-0}"
fi

# --- working tree ------------------------------------------------------------------
# `:(exclude).mkit` on every enumeration of the user's work, for the same reason
# mkit_tree_fingerprint carries it: the run directory now lives *inside* the working
# directory, so without this the scratch mkit just created is reported back to the skill
# as the user's own change. `run_ignored=no` is exactly the session that hits it — an
# isolated one, which cannot write the exclude file — and there the damage is a `clean=no`
# and an `untracked_file_list` naming mkit's own logs, which is what a commit or review
# scope is then built from. The reserved root is never the user's work by construction.
porcelain="$(git status --porcelain -- . ':(exclude).mkit')"
conflicted="$(git diff --name-only --diff-filter=U | grep -c . || true)"
printf 'clean=%s\n' "$([ -z "$porcelain" ] && echo yes || echo no)"
printf 'staged=%s unstaged=%s untracked=%s conflicted=%s\n' \
	"$(printf '%s\n' "$porcelain" | awk 'NF && substr($0,1,1) !~ /[ ?]/' | grep -c . || true)" \
	"$(printf '%s\n' "$porcelain" | awk 'NF && substr($0,2,1) !~ /[ ?]/' | grep -c . || true)" \
	"$(printf '%s\n' "$porcelain" | awk '/^\?\?/' | grep -c . || true)" \
	"$conflicted"

if [ -n "$porcelain" ]; then
	n="$(printf '%s\n' "$porcelain" | grep -c .)"
	printf 'status:\n'
	printf '%s\n' "$porcelain" | head -"$status_max"
	[ "$n" -gt "$status_max" ] && printf '... %d more entries not shown (status_max=%d)\n' \
		"$((n - status_max))" "$status_max"
fi

# --- diff scope --------------------------------------------------------------------
# Both stats, always. A bare `git diff --shortstat` reports nothing when the work is
# fully staged, which reads exactly like a clean tree — the single most expensive
# misread in this bundle, and the reason this is one script and not two commands.
emit_scope() {
	local label="$1" rev="$2"
	local stat
	stat="$(git diff $rev --shortstat | sed 's/^ *//')"
	printf '%s_stat=%s\n' "$label" "${stat:-none}"
	local list count
	list="$(git diff $rev --name-only -- . ':(exclude)*.lock' ':(exclude)*.snap')"
	count="$(printf '%s\n' "$list" | grep -c . || true)"
	printf '%s_files=%s\n' "$label" "$count"
	if [ "$count" -gt 0 ]; then
		printf '%s_file_list:\n' "$label"
		printf '%s\n' "$list" | head -"$files_max"
		[ "$count" -gt "$files_max" ] && printf '... %d more files not shown (files_max=%d)\n' \
			"$((count - files_max))" "$files_max"
	fi
	return 0
}

# `git diff` never lists an untracked file, so a dirty tree containing new files reported
# `untracked=6` beside a file list naming none of them — a review or commit scope that
# silently omits every new implementation file. Same shape, its own keys.
emit_untracked() {
	local list count
	list="$(git ls-files --others --exclude-standard \
		-- . ':(exclude)*.lock' ':(exclude)*.snap' ':(exclude).mkit')"
	count="$(printf '%s\n' "$list" | grep -c . || true)"
	printf 'untracked_files=%s\n' "$count"
	if [ "$count" -gt 0 ]; then
		printf 'untracked_file_list:\n'
		printf '%s\n' "$list" | head -"$files_max"
		[ "$count" -gt "$files_max" ] && printf '... %d more files not shown (files_max=%d)\n' \
			"$((count - files_max))" "$files_max"
	fi
	return 0
}

if [ -n "$range" ]; then
	emit_scope range "$range"
elif [ -n "$porcelain" ]; then
	emit_scope unstaged ""
	emit_scope staged "--cached"
	emit_untracked
fi

# --- base..HEAD, for the finishers -------------------------------------------------
# An unresolvable --base used to fall through silently and still exit 0, so `finish`/`pr`
# got a fact set with no commits_ahead_of_base or ff_from_base and no way to tell that
# from a base with nothing on it. Say so on stdout, and fail.
if [ -n "$base" ] && ! git rev-parse --verify --quiet "$base" >/dev/null; then
	printf 'base_state=unresolvable\n'
	mkit_die "--base does not resolve to a commit: $base" 1
fi
if [ -n "$base" ] && git rev-parse --verify --quiet "$base" >/dev/null; then
	printf 'base_state=ok\n'
	ahead_of_base="$(git rev-list --count "$base..HEAD")"
	printf 'commits_ahead_of_base=%s\n' "$ahead_of_base"
	if [ "$ahead_of_base" -gt 0 ]; then
		printf 'commits:\n'
		git log --oneline "$base..HEAD"
		emit_scope base "$base"
	fi
	git merge-base --is-ancestor "$base" HEAD 2>/dev/null &&
		printf 'ff_from_base=yes\n' || printf 'ff_from_base=no\n'
fi

# --- reviewers / GitHub ------------------------------------------------------------
codeowners=none
for c in "$toplevel/.github/CODEOWNERS" "$toplevel/CODEOWNERS" "$toplevel/docs/CODEOWNERS"; do
	[ -f "$c" ] && {
		codeowners="$c"
		break
	}
done
printf 'codeowners=%s\n' "$codeowners"

if [ "$want_gh" = yes ]; then
	# `pr=none` used to mean four different things — no PR, gh unauthenticated, no remote,
	# or jq missing — and only the first justifies opening one. Name which it was.
	if ! command -v gh >/dev/null 2>&1; then
		printf 'pr=gh-missing\n'
	elif ! command -v jq >/dev/null 2>&1; then
		printf 'pr=jq-missing\n'
	else
		pr_json="$(gh pr view --json url,state,isDraft 2>/dev/null || true)"
		if [ -n "$pr_json" ]; then
			printf 'pr=%s pr_state=%s pr_draft=%s\n' \
				"$(printf '%s' "$pr_json" | jq -r .url)" \
				"$(printf '%s' "$pr_json" | jq -r .state)" \
				"$(printf '%s' "$pr_json" | jq -r .isDraft)"
		elif ! gh auth status >/dev/null 2>&1; then
			printf 'pr=gh-unauthenticated\n'
		elif [ -z "$remote" ]; then
			printf 'pr=no-remote\n'
		else
			printf 'pr=none\n'
		fi
	fi
fi

# --- causes and remedies, last -----------------------------------------------------
# Values with spaces never go on a key=value line: several of those lines pack more than
# one pair, so a reader splitting on whitespace would mis-parse one. Anything that needs a
# sentence goes here, after every key, and every sentence names a remedy that works.
if [ -n "$notes" ]; then
	printf 'notes:%s\n' "$notes"
fi
