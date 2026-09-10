#!/usr/bin/env bats
#
# Two static assertions over the shipped payload — regression guards, not the primary
# verification of anything.
#
# They exist because neither invariant has a behavioral seam. A bats run cannot create an
# OS sandbox, and the Darwin per-user temp directory cannot be made unwritable from inside
# a test; nor can a test refuse a write the way the auto-mode classifier does. The
# alternative was a `mktemp` recorder on PATH — a second shim to maintain, asserting an
# argument shape rather than a behavior — so the properties are asserted over the source
# instead, in the same family as the existing rule that payload scripts never call the
# output-reshaping proxy.
#
# The primary verification of the temp-file fix is the rest of the suite: 49 of its tests
# failed under the sandbox before it and pass after.

load helpers.bash

PAYLOAD=""

setup() {
	PAYLOAD="$(cd "$(dirname "${BATS_TEST_FILENAME}")/../../plugin" && pwd)"
}

# Every shipped shell file: the scripts and the sourced library.
# Not tools/ — that is staging for a port and is not part of the plugin payload.
payload_shell() {
	find "$PAYLOAD" -type f -name '*.sh' | LC_ALL=C sort
}

@test "the payload ships at least the files these assertions are about" {
	n="$(payload_shell | grep -c .)"
	[ "$n" -ge 5 ]
	payload_shell | grep -q '/scripts/lib/common.sh$'
	payload_shell | grep -q '/scripts/facts.sh$'
}

# --- invariant 1: no template-less mktemp ---------------------------------------------
#
# On macOS `mktemp` with no template — bare, or `-t <prefix>` — resolves the Darwin
# per-user temp directory and **ignores `$TMPDIR`**, so under the OS sandbox it fails
# with `mkstemp failed on /var/folders/…: Operation not permitted`. It is not fixable by
# environment, which is why both forms are banned outright rather than discouraged.
# Ephemeral files come from `mkit_tmpfile`; anything that must outlive the command is
# named relative to a root the caller already resolved.

# Each `mktemp` invocation on a line of its own, so one templated call cannot vouch for a
# bare one beside it. The check has to be per-invocation, not per-line: a line-based
# `grep -v XXXXXX` discards the whole line on the first template it sees, so
# `a=$(mktemp "$d/x.XXXXXX"); b=$(mktemp)` read as clean — and the `-t` assertion below
# cannot see a bare call either, which left the original failure mechanism unguarded.
#
# Segmenting at `;`/`&&`/`||` and immediately before every further `mktemp` is enough for
# that: the argument list of one call never contains those operators.
payload_mktemp_calls() {
	payload_code "$1" |
		awk '{ gsub(/;|&&|\|\|/, "\n"); gsub(/mktemp/, "\nmktemp"); print }' |
		grep -E '(^|[^[:alnum:]_])mktemp([^[:alnum:]_]|$)' || true
}

@test "no shipped script uses a template-less mktemp" {
	offenders=""
	while IFS= read -r f; do
		# A template is a path argument containing XXXXXX. Anything else — a bare
		# `mktemp`, `mktemp -d` with no path, `mktemp -t prefix` — is the banned form.
		hits="$(payload_mktemp_calls "$f" | grep -vE 'XXXXXX' || true)"
		[ -n "$hits" ] && offenders="$offenders
$f:
$hits"
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'template-less mktemp in the payload:%s\n' "$offenders"
		return 1
	}
}

@test "no shipped script uses mktemp -t at all" {
	offenders=""
	while IFS= read -r f; do
		hits="$(payload_mktemp_calls "$f" |
			grep -E 'mktemp[[:space:]]+(-[a-zA-Z]+[[:space:]]+)*-t([[:space:]]|$)' || true)"
		[ -n "$hits" ] && offenders="$offenders
$f: $hits"
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'mktemp -t in the payload:%s\n' "$offenders"
		return 1
	}
}

@test "the mktemp scanner sees each call on a line separately" {
	# The self-check for the segmentation above: without it this fixture reads clean.
	fixture="$BATS_TEST_TMPDIR/mktemp-fixture.sh"
	printf '%s\n' 'a="$(mktemp "$d/x.XXXXXX")"; b="$(mktemp)"' >"$fixture"
	run bash -c "$(declare -f payload_uncommented payload_code payload_mktemp_calls); payload_mktemp_calls '$fixture' | grep -vcE XXXXXX"
	[ "$output" = 1 ]
}

