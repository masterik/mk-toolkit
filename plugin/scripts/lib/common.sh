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
# repo. It holds exactly two things: `bootstrap.state` (which one-time messages the
# SessionStart hook has already said) and `bootstrap.disabled` (the tombstone that
# silences it).
#
# `~/.mkit`, not `~/.claude/mkit`, and the reason is a hard boundary rather than taste
# (docs/adr/0002): `~/.claude` — and whatever `CLAUDE_CONFIG_DIR` points at — is a
# protected region of the OS sandbox, where an allowlist entry is *inert*. Measured: a
# path under `~/.claude` already covered by `sandbox.filesystem.allowWrite` still fails
# with `Operation not permitted`, while `~/.codex` in the same list succeeds. The only
# lever inside the region is `filesystem.disabled`, which is not a remedy, it is a
# decision to run unsandboxed. Outside it, one `permissions.additionalDirectories` entry
# genuinely opens the path — so this directory is somewhere a remedy sentence can point.
#
# Not `$TMPDIR`, either: sandboxed and unsandboxed commands resolve `$TMPDIR` to
# different directories, and the SessionStart hook runs unsandboxed while the skills that
# read the same state do not.
#
# MKIT_HOME is not a convenience: the bats suite exports it at a temp path so a developer
# whose own bootstrap.state already records a warning cannot make the hook's say-it-once
# assertions pass or fail by accident — the tests would be measuring the developer, not
# the code. Any future user-scoped file goes here for the same reason.
mkit_user_dir() {
	printf '%s\n' "${MKIT_HOME:-$HOME/.mkit}"
}

# Is the user-scoped directory actually writable? A fact, reported at the first call, so
# a blocked write is a starting fact rather than a mid-run `Operation not permitted`.
#
# Probed by writing, not by `[ -w ]`: the sandbox denies the write itself while leaving
# the mode bits saying yes, and the case it actually produces is a directory that exists
# and takes a `mkdir` but refuses an append.
#
# Net-zero: the probe file always goes, and every directory this function had to create is
# removed again. `facts.sh` calls this, and `facts.sh` writes nothing outside the run
# directory it opens — creating user-scoped state as a side effect of reporting on it
# would make the report the thing that changed the answer.
#
# "Every" is load-bearing, and is why this counts ancestors instead of setting a flag:
# `MKIT_HOME` need not have an existing parent (`…/new-parent/state`), `mkdir -p` creates
# the whole chain, and one `rmdir` at the end removes only the leaf. That left structure
# behind on disk while still returning success — a report that changed what it reported on.
mkit_user_dir_writable() {
	local dir probe rc=0 created="" d n
	dir="$(mkit_user_dir 2>/dev/null)" || return 1
	[ -n "$dir" ] || return 1

	# Every absent ancestor, shallowest first — i.e. exactly the ones the `mkdir -p` below
	# is about to create, and nothing else. A directory that already exists is never ours.
	d="$dir"
	while [ ! -d "$d" ]; do
		created="$d
$created"
		case "$d" in
		*/?*)
			d="${d%/*}"
			[ -n "$d" ] || d=/
			;;
		*) break ;;
		esac
	done

	if [ -n "$created" ]; then
		mkdir -p "$dir" 2>/dev/null || return 1
	fi

	probe="$dir/.writable.$$"
	{ printf 'probe\n' >"$probe"; } 2>/dev/null || rc=1
	rm -f -- "$probe" 2>/dev/null

	# Unwind deepest first, `rmdir` only, and stop at the first refusal: anything that
	# gained content since the mkdir belongs to whoever put it there, and rmdir declining
	# is precisely the check for that. `rm -r` here would delete a concurrent run's state.
	n="$(printf '%s\n' "$created" | grep -c . || true)"
	d="$dir"
	while [ "$n" -gt 0 ]; do
		[ -n "$d" ] || break
		rmdir "$d" 2>/dev/null || break
		d="${d%/*}"
		n=$((n - 1))
	done
	return "$rc"
}

