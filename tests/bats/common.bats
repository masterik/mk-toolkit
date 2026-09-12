#!/usr/bin/env bats
load helpers.bash

setup() { mkit_setup_repo; }
teardown() { mkit_teardown_repo; }

src() { printf '. "%s/lib/common.sh"; ' "$SCRIPTS"; }

@test "mkit_check_slug accepts a plain name" {
	run bash -c "$(src)mkit_check_slug commit"
	[ "$status" -eq 0 ]
}

@test "mkit_check_slug rejects a space" {
	run bash -c "$(src)mkit_check_slug 'bad name'"
	[ "$status" -eq 2 ]
	[[ "$output" == *"may only contain"* ]]
}

@test "mkit_check_slug rejects a path traversal" {
	run bash -c "$(src)mkit_check_slug '../etc'"
	[ "$status" -eq 2 ]
}

@test "mkit_check_slug rejects empty" {
	run bash -c "$(src)mkit_check_slug ''"
	[ "$status" -eq 2 ]
	[[ "$output" == *"empty name"* ]]
}

@test "mkit_require_repo passes inside a repo" {
	run bash -c "$(src)mkit_require_repo"
	[ "$status" -eq 0 ]
}

@test "mkit_require_repo fails outside a repo" {
	cd "$MKIT_TMP/.."
	run bash -c "$(src)mkit_require_repo"
	[ "$status" -eq 1 ]
	[[ "$output" == *"not inside a git repository"* ]]
}

@test "mkit_plugin_root resolves to the checkout root" {
	run bash -c "$(src)mkit_plugin_root"
	[ "$status" -eq 0 ]
	[ -f "$output/.claude-plugin/plugin.json" ]
}

@test "mkit_refs_dir points at skills/_shared/references" {
	run bash -c "$(src)mkit_refs_dir"
	[ "$status" -eq 0 ]
	[[ "$output" == */skills/_shared/references ]]
	[ -d "$output" ]
}

# --- the gate ledger -----------------------------------------------------------------

# --- where state lives, and whether it can be written ---------------------------------
#
# `~/.claude/mkit` was inside the OS sandbox's protected-path region, where an allowlist
# entry is inert: a path there already covered by `sandbox.filesystem.allowWrite` still
# failed with `Operation not permitted`. So the remedy the docs implied could not work,
# and the directory moved. docs/adr/0002 records the measurement.

@test "mkit_user_dir defaults to ~/.mkit, outside the sandbox's protected region" {
	run env -u MKIT_HOME HOME=/home/example bash -c "$(src)mkit_user_dir"
	[ "$status" -eq 0 ]
	[ "$output" = /home/example/.mkit ]
	# Never under ~/.claude: that region cannot be granted, only disabled wholesale.
	[[ "$output" != *"/.claude/"* ]]
}

@test "MKIT_HOME still overrides it, which is what sandboxes the suite" {
	run bash -c "$(src)mkit_user_dir"
	[ "$status" -eq 0 ]
	[ "$output" = "$MKIT_HOME" ]
}

@test "mkit_user_dir_writable reports yes for a writable dir and creates nothing" {
	[ ! -d "$MKIT_HOME" ]
	run bash -c "$(src)mkit_user_dir_writable"
	[ "$status" -eq 0 ]
	[ ! -d "$MKIT_HOME" ]
}

@test "mkit_user_dir_writable reports no for a dir that refuses a write" {
	mkdir -p "$MKIT_HOME"
	chmod 500 "$MKIT_HOME"
	run bash -c "$(src)mkit_user_dir_writable"
	chmod 700 "$MKIT_HOME"
	[ "$status" -eq 1 ]
}

@test "mkit_user_dir_writable leaves no probe file behind" {
	mkdir -p "$MKIT_HOME"
	run bash -c "$(src)mkit_user_dir_writable"
	[ "$status" -eq 0 ]
	[ -z "$(ls -A "$MKIT_HOME")" ]
}

@test "the remedy names additionalDirectories, and never a protected path" {
	run bash -c "$(src)mkit_user_dir_remedy"
	[ "$status" -eq 0 ]
	[[ "$output" == *"permissions.additionalDirectories"* ]]
	[[ "$output" == *"$MKIT_HOME"* ]]
	[[ "$output" != *".claude/mkit"* ]]
}

