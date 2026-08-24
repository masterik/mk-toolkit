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

# The user-scoped config directory — the one piece of mkit state that lives outside a
# repo, and the only thing `install.sh` writes. Under `~/.claude` rather than XDG
# because mkit is a Claude Code plugin and this sits beside the runtime's own state.
#
# MKIT_HOME is not a convenience: the bats suite exports it at a temp path so a user
# who has run install.sh does not have their real user-scoped default leak into every
# test that asserts a pristine repo is disabled. Any future user-scoped file goes here
# for the same reason.
mkit_user_dir() {
	printf '%s\n' "${MKIT_HOME:-$HOME/.claude/mkit}"
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

# sha256 of stdin, via macOS's `shasum`. Its absence is not an error — the ledger simply
# reports no-hash and the gate behaves exactly as before. Never add a hard prerequisite
# for a latency optimization.
#
# Still guarded rather than called bare: `shasum` is a Perl script, so a stripped or
# containerized environment can lack it even on macOS. The GNU `sha256sum` fallback is
# gone with the rest of the non-macOS accommodations.
mkit_sha256() {
	command -v shasum >/dev/null 2>&1 || return 1
	shasum -a 256
}

mkit_have_hash() {
	command -v shasum >/dev/null 2>&1
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

# --- user-scoped setup: prerequisites, the bin wrapper, one-time state -----------------
#
# Everything below is shared by `install.sh` (run by hand) and
# `scripts/hooks/session-bootstrap.sh` (the SessionStart hook). Two callers is the whole
# point: the wrapper's "is this file mine?" test is only as good as there being exactly
# one producer of the wrapper, and the degradation sentences are only one source of truth
# if neither caller writes its own.

mkit_have() {
	command -v "$1" >/dev/null 2>&1
}

# The prerequisite table, one row per tool: <tool>\t<state>\t<consequence>.
#
#   state  MISSING  a hard requirement — the journal cannot read or write without it
#          missing  a soft one — some feature degrades, nothing breaks
#          ok       present
#
# A table rather than a print function, because the two callers need different subsets:
# install.sh prints every row (a human watching wants to see the `ok`s) and derives its
# refuse-to-install status from whether any row is MISSING, while the hook prints only
# the non-ok rows, once each, and never blocks on them. Same sentences either way.
#
#   mkit_prereq_rows [--missing-only]
#
# Returns 1 if any hard requirement is missing, so a caller can branch on the status
# without parsing the rows back.
mkit_prereq_rows() {
	local missing_only=no missing_hard=0 tool state text
	[ "${1:-}" = --missing-only ] && missing_only=yes

	# `bash` is deliberately absent from this table. A bash script cannot report that
	# bash is missing, so the row could only ever read `ok` — and a check that can only
	# produce one answer is not a check.
	for tool in git jq; do
		if mkit_have "$tool"; then
			state=ok text=''
		else
			state=MISSING text='the journal cannot read or write without it'
			missing_hard=1
		fi
		[ "$missing_only" = yes ] && [ "$state" = ok ] && continue
		printf '%s\t%s\t%s\n' "$tool" "$state" "$text"
	done

	if mkit_have node; then
		state=ok text=''
	else
		state=missing text='only findings.mjs (the review skill) needs it'
	fi
	[ "$missing_only" = yes ] && [ "$state" = ok ] || printf '%s\t%s\t%s\n' node "$state" "$text"

	if mkit_have_hash; then
		state=ok text=''
	else
		state=missing text='the gate ledger still records, but reports gate_cache=no-hash'
	fi
	[ "$missing_only" = yes ] && [ "$state" = ok ] || printf '%s\t%s\t%s\n' sha256 "$state" "$text"

	return "$missing_hard"
}

# Where the `mkit-journal` wrapper goes: the first *existing* of ~/.local/bin, ~/bin.
# Empty output and status 1 when neither exists — and neither is ever created, because a
# directory mkit invented would not be on PATH anyway and would outlive an uninstall.
#
# MKIT_BIN is not a convenience, for the same reason MKIT_HOME is not: the bats suite
# points it at a temp dir so a bug here cannot write an executable into the developer's
# real ~/.local/bin.
mkit_bin_dir() {
	local candidate
	if [ -n "${MKIT_BIN:-}" ]; then
		printf '%s\n' "$MKIT_BIN"
		return 0
	fi
	for candidate in "${HOME:-}/.local/bin" "${HOME:-}/bin"; do
		case "$candidate" in
		/*/.local/bin | /*/bin) ;;
		*) continue ;; # HOME unset — do not resolve to a relative path
		esac
		[ -d "$candidate" ] && printf '%s\n' "$candidate" && return 0
	done
	return 1
}

