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

# Opt-in, for the suites that exercise user-scoped *setup* — install.sh and the
# SessionStart hook. Those two write an executable into a bin directory, so unlike every
# other suite they can reach outside MKIT_HOME.
#
# MKIT_BIN alone would be enough if the plumbing were correct, which is exactly why HOME
# is sandboxed too: a bug in that plumbing must not be able to drop a file into the
# developer's real ~/.local/bin. Belt and braces, deliberately redundant.
#
# Call it *after* mkit_setup_repo — that sets user.email/user.name on the throwaway repo,
# so nothing here needs a ~/.gitconfig. Opt-in rather than default so the existing suites
# keep running against the same environment they were written for.
mkit_sandbox_home() {
	export HOME="$MKIT_TMP/home"
	export MKIT_BIN="$MKIT_TMP/bin"
	mkdir -p "$HOME" "$MKIT_BIN"
}

# A PATH containing symlinks to only the externals the setup scripts legitimately call,
# minus the tools named as arguments. The honest way to simulate a missing jq: excluding
# whole PATH directories takes out more than intended, since macOS keeps jq beside
# dirname and sed. Doubles as an executable inventory of the scripts' external surface.
#
#   mkit_fake_path jq node    -> prints a PATH with everything but jq and node
mkit_fake_path() {
	local excluded=" $* " dir tool path
	dir="$MKIT_TMP/fakebin"
	rm -rf -- "$dir"
	mkdir -p "$dir"
	# `bash` and `sh` are here because `env PATH=<fake> bash -c ...` resolves the
	# interpreter itself on the new PATH — omit them and every such run exits 127 with an
	# empty output, which reads exactly like the hook staying silent.
	for tool in bash sh mkdir mv mktemp chmod grep awk rm cut head dirname sed cat git jq node shasum; do
		case "$excluded" in *" $tool "*) continue ;; esac
		path="$(command -v "$tool" 2>/dev/null)" || continue
		ln -sf "$path" "$dir/$tool"
	done
	printf '%s\n' "$dir"
}