# The grant covers the directory's interior, so it cannot create the directory —
# `mkdir` there is a write to the parent, which nothing grants. A sentence naming only
# the grant produced a configuration that looked right and changed nothing (ADR 0002,
# amended), so both halves are asserted rather than left to review.
@test "the remedy names creating the directory as well as granting it" {
	run bash -c "$(src)mkit_user_dir_remedy"
	[ "$status" -eq 0 ]
	[[ "$output" == *"mkdir -p $MKIT_HOME"* ]]
	# Human-run, because no sandboxed session can perform it.
	[[ "$output" == *"! mkdir"* ]]
}

# MKIT_HOME can point anywhere, a path with spaces included, and the mkdir half of the
# remedy is meant to be copied and run. Unquoted it creates two wrong directories
# rather than failing, which is worse than failing.
@test "the remedy quotes a user dir containing spaces" {
	run env MKIT_HOME="$MKIT_TMP/my state" bash -c "$(src)mkit_user_dir_remedy"
	[ "$status" -eq 0 ]
	[[ "$output" == *"mkdir -p '$MKIT_TMP/my state'"* ]]
}

@test "mkit_dir_or_die resolves inside the toplevel, never the git dir" {
	run bash -c "$(src)mkit_dir_or_die"
	[ "$status" -eq 0 ]
	[ "$output" = "$MKIT_TMP/.mkit" ]
}

@test "mkit_dir_or_die follows a linked worktree into itself" {
	git worktree add -q -b wt-c "$MKIT_TMP/wt-c" >/dev/null
	run bash -c "cd '$MKIT_TMP/wt-c' && $(src)mkit_dir_or_die"
	[ "$status" -eq 0 ]
	[ "$output" = "$MKIT_TMP/wt-c/.mkit" ]
}

@test "mkit_dir_or_die fails outside a repo" {
	cd "$MKIT_TMP/.."
	run bash -c "$(src)mkit_dir_or_die"
	[ "$status" -eq 1 ]
}

# --- mkit_tmpfile -----------------------------------------------------------------------
#
# On macOS `mktemp` with no template — bare or `-t` — resolves the Darwin per-user temp
# directory and ignores `$TMPDIR`, so under the sandbox it fails outright. Everything
# ephemeral goes through here instead, with an explicit template.

@test "mkit_tmpfile creates a file under \$TMPDIR with the given prefix" {
	run bash -c "$(src)mkit_tmpfile mkit-probe"
	[ "$status" -eq 0 ]
	[ -f "$output" ]
	[[ "$output" == "${TMPDIR%/}/mkit-probe."* ]]
	rm -f -- "$output"
}

@test "mkit_tmpfile honours \$TMPDIR rather than the Darwin per-user temp dir" {
	alt="$MKIT_TMP/alt-tmp"
	mkdir -p "$alt"
	run env TMPDIR="$alt" bash -c "$(src)mkit_tmpfile mkitfp"
	[ "$status" -eq 0 ]
	[[ "$output" == "$alt/mkitfp."* ]]
	[ -f "$output" ]
}

@test "mkit_tmpfile normalizes a trailing slash on \$TMPDIR" {
	alt="$MKIT_TMP/alt-tmp2"
	mkdir -p "$alt"
	run env TMPDIR="$alt/" bash -c "$(src)mkit_tmpfile mkitfp"
	[ "$status" -eq 0 ]
	[[ "$output" == "$alt/mkitfp."* ]]
}

@test "mkit_tmpfile fails, rather than falling back, when \$TMPDIR is unwritable" {
	alt="$MKIT_TMP/ro-tmp"
	mkdir -p "$alt"
	chmod 500 "$alt"
	run env TMPDIR="$alt" bash -c "$(src)mkit_tmpfile mkitfp"
	chmod 700 "$alt"
	[ "$status" -ne 0 ]
	[ -z "$output" ]
}

# --- the ignore rule --------------------------------------------------------------------
#
# The rule is a pair, not a line: `.mkit/*` excludes the scratch while leaving `.mkit/`
# itself includable, and `!.mkit/config.toml` re-includes the one committed file. Git
# cannot re-include a file whose parent directory is excluded, so the old directory-only
# `.mkit/` made repo config impossible (ADR 0001's config-path amendment).

