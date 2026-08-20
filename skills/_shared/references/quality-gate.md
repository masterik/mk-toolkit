# Quality gate detection

Shared by `commit` (fast check), `finish-feature` and `create-pr` (full gate). **Do not hardcode commands** —
detect what the repo uses, so the bundle works in any project.

## Discover the commands

- **Node / Bun / Deno**: `package.json` `scripts` — `lint`, `test`, `test:all`, `build`, `typecheck`,
  `format`. Package manager = whichever lockfile exists (`bun.lock*` → `bun run`, `pnpm-lock.yaml` → `pnpm`,
  `yarn.lock` → `yarn`, else `npm run`).
- **.NET**: `dotnet build`, `dotnet test`.
- **Rust**: `cargo clippy`, `cargo test`, `cargo build`.
- **Go**: `go vet ./...`, `go test ./...`, `go build ./...`.
- **Python**: `ruff`/`flake8`, `pytest`, `mypy` — per `pyproject.toml` / `tox.ini` / Makefile.
- **Makefile / Justfile**: prefer a documented `make check` / `just check` target.

**Also** honor anything the repo's own docs or CLAUDE.md name as the canonical check — in addition to the
detected commands, not instead of them.

## Two tiers

- **Fast check** (`commit`, per logical commit): the single fastest meaningful check — usually lint,
  typecheck, or the touched package's unit tests. It runs repeatedly; keep it quick.
- **Full gate** (`finish-feature`, `create-pr`, once before merge/PR): the complete pre-integration sequence
  in order, **stop on first failure**. Typically `lint → test → build`.

## When a step fails — triage the log, don't read it

The log is already on disk (`output-discipline.md`: each step redirects to `<run-dir>/gate-<step>.log`). The
decision — fix it, or get an explicit OK — needs a diagnosis, not a transcript.

**Delegate the diagnosis when the log is long** (>~100 lines, which any real failing suite is). Hand a
subagent the log path, the failing step, its exit code, and the changed-file list. It reads the log and only
the sources needed to explain the failure, and **returns at most 15 lines**:

- which tests or checks failed, by name
- probable root cause, one or two sentences
- caused by the change under review, or pre-existing
- a concrete suggested fix

No log excerpts beyond the decisive ones, no file contents, no narration. **Read-only** — triage diagnoses,
never fixes: what to do about a red gate is the caller's decision, and for `finish-feature`/`create-pr` the
user's (fix, or proceed as a draft).

**On a short log, read the tail yourself.** Under ~100 lines the round trip costs more than it saves, and a
compiler error is usually its own diagnosis.

## Rules

- Open the run directory before the first step (`output-discipline.md`) — the gate's first act is a redirect
  into it, and a missing path sends the log to `/`.
- Run the gate from the repo root of the current worktree.
- Report exactly which step failed, its exit code, and the tail — **never the whole log**. Never silently
  continue past a failure.
- For a **draft** PR or an explicit override, the user may proceed past a failing gate: surface it, then
  respect the decision.
- Never fabricate a passing result. If a check was skipped (no such script, too slow, user opted out), say so.
