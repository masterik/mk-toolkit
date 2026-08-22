#!/usr/bin/env bash
#
# Shared helpers for the mkit scripts. Sourced, never executed.
#
# Every function here is mechanical: it reports a fact or fails loudly. No helper
# decides anything a SKILL.md is responsible for deciding.

# shellcheck shell=bash

mkit_die() {
	printf 'mkit: %s\n' "$1" >&2
	exit "${2:-1}"
}

# Absolute path of the plugin checkout, derived from this file's own location.
# This is what removes "resolve ${CLAUDE_PLUGIN_ROOT} before handing a path to a
# subagent" from the skills: the script already knows where it lives.
mkit_plugin_root() {
	local here
	here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
	printf '%s\n' "${here%/scripts}"
}

mkit_refs_dir() {
	printf '%s/skills/_shared/references\n' "$(mkit_plugin_root)"
}

# Fail unless we are inside a work tree. Every mkit script needs this.
mkit_require_repo() {
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 ||
		mkit_die 'not inside a git repository' 1
}

# Absolute path of this repo's mkit directory. Creating it is the caller's business;
# this only resolves it, or fails.
#
# --absolute-git-dir, never a relative `.git/...`: the path is handed to subagents and
# reused across shells, where a relative path would resolve somewhere else. Inside the
# git dir it is never committed, never shows up in `git status`, and a linked worktree
# gets its own.
mkit_dir_or_die() {
	local git_dir
	git_dir="$(git rev-parse --absolute-git-dir 2>/dev/null)" ||
		mkit_die 'not inside a git repository' 1
	printf '%s/mkit\n' "$git_dir"
}

# A path component that cannot traverse or glob.
mkit_check_slug() {
	case "$1" in
	'') mkit_die "empty name where a name is required" 2 ;;
	*[!a-zA-Z0-9_-]*) mkit_die "name may only contain [a-zA-Z0-9_-], got: $1" 2 ;;
	esac
}

# ripgrep when present, grep -E otherwise. rg is preferred (faster, and its -m
# cap bounds output at the source), but the scripts must not hard-require it.
#   mkit_search <max-count> <pattern> <file>
mkit_search() {
	local max="$1" pat="$2" file="$3"
	if command -v rg >/dev/null 2>&1; then
		rg --no-heading --line-number --color never --max-count "$max" -e "$pat" -- "$file" 2>/dev/null
	else
		grep -n -E -m "$max" -e "$pat" -- "$file" 2>/dev/null
	fi
}

# Never call `rtk` from a script. It reshapes output for an agent to read (it
# strips the leading space from `git diff --stat`, for one), which is exactly
# what a parser must not tolerate. Scripts consume --porcelain, --shortstat/--name-only
# and --format=json, and do their own compaction; rtk stays at the agent's own
# command boundary.

# Absolute path of the real `wt` binary, or empty.
#
# `command -v wt` is not enough: worktrunk's shell integration installs `wt` as a shell
# function (it has to, to cd the parent shell), and an exported function makes
# `command -v` answer "wt" with no path. Walk PATH for an actual executable instead.
mkit_wt_bin() {
	local d IFS=:
	for d in $PATH; do
		[ -n "$d" ] || d=.
		if [ -x "$d/wt" ] && [ -f "$d/wt" ]; then
			printf '%s/wt\n' "$d"
			return 0
		fi
	done
	return 1
}

# `wt list --format=json` emits schema 1 (bare array) or schema 2 (envelope with
# .items) depending on the user's config, and prints a migration notice on stderr.
# Normalize to the item array so callers do not care which.
mkit_wt_items() {
	local bin
	bin="$(mkit_wt_bin)" || return 1
	command -v jq >/dev/null 2>&1 || return 1
	"$bin" list --format=json 2>/dev/null |
		jq -c 'if type=="array" then . else (.items // []) end' 2>/dev/null
}