# The one remedy sentence for an unwritable user-scoped directory, in one place because
# `facts.sh` and `install.sh` must not word it differently.
#
# `permissions.additionalDirectories` rather than `sandbox.filesystem.allowWrite`: the
# former grants the sandbox write *and* makes the path a working directory, which is what
# also satisfies the auto-mode classifier's "no writes outside the working directories"
# rule. allowWrite alone leaves that rule biting, so it is named as the narrower
# alternative and never as the remedy.
mkit_user_dir_remedy() {
	printf 'add %s to permissions.additionalDirectories (sandbox.filesystem.allowWrite grants the sandbox only)\n' \
		"$(mkit_user_dir)"
}

# A temp file that dies with the command, always with an explicit template.
#
#   mkit_tmpfile <prefix>     prints the path, or returns 1
#
# The template is the whole point. On macOS `mktemp` with no template — bare or `-t` —
# resolves the Darwin per-user temp directory and **ignores `$TMPDIR`**, so under the OS
# sandbox it fails with `mkstemp failed on /var/folders/…: Operation not permitted`. That
# is not fixable by environment, which is why the bare and `-t` forms are banned from the
# payload outright (a static test asserts it) rather than merely discouraged.
#
# The division is by lifetime, not by caller: a file that dies with the command comes from
# here, a file a later step or a later session reads goes in the run directory. That seam
# is what keeps the fingerprint working in a session where the run directory is
# unreachable.
mkit_tmpfile() {
	local prefix="${1:-mkit}" dir="${TMPDIR:-/tmp}"
	# TMPDIR conventionally ends in a slash on macOS; a doubled separator is harmless but
	# the path is printed and compared, so normalize it.
	while [ "$dir" != / ] && [ "${dir%/}" != "$dir" ]; do dir="${dir%/}"; done
	mktemp "$dir/$prefix.XXXXXX" 2>/dev/null
}

# Fail unless we are inside a work tree. Every mkit script needs this.
mkit_require_repo() {
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 ||
		mkit_die 'not inside a git repository' 1
}

# Absolute path of the primary worktree. `git worktree list` always lists it first,
# regardless of which worktree the caller runs from. Shared by `facts.sh` (reports on the
# one worktree it is running in) and `branch-scan.sh` (reports on every worktree in the
# repo) — both need this exact lookup, and it is short enough that duplicating it bought
# nothing but a second place to get the offset wrong.
mkit_primary_worktree() {
	git worktree list --porcelain | awk '/^worktree /{print substr($0,10); exit}'
}

# Absolute path of this repo's mkit directory. Creating it is the caller's business;
# this only resolves it, or fails.
#
# `<toplevel>/.mkit`, not `<git-dir>/mkit`, and the move is not cosmetic (docs/adr/0002).
# Under a shared `.git` the run directory resolved into the **main checkout** from a
# linked worktree, where Claude Code's worktree-isolation guard refuses every write —
# *"This session is isolated in the worktree …; edit the worktree copy of this file
# instead of the shared-checkout path"* — and `facts.sh` opens the run directory as every
# skill's first call, so the whole toolkit was unusable in exactly the sessions it is
# driven from. Inside the working directory all three boundaries permit it with no
# configuration: the sandbox writes the cwd by default, the guard only blocks the main
# checkout, and the classifier's out-of-working-directory rules do not apply.
#
# `--show-toplevel`, so a linked worktree gets its own — the same property the git dir
# gave, now from the side of the boundary the session is on. Absolute, never a relative
# `.mkit/...`: the path is handed to subagents and reused across shells.
#
# The cost, stated rather than discovered: `.git/mkit` was invisible to everything that
# walks a working tree, `<toplevel>/.mkit` is invisible only to tools that honour
# `.gitignore`. Which is why `mkit_ensure_run_ignored` exists and is load-bearing.
mkit_dir_or_die() {
	local toplevel
	toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" ||
		mkit_die 'not inside a git repository' 1
	[ -n "$toplevel" ] || mkit_die 'not inside a git repository' 1
	printf '%s/.mkit\n' "$toplevel"
}

