#!/usr/bin/env bash
#
# Detect the repo's quality-gate commands. Reports candidates; never runs them, never
# decides which one the skill should trust.
#
#   usage: gate-detect.sh [--dir <path>] [--no-cache]
#
# Output:
#   ecosystem=node,rust        every stack detected, in the order found
#   pm=bun                     package manager, from the lockfile
#   fast=<one command>         the fastest meaningful check
#   full=<cmd>|<cmd>|<cmd>     the pre-integration sequence, in order
#   alt_fast=<cmd>,<cmd>       other plausible fast checks, for an override
#   scripts=lint,test,build    what actually exists, so a skill can see what was skipped
#   scripts_state=ok|no-jq|unreadable|n-a
#                              whether that list is trustworthy — `scripts=none` with
#                              no-jq/unreadable means undetected, not absent
#   workspaces=yes|no          monorepo hint
#   docs_candidates:           lines from the repo's own docs naming a check command —
#                              a *candidate*, because "the canonical check" is a
#                              judgement the skill makes, not a grep result
#
# Plus, when the gate ledger has something to say about the commands above:
#
#   gate_fingerprint=<hash>    identifies the content a gate command would read now
#   gate_max_age_min=60        the bound past which a matching pass reports `stale`
#   fast_cache=<class> exit=0 age=6m      or a bare `fast_cache=none`
#   full_cache=<class>|<class>|<class>    pipe-parallel with full=
#   full_cache_exit=0|1|-                 `-` where the class is none
#   full_cache_age=6m|6m|-                `-` where the class is none
#
#   <class> is one of: fresh · failed · drifted · stale · unknown-head · none.
#   The table naming what each means, and what a skill should do about it, is in
#   skills/_shared/references/quality-gate.md.
#
#   gate_cache=off|empty|no-hash|no-fingerprint|no-jq
#                              printed *instead of* gate_fingerprint when there is
#                              nothing to classify against — one value per distinct
#                              cause, because only some of them mean "nothing to use"
#
# This annotates a proposal, which is what this script already does. It reports what was
# proven and over which content; it never says a step may be skipped. That trade-off —
# latency against a safety net — is the SKILL.md's, and a skipped step must be reported
# as `cached`, never as a pass. `--no-cache` drops the annotation entirely.
#
# Why a script: the mapping from lockfile to package manager and from script names to
# tiers is a table, and reading it here keeps a 25-script package.json (~700 tokens)
# out of context. Exit: 0 ok, 1 no repo, 2 bad usage.

set -euo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

dir="."
no_cache=no
while [ $# -gt 0 ]; do
	case "$1" in
	--dir)
		dir="${2:-}"
		[ -n "$dir" ] || mkit_die '--dir needs a path' 2
		shift 2
		;;
	--no-cache)
		no_cache=yes
		shift
		;;
	*) mkit_die "unknown option: $1" 2 ;;
	esac
done
cd "$dir" || mkit_die "cannot enter: $dir" 2
mkit_require_repo
root="$(git rev-parse --show-toplevel)"
cd "$root"

eco=""
fast=""
full=""
alt=""
add_eco() { eco="${eco:+$eco,}$1"; }
add_alt() { alt="${alt:+$alt,}$1"; }
add_full() { full="${full:+$full|}$1"; }