# --- invariant 2: the writable set is a property of the payload -------------------------
#
# mkit writes in exactly three places, and nowhere else:
#
#   $TMPDIR                anything that dies with the command   (mkit_tmpfile)
#   <toplevel>/.mkit/      run directories, gate.jsonl, and the   (mkit_dir_or_die)
#                          one committed file, config.toml
#   ~/.mkit/               nothing today; still the declared      (mkit_user_dir)
#                          home for user-scoped state, and the
#                          probe target facts.sh reports on
#
# plus one named exception: the common dir's `info/exclude`, where the payload writes the
# two-line rule (`.mkit/*` and `!.mkit/config.toml`) that keeps the scratch out of
# `git status` while leaving repo config committable. Nothing is ever written to the user's working tree — an
# improvised helper script landed in a target repository's working tree once, and that
# incident is why this is an assertion rather than a habit.
#
# Asserted as a shape, which is what makes it mechanical: every write target is a shell
# parameter, never a literal path, and the parameter names are a reviewed allowlist. A
# future `> /tmp/build.log` or `>> "$HOME/.claude/jobs/x"` trips this immediately, and the
# allowlist is the review surface for anything genuinely new.

# The write targets the payload is allowed to name, and the root each belongs to.
#
#   tmp_p tmp_d tmp_s tmp_h pr_cache   $TMPDIR, via mkit_tmpfile
#   dir prefix                          the same, inside mkit_tmpfile itself
#   mkit_dir run_dir d ledger tmp       <toplevel>/.mkit — run dirs, gate.jsonl and its
#   alive lock log skill                rewrite files, the step logs, the ledger lock
#   user_dir probe                      ~/.mkit, via mkit_user_dir — the writability
#                                       probe is the only thing that touches it now
#   common exclude                      the one named exception: the common-dir exclude
#   dirname                             `$(dirname -- "$X")`, the parent of a target
#                                       already on this list
WRITE_VARS='tmp_p tmp_d tmp_s tmp_h pr_cache dir prefix mkit_dir run_dir d ledger tmp alive lock log skill user_dir probe common exclude dirname'

# Comments go first: these files document the very mistakes being asserted against.
payload_uncommented() {
	sed -E -e 's/^[[:space:]]*#.*$//' -e 's/[[:space:]]#[[:space:]].*$//' "$1"
}

# ...and then single-quoted strings, because every `printf` format and `mkit_die` message
# in the payload is single-quoted, so a format string naming a path is not a write.
#
# That has a cost, and it is paid by the assertion below rather than absorbed silently: a
# genuine single-quoted target — `> '/tmp/build.log'`, `mkdir '/tmp/x'` — disappears here
# along with its quotes, invisible to every scan built on this. So the form is banned
# outright and checked against `payload_uncommented`, where the quotes are still there.
payload_code() {
	payload_uncommented "$1" | sed -E "s/'[^']*'//g"
}

@test "no shipped script writes to a single-quoted literal" {
	offenders=""
	while IFS= read -r f; do
		hits="$(payload_uncommented "$f" |
			grep -nE ">>?[[:space:]]*'|(^|[^[:alnum:]_])(mkdir|mktemp|touch|tee|mv|cp)([[:space:]]+(-[a-zA-Z-]+|--))*[[:space:]]+'" || true)"
		[ -n "$hits" ] && offenders="$offenders
$f: $hits"
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'single-quoted write target in the payload:%s\n' "$offenders"
		printf 'the write scanners cannot see through single quotes — use a parameter.\n'
		return 1
	}
}

