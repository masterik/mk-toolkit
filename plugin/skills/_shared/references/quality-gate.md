# Quality gate

Shared by `finish` and `pr`, both full tier, once each before merge/PR. `commit` and `review` don't
gate — they work directly on the diff. **Nothing is hardcoded** — the commands come from the target
repo. Read "Find the gate" when the gate is about to run, not before: the manifests it reads cost
context nothing earlier needs.

## Find the gate

Walk the table top to bottom; the first row that yields commands is the gate. Rows 3 and 4 combine
into one sequence.

| # | Source | The gate is |
| --- | --- | --- |
| 1 | `docs/agents/gate.md` | the commands it lists, in its order — the user's override; nothing below is consulted |
| 2 | a task-runner `check` or `ci` target | that one command (`just check`, `make ci`, `task check`, `deno task ci`) |
| 3 | per-step task-runner recipes | laid over row 4, step by step (below) |
| 4 | the ecosystem default | the manifest table's commands (below) |
| 5 | nothing | ask the user which commands to run; never invent one |

Say which row the gate came from in the report (`gate from: just check`, `gate from: go.mod + justfile`).

**Task runners**, in this order: `justfile` (also `Justfile`, `.justfile`), `Makefile` (`makefile`,
`GNUmakefile`), `Taskfile.yml`/`.yaml`, `deno.json`/`.jsonc` `tasks`. A recipe counts only if it runs
bare — a just recipe with a required parameter (`release version:`) cannot stand in for a step. The
first runner that has a recipe wins for that name. Grep the runner file for the names you need; don't
read it whole.

**Row 2 — read the recipe first.** A `check` that only lints would silently drop the tests and build.
If its body plainly covers less than rows 3–4 would run, it is not the whole gate: fall through to
rows 3–4 and say why.

**Row 4 — the ecosystem table.** Every manifest present at the repo root contributes, in this table's
order:

| Manifest | Steps, in order |
| --- | --- |
| `package.json` | `<pm> run <s>` for each of `lint`, `typecheck`, `test`, `build` that is a script; `<pm>` from the lockfile: `bun.lock`/`bun.lockb` → bun, `pnpm-lock.yaml` → pnpm, `yarn.lock` → yarn, none → npm |
| `deno.json`/`deno.jsonc` | `deno lint`, `deno test` |
| `Cargo.toml` | `cargo clippy --all-targets -- -D warnings`, `cargo test`, `cargo build` |
| `go.mod` | `go vet ./...`, `go test ./...`, `go build ./...` |
| `pyproject.toml`/`setup.cfg`/`tox.ini` | `ruff check .` (`python -m flake8` without ruff), `pytest -q`, `mypy .` if `pyproject.toml` has `[tool.mypy]` |
| `*.sln`/`*.csproj` | `dotnet build --nologo`, `dotnet test --nologo` |

Read `package.json` scripts with `jq -r '.scripts | keys[]' package.json` (or grep its `"scripts"`
block), never the whole file. Unparseable =
the gate is **undetected**, not absent: say so, never report it as a pass.

**Row 3 over row 4 — the overlay**, for the steps `lint`, `typecheck`, `vet`, `test`, `build`:

- a runner recipe named for a step **replaces** that step's ecosystem command (`just test` over
  `go test ./...`); in a polyglot repo it replaces every ecosystem's command for that step, once;
- a step the runner lacks **keeps** the ecosystem's command;
- a runner step no ecosystem proposes (a Go repo's `just lint`) **joins** the sequence in the order
  `lint` → `typecheck` → `vet` → `test` → `build`.

Example: `go.mod` + a justfile with `lint` and `test` → `just lint`, `go vet ./...`, `just test`,
`go build ./...`.

A repo's own docs (`CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`) naming a different check command is
evidence, not an override: run what the table gives, report the mismatch, and suggest
`docs/agents/gate.md` to settle it. A repo with no checks at all (Markdown only) has no gate: say
`no gate`, never `gate=ok`.

## Run it

Run each step yourself, in order, from the repo root of the current worktree, **stopping at the
first failure**. Send each step's full output to a log in the run directory (`run=` from `mkit
facts`, passed as a literal — `output-discipline.md`), so it never enters context:

```bash
bun run lint > <run-dir>/gate-lint.log 2>&1; echo "lint exit=$?"
```

`<step>` is the step's name (`lint`, `test`…); a row 1–2 command gets a short label of its own
(`check`). A name already used gets `-2`, `-3`.

One line per step in the chat: the step, `ok` or `FAIL exit=N`, and the log path on a failure. On a
failure, read a short excerpt of that log — the lines naming the failure (`grep -nE
'FAIL|Error|error:|panic:|not ok |Traceback'`, at most ~40) plus the last ~30 lines — never the whole
log. Then report `gate=ok` or `gate=FAILED step=<step> exit=<N>`.

### Reusing a pass — one rule

A gate result lives only as long as this session's memory of it. In `pr` or `finish`, **skip the
gate** only when all three hold:

- a gate passed earlier **in this session** (every step `ok`, same commands);
- `git rev-parse HEAD` is the commit it ran at;
- `git status --porcelain` is empty.

Say so in the report — `gate skipped: passed at <short-sha> earlier this session, tree clean` — never
as `gate=ok`. Anything else runs the gate: a new session, a compacted context you cannot quote the
result from, a different HEAD, or a dirty tree. A result you can no longer see never counts as a pass.

## When a step fails

The step, its exit code and the excerpt (failure lines plus the tail) are usually the whole
diagnosis, and a compiler error is always its own.

**Delegate only when that is genuinely not enough**: a long suite whose failures do not name a
cause, or a failure you cannot attribute to the change. Hand a subagent the log path (it is on
disk), the step, the exit code and the changed-file list; it reads the log and only the sources
needed, and **returns at most 15 lines**: what failed by name, probable root cause in a
sentence or two, caused-by-this-change or pre-existing, a concrete suggested fix. **Read-only**
— triage diagnoses, never fixes: what to do about a red gate is the caller's decision, and for
`finish`/`pr` the user's.

## Rules

- Open the run directory before the first step — `mkit facts` did it. Never write a log to a path
  that does not exist.
- Run the gate from the repo root of the current worktree.
- Report exactly which step failed and its exit code. Never silently continue past a failure.
- For a **draft** PR or an explicit override, the user may proceed past a failing gate: surface
  it, then respect the decision.
- Never fabricate a passing result. If a check was skipped — no such script, too slow, user
  opted out, or **reused under the rule above** — say so, and say which.
