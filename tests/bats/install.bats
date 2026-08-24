#!/usr/bin/env bats
#
# install.sh — the by-hand half of user-scoped setup. There was no coverage for this file
# at all before; the SessionStart hook now shares its wrapper generator and its
# prerequisite table with it, so drift between the two is a real failure mode and the
# last test here is the one that catches it.
#
# `[[ ... ]]` is avoided throughout: on bash 3.2 errexit does not fire for a failing
# `[[ ... ]]`, so such an assertion only aborts a test as its last command.

load helpers.bash

setup() {
	mkit_setup_repo
	mkit_sandbox_home
	INSTALL="$(cd "$SCRIPTS/.." && pwd)/install.sh"
	USER_DIR="$MKIT_HOME"
	MARKER="$USER_DIR/journal.default"
	TOMBSTONE="$USER_DIR/bootstrap.disabled"
	STATE="$USER_DIR/bootstrap.state"
	WRAPPER="$MKIT_BIN/mkit-journal"
	HOOK="$SCRIPTS/hooks/session-bootstrap.sh"
}

teardown() {
	mkit_teardown_repo
}

has_text() { printf '%s\n' "$1" | grep -qF -- "$2"; }

@test "--status on a pristine user dir reports the default off and writes nothing" {
	run bash "$INSTALL" --status
	[ "$status" -eq 0 ]
	has_text "$output" 'user default: off'
	[ ! -f "$MARKER" ]
	[ ! -e "$WRAPPER" ]
}

@test "--status still lists every prerequisite row, including the ok ones" {
	run bash "$INSTALL" --status
	has_text "$output" 'prerequisites:'
	has_text "$output" 'git'
	has_text "$output" 'jq'
	has_text "$output" 'node'
	has_text "$output" 'sha256'
}

@test "a plain run writes the marker and the wrapper" {
	run bash "$INSTALL"
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
	[ -x "$WRAPPER" ]
	run "$WRAPPER" enabled --why
	[ "$output" = 'enabled user' ]
}

@test "--no-bin writes the marker but no wrapper" {
	run bash "$INSTALL" --no-bin
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
	[ ! -e "$WRAPPER" ]
}

@test "--bin honours an explicit directory" {
	mkdir -p "$MKIT_TMP/otherbin"
	run bash "$INSTALL" --bin "$MKIT_TMP/otherbin"
	[ "$status" -eq 0 ]
	[ -x "$MKIT_TMP/otherbin/mkit-journal" ]
	[ ! -e "$WRAPPER" ]
}

@test "a plain run pre-spends the hook's notice, so the hook stays silent after it" {
	run bash "$INSTALL"
	run grep -qxF 'notice/v1' "$STATE"
	[ "$status" -eq 0 ]
	# The point of pre-spending: install.sh just said all of this out loud, at length.
	run bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "--uninstall removes both files and writes the tombstone" {
	bash "$INSTALL" >/dev/null
	run bash "$INSTALL" --uninstall
	[ "$status" -eq 0 ]
	[ ! -f "$MARKER" ]
	[ ! -e "$WRAPPER" ]
	[ -f "$TOMBSTONE" ]
	has_text "$output" 'pinned off'
}

@test "the tombstone is non-empty and says how to undo itself" {
	bash "$INSTALL" >/dev/null
	bash "$INSTALL" --uninstall >/dev/null
	run cat "$TOMBSTONE"
	has_text "$output" 'install.sh'
}

@test "--uninstall makes the removal outlive the next session" {
	bash "$INSTALL" >/dev/null
	bash "$INSTALL" --uninstall >/dev/null
	run bash -c "'$HOOK' </dev/null"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	[ ! -f "$MARKER" ]
}

@test "--uninstall leaves a wrapper it did not generate alone" {
	printf '#!/bin/sh\necho someone elses tool\n' >"$WRAPPER"
	chmod 755 "$WRAPPER"
	run bash "$INSTALL" --uninstall
	[ "$status" -eq 0 ]
	[ -e "$WRAPPER" ]
	has_text "$output" 'left alone'
}

@test "--uninstall does not follow a symlink at the wrapper path" {
	ln -s /bin/echo "$WRAPPER"
	run bash "$INSTALL" --uninstall
	[ "$status" -eq 0 ]
	[ -L "$WRAPPER" ]
}

@test "a re-run clears the tombstone and restores the default" {
	bash "$INSTALL" >/dev/null
	bash "$INSTALL" --uninstall >/dev/null
	[ -f "$TOMBSTONE" ]
	run bash "$INSTALL"
	[ "$status" -eq 0 ]
	[ ! -f "$TOMBSTONE" ]
	[ -f "$MARKER" ]
	has_text "$output" 'un-pinned'
}

@test "--uninstall --purge removes the tombstone and says the hook will re-run" {
	bash "$INSTALL" >/dev/null
	run bash "$INSTALL" --uninstall --purge
	[ "$status" -eq 0 ]
	[ ! -f "$TOMBSTONE" ]
	[ ! -f "$MARKER" ]
	has_text "$output" 'set this up again'
	# And it means it: the hook is free to act again.
	run bash -c "'$HOOK' </dev/null"
	[ -f "$MARKER" ]
}

@test "--status reports a pinned-off default distinctly from a never-set-up one" {
	bash "$INSTALL" >/dev/null
	bash "$INSTALL" --uninstall >/dev/null
	run bash "$INSTALL" --status
	has_text "$output" 'pinned off'
}

@test "--status reports the tombstone path so it can be found" {
	bash "$INSTALL" >/dev/null
	bash "$INSTALL" --uninstall >/dev/null
	run bash "$INSTALL" --status
	has_text "$output" 'bootstrap.disabled'
}

@test "a hard prerequisite gap refuses the install unless forced" {
	run env PATH="$(mkit_fake_path jq)" bash "$INSTALL"
	[ "$status" -eq 1 ]
	[ ! -f "$MARKER" ]
	run env PATH="$(mkit_fake_path jq)" bash "$INSTALL" --force
	[ "$status" -eq 0 ]
	[ -f "$MARKER" ]
}

@test "an unknown flag exits 2" {
	run bash "$INSTALL" --nonsense
	[ "$status" -eq 2 ]
}

@test "install.sh and session-bootstrap.sh produce byte-identical wrappers" {
	# The anti-drift test. Two producers of one file is exactly how the hook's
	# "is this wrapper mine?" check rots: let the bodies diverge and the hook either
	# clobbers install.sh's wrapper or refuses to touch its own. Everything about the
	# shared generator in lib/common.sh is load-bearing only while this passes.
	bash "$INSTALL" >/dev/null
	cp "$WRAPPER" "$MKIT_TMP/from-install"
	rm -f "$WRAPPER"
	bash -c "'$HOOK' </dev/null" >/dev/null
	run diff "$MKIT_TMP/from-install" "$WRAPPER"
	[ "$status" -eq 0 ]
}
