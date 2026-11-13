# Quality gate

Shared by `finish` and `pr`, both full tier. `commit` and `review` don't gate — they work
directly on the diff. **Nothing is hardcoded** — the commands are detected from the target repo.

## Detect, then choose

```bash
mkit gate detect
# pm=bun
# ecosystem=node
# full:
#   1 source=discovered cmd=bun run lint
#   2 source=pinned cmd=pytest -q || exit 1
#   3 source=discovered cmd=bun run build
# documented=just check
# scripts=build,dev,format,lint,test,test:e2e,test:unit
# scripts_state=ok
# workspaces=yes
# docs_candidates:
#   CLAUDE.md:21:`bun run check` runs formatting, lint, typecheck, and tests
```

It reads the lockfile, `package.json` scripts, `Cargo.toml`, `go.mod`, `pyproject.toml`, a
`Makefile`/`justfile` `check` target and the repo's own docs — which keeps a 25-script
`package.json` (~700 tokens) out of context.

**`full` is a proposal; `documented` and `docs_candidates` are why.** What a repo calls its
canonical check is a claim in its own prose, not something a lockfile settles — so when either
names a command the chain missed, prefer it and say which you used. Anything reported absent
stays absent: never invent a step.

**One step per line, in order, and `cmd=` runs to the end of the line** — so a command may
contain spaces, pipes or `||` without any escaping. Read the fields by name, never by position,
and never split the block on `|`: `pytest -q || exit 1` is an ordinary pinned step.

**`source=` says where that step came from**:

| value | meaning |
| --- | --- |
| `discovered` | inferred from a manifest this run; cannot go stale |
| `pinned` | `[gate.commands]` in the repo config — a human said this one is right |
| `documented` | the repo's own `check:` target, used because nothing was discovered |

`documented=` appears on its own line when a `check:` target exists *and* a chain was
discovered anyway. It never replaces that chain — a `check:` that only lints would silently
drop the repo's tests and build — so it is evidence for an override, not an override.

**Check `scripts_state` before believing `scripts=none`.** `ok` means the list is real;
`unreadable` means detection was blind, not that the repo declares nothing. On a blind read, say
the gate was undetected rather than absent, and never report a skipped gate as a pass. `n-a` is
the honest no-package.json case: a repo with no declared check (this plugin is one) has no gate
to run, and saying so beats substituting a command the repo never named.

## One tier, two consumers

There is one tier — the `full:` block — used **by `finish` and `pr`, once each, before merge/PR**: the
complete pre-integration sequence, in order, stopping at the first failure. `commit` and
`review` used to consume a `fast=` tier; they no longer gate at all, and the tier went with
them.

## Run it

Run each step yourself, in order, from the repo root of the current worktree, **stopping at the
first failure**. Send each step's full output to a log in the run directory (`run=` from `mkit
facts`, passed as a literal — `output-discipline.md`), so it never enters context:

```bash
bun run lint > <run-dir>/gate-lint.log 2>&1; echo "lint exit=$?"
```

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