# The one marker that identifies a wrapper mkit generated. A fixed, implausible-to-type
# string, and deliberately not naming a producer: both install.sh and the SessionStart
# hook write this same body, and a comment saying "generated by install.sh" would be a
# lie in the hook's case and would rot the ownership test.
MKIT_WRAPPER_MARK='mkit-generated wrapper'

mkit_wrapper_body() {
	printf '#!/usr/bin/env bash\n'
	printf '# %s — do not edit; re-generated on session start.\n' "$MKIT_WRAPPER_MARK"
	printf 'exec "%s/scripts/journal.sh" "$@"\n' "$1"
}

# Is the file at $1 one of ours? Checked in the first few lines only, so a wrapper that
# somehow grew a body cannot smuggle the mark in from further down.
mkit_wrapper_is_ours() {
	[ -f "$1" ] || return 1
	head -n 5 -- "$1" 2>/dev/null | grep -qF -- "$MKIT_WRAPPER_MARK" 2>/dev/null
}

# Ours *and* pointing at plugin root $2. A wrapper bakes its plugin root in at write
# time, so a marketplace upgrade (…/mkit/0.7.0 → …/0.8.0) or a moved checkout leaves one
# that is ours but stale — which the hook then rewrites, since it is the only component
# that runs from the new root on every session.
mkit_wrapper_is_current() {
	mkit_wrapper_is_ours "$1" || return 1
	grep -qxF -- "exec \"$2/scripts/journal.sh\" \"\$@\"" "$1" 2>/dev/null
}

# Write the wrapper at $1 for plugin root $2, atomically.
#
# mktemp in the same directory + chmod + `mv -f`, never `cat >`. Three independent
# reasons, any one of which is sufficient: `cat >` truncates in place, so a
# `mkit-journal` executing in that window reads a half-file and dies on a partial
# `exec`; the hook runs under a 5s timeout whose SIGTERM can land mid-write; and
# `rename(2)` leaves an already-open inode alone. A killed writer leaves a temp file,
# never a broken executable on the user's PATH.
#
# No lock. macOS ships no flock(1), so the portable mutex is a lock *directory* with
# stale-lock timeout handling — a subsystem, to protect a write that is already atomic.
# Two racing writers here produce byte-identical content anyway.
mkit_write_wrapper() {
	local path="$1" root="$2" dir tmp
	dir="$(dirname -- "$path")"
	tmp="$(mktemp "$dir/.mkit-journal.XXXXXX" 2>/dev/null)" || return 1
	# 755 explicitly, not `chmod +x`: mktemp creates 0600, and `+x` on that yields 0711 —
	# executable but unreadable, which is a strange thing to leave on someone's PATH.
	if ! mkit_wrapper_body "$root" >"$tmp" 2>/dev/null ||
		! chmod 755 "$tmp" 2>/dev/null ||
		! mv -f -- "$tmp" "$path" 2>/dev/null; then
		rm -f -- "$tmp" 2>/dev/null
		return 1
	fi
	return 0
}

# --- one-time state: "have I already said this?" ---------------------------------------
#
# A line-per-key file, the same shape as the hook budget in journal-nudge.sh: `grep -qxF`
# membership, an atomic `>>` append to add, a mktemp+mv rewrite to drop. Unlike that
# file this one needs no prune — its key space is fixed by construction (a handful of
# `notice/` and `prereq/` keys), not an unbounded stream of prompt ids.

