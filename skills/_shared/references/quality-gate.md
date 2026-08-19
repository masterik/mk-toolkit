# Quality gate detection

Shared by `commit` (fast check), `finish-feature`, and `create-pr` (full gate). **Do not hardcode commands** — detect what the repo uses so the bundle works in any project.

## Discover the commands

Inspect the repo root for the toolchain, then pick the matching scripts:

- **Node / Bun / Deno**: read `package.json` `scripts`. Look for `lint`, `test`, `test:all`, `build`, `typecheck`, `format`. The package manager is whichever lockfile exists (`bun.lock*` → `bun run`, `pnpm-lock.yaml` → `pnpm`, `yarn.lock` → `yarn`, else `npm run`).
- **.NET**: `dotnet build`, `dotnet test`.
- **Rust**: `cargo clippy`, `cargo test`, `cargo build`.
- **Go**: `go vet ./...`, `go test ./...`, `go build ./...`.
- **Python**: `ruff`/`flake8`, `pytest`, `mypy` — per `pyproject.toml` / `tox.ini` / Makefile.
- **Makefile / Justfile**: prefer a documented `make check` / `just check` target if present.

Also honor anything the repo's own docs or CLAUDE.md name as the canonical check.

## Two tiers

- **Fast check** (used by `commit`, per logical commit): the single fastest meaningful check — usually lint or typecheck, or the unit tests for the touched package. Keep it quick; it runs repeatedly.
- **Full gate** (used by `finish-feature` and `create-pr`, once before merge/PR): the project's complete pre-integration sequence, run in order, **stop on first failure**. Typically `lint → test → build`.

## When a step fails — triage the log, don't read it

The log is already on disk (`output-discipline.md`: every gate step is redirected to
`<run-dir>/gate-<step>.log`, in the run directory the caller opened before running the gate), so the decision the caller faces — fix it, or get an explicit OK to proceed —
needs a diagnosis, not a transcript.

**Delegate the diagnosis when the log is long** (roughly >100 lines, which any real failing test suite is).
Hand a subagent the log path, the failing step and its exit code, and the changed-file list. It reads the
log — and only the sources it needs to explain the failure — and **returns at most 15 lines**:

- which tests or checks failed, by name
- the probable root cause, in one or two sentences
- whether it looks caused by the change under review or pre-existing
- a suggested fix, concretely

No log excerpts beyond the decisive ones, no file contents, no narration. **Read-only** — triage
diagnoses, it never fixes: what to do about a red gate is the caller's decision, and for
`finish-feature`/`create-pr` it is the user's (fix, or proceed as a draft).

**On a short log, read the tail yourself.** Under ~100 lines a round trip costs more than it saves, and a
compiler error is usually its own diagnosis.

## Rules

- Open the run directory before the first step, with `run-open.sh` (`output-discipline.md`) — the gate's
  very first action is a redirect into it, and a missing path sends the log to `/`.
- Run the gate from the repo root of the current worktree.
- Report exactly which step failed, its exit code, and the tail of its log — **never the whole log**
  (`output-discipline.md`). Do not silently continue past a failure.
- For a **draft** PR or an explicit user override, the user may choose to proceed past a failing gate — surface the failure, then respect their decision.
- Never fabricate a passing result. If a check was skipped (no such script, too slow, user opted out), say so.
