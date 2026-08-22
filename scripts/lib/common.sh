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

# --- the gate ledger ------------------------------------------------------------------
#
# Absolute path of the gate ledger: one JSONL record per quality-gate step, keyed by a
# fingerprint of the content that step ran over. Beside journal.jsonl in the same mkit
# directory, so a linked worktree gets its own — a worktree's gate results are its own.
# Never dies: the writer runs inside a gate whose verdict must not depend on whether
# the ledger is reachable. Returns 1 and prints nothing when there is no git dir.
mkit_gate_ledger_path() {
	local git_dir
	git_dir="$(git rev-parse --absolute-git-dir 2>/dev/null)" || return 1
	[ -n "$git_dir" ] || return 1
	printf '%s/mkit/gate.jsonl\n' "$git_dir"
}

# sha256 of stdin. shasum is BSD/macOS, sha256sum is GNU; a system with neither is not
# an error — the ledger simply reports no-hash and the gate behaves exactly as before.
# Never add a hard prerequisite for a latency optimization.
mkit_sha256() {
	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256
	elif command -v sha256sum >/dev/null 2>&1; then
		sha256sum
	else
		return 1
	fi
}

mkit_have_hash() {
	command -v shasum >/dev/null 2>&1 || command -v sha256sum >/dev/null 2>&1
}

