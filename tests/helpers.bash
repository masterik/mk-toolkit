# Shared bats setup for the bundled scripts: a throwaway git repo per test under
# $TMPDIR, with MKIT_HOME inside it — never HOME — torn down after.

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")/../plugin/skills/_shared/scripts" && pwd)"
# /bin/bash is 3.2 on macOS and the scripts must run there unchanged (ADR 0006
# decision 3); on Linux it is bash 5.
TEST_BASH="${MKIT_TEST_BASH:-/bin/bash}"

facts() { "$TEST_BASH" "$SCRIPTS/facts.sh" "$@"; }
branch_status() { "$TEST_BASH" "$SCRIPTS/branch-status.sh" "$@"; }

mkit_setup_repo() {
	REPO="$(mktemp -d "${TMPDIR:-/tmp}/mkit-test.XXXXXX")"
	# Canonical: TMPDIR is a symlink on macOS, and git reports paths resolved.
	REPO="$(cd "$REPO" && pwd -P)"
	TEST_TMP="$(cd "$BATS_TEST_TMPDIR" && pwd -P)"
	# Git Bash: git and the scripts print `C:/x`, the shell `/c/x` (#58).
	if command -v cygpath >/dev/null 2>&1; then
		REPO="$(cygpath -m "$REPO")" TEST_TMP="$(cygpath -m "$TEST_TMP")"
	fi
	export MKIT_HOME="$REPO/.mkit-home"
	# The developer's git config must not reach the test (hooks, signing, templates).
	export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
	# Never let a git call in a test walk up into an outer repo.
	export GIT_CEILING_DIRECTORIES="$(dirname "$REPO")"
	cd "$REPO" || return 1
	git init -q -b main .
	git config user.email test@example.com
	git config user.name "mkit test"
	printf 'seed\n' >seed.txt
	printf '.mkit-home/\n' >.git/info/exclude
	commit_here seed
}

mkit_teardown_repo() {
	[ -n "${REPO:-}" ] && [ -d "$REPO" ] && rm -rf -- "$REPO"
}

# Commit everything in the current work tree — only after proving it is a throwaway
# repo. A nested repo that failed to init would otherwise commit to the outer one.
commit_here() {
	local top
	top="$(git rev-parse --show-toplevel)" || return 1
	case "$top" in
	"$REPO" | "$TEST_TMP"/*) ;;
	*)
		printf 'refusing to commit outside the throwaway repo: %s\n' "$top" >&2
		return 1
		;;
	esac
	git add -A
	git commit -q -m "$1"
}

# `key=value` from $output; some lines pack several pairs, so split on spaces too.
field() { printf '%s\n' "$output" | sed '/^notes:/,$d' | tr ' ' '\n' | sed -n "s/^$1=//p" | head -1; }

# Everything above `notes:`.
facts_only() { printf '%s\n' "$output" | sed '/^notes:/,$d'; }

# A PATH holding only the tools the scripts call, minus the ones named.
fake_path() {
	local dir="$BATS_TEST_TMPDIR/fakebin" tool p excluded=" $* "
	rm -rf -- "$dir"
	mkdir -p "$dir"
	for tool in bash git sed grep awk head cut paste tr mktemp mkdir rm rmdir dirname cat jq gh; do
		case "$excluded" in *" $tool "*) continue ;; esac
		p="$(command -v "$tool" 2>/dev/null)" || continue
		# A wrapper, not a symlink: Git Bash's `ln -s` copies the .exe away from the
		# msys DLL beside it, and the copy does not start (#58).
		printf '#!/bin/sh\nexec "%s" "$@"\n' "$p" >"$dir/$tool"
		chmod +x "$dir/$tool"
	done
	printf '%s\n' "$dir"
}

# A stub `gh` first on PATH. It logs every call to $GH_LOG, fails `auth status` when
# GH_AUTH=fail, and answers `pr view`/`pr list` from the JSON in $GH_VIEW/$GH_LIST
# (absent = gh's "no pull requests found", exit 1), applying `--jq` with the real jq
# so the scripts' filters are exercised. GH_ERROR set: `pr view`/`pr list` fail with it;
# GH_FAIL_SILENT=1: they fail with no output at all.
stub_gh() {
	local dir="$BATS_TEST_TMPDIR/stubgh"
	mkdir -p "$dir"
	export GH_LOG="$BATS_TEST_TMPDIR/gh.log"
	cat >"$dir/gh" <<'EOS'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$GH_LOG"
case "$1 $2" in
"auth status") [ "${GH_AUTH:-ok}" = ok ] ;;
"pr view" | "pr list")
	[ -z "${GH_FAIL_SILENT:-}" ] || exit 1
	[ -z "${GH_ERROR:-}" ] || { printf '%s\n' "$GH_ERROR" >&2; exit 1; }
	if [ "$2" = view ]; then json="${GH_VIEW:-}"; else json="${GH_LIST:-}"; fi
	[ -n "$json" ] || { echo 'no pull requests found' >&2; exit 1; }
	jq_expr=''
	while [ $# -gt 0 ]; do
		[ "$1" = --jq ] && jq_expr="$2"
		shift
	done
	if [ -n "$jq_expr" ]; then printf '%s' "$json" | jq -r "$jq_expr"; else printf '%s\n' "$json"; fi
	;;
*) exit 1 ;;
esac
EOS
	chmod +x "$dir/gh"
	PATH="$dir:$PATH"
}

# A `git` first on PATH that fails — exit 128, like git's own fatal — for any call
# whose argument list contains the words in $GIT_FAIL, and runs the real git for
# every other call. With a second argument N, the first N matching calls run the real
# git and only later ones fail (branch-status.sh repeats a query facts.sh made first).
stub_git_fail() {
	local dir="$BATS_TEST_TMPDIR/stubgit" real
	real="$(command -v git)" || return 1
	mkdir -p "$dir"
	cat >"$dir/git" <<EOS
#!/usr/bin/env bash
case " \$* " in
*" \$GIT_FAIL "*)
	n=\$(cat "\$GIT_FAIL_COUNT" 2>/dev/null || echo 0)
	echo \$((n + 1)) >"\$GIT_FAIL_COUNT"
	[ "\$n" -lt "\$GIT_FAIL_SKIP" ] || { echo "fatal: stubbed failure: \$*" >&2; exit 128; }
	;;
esac
exec '$real' "\$@"
EOS
	chmod +x "$dir/git"
	export GIT_FAIL="$1" GIT_FAIL_SKIP="${2:-0}" GIT_FAIL_COUNT="$dir/count"
	rm -f -- "$GIT_FAIL_COUNT"
	PATH="$dir:$PATH"
}