# Every construct that can create or modify a file, reduced to the argument that names
# its target: a redirection, or the first non-flag operand of a file-creating command.
#
# A bare number is dropped. The only ones that occur are `> 0` fragments from *inside* a
# multi-line embedded awk program, whose opening quote is on an earlier line than the
# stripper can see; a file literally named `0` is not a thing anyone writes, so filtering
# them costs no coverage.
payload_write_targets() {
	payload_code "$1" |
		grep -oE '(>>?[[:space:]]*|(mkdir|mktemp|touch|tee|mv|cp)([[:space:]]+(-[a-zA-Z-]+|--))*[[:space:]]+)[^[:space:];&|)]+' |
		sed -E 's/^(>>?|mkdir|mktemp|touch|tee|mv|cp)[[:space:]]*//; s/^((-[a-zA-Z-]+|--)[[:space:]]+)*//' |
		grep -vE '^(&[0-9-]|/dev/null$|[0-9]+$)' |
		sed 's/^"//; s/"$//' |
		grep -v '^$'
}

# `payload_write_targets` reads the *first* non-flag operand of a command, which is the
# source for `mv`/`cp` and only the first of several for `mkdir`/`touch`/`tee`. So the
# destination — the operand that actually gets written — went unchecked. These two
# functions cover it: the last operand of every copy, and a count of how many targets each
# command takes, which is the assumption the first-operand matcher rests on.
#
# `$(…)` collapses to one token first, or `mkdir -p "$(dirname -- "$ledger")"` reads as
# three operands. Nothing in the payload nests a substitution inside another.
payload_collapsed() {
	payload_code "$1" | sed -E 's/\$\([^()]*\)/\$SUBST/g'
}

payload_copy_destinations() {
	payload_collapsed "$1" |
		grep -oE '(^|[^[:alnum:]_])(mv|cp)([[:space:]]+(-[a-zA-Z-]+|--))*([[:space:]]+[^[:space:];&|]+){2}' |
		sed -E 's/.*[[:space:]]([^[:space:]]+)$/\1/' |
		sed 's/^"//; s/"$//' || true
}

@test "every mv/cp destination is a parameter on the reviewed allowlist" {
	offenders=""
	while IFS= read -r f; do
		while IFS= read -r t; do
			[ -n "$t" ] || continue
			name="$(printf '%s' "$t" | sed -E 's/^\$\(?\{?([A-Za-z_][A-Za-z0-9_]*).*/\1/')"
			case " $WRITE_VARS " in
			*" $name "*) continue ;;
			esac
			offenders="$offenders
  $f: $t"
		done <<-INNER
			$(payload_copy_destinations "$f")
		INNER
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'mv/cp destination outside the reviewed set:%s\n' "$offenders"
		return 1
	}
}

@test "the copy-destination scanner sees the destination, not the source" {
	# Self-check: an allowlisted source with a forbidden destination is the exact shape
	# the first-operand matcher validated and waved through.
	fixture="$BATS_TEST_TMPDIR/copy-fixture.sh"
	printf '%s\n' 'mv -f -- "$tmp" "$HOME/output"' >"$fixture"
	run bash -c "$(declare -f payload_uncommented payload_code payload_collapsed payload_copy_destinations); payload_copy_destinations '$fixture'"
	[ "$output" = '$HOME/output' ]
}

@test "no file-creating command in the payload takes more than one target operand" {
	# The first-operand matcher is sufficient only while this holds. `mkdir a b` writes
	# both and would be checked on `a` alone, so a second target is not a style question
	# here — it is a hole in the guard. Add one and this test names the file.
	#
	# Redirections come off first: `mkdir -p "$dir" 2>/dev/null` is one target, and
	# counting `2>/dev/null` as a second made every mkdir in the payload an offender.
	offenders=""
	while IFS= read -r f; do
		hits="$(payload_collapsed "$f" |
			sed -E 's/[0-9]*>>?[[:space:]]*[^[:space:];&|]+//g' |
			grep -nE '(^|[^[:alnum:]_])(mkdir|touch|tee)([[:space:]]+(-[a-zA-Z-]+|--))*([[:space:]]+[^-[:space:];&|<>][^[:space:];&|<>]*){2}' || true)"
		[ -n "$hits" ] && offenders="$offenders
$f: $hits"
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'multi-target file-creating command in the payload:%s\n' "$offenders"
		printf 'payload_write_targets reads only the first operand — split the command.\n'
		return 1
	}
}