# Is `.mkit/` ignored in this repo? A plain question with a plain answer, asked of git
# rather than of a file's contents so a `.gitignore` line, a global excludes file and the
# common-dir exclude all count.
#
# The trailing slash is load-bearing. `.mkit/` is the natural rule to write and it is a
# directory-only pattern, so `check-ignore .mkit` answers "no" whenever the directory does
# not exist yet — which is every first call, the one that has to get this right. Asking
# about `.mkit/` matches a directory-only pattern and a plain `.mkit` alike.
mkit_run_ignored() {
	local toplevel
	toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || return 1
	git -C "$toplevel" check-ignore -q .mkit/ 2>/dev/null
}

# Make `.mkit/` ignored, once, and report whether it now is. Best effort: returns 0 when
# the directory is ignored on exit, 1 otherwise, and never fails a caller.
#
# Three things break while it is unignored, all measured in a throwaway repo with a linked
# worktree, and none of them cosmetic:
#
#   - **worktree teardown.** `git status --porcelain` reports `?? .mkit/`, and
#     `git worktree remove` refuses with *"contains modified or untracked files, use
#     --force"* — which breaks `finish`/`cleanup` and Claude Code's own sweep, since that
#     keeps any worktree holding changed or untracked files.
#   - **`git add -A`**, which would commit run artefacts into the user's project. Already
#     happened once, with an improvised helper script.
#   - **the gate cache.** `mkit_tree_fingerprint` enumerates with `git ls-files --others
#     --exclude-standard`, so an unignored scratch directory enters the fingerprint and
#     then changes while the gate runs — a run invalidating its own cache entry.
#
# Written into the **common dir's** `info/exclude`: shared by every worktree, uncommitted,
# no diff noise, and writable — only `.git/config` and `.git/hooks` are protected inside a
# working directory. A committed `.gitignore` line is the variant for repos whose
# teammates run mkit from fresh clones; either satisfies `mkit_run_ignored`.
#
# It cannot be written from a worktree-isolated session (the file lives in the main
# checkout), which is why the answer is reported as a starting fact rather than assumed.
mkit_ensure_run_ignored() {
	local common exclude
	mkit_run_ignored && return 0
	common="$(cd "$(git rev-parse --git-common-dir 2>/dev/null)" 2>/dev/null && pwd)" || return 1
	[ -n "$common" ] || return 1
	exclude="$common/info/exclude"
	mkdir -p "$common/info" 2>/dev/null || return 1
	# Appended with a comment naming the writer, because an unexplained line in someone
	# else's exclude file is indistinguishable from cruft.
	{
		printf '\n# mkit scratch root (run directories + gate.jsonl). Added by mkit.\n'
		printf '.mkit/\n'
	} >>"$exclude" 2>/dev/null || return 1
	mkit_run_ignored
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
# fingerprint of the content that step ran over. Lives in the repo's mkit directory, so a
# linked worktree gets its own — a worktree's gate results are its own.
# Never dies: the writer runs inside a gate whose verdict must not depend on whether
# the ledger is reachable. Returns 1 and prints nothing when there is no work tree.
mkit_gate_ledger_path() {
	local toplevel
	toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || return 1
	[ -n "$toplevel" ] || return 1
	printf '%s/.mkit/gate.jsonl\n' "$toplevel"
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
# The flagship flow is `pr` (gate before opening) → `finish` (gate again before merge).
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

		# `$TMPDIR` with an explicit template, via mkit_tmpfile — never the run directory.
		# These four files die with the call, and rooting them in the run directory would
		# recreate the original failure one layer down: the fingerprint is called from gate
		# detection, in sessions where the run directory may itself be unreachable.
		tmp_p="$(mkit_tmpfile mkitfp)" || exit 1
		tmp_d="$(mkit_tmpfile mkitfp)" || exit 1
		tmp_s="$(mkit_tmpfile mkitfp)" || exit 1
		tmp_h="$(mkit_tmpfile mkitfp)" || exit 1
		trap 'rm -f "$tmp_p" "$tmp_d" "$tmp_s" "$tmp_h"' EXIT

		# Everything that differs from HEAD, plus everything untracked and not ignored,
		# sorted into the four things a path can be. The enumeration is the point: one
		# unhashable path used to abort the whole batch below.
		#
		# `:(exclude).mkit` on both sides, belt to the exclude file's braces: mkit's own
		# scratch root must never enter its own fingerprint, or a run invalidates its own
		# cache entry mid-gate. `mkit_ensure_run_ignored` normally keeps it out of
		# `--others` already; this holds even in the session where that write was refused.
		{
			git diff --name-only -z --no-renames HEAD -- . ':(exclude).mkit' 2>/dev/null
			git ls-files --others --exclude-standard -z -- . ':(exclude).mkit' 2>/dev/null
		} | tr '\0' '\n' | LC_ALL=C sort -u | while IFS= read -r p; do
			[ -n "$p" ] || continue
			if [ -L "$p" ]; then
				# Checked before -f, which follows the link. git stores a symlink as a
				# blob of its *target path*, but `hash-object` follows it and would hash
				# the target's content — so a repo with any tracked symlink would read
				# `drifted` forever. One fork each instead; symlinks in a dirty set are rare.
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
				#
				# The reserved root is dropped here in awk, not as a pathspec: `ls-tree`
				# refuses pathspec magic outright (`fatal: pathspec magic not supported by
				# this command: 'exclude'`). Without the guard the exclusion above was
				# half-applied — the overlays dropped a tracked `.mkit/` path while this
				# mapping still contributed its HEAD blob, so committing an otherwise
				# identical worktree changed the fingerprint and broke the commit-
				# invariance the whole ledger rests on. Only reachable in a repo that
				# tracked `.mkit/` before upgrading: .gitignore and the exclude file stop
				# it being tracked from here on, but neither untracks what already is.
				git ls-tree -r -z HEAD 2>/dev/null | tr '\0' '\n' |
					awk -F'\t' 'NF > 1 && $2 != ".mkit" && $2 !~ /^\.mkit\// {
						split($1, a, " "); print "B\t" a[3] "\t" $2
					}'
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

# --- user-scoped setup: prerequisites, one-time state -----------------------------------
#
# Everything below is shared by `install.sh` (run by hand) and
# `scripts/hooks/session-bootstrap.sh` (the SessionStart hook). Two callers is the whole
# point: the degradation sentences are only one source of truth if neither caller writes
# its own.

mkit_have() {
	command -v "$1" >/dev/null 2>&1
}

# The prerequisite table, one row per tool: <tool>\t<state>\t<consequence>.
#
#   state  MISSING  a hard requirement — a skill cannot get its starting facts without it
#          missing  a soft one — some feature degrades, nothing breaks
#          ok       present
#
# A table rather than a print function, because the two callers need different subsets:
# install.sh prints every row (a human watching wants to see the `ok`s) and derives its
# exit status from whether any row is MISSING, while the hook prints only the non-ok
# rows, once each, and never blocks on them. Same sentences either way.
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
	#
	# One sentence per tool rather than one for the pair: "a hard requirement" is the
	# same verdict either way, but what breaks is not, and a report that cannot say
	# which feature just died sends the reader to the wrong place.
	for tool in git jq; do
		if mkit_have "$tool"; then
			state=ok text=''
		else
			state=MISSING
			case "$tool" in
			git) text='every skill reads the repo through it' ;;
			jq) text='facts.sh, branch-scan.sh and the gate ledger all parse JSON with it' ;;
			esac
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

# --- one-time state: "have I already said this?" ---------------------------------------
#
# A line-per-key file: `grep -qxF` membership, an atomic `>>` append to add, a mktemp+mv
# rewrite to drop. It needs no prune — its key space is fixed by construction (a handful
# of `prereq/` keys), not an unbounded stream.

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
# Defensive rather than load-bearing today: the hook's payload is assembled from the fixed
# prerequisite sentences and interpolates no path at all — not $HOME, not $MKIT_HOME — so
# nothing user-controlled currently reaches it. That is a property of the present message
# set, not a guarantee, and it is the kind of property a later message quietly revokes. The
# escape stays so the first string that does carry a path cannot turn one stray quote into
# a document that parses as nothing.
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