@test "mkit_run_ignored answers no before the rule exists, and yes after" {
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -ne 0 ]
	mkdir -p .git/info
	printf '.mkit/*\n' >>.git/info/exclude
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -eq 0 ]
}

# The probe is a concrete scratch path rather than the directory, and this pins why
# the directory form is not relied on. `.mkit/` *with* the trailing slash is matched by
# `.mkit/*` — git reads the slash as naming something inside — while the bare `.mkit`
# is not. The distinction is invisible at a glance and already load-bearing here for a
# second, unrelated reason, so the probe avoids it entirely.
@test "the directory probe depends on a trailing-slash subtlety the scratch probe avoids" {
	mkdir -p .git/info
	printf '.mkit/*\n!.mkit/config.toml\n' >>.git/info/exclude

	run git check-ignore -q .mkit/
	[ "$status" -eq 0 ]
	run git check-ignore -q .mkit
	[ "$status" -ne 0 ]

	# What mkit_run_ignored actually asks, and it turns on none of that.
	run git check-ignore -q .mkit/gate.jsonl
	[ "$status" -eq 0 ]
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -eq 0 ]
}

# One probe cannot answer for the whole scratch root. An unrelated `*.jsonl` rule hides
# the ledger while leaving every `.mkit/<skill>-*/` run directory untracked — under
# which `run_ignored=yes` would be reported to a skill whose worktree teardown then
# fails on the untracked run dir.
@test "mkit_run_ignored is not satisfied by a rule that hides only the ledger" {
	printf '*.jsonl\n' >.gitignore
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -ne 0 ]
}

@test "mkit_run_ignored still accepts a legacy directory-only rule" {
	mkdir -p .git/info
	printf '.mkit/\n' >>.git/info/exclude
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -eq 0 ]
}

@test "mkit_run_ignored sees the rule before the directory exists" {
	mkdir -p .git/info
	printf '.mkit/*\n' >>.git/info/exclude
	[ ! -d .mkit ]
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -eq 0 ]
}

@test "mkit_run_ignored accepts a committed .gitignore rule too" {
	printf '.mkit/*\n!.mkit/config.toml\n' >.gitignore
	git add .gitignore
	git commit -q -m 'ignore mkit scratch'
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -eq 0 ]
}

@test "mkit_ensure_run_ignored writes both lines and is idempotent" {
	run bash -c "$(src)mkit_ensure_run_ignored"
	[ "$status" -eq 0 ]
	[ "$(grep -cxF '.mkit/*' .git/info/exclude)" -eq 1 ]
	[ "$(grep -cxF '!.mkit/config.toml' .git/info/exclude)" -eq 1 ]
	run bash -c "$(src)mkit_ensure_run_ignored"
	[ "$status" -eq 0 ]
	[ "$(grep -cxF '.mkit/*' .git/info/exclude)" -eq 1 ]
	[ "$(grep -cxF '!.mkit/config.toml' .git/info/exclude)" -eq 1 ]
}

# The pair's whole purpose: scratch invisible, config committable. Asserted against git
# itself rather than against the file's contents, because what matters is the effect.
@test "the rule hides the scratch and leaves the config committable" {
	bash -c "$(src)mkit_ensure_run_ignored"
	mkdir -p .mkit/review-x
	printf 'log\n' >.mkit/review-x/step.log
	printf '{}\n' >.mkit/gate.jsonl
	[ -z "$(git status --porcelain)" ]

	printf 'version = 1\n' >.mkit/config.toml
	run bash -c "$(src)mkit_config_committable"
	[ "$status" -eq 0 ]
	run git add .mkit/config.toml
	[ "$status" -eq 0 ]
	[[ "$(git status --porcelain)" == *".mkit/config.toml"* ]]
}

# A repo set up before the config existed. `mkit init` there would write a file that
# never reaches a fresh clone, so the state is detected rather than discovered later.
@test "a legacy directory-only rule shadows the config, and the remedy names the file" {
	printf '.mkit/\n' >.gitignore
	run bash -c "$(src)mkit_run_ignored"
	[ "$status" -eq 0 ]
	run bash -c "$(src)mkit_config_committable"
	[ "$status" -ne 0 ]

	run bash -c "$(src)mkit_config_ignored_remedy"
	[ "$status" -eq 0 ]
	[[ "$output" == *".gitignore"* ]]
	[[ "$output" == *'!.mkit/config.toml'* ]]
}