@test "the scanner finds the writes the payload actually makes" {
	# A guard whose matcher silently stopped matching is worse than no guard. These are
	# the writes that must be visible to it for the two assertions below to mean anything.
	all=""
	while IFS= read -r f; do
		all="$all
$(payload_write_targets "$f" || true)"
	done <<-EOF
		$(payload_shell)
	EOF
	printf '%s\n' "$all" | grep -q 'tmp_p'     # the fingerprint's ephemeral files
	printf '%s\n' "$all" | grep -q 'pr_cache'  # the branch classifier's PR cache
	printf '%s\n' "$all" | grep -q 'ledger'    # gate.jsonl
	printf '%s\n' "$all" | grep -q 'exclude'   # the ignore rule
	printf '%s\n' "$all" | grep -q 'mkit_dir'  # the run root
}

@test "every write target in the payload is a parameter, not a literal path" {
	offenders=""
	while IFS= read -r f; do
		while IFS= read -r t; do
			[ -n "$t" ] || continue
			case "$t" in
			'$'*) continue ;;
			esac
			offenders="$offenders
  $f: $t"
		done <<-INNER
			$(payload_write_targets "$f")
		INNER
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'literal write target in the payload:%s\n' "$offenders"
		return 1
	}
}

@test "every write target names a variable on the reviewed allowlist" {
	offenders=""
	while IFS= read -r f; do
		while IFS= read -r t; do
			[ -n "$t" ] || continue
			# "$ledger.XXXXXX" -> ledger; "$dir/$prefix.XXXXXX" -> dir;
			# "$(dirname -- "$file")" -> dirname.
			name="$(printf '%s' "$t" | sed -E 's/^\$\(?\{?([A-Za-z_][A-Za-z0-9_]*).*/\1/')"
			case " $WRITE_VARS " in
			*" $name "*) continue ;;
			esac
			offenders="$offenders
  $f: \$$name"
		done <<-INNER
			$(payload_write_targets "$f")
		INNER
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'write target outside the reviewed set:%s\n' "$offenders"
		printf 'if it is legitimate, add it to WRITE_VARS with the root it belongs to.\n'
		return 1
	}
}

@test "no shipped script names the Darwin temp dir, /tmp, or a protected path" {
	offenders=""
	while IFS= read -r f; do
		hits="$(payload_code "$f" | grep -nE '/var/folders/|(^|[^[:alnum:]_./])/tmp/|[.]claude/mkit|[.]claude/jobs' || true)"
		[ -n "$hits" ] && offenders="$offenders
$f: $hits"
	done <<-EOF
		$(payload_shell)
	EOF
	[ -z "$offenders" ] || {
		printf 'hardcoded temp or protected path in the payload:%s\n' "$offenders"
		return 1
	}
}

@test "the one non-shell payload file writes only inside the run directory" {
	# The scanner above reads shell. `findings.mjs` is the payload's other half, and the
	# same invariant has to hold for it: every write goes through one `writeJsonl`, and
	# every path handed to it is `join(runDir, …)`.
	mjs="$PAYLOAD/scripts/findings.mjs"
	[ -f "$mjs" ]
	# Exactly one writer, so there is one place to check.
	[ "$(grep -cE '\b(writeFileSync|appendFileSync|createWriteStream|mkdirSync|rmSync|unlinkSync)\(' "$mjs")" -eq 1 ]
	grep -qE 'writeFileSync\(path,' "$mjs"
	# And every call site roots its path in runDir.
	targets="$(grep -oE 'writeJsonl\([^,]+,' "$mjs" | sed -E 's/writeJsonl\(//; s/,$//')"
	[ -n "$targets" ]
	while IFS= read -r t; do
		[ -n "$t" ] || continue
		case "$t" in
		'join(runDir' | out) continue ;;
		esac
		printf 'findings.mjs writes to an unreviewed target: %s\n' "$t"
		return 1
	done <<-EOF
		$targets
	EOF
	# `out` is only ever assigned from join(runDir, …) — assert that rather than trusting
	# the name.
	while IFS= read -r line; do
		case "$line" in
		*'join(runDir,'*) continue ;;
		esac
		printf 'findings.mjs assigns out from something other than runDir: %s\n' "$line"
		return 1
	done <<-EOF
		$(grep -E '^\s*const out = ' "$mjs" | grep -v 'const out = \[\]')
	EOF
}
