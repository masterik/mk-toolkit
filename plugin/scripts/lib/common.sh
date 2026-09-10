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

# The user-scoped config directory — where mkit state that outlives a repo goes.
#
# **Empty today.** It held exactly two files, `bootstrap.state` and `bootstrap.disabled`,
# and both died with the `SessionStart` hook. The directory keeps its definition anyway:
# it is the answer to "where does user-scoped state go", the path a remedy sentence can
# point at, and what `MKIT_HOME` redirects — all three of which the binary needs before
# it writes its first user-scoped file. `facts.sh` still probes it, so a machine that
# would refuse that write says so at the first call rather than at the first write.
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
# Not `$TMPDIR`, either: that is the location for what dies with the command, and
# sandboxed and unsandboxed commands do not even resolve it to the same directory.
#
# MKIT_HOME is not a convenience: the bats suite exports it at a temp path so a run can
# never read or write a developer's real state — the tests would be measuring the
# developer, not the code. Any future user-scoped file goes here for the same reason.
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

# The one remedy sentence for an unwritable user-scoped directory. One producer, so a
# second caller — `mkit doctor` — cannot word it differently. The binary shells out to
# this function rather than re-wording it; that is the whole reason it is a function.
#
# `permissions.additionalDirectories` rather than `sandbox.filesystem.allowWrite`: the
# former grants the sandbox write *and* makes the path a working directory, which is what
# also satisfies the auto-mode classifier's "no writes outside the working directories"
# rule. allowWrite alone leaves that rule biting, so it is named as the narrower
# alternative and never as the remedy.
#
# **Both halves, always.** The grant covers the directory's *interior*; creating the
# directory is a write to its parent, which nothing grants — measured:
# `mkdir: /Users/mk/.mkit: Operation not permitted`. A sentence naming only the grant
# produced a configuration that looked right and changed nothing, which is the exact
# failure ADR 0002 was written about, one level down. The `mkdir` half is human-run by
# construction: no sandboxed session can perform it, so it is spelled with the `!`
# prefix that runs a command in the user's own shell.
mkit_user_dir_remedy() {
	local dir
	dir="$(mkit_user_dir)"
	printf 'run `! mkdir -p %s` (a sandboxed session cannot create it), then add %s to permissions.additionalDirectories (sandbox.filesystem.allowWrite grants the sandbox only)\n' \
		"$(mkit_shell_quote "$dir")" "$dir"
}

# Quote a path for the copy-and-run half of a remedy. Only the command is quoted;
# the allowlist entry is JSON the user types into settings, not shell.
#
# MKIT_HOME can point anywhere, including a path with spaces, and a remedy printed
# as `! mkdir -p /Users/x/my state` creates two wrong directories rather than
# failing — the worst kind of wrong, since it looks like it worked. bash 3.2 has
# printf %q, but its output is unquoted-with-backslashes and reads badly in a
# sentence, so this is the single-quote form.
mkit_shell_quote() {
	case "$1" in
	*[!A-Za-z0-9/._-]*) printf "'%s'\n" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")" ;;
	*) printf '%s\n' "$1" ;;
	esac
}

