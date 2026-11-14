# Quality gate

Run by `pr` and `finish`, once each, before the PR or the merge. **Nothing is hardcoded** — the commands come
from the target repo.

## Find the gate

Walk the table top to bottom; the first row that yields commands is the gate (rows 3 and 4 combine).

| # | Source | The gate is |
| --- | --- | --- |
| 1 | `docs/agents/gate.md` | the commands it lists, in its order; nothing below is consulted |
| 2 | a task-runner `check` or `ci` target | that one command (`just check`, `make ci`, `task check`, `deno task ci`) |
| 3 | per-step task-runner recipes | laid over row 4, step by step (below) |
| 4 | the ecosystem default | the manifest table's commands (below) |
| 5 | nothing | code to check → ask the user which commands to run, never invent one; no code (Markdown only) → say `no gate`, never `gate=ok` |

Say which row it came from in the report (`gate from: just check`, `gate from: go.mod + justfile`).

**Task runners**, in this order: `justfile`/`Justfile`/`.justfile`, `Makefile`/`makefile`/`GNUmakefile`,
`Taskfile.yml`/`.yaml`, `deno.json`/`.jsonc` `tasks`. Grep the file for the names you need — except Deno's:
parse its `tasks` object after stripping JSONC comments and trailing commas (a grep matches a commented-out
task). A recipe counts only if it runs bare (no required parameter); the first runner with a recipe wins for
that name.

**Row 2 — read the recipe first.** A `check` whose body plainly covers less than rows 3–4 (lint only) is not
the whole gate: fall through to rows 3–4 and say why.

**Row 4 — every manifest at the repo root contributes, in this order:**

| Manifest | Steps, in order |
| --- | --- |
| `package.json` | `<pm> run <s>` for each of `lint`, `typecheck`, `test`, `build` that is a script; `<pm>`: `bun.lock`/`bun.lockb` → bun, `pnpm-lock.yaml` → pnpm, `yarn.lock` → yarn, else npm |
| `deno.json`/`deno.jsonc` | `deno lint`, `deno test` |
| `Cargo.toml` | `cargo clippy --all-targets -- -D warnings`, `cargo test`, `cargo build` |
| `go.mod` | `go vet ./...`, `go test ./...`, `go build ./...` |
| `pyproject.toml`/`setup.cfg`/`tox.ini` | `ruff check .` (`python -m flake8` without ruff), `pytest -q`, `mypy .` if `pyproject.toml` has `[tool.mypy]` |
| `*.sln`/`*.csproj` | `dotnet build --nologo`, `dotnet test --nologo` |

Read only `package.json`'s `"scripts"` block. Unparseable = the gate is **undetected**, never a pass.

**Row 3 over row 4**, for the steps `lint`, `typecheck`, `vet`, `test`, `build`: a runner recipe named for a
step **replaces** that step's ecosystem command (once, in a polyglot repo); a step the runner lacks **keeps**
the ecosystem's; a runner step no ecosystem proposes **joins** in that step order. Example: `go.mod` + a
justfile with `lint` and `test` → `just lint`, `go vet ./...`, `just test`, `go build ./...`.

A repo doc (`CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`) naming another check command is evidence, not an
override: run what the table gives, report the mismatch, suggest `docs/agents/gate.md`.

## Run it

From the repo root of the current worktree, each step in order, **stopping at the first failure**, its full
output to a log in the run directory:

```bash
bun run lint > "<run-dir>/gate-lint.log" 2>&1
```

The tool result gives the exit status. `<step>` is the step's name; a row 1–2 command gets its own short
label (`check`); a repeated name gets `-2`, `-3`.

One line per step in chat: `lint ok`, or `test FAIL exit=1 log=<run-dir>/gate-test.log`. On a failure read
only an excerpt — the failure lines (`grep -nE 'FAIL|Error|error:|panic:|not ok |Traceback'`, at most ~40)
plus the last ~30 lines. Then `gate=ok` or `gate=FAILED step=<step> exit=<N>`. Never re-run a gate to see
output you discarded; the log has it.

### Reusing a pass — one rule

Skip the gate only when all three hold: a gate passed earlier **in this session** (every step `ok`, same
commands); `git rev-parse HEAD` is the commit it ran at; `git status --porcelain` is empty. Report it as
`gate skipped: passed at <short-sha> earlier this session, tree clean`, never as `gate=ok`. A result you can
no longer see (new session, compacted context) never counts.

## When a step fails

The step, exit code and excerpt are usually the whole diagnosis. **Delegate only when they are not enough**
(a long suite whose failures name no cause, a failure you cannot attribute to the change): hand a subagent
the log path, step, exit code and changed-file list; it reads the log and only the sources needed and
**returns at most 15 lines** — what failed, probable root cause, caused-by-this-change or pre-existing, a
suggested fix. **Read-only**: what to do about a red gate is the user's decision.

Never continue silently past a failure; for a **draft** PR or an explicit override the user may proceed —
surface it, then respect the decision. Never fabricate a pass: a skipped check (no such script, user opted
out, the reuse rule) is reported as skipped, and which.