mkit_state_has() {
	[ -f "$1" ] || return 1
	grep -qxF -- "$2" "$1" 2>/dev/null
}

mkit_state_add() {
	local file="$1" key="$2"
	mkdir -p "$(dirname -- "$file")" 2>/dev/null || return 1
	# Braced, so the stderr redirect is in place before the append can report its own
	# failure — the bare `>>"$f" 2>/dev/null` form lets that diagnostic escape.
	{ printf '%s\n' "$key" >>"$file"; } 2>/dev/null || return 1
	return 0
}

# Drop $2 from $1, and dedupe while rewriting: two sessions starting at once can each
# append the same key, which is harmless for membership but worth cleaning up when a
# rewrite is happening anyway.
mkit_state_drop() {
	local file="$1" key="$2" tmp
	[ -f "$file" ] || return 0
	mkit_state_has "$file" "$key" || return 0
	tmp="$(mktemp "$file.XXXXXX" 2>/dev/null)" || return 1
	if ! awk -v k="$key" '$0 != k && !seen[$0]++' "$file" >"$tmp" 2>/dev/null ||
		! mv -f -- "$tmp" "$file" 2>/dev/null; then
		rm -f -- "$tmp" 2>/dev/null
		return 1
	fi
	return 0
}

# Keys in state file $1 that are stale — a `prereq/<tool>` recorded as warned about, for
# a tool that is now present. $2 is the current `--missing-only` table, so the comparison
# is against what is true *now* rather than against what the file remembers: the state
# file is the ledger of what has been said, never the source of truth for what is missing.
#
# Prints the stale keys, one per line, for the caller to drop. At most one rewrite per
# tool ever happens — the session right after it gets installed — so the steady state
# stays grep-only.
mkit_state_missing_keys() {
	local file="$1" rows="$2" line key tool
	[ -f "$file" ] || return 0
	while IFS= read -r line; do
		case "$line" in
		prereq/*) ;;
		*) continue ;;
		esac
		tool="${line#prereq/}"
		# Still missing → the key is earned, keep it. Silenced like every other external
		# call on this path: the sole caller is a hook contractually forbidden from
		# writing to stderr, and "grep is missing too" is not a message it can act on.
		printf '%s\n' "$rows" | cut -f1 2>/dev/null | grep -qxF -- "$tool" 2>/dev/null && continue
		printf '%s\n' "$line"
	done <"$file"
	return 0
}

# --- JSON, without jq -----------------------------------------------------------------
#
# Escape stdin as the *contents* of a JSON string (no surrounding quotes), on one line.
#
# Why not jq: the one caller is the SessionStart hook, whose job includes reporting that
# `jq` is missing. Building that report with jq would make the message unavailable in
# exactly the case that must produce it. awk is POSIX and present wherever bash is, so
# this leaves the hook with no external prerequisite at all.
#
# Correctness matters more than it looks: the strings interpolate $HOME, $MKIT_HOME and
# the bin dir, and a home directory containing a quote or a backslash is a real case the
# sibling hook already has a test for.
# stderr is silenced because the only caller is a hook forbidden from writing any. If awk
# itself were missing the result is an empty string in a still-valid JSON document — a
# message that says nothing, rather than a document that parses as nothing.
mkit_json_escape() {
	awk 2>/dev/null '
		BEGIN {
			for (i = 0; i < 32; i++) ctl[sprintf("%c", i)] = sprintf("\\u%04x", i)
			ctl[sprintf("%c", 127)] = "\\u007f"
			first = 1
		}
		{
			line = $0
			out = ""
			n = length(line)
			for (i = 1; i <= n; i++) {
				c = substr(line, i, 1)
				if (c == "\\") out = out "\\\\"
				else if (c == "\"") out = out "\\\""
				else if (c == "\t") out = out "\\t"
				else if (c in ctl) out = out ctl[c]
				else out = out c
			}
			if (first) { printf "%s", out; first = 0 }
			else printf "\\n%s", out
		}
	'
}
