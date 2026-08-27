#!/usr/bin/env bash
#
# Run quality-gate steps, log each in full, and print a bounded verdict.
#
#   usage: gate-run.sh <run-dir> [--tail N] [--grep N] [--keep-going] [--no-ledger] <step> -- <command...>
#          gate-run.sh <run-dir> [--tail N] [--grep N] [--keep-going] [--no-ledger] --chain '<step>=<command>' ...
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
#
# Side effect — the gate ledger. Each finished step also appends one record to
# <git-dir>/mkit/gate.jsonl: what was proven, over which content. `gate-detect.sh` reads
# it back and classifies; nothing here ever skips a step. The ledger is strictly a side
# effect: it never changes the exit code, the output, or where a chain stops, and a
# failed append is silent. Losing a cache entry is nothing; failing a gate over its own
# bookkeeping is unacceptable. `--no-ledger` turns the write off.

set -uo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

FAIL_PAT='FAIL|FAILED|Error:|error:|error\[|ERROR|✗|✖|✘|panic:|Exception|AssertionError|not ok |Traceback|\[error\]|failed with'

tail_n=30
grep_n=40
keep_going=no
no_ledger=no
run_dir=""
mode=""
declare -a steps=()
# Parallel to `steps`: the *normalized* command to record, one per step.
#
# The two call forms disagree about the command string, and the skills split exactly
# along that seam — `commit`/`review` call the single-step form, `finish`/`pr` call
# `--chain`. The single-step branch shell-quotes each argument (deliberately: it fixes a
# real argv-flattening bug), so the same command records as `'bun' 'run' 'lint'` there
# and `bun run lint` under --chain. Keyed on that, the flagship review -> finish lookup
# would miss every time and the feature would appear to work while caching nothing.
# So the ledger key is always the pre-quoting form; the string actually executed is
# untouched.
declare -a norms=()

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
	--no-ledger)
		no_ledger=yes
		shift
		;;
	--chain)
		mode=chain
		shift
		while [ $# -gt 0 ]; do
			case "$1" in
			--tail | --grep | --keep-going | --no-ledger) break ;;
			*)
				steps+=("$1")
				# The spec's right-hand side is already the normalized form.
				norms+=("${1#*=}")
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
		joined=""
		# Joining argv on single spaces is lossy: `-- printf '%s' 'foo bar'` and
		# `-- printf '%s' foo bar` join to the same string while executing differently, so
		# a key built from it could serve one command's proof for the other — a false
		# `fresh`, the direction a ledger may never be wrong in. Every real gate command
		# is whitespace-free words (`bun run lint`, `go vet ./...`), so the join is what
		# makes the cross-skill lookup hit; fall back to the unambiguous quoted form the
		# moment any argument would not survive the round trip.
		joinable=yes
		for arg in "$@"; do
			case "$arg" in
			*[[:space:]]*) joinable=no ;;
			esac
			case "$arg" in
			*\'*) esc="$(printf '%s' "$arg" | sed "s/'/'\\\\''/g")" ;;
			*) esc="$arg" ;;
			esac
			quoted="${quoted:+$quoted }'$esc'"
			joined="${joined:+$joined }$arg"
		done
		steps=("$mode=$quoted")
		if [ "$joinable" = yes ]; then
			norms=("$joined")
		else
			norms=("$quoted")
		fi
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

# --- the gate ledger ------------------------------------------------------------------
# What was proven, over which content. Every line below is best-effort: this block may
# not change the exit code, the output, or where a chain stops, and it never skips a
# step. Deciding whether a proof is still good enough to skip on belongs to the SKILL.md,
# which reads `gate-detect.sh`'s classification and must label a skipped step `cached`.
LEDGER_KEEP=200
ledger=""
fingerprint=""
led_head=""
led_branch=""
led_skill=""

if [ "$no_ledger" = no ] && command -v jq >/dev/null 2>&1; then
	# Computed ONCE, before the first step — never per step. A chain whose earlier step
	# regenerates a tracked file would otherwise record two different keys for one gate
	# run, and the later record would claim proof over content that step never read.
	fingerprint="$(mkit_tree_fingerprint)" || fingerprint=""
	if [ -n "$fingerprint" ]; then
		ledger="$(mkit_gate_ledger_path)" || ledger=""
		led_head="$(git rev-parse HEAD 2>/dev/null)" || led_head=""
		led_branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null)" || led_branch=""
		# `review-20260819T111347Z-RPfCbj` -> `review`. Diagnostic only: no classification
		# reads it. It is here so "is anything ever *consuming* this ledger?" is
		# answerable from the file alone.
		led_skill="$(basename -- "$run_dir")"
		led_skill="${led_skill%%-*}"
	fi
fi