# --- Node / Bun / Deno -------------------------------------------------------------
scripts_list=""
scripts_state=n-a
workspaces=no
if [ -f package.json ]; then
	add_eco node
	pm=npm
	if [ -f bun.lock ] || [ -f bun.lockb ]; then
		pm=bun
	elif [ -f pnpm-lock.yaml ]; then
		pm=pnpm
	elif [ -f yarn.lock ]; then
		pm=yarn
	elif [ -f package-lock.json ]; then
		pm=npm
	fi
	printf 'pm=%s\n' "$pm"
	run="$pm run"
	[ "$pm" = npm ] && run="npm run"

	# A missing jq and an unparseable package.json both used to leave scripts_list empty,
	# so a real node repo reported `scripts=none fast=none` — indistinguishable from a
	# package.json that genuinely declares no scripts. Say which happened instead.
	if ! command -v jq >/dev/null 2>&1; then
		scripts_state=no-jq
	elif ! scripts_list="$(jq -r '(.scripts // {}) | keys | join(",")' package.json 2>/dev/null)"; then
		scripts_list=""
		scripts_state=unreadable
	else
		scripts_state=ok
		jq -e '(.workspaces // empty)' package.json >/dev/null 2>&1 && workspaces=yes
	fi
	has() { case ",$scripts_list," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

	# Fast tier: the cheapest signal that can still fail on a real defect.
	for s in typecheck tsc check lint test:unit; do
		has "$s" && {
			fast="$run $s"
			break
		}
	done
	[ -n "$fast" ] || { has test && fast="$run test"; }
	for s in typecheck lint test:unit test; do
		has "$s" && [ "$run $s" != "$fast" ] && add_alt "$run $s"
	done
	# Full tier: lint -> test -> build, whichever exist, in that order.
	for s in lint typecheck test build; do has "$s" && add_full "$run $s"; done
fi
if [ -f deno.json ] || [ -f deno.jsonc ]; then
	add_eco deno
	[ -n "$fast" ] || fast="deno check ."
	add_full "deno lint"
	add_full "deno test"
fi

# --- Rust ---------------------------------------------------------------------------
if [ -f Cargo.toml ]; then
	add_eco rust
	[ -n "$fast" ] || fast="cargo clippy --all-targets -- -D warnings"
	add_alt "cargo check --all-targets"
	add_full "cargo clippy --all-targets -- -D warnings"
	add_full "cargo test"
	add_full "cargo build"
fi

# --- Go -----------------------------------------------------------------------------
if [ -f go.mod ]; then
	add_eco go
	[ -n "$fast" ] || fast="go vet ./..."
	add_full "go vet ./..."
	add_full "go test ./..."
	add_full "go build ./..."
fi

# --- Python -------------------------------------------------------------------------
if [ -f pyproject.toml ] || [ -f setup.cfg ] || [ -f tox.ini ]; then
	add_eco python
	pyfast=""
	command -v ruff >/dev/null 2>&1 && pyfast="ruff check ."
	[ -n "$pyfast" ] || pyfast="python -m flake8"
	[ -n "$fast" ] || fast="$pyfast"
	add_full "$pyfast"
	add_full "pytest -q"
	if [ -f pyproject.toml ] && mkit_search 1 '\[tool\.mypy\]' pyproject.toml >/dev/null; then
		add_full "mypy ."
	fi
fi

# --- .NET ---------------------------------------------------------------------------
if ls ./*.sln >/dev/null 2>&1 || ls ./*.csproj >/dev/null 2>&1; then
	add_eco dotnet
	[ -n "$fast" ] || fast="dotnet build --nologo"
	add_full "dotnet build --nologo"
	add_full "dotnet test --nologo"
fi

# --- Make / Just: a documented `check` target beats anything inferred ---------------
for mf in Makefile makefile GNUmakefile; do
	[ -f "$mf" ] || continue
	add_eco make
	if mkit_search 1 '^check:' "$mf" >/dev/null; then
		add_alt "make check"
		[ -n "$fast" ] && add_alt "$fast"
		fast="make check"
	fi
	break
done
for jf in justfile Justfile .justfile; do
	[ -f "$jf" ] || continue
	add_eco just
	if mkit_search 1 '^check:' "$jf" >/dev/null; then
		add_alt "just check"
		[ -n "$fast" ] && add_alt "$fast"
		fast="just check"
	fi
	break
done

fast_out="${fast:-none}"
full_out="${full:-${fast:-none}}"

# --- the gate ledger: annotate the proposals above -----------------------------------
# `gate-run.sh` records (cmd, exit, fingerprint, ts) for every step it finishes. Here we
# compare each proposed command against the newest record for that exact command string.
#
# Why here and not in a new script: this file is already "everything you need to know
# before running a gate", it already emits the exact command strings, and this keeps the
# skills at two calls. It also stays honest about authority — annotating a proposal is
# all this script ever does. Nothing below skips anything.
#
# The command strings emitted above are the normalized form by construction (this script
# never shell-quotes), so they compare directly against what `gate-run.sh` records.
GATE_MAX_AGE_MIN=60

gate_cause=""
gate_fp=""
ledger=""
fast_cache=""
full_cache=""
full_cache_exit=""
full_cache_age=""

# One value per distinct cause, per the `pr=gh-missing` / `pr=jq-missing` / `pr=none`
# lesson: only some of these mean "there was nothing to use".
if [ "$no_cache" = yes ]; then
	gate_cause=off
elif ! command -v jq >/dev/null 2>&1; then
	gate_cause=no-jq
elif ! mkit_have_hash; then
	gate_cause=no-hash
else
	ledger="$(mkit_gate_ledger_path)" || ledger=""
	gate_fp="$(mkit_tree_fingerprint)" || gate_fp=""
	if [ -z "$gate_fp" ]; then
		# NOT `no-hash`: a hash tool exists (the branch above proved it), so the
		# fingerprint failed for some other reason — no git dir, an unusable temp dir, or
		# a short hash batch refusing to answer. Telling the reader to install `shasum`
		# when they already have one is exactly the misdiagnosis `scripts_state` exists
		# to avoid.
		gate_cause=no-fingerprint
	elif [ -z "$ledger" ] || [ ! -s "$ledger" ]; then
		gate_cause=empty
	fi
fi

if [ -z "$gate_cause" ]; then
	# Every command to look up, fast first then the full tier in order, so the rows come
	# back positionally aligned with what is printed below.
	declare -a gate_cmds=()
	[ "$fast_out" != none ] && gate_cmds+=("$fast_out")
	if [ "$full_out" != none ]; then
		old_ifs="$IFS"
		IFS='|'
		for c in $full_out; do gate_cmds+=("$c"); done
		IFS="$old_ifs"
	fi

	if [ "${#gate_cmds[@]}" -gt 0 ]; then
		cmds_json="$(printf '%s\n' "${gate_cmds[@]}" |
			jq -R -s 'split("\n") | map(select(length > 0))' 2>/dev/null)" || cmds_json=""
		rows=""
		if [ -n "$cmds_json" ]; then
			# One jq for every command: the newest record for an exact `cmd` match wins.
			# A step *name* is not identity — `bun run test` and `bun run test --coverage`
			# are different checks, so only the command string can be the key.
			rows="$(jq -r -s \
				--argjson cmds "$cmds_json" \
				--argjson now "$(date -u +%s)" '
				. as $recs
				| $cmds[] as $c
				| ($recs | map(select((.kind // "gate") == "gate" and .cmd == $c)) | last) as $r
				| def dash: if (. // "") == "" then "-" else . end;
				  if $r == null then "none\t-\t-\t-"
				  else [($r.fingerprint | dash), (($r.exit // 0) | tostring),
				        (($now - ($r.epoch // 0)) | tostring), ($r.head | dash)]
				       | join("\t")
				  end' "$ledger" 2>/dev/null)" || rows=""
		fi

		if [ -n "$rows" ]; then
			# `failed` is checked before the age bound on purpose: it no longer means
			# "skip", it means "this tree was red on exactly this content" — worth saying
			# however old it is. `stale` exists only to stop a *pass* being trusted past
			# the point where the environment may have moved under it.
			classify() {
				local rfp="$1" rex="$2" rage="$3" rhead="$4"
				cls=none
				[ "$rfp" != none ] && [ "$rfp" != '-' ] || return 0
				if [ "$rhead" != '-' ] && [ -n "$rhead" ] &&
					! git cat-file -e "$rhead^{commit}" 2>/dev/null; then
					cls=unknown-head
					return 0
				fi
				if [ "$rfp" != "$gate_fp" ]; then
					cls=drifted
					return 0
				fi
				if [ "$rex" != 0 ]; then
					cls=failed
					return 0
				fi
				case "$rage" in *[!0-9]*) cls=stale && return 0 ;; esac
				[ "$rage" -le $((GATE_MAX_AGE_MIN * 60)) ] || {
					cls=stale
					return 0
				}
				cls=fresh
			}

			i=0
			while IFS="$(printf '\t')" read -r rfp rex rage rhead; do
				# A record with an empty field would let `read` collapse two tabs into
				# one delimiter and shift every column right — exit read as the
				# fingerprint, age read as the exit code. jq's `dash` above stops that at
				# the source; this is the second lock, because a shifted row does not
				# fail loudly, it reports a confident wrong class.
				[ -n "$rfp" ] && [ -n "$rex" ] && [ -n "$rage" ] || rfp=none
				classify "$rfp" "$rex" "$rage" "$rhead"
				if [ "$cls" = none ]; then
					age_h='-'
					rex='-'
				else
					age_h="$(mkit_age_human "$rage")"
				fi
				if [ "$i" -eq 0 ] && [ "$fast_out" != none ]; then
					fast_cache="$cls"
					[ "$cls" = none ] || fast_cache="$cls exit=$rex age=$age_h"
				else
					full_cache="${full_cache:+$full_cache|}$cls"
					full_cache_exit="${full_cache_exit:+$full_cache_exit|}$rex"
					full_cache_age="${full_cache_age:+$full_cache_age|}$age_h"
				fi
				i=$((i + 1))
			done <<-EOF
				$rows
			EOF
		fi
	fi
fi

printf 'ecosystem=%s\n' "${eco:-none}"
printf 'fast=%s\n' "$fast_out"
[ -n "$fast_cache" ] && printf 'fast_cache=%s\n' "$fast_cache"
printf 'full=%s\n' "$full_out"
if [ -n "$full_cache" ]; then
	printf 'full_cache=%s\n' "$full_cache"
	printf 'full_cache_exit=%s\n' "$full_cache_exit"
	printf 'full_cache_age=%s\n' "$full_cache_age"
fi
if [ -n "$gate_cause" ]; then
	printf 'gate_cache=%s\n' "$gate_cause"
else
	printf 'gate_fingerprint=%s\n' "$gate_fp"
	# Reported, not just applied. `stale` is the one class that is a judgement about
	# time rather than about content, so the number behind it belongs in the output where
	# the skill making the skip decision can see what its proof was measured against.
	printf 'gate_max_age_min=%s\n' "$GATE_MAX_AGE_MIN"
fi
printf 'alt_fast=%s\n' "${alt:-none}"
printf 'scripts=%s\n' "${scripts_list:-none}"
printf 'scripts_state=%s\n' "$scripts_state"
printf 'workspaces=%s\n' "$workspaces"

# --- what the repo's own docs claim -------------------------------------------------
# Candidates only. Which of these is canonical is the skill's call.
docs=""
for f in CLAUDE.md AGENTS.md README.md CONTRIBUTING.md docs/CONTRIBUTING.md; do
	[ -f "$f" ] || continue
	hits="$(mkit_search 4 '(^|[^a-z])(make|just|npm|pnpm|yarn|bun|deno|cargo|go|dotnet|uv|poetry|task) (run )?(check|lint|test|build|verify|ci)' "$f" || true)"
	[ -n "$hits" ] && docs="${docs}$(printf '%s\n' "$hits" | sed "s|^|$f:|")
"
done
if [ -n "$docs" ]; then
	printf 'docs_candidates:\n%s' "$docs" | head -21
else
	printf 'docs_candidates=none\n'
fi
