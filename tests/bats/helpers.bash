# Shared bats setup: a throwaway git repo per test, torn down after.
#
# Every *.bats file sources this and calls mkit_setup_repo / mkit_teardown_repo
# from its own setup()/teardown() — bats does not let a sourced file's
# setup() override the test file's.

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../scripts" && pwd)"

mkit_setup_repo() {
	MKIT_TMP="$(mktemp -d "${TMPDIR:-/tmp}/mkit-test.XXXXXX")"
	# Canonicalize: TMPDIR is a symlink on macOS (/tmp -> /private/tmp), and git
	# reports --absolute-git-dir resolved, so a raw mktemp path never matches it.
	MKIT_TMP="$(cd "$MKIT_TMP" && pwd -P)"
	# Point the user-scoped config at the throwaway repo before anything runs. Without
	# this the suite reads the developer's real ~/.claude/mkit, so a machine where
	# install.sh has written journal.default would see every "a pristine repo is
	# disabled" assertion fail — the tests would be measuring the developer, not the
	# code. Exported, because the scripts run as children.
	export MKIT_HOME="$MKIT_TMP/.mkit-home"
	cd "$MKIT_TMP" || return 1
	git init -q -b main .
	git config user.email test@example.com
	git config user.name "mkit test"
	git config commit.gpgsign false
	printf 'seed\n' >seed.txt
	git add seed.txt
	git commit -q -m 'seed'
}

mkit_teardown_repo() {
	[ -n "${MKIT_TMP:-}" ] && [ -d "$MKIT_TMP" ] && rm -rf -- "$MKIT_TMP"
}