# The fix depends on which rule matched, so the rule is named. A pattern that excludes
# the parent directory cannot be undone by a negation at all — git never descends into
# an excluded directory — but any other pattern is lifted by one. Telling the second
# reader to go replace a `.mkit/` rule sends them looking for a line their file does
# not contain.
@test "the shadowed-config remedy names the rule that actually matched" {
	printf '*.toml\n' >.gitignore
	run bash -c "$(src)mkit_config_committable"
	[ "$status" -ne 0 ]

	run bash -c "$(src)mkit_config_ignored_remedy"
	[ "$status" -eq 0 ]
	[[ "$output" == *'*.toml'* ]]
	[[ "$output" == *'!.mkit/config.toml'* ]]
	# The rule to edit is a file pattern, so there is nothing to "replace".
	[[ "$output" != *'replace the `.mkit/` rule'* ]]
}

# git's default `check-ignore -v` prints `<source>:<line>:<pattern>\t<path>`, so
# `cut -d:` truncates any source path containing a colon — and the whole point of the
# sentence is naming the file where editing the rule does something.
@test "the shadowed-config remedy survives a colon in the excludes-file path" {
	mkdir -p "$MKIT_TMP/od:d"
	printf '.mkit/\n' >"$MKIT_TMP/od:d/ignore"
	git config core.excludesFile "$MKIT_TMP/od:d/ignore"

	run bash -c "$(src)mkit_config_ignored_remedy"
	[ "$status" -eq 0 ]
	[[ "$output" == *"$MKIT_TMP/od:d/ignore"* ]]
}

# `.gitignore` outranks the common dir's `info/exclude`, so a negation written into the
# exclude cannot lift a rule that lives in a committed `.gitignore`. Measured, and the
# reason the remedy names whichever file git reported rather than a fixed one.
@test "an exclude-file negation cannot lift a .gitignore rule" {
	printf '.mkit/\n' >.gitignore
	mkdir -p .git/info
	printf '!.mkit/config.toml\n' >>.git/info/exclude
	run bash -c "$(src)mkit_config_committable"
	[ "$status" -ne 0 ]
}

@test "mkit_config_path resolves inside the toplevel" {
	run bash -c "$(src)mkit_config_path"
	[ "$status" -eq 0 ]
	[ "$output" = "$MKIT_TMP/.mkit/config.toml" ]
}

@test "a linked worktree inherits the rule from the common dir" {
	bash -c "$(src)mkit_ensure_run_ignored"
	git worktree add -q -b wt-d "$MKIT_TMP/wt-d" >/dev/null
	run bash -c "cd '$MKIT_TMP/wt-d' && $(src)mkit_run_ignored"
	[ "$status" -eq 0 ]
}

# --- the reserved root, on both sides of the fingerprint --------------------------------

# --- the writability probe is net-zero, however deep ------------------------------------

@test "the writability probe removes every directory it had to create" {
	# `mkdir -p` creates the whole chain; one rmdir removes only the leaf. facts.sh
	# calls this purely to *report*, so structure left behind means the report changed
	# what it reported on.
	export MKIT_HOME="$MKIT_TMP/absent-parent/deeper/state"
	[ ! -d "$MKIT_TMP/absent-parent" ]
	run bash -c "$(src)mkit_user_dir_writable"
	[ "$status" -eq 0 ]
	[ ! -d "$MKIT_TMP/absent-parent" ]
}

@test "the writability probe never removes a directory that already existed" {
	# The converse hazard, and the reason it counts what it created rather than rmdir-ing
	# on the way out: this directory is the user's state, not the probe's scratch.
	export MKIT_HOME="$MKIT_TMP/pre-existing/state"
	mkdir -p "$MKIT_HOME"
	printf 'x\n' >"$MKIT_HOME/kept"
	run bash -c "$(src)mkit_user_dir_writable"
	[ "$status" -eq 0 ]
	[ -d "$MKIT_HOME" ]
	[ -f "$MKIT_HOME/kept" ]
}
