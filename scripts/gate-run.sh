#!/usr/bin/env bash
#
# Run quality-gate steps, log each in full, and print a bounded verdict.
#
#   usage: gate-run.sh <run-dir> [--tail N] [--grep N] [--keep-going] <step> -- <command...>
#          gate-run.sh <run-dir> [--tail N] [--grep N] [--keep-going] --chain '<step>=<command>' ...
#
#   The option flags must come BEFORE `--` (and before the `--chain` specs): everything
#   after `--` is the command, so a flag placed there is passed to the command instead.
#
#   gate-run.sh "$RUN" lint -- bun run lint
#   gate-run.sh "$RUN" --chain 'lint=bun run lint' 'test=bun run test' 'build=bun run build'
#
# Why a script: the pattern is four invariants that must all hold at once — the command's
# full output goes to <run-dir>/gate-<step>.log, the exit code is captured before anything
# else can clobber it, the chain stops on the first failure, and what reaches the agent is
# a verdict plus a bounded excerpt rather than the log. Hand-rolled, one of them is always
# the one that slips: an unquoted run dir sends the log to /, or `cmd | tail` reports
# tail's exit code and a red suite passes.
#
# Output, per step:  <step> ok <secs>s
#          on fail:  <step> FAIL exit=N <secs>s log=<path>, then the grepped failures and
#                    the tail, each capped
# Last line:         gate=ok steps=... | gate=FAILED step=<step> exit=N
#
# Exit: the failing step's exit code, 0 if everything passed, 2 on bad usage.

set -uo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

FAIL_PAT='FAIL|FAILED|Error:|error:|error\[|ERROR|✗|✖|✘|panic:|Exception|AssertionError|not ok |Traceback|\[error\]|failed with'

tail_n=30
grep_n=40
keep_going=no
run_dir=""
mode=""
declare -a steps=()

[ $# -ge 2 ] || mkit_die 'usage: gate-run.sh <run-dir> <step> -- <command...> | gate-run.sh <run-dir> --chain "step=cmd" ...' 2
run_dir="$1"
shift
[ -d "$run_dir" ] || mkit_die "run directory does not exist: $run_dir (open it with run-open.sh)" 2

while [ $# -gt 0 ]; do
	case "$1" in
	--tail)
		tail_n="${2:-30}"
		shift 2
		;;
	--grep)
		grep_n="${2:-40}"
		shift 2
		;;
	--keep-going)
		keep_going=yes
		shift
		;;
	--chain)
		mode=chain
		shift
		while [ $# -gt 0 ]; do
			case "$1" in
			--tail | --grep | --keep-going) break ;;
			*)
				steps+=("$1")
				shift
				;;
			esac
		done
		;;
	--)
		shift
		[ -n "$mode" ] || mkit_die 'no step name before --' 2
		# `$*` would flatten argv on spaces and `bash -c` below would re-split it, so
		# `-- printf '[%s]\n' 'foo bar'` ran as two arguments. Single-quote each word
		# (doubling any embedded quote) so the command sees exactly what was passed.
		quoted=""
		for arg in "$@"; do
			case "$arg" in
			*\'*) esc="$(printf '%s' "$arg" | sed "s/'/'\\\\''/g")" ;;
			*) esc="$arg" ;;
			esac
			quoted="${quoted:+$quoted }'$esc'"
		done
		steps=("$mode=$quoted")
		mode=chain
		break
		;;
	-*) mkit_die "unknown option: $1" 2 ;;
	*)
		# a bare word before `--` is the single step's name
		mode="$1"
		shift
		;;
	esac
done

[ "${#steps[@]}" -gt 0 ] || mkit_die 'nothing to run' 2

overall=0
failed_step=""
ran=""

for spec in "${steps[@]}"; do
	step="${spec%%=*}"
	cmd="${spec#*=}"
	[ -n "$step" ] && [ "$step" != "$cmd" ] || mkit_die "malformed step spec: $spec (want 'name=command')" 2
	# An empty command makes `bash -c ''` exit 0 and the step print ok — a green gate that
	# ran nothing, the one failure class quality-gate.md forbids outright.
	[ -n "$cmd" ] || mkit_die "step '$step' has an empty command: a gate cannot pass by running nothing" 2
	mkit_check_slug "$step"

	log="$run_dir/gate-$step.log"
	start=$SECONDS
	# The exit code is captured on the very next line, before any pipe or test can
	# replace $?. Everything the command says lands in the log, nothing on our stdout.
	bash -c "$cmd" >"$log" 2>&1
	code=$?
	secs=$((SECONDS - start))

	if [ "$code" -eq 0 ]; then
		printf '%s ok %ds\n' "$step" "$secs"
		ran="${ran:+$ran,}$step"
		continue
	fi

	printf '%s FAIL exit=%d %ds log=%s\n' "$step" "$code" "$secs" "$log"
	hits="$(mkit_search "$grep_n" "$FAIL_PAT" "$log" || true)"
	if [ -n "$hits" ]; then
		printf 'failures (max %d):\n%s\n' "$grep_n" "$hits"
	fi
	printf 'tail -%d:\n' "$tail_n"
	tail -n "$tail_n" -- "$log"
	printf 'log_lines=%s\n' "$(wc -l <"$log" | tr -d ' ')"

	overall="$code"
	failed_step="$step"
	ran="${ran:+$ran,}$step"
	[ "$keep_going" = yes ] || break
done

if [ "$overall" -eq 0 ]; then
	printf 'gate=ok steps=%s\n' "$ran"
else
	printf 'gate=FAILED step=%s exit=%d\n' "$failed_step" "$overall"
fi
exit "$overall"
