#!/usr/bin/env bash
#
# Detect the repo's quality-gate commands. Reports candidates; never runs them, never
# decides which one the skill should trust.
#
#   usage: gate-detect.sh [--dir <path>]
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
# Why a script: the mapping from lockfile to package manager and from script names to
# tiers is a table, and reading it here keeps a 25-script package.json (~700 tokens)
# out of context. Exit: 0 ok, 1 no repo, 2 bad usage.

set -euo pipefail

# shellcheck source=lib/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/common.sh"

dir="."
while [ $# -gt 0 ]; do
	case "$1" in
	--dir)
		dir="${2:-}"
		[ -n "$dir" ] || mkit_die '--dir needs a path' 2
		shift 2
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

printf 'ecosystem=%s\n' "${eco:-none}"
printf 'fast=%s\n' "${fast:-none}"
printf 'full=%s\n' "${full:-${fast:-none}}"
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