# Short hash identifying *the content a quality-gate command would read*, printed on
# stdout. Empty output and exit 1 mean "no fingerprint" — callers degrade, never fail.
#
# The load-bearing property is that it is **invariant under staging and committing**.
# The flagship flow is `review` (gate over a dirty tree) → `finish` (commit, then gate).
# A key built from HEAD plus the dirty set would classify as drifted the instant the
# commit lands, even though not one byte the gate reads changed — and the whole feature
# would save exactly nothing. So the key is the canonical `path → blob` mapping the
# commands actually see:
#
#   1. `git ls-tree -r HEAD`                       the committed mapping
#   2. overlay every path that differs from HEAD   with its *worktree* blob sha
#   3. drop paths deleted in the worktree          (a deletion must leave the mapping,
#                                                   not carry its committed blob)
#   4. add untracked-but-not-ignored paths         same overlay, same batch
#   5. sort, sha256, keep 16 hex characters
#
# Staging is invisible because staging does not change a worktree blob, and the dirty
# pre-commit tree and the clean post-commit tree yield the same mapping.
#
# Two mechanical requirements, both learned the hard way:
#
#   - **Batched hashing.** Every worktree path is hashed in ONE `git hash-object
#     --stdin-paths`. A per-path loop forks once per file and is ~55x slower at 1000
#     dirty files (14.8s vs 0.27s). Batching is part of the spec, not an optimization.
#   - **`--no-renames` and `ls-files --others`**, rather than parsing `git status
#     --porcelain`. Porcelain pairs a rename with a second NUL record that a reader must
#     consume or desynchronize from, and reports an untracked *directory* as one entry
#     whose contents are then invisible. These two plumbing commands have neither trap.
#
# What it cannot see, by construction: file mode (`chmod +x` does not change a blob sha
# — a documented gap), dependency installs, tool versions, env vars, and anything
# ignored. A match therefore means "the tracked content is identical", not "the
# environment is identical" — which is why the ledger also carries an age bound.
#
# Cost: ~0.09s on a clean 2000-file repo, ~0.26s with 1000 dirty files. Entirely inside
# a script, so it costs zero context tokens.
mkit_tree_fingerprint() {
	(
		# Sourced into scripts that run under `set -e`/`pipefail`: a missing HEAD or an
		# absent hash tool must return 1, never kill the gate around us.
		set +e
		set +o pipefail
		local root tmp_p tmp_d p out
		mkit_have_hash || exit 1
		root="$(git rev-parse --show-toplevel 2>/dev/null)"
		[ -n "$root" ] || exit 1
		cd "$root" || exit 1

		tmp_p="$(mktemp -t mkitfp)" || exit 1
		tmp_d="$(mktemp -t mkitfp)" || exit 1
		tmp_s="$(mktemp -t mkitfp)" || exit 1
		tmp_h="$(mktemp -t mkitfp)" || exit 1
		trap 'rm -f "$tmp_p" "$tmp_d" "$tmp_s" "$tmp_h"' EXIT

		# Everything that differs from HEAD, plus everything untracked and not ignored,
		# sorted into the four things a path can be. The enumeration is the point: one
		# unhashable path used to abort the whole batch below.
		{
			git diff --name-only -z --no-renames HEAD 2>/dev/null
			git ls-files --others --exclude-standard -z 2>/dev/null
		} | tr '\0' '\n' | LC_ALL=C sort -u | while IFS= read -r p; do
			[ -n "$p" ] || continue
			if [ -L "$p" ]; then
				# Checked before -f, which follows the link. git stores a symlink as a
				# blob of its *target path*, but `hash-object` follows it and would hash
				# the target's content — so a repo with any tracked symlink would read
				# `drifted` forever. One fork each instead, the same exception
				# `journal.sh` makes for the same reason; symlinks in a dirty set are rare.
				printf '%s\t%s\n' \
					"$(printf '%s' "$(readlink "$p")" | git hash-object --stdin 2>/dev/null)" \
					"$p" >>"$tmp_s"
			elif [ -f "$p" ]; then
				printf '%s\n' "$p" >>"$tmp_p"
			else
				# Gone — or no longer a regular file. A tracked file replaced by a
				# DIRECTORY is the real case (splitting a module into a package), and it
				# must never reach the batch: `git hash-object --stdin-paths` aborts on
				# the first path it cannot hash, and `paste` would then pair every
				# alphabetically-later path with the WRONG sha. Two different trees
				# hashing alike is the one failure a gate ledger may not have.
				printf '%s\n' "$p" >>"$tmp_d"
			fi
		done

		# Hashed here rather than inside the pipeline below, so a short batch can fail the
		# whole function. Inside a command substitution an early exit would still let the
		# downstream sha256 produce a confident, wrong answer.
		if [ -s "$tmp_p" ]; then
			git hash-object --stdin-paths <"$tmp_p" >"$tmp_h" 2>/dev/null
			# The backstop for the same abort, and for anything else that shortens the
			# batch: one answer per path, or no fingerprint at all. Degrading to "run the
			# gate" is free; a wrong hash is not.
			[ "$(wc -l <"$tmp_h" | tr -d ' ')" = "$(wc -l <"$tmp_p" | tr -d ' ')" ] || exit 1
		fi

		out="$(
			{
				# `-z` so paths are never quoted; the tab before the path is what makes
				# `awk -F'\t'` safe for paths containing spaces.
				git ls-tree -r -z HEAD 2>/dev/null | tr '\0' '\n' |
					awk -F'\t' 'NF > 1 { split($1, a, " "); print "B\t" a[3] "\t" $2 }'
				[ -s "$tmp_p" ] && paste -d'\t' "$tmp_h" "$tmp_p" |
					awk -F'\t' 'NF > 1 { print "B\t" $1 "\t" $2 }'
				[ -s "$tmp_s" ] && awk -F'\t' 'NF > 1 { print "B\t" $1 "\t" $2 }' "$tmp_s"
				[ -s "$tmp_d" ] && awk '{ print "D\t\t" $0 }' "$tmp_d"
				true
			} | awk -F'\t' '
				$1 == "D" { del[$3] = 1; next }
				$1 == "B" { blob[$3] = $2; next }
				END { for (p in blob) if (!(p in del)) printf "%s\t%s\n", p, blob[p] }
			' | LC_ALL=C sort | mkit_sha256 | cut -c1-16
		)"
		# A truncated pipeline can still print something; only 16 hex characters count.
		case "$out" in
		'' | *[!0-9a-f]*) exit 1 ;;
		esac
		printf '%s\n' "$out"
	)
}

# "6m" / "2h" / "3d" from a count of seconds. Ages are reported on every ledger class so
# a human can always see how old a proof is.
mkit_age_human() {
	local s="$1"
	case "$s" in '' | *[!0-9]*) printf '?' && return 0 ;; esac
	if [ "$s" -lt 60 ]; then
		printf '%ds' "$s"
	elif [ "$s" -lt 3600 ]; then
		printf '%dm' "$((s / 60))"
	elif [ "$s" -lt 86400 ]; then
		printf '%dh' "$((s / 3600))"
	else
		printf '%dd' "$((s / 86400))"
	fi
}
