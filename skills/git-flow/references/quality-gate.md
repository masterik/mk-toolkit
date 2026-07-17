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

## Rules

- Run the gate from the repo root of the current worktree.
- Report exactly which step failed and its output. Do not silently continue past a failure.
- For a **draft** PR or an explicit user override, the user may choose to proceed past a failing gate — surface the failure, then respect their decision.
- Never fabricate a passing result. If a check was skipped (no such script, too slow, user opted out), say so.