# Keep the newest LEDGER_KEEP records, dropping records whose head no longer resolves
# first — those always classify `unknown-head` anyway, so they are the cheapest to lose.
# Batched: the resolve pass costs one `git cat-file` per distinct head, so paying it on
# every append would make the expensive part of a gate its own bookkeeping.
ledger_trim() {
	local n heads tmp alive lock
	n="$(wc -l <"$ledger" 2>/dev/null | tr -d ' ')"
	case "$n" in '' | *[!0-9]*) return 0 ;; esac
	[ "$n" -gt $((LEDGER_KEEP * 2)) ] || return 0

	# The lock excludes **trim against trim** and nothing else: `ledger_append` takes no
	# lock, so a record appended between the head scan and the `mv -f` below can be lost.
	# That is a deliberate trade — one cache record, after which the lookup reads `none`
	# and the step runs — because locking the append would cost it the never-blocks,
	# never-fails property the whole side effect rests on.
	lock="$ledger.lock"
	if ! mkdir "$lock" 2>/dev/null; then
		# A trim killed mid-rewrite leaves the directory behind and rotation would then
		# be off forever, silently. Break a lock nothing could still be holding; the same
		# 60-minute liveness heuristic `run-open.sh --prune` uses.
		[ -n "$(find "$lock" -maxdepth 0 -mmin +60 2>/dev/null)" ] || return 0
		rm -rf -- "$lock" 2>/dev/null
		mkdir "$lock" 2>/dev/null || return 0
	fi
	# Released on the paths a signal takes, too: the rewrite below is the one window where
	# an abandoned lock costs something.
	trap 'rm -rf -- "$lock" 2>/dev/null' EXIT

	# jq's status is load-bearing. One malformed line and jq stops there, having printed
	# only the heads before it — `paste` then pairs those heads with the WRONG records and
	# the rewrite keeps a handful of the oldest and drops every valid record after them.
	# A trim that cannot read the file cleanly does not trim.
	if heads="$(jq -r '.head // "-"' "$ledger" 2>/dev/null)" && [ -n "$heads" ]; then
		tmp="$(mktemp "$ledger.XXXXXX" 2>/dev/null)"
		alive="$(mktemp "$ledger.XXXXXX" 2>/dev/null)"
		if [ -n "$tmp" ] && [ -n "$alive" ]; then
			printf '%s\n' "$heads" | LC_ALL=C sort -u | while IFS= read -r h; do
				[ -n "$h" ] && [ "$h" != "-" ] || continue
				git cat-file -e "$h^{commit}" 2>/dev/null && printf '%s\n' "$h"
			done >"$alive"
			# jq -c escapes tabs inside strings, so the first tab is exactly the one
			# `paste` inserted — the split back to (head, record) is unambiguous.
			paste -d'\t' <(printf '%s\n' "$heads") "$ledger" 2>/dev/null |
				awk -F'\t' -v alive="$alive" '
					BEGIN { while ((getline h < alive) > 0) ok[h] = 1 }
					ok[$1] { print substr($0, index($0, "\t") + 1) }
				' | tail -n "$LEDGER_KEEP" >"$tmp" 2>/dev/null &&
				mv -f "$tmp" "$ledger" 2>/dev/null
		fi
		rm -f -- "$tmp" "$alive" 2>/dev/null
	fi
	rm -rf -- "$lock" 2>/dev/null
	trap - EXIT
	return 0
}

# One record per step, appended as that step finishes — not one per chain. A chain that
# stops at `test` leaves two records, and a later skill gets per-step answers
# (`lint=fresh test=failed build=none`) instead of an all-or-nothing chain verdict.
#   ledger_append <step> <normalized-cmd> <exit> <secs> <log>
ledger_append() {
	local line
	[ -n "$ledger" ] && [ -n "$fingerprint" ] || return 0
	mkdir -p "$(dirname -- "$ledger")" 2>/dev/null
	# `epoch` beside the ISO `ts`: age arithmetic must not go through `date -d` (GNU) or
	# `date -j -f` (BSD), which is precisely the portability trap this layer avoids.
	line="$(jq -cn \
		--arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		--argjson epoch "$(date -u +%s)" \
		--arg fingerprint "$fingerprint" \
		--arg step "$1" \
		--arg cmd "$2" \
		--argjson exit "$3" \
		--argjson secs "$4" \
		--arg head "$led_head" \
		--arg branch "$led_branch" \
		--arg skill "$led_skill" \
		--arg log "$5" \
		'{kind: "gate", ts: $ts, epoch: $epoch, fingerprint: $fingerprint,
		  step: $step, cmd: $cmd, exit: $exit, secs: $secs,
		  head: $head, branch: $branch, skill: $skill, log: $log}' 2>/dev/null)" || return 0
	[ -n "$line" ] || return 0
	printf '%s\n' "$line" >>"$ledger" 2>/dev/null || return 0
	ledger_trim
	return 0
}

overall=0
failed_step=""
ran=""

for ((i = 0; i < ${#steps[@]}; i++)); do
	spec="${steps[i]}"
	step="${spec%%=*}"
	cmd="${spec#*=}"
	# What the ledger is keyed on — the command before the single-step form quoted it.
	ncmd="${norms[i]:-$cmd}"
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
	ledger_append "$step" "$ncmd" "$code" "$secs" "$log"

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