# Absolute path of this repo's committed config file, or failure outside a work tree.
#
# `<toplevel>/.mkit/config.toml` — the same directory as the scratch, and the one file in
# it that is committed (ADR 0001, amended). Resolving it is all this does; `mkit init`
# writes it and `mkit repo profile` reads it.
mkit_config_path() {
	local toplevel
	toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || return 1
	[ -n "$toplevel" ] || return 1
	printf '%s/.mkit/config.toml\n' "$toplevel"
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

# Is mkit's scratch ignored in this repo? A plain question with a plain answer, asked of
# git rather than of a file's contents so a `.gitignore` line, a global excludes file and
# the common-dir exclude all count.
#
# Asked about `.mkit/gate.jsonl` — a real scratch path — rather than about `.mkit/`.
#
# The old subject still works, and the reason is obscure enough to be worth not relying
# on: `check-ignore .mkit/`, *with the trailing slash*, is matched by `.mkit/*`, because
# git reads the trailing slash as naming something inside the directory. Drop the slash
# and it stops matching. That subtlety was already load-bearing once here — a
# directory-only pattern needs the slash to match before the directory exists — and
# depending on it twice, for two different reasons, is a trap for whoever edits this next.
#
# A concrete path inside the scratch depends on none of it. It answers correctly before
# anything exists on disk (`check-ignore` is pattern matching, not a stat), which is the
# first call, the one that has to be right; and every rule shape matches it — the legacy
# directory-only `.mkit/` and the current `.mkit/*` pair alike.
MKIT_SCRATCH_PROBE='.mkit/gate.jsonl'

# The ledger is not the only thing that must be ignored, and one probe cannot speak
# for both. An unrelated `*.jsonl` rule hides `gate.jsonl` while leaving every
# `.mkit/<skill>-*/` run directory untracked — under which `run_ignored=yes` would be
# reported to a skill whose worktree teardown then fails. So a run-directory-shaped
# path is probed too, and both must be ignored for the answer to be yes.
MKIT_RUN_PROBE='.mkit/probe-0/log'

mkit_run_ignored() {
	local toplevel
	toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || return 1
	git -C "$toplevel" check-ignore -q -- "$MKIT_SCRATCH_PROBE" 2>/dev/null &&
		git -C "$toplevel" check-ignore -q -- "$MKIT_RUN_PROBE" 2>/dev/null
}

# The other half of the same question, and the one a legacy repo gets wrong: can the
# committed config file actually be committed here?
#
# A repo set up before the config existed carries a directory-only `.mkit/` rule, under
# which `git add .mkit/config.toml` is refused and a fresh clone inherits nothing — so
# `mkit init` would write a file that silently never travels. Returns 0 when the path is
# committable, 1 when a rule is shadowing it.
mkit_config_committable() {
	local toplevel
	toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || return 1
	! git -C "$toplevel" check-ignore -q -- .mkit/config.toml 2>/dev/null
}

# The one remedy sentence for a shadowed config path. One producer, same rule as
# `mkit_user_dir_remedy`.
#
# Which file to name is not cosmetic: git reads `.gitignore` at a *higher* precedence than
# the common dir's `info/exclude`, so a negation written into the exclude cannot lift a
# `.mkit/` line that lives in a committed `.gitignore`. Where git reports the source, name
# it; that is the only file where editing the rule does anything.
mkit_config_ignored_remedy() {
	local toplevel fields src pat parent
	toplevel="$(git rev-parse --show-toplevel 2>/dev/null)" || return 1
	# -z, so each field is read whole. The default format is
	# `<source>:<line>:<pattern>\t<path>`, and `cut -d:` truncates any source path
	# containing a colon — a core.excludesFile under such a directory would be named
	# wrong, which is the one thing this sentence exists to get right. git accepts
	# -z only with --stdin ("-z only makes sense with --stdin"), hence the pipe.
	fields="$(printf '.mkit/config.toml\0' | git -C "$toplevel" check-ignore -v -z --stdin 2>/dev/null | tr '\0' '\n')"
	src="$(printf '%s\n' "$fields" | sed -n 1p)"
	pat="$(printf '%s\n' "$fields" | sed -n 3p)"
	: "${src:=.gitignore}"

	# Which fix applies depends on whether the rule excludes the `.mkit` *directory*
	# or only the file. A directory cannot be undone by a negation at all — git never
	# descends into an excluded one — so that rule has to become `.mkit/*`; anything
	# else is lifted by a negation after it, and telling that reader to go replace a
	# `.mkit/` rule sends them looking for a line their file does not contain.
	#
	# Two signals, because neither is complete alone (measured over every rule shape,
	# see the Go counterpart's TestParentExcludedMatchesGit):
	#
	#   - a directory-form pattern, trailing `/`, matches only directories — so if it
	#     decided a *file* path it matched a directory component, and `.mkit` is the
	#     only one. This is the case the probe below misses before the directory
	#     exists, which on a fresh clone it does not: `.mkit/`, `**/.mkit/`, `.mki?/`.
	#   - asking git about the bare `.mkit` catches every non-directory pattern that
	#     swallows the parent: `.m*`, `.mkit*`, `/.mkit`, a bare `*`.
	if [ -z "$pat" ]; then
		printf 'an ignore rule in %s excludes `.mkit/config.toml`; if it is a `.mkit/` rule, replace it with `.mkit/*` followed by `!.mkit/config.toml` — git cannot re-include a file whose parent directory is excluded\n' "$src"
		return 0
	fi
	case "$pat" in
	*/) parent=yes ;;
	*) if git -C "$toplevel" check-ignore -q -- .mkit 2>/dev/null; then parent=yes; else parent=no; fi ;;
	esac
	if [ "$parent" = yes ]; then
		printf 'replace the `%s` rule in %s with `.mkit/*` followed by `!.mkit/config.toml` — git cannot re-include a file whose parent directory is excluded\n' \
			"$pat" "$src"
	else
		printf 'the `%s` rule in %s excludes it; add `!.mkit/config.toml` after that line in the same file\n' \
			"$pat" "$src"
	fi
}

# Make mkit's scratch ignored, once, and report whether it now is. Best effort: returns 0
# when the scratch is ignored on exit, 1 otherwise, and never fails a caller.
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
# Two lines, not one, and the pair is the unit: `.mkit/*` excludes the scratch while
# leaving `.mkit/` itself includable, and `!.mkit/config.toml` re-includes the one
# committed file. Writing only the first would hide repo config from `git add`; writing
# the old directory-only `.mkit/` would make the negation impossible to add later.
#
# Written into the **common dir's** `info/exclude`: shared by every worktree, uncommitted,
# no diff noise, and writable — only `.git/config` and `.git/hooks` are protected inside a
# working directory. A committed `.gitignore` carrying the same pair is the variant for
# repos whose teammates run mkit from fresh clones, and is what this repo itself ships.
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
		printf '# config.toml is repo config and stays committable — the pair is the rule.\n'
		printf '.mkit/*\n'
		printf '!.mkit/config.toml\n'
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
		local root tmp_p tmp_d tmp_s tmp_h p out
		mkit_have_hash || exit 1
		root="$(git rev-parse --show-toplevel 2>/dev/null)"
		[ -n "$root" ] || exit 1
		cd "$root" || exit 1

		# `$TMPDIR` with an explicit template, via mkit_tmpfile — never the run directory.
		# These four files die with the call, and rooting them in the run directory would
		# recreate the original failure one layer down: the fingerprint is called from gate
		# detection, in sessions where the run directory may itself be unreachable.
		#
		# Trap goes up front, before the first allocation: if tmp_d/tmp_s/tmp_h fails after
		# tmp_p already exists, tmp_p must still be removed. `rm -f ""` on a not-yet-assigned
		# one is a no-op, so installing the trap before any of them are set is safe.
		tmp_p="" tmp_d="" tmp_s="" tmp_h=""
		trap 'rm -f "$tmp_p" "$tmp_d" "$tmp_s" "$tmp_h"' EXIT
		tmp_p="$(mkit_tmpfile mkitfp)" || exit 1
		tmp_d="$(mkit_tmpfile mkitfp)" || exit 1
		tmp_s="$(mkit_tmpfile mkitfp)" || exit 1
		tmp_h="$(mkit_tmpfile mkitfp)" || exit 1

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
