# Quality gate

Shared by `commit` (fast tier), `finish` and `pr` (full tier). **Nothing is hardcoded** —
the commands are detected from the target repo.

## Detect, then choose

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gate-detect.sh
# pm=bun
# ecosystem=node
# fast=bun run lint
# full=bun run lint|bun run test|bun run build
# alt_fast=bun run test:unit,bun run test
# scripts=build,dev,format,lint,test,test:e2e,test:unit
# scripts_state=ok
# workspaces=yes
# docs_candidates:
#   CLAUDE.md:21:`bun run check` runs formatting, lint, typecheck, and tests
```

It reads the lockfile, `package.json` scripts, `Cargo.toml`, `go.mod`, `pyproject.toml`, a
`Makefile`/`justfile` `check` target and the repo's own docs — which keeps a 25-script
`package.json` (~700 tokens) out of context.

**`fast` and `full` are proposals; `docs_candidates` is why.** What a repo calls its canonical
check is a claim in its own prose, not something a lockfile settles — so when `docs_candidates`
names a command the tiers missed, prefer it and say which you used. Anything reported absent
stays absent: never invent a step.

**Check `scripts_state` before believing `scripts=none`.** `ok` means the list is real; `no-jq`
or `unreadable` means detection was blind, not that the repo declares nothing — a node repo can
report `ecosystem=node fast=none` for want of `jq`. On a blind read, say the gate was undetected
rather than absent, and never report a skipped gate as a pass. `n-a` is the honest no-package.json
case: a repo with no declared check (this plugin is one) has no gate to run, and saying so beats
substituting a command the repo never named.

## Two tiers

- **Fast** (`commit`, per logical commit): the single fastest meaningful check. It runs
  repeatedly; keep it quick.
- **Full** (`finish`, `pr`, once before merge/PR): the complete pre-integration sequence, in
  order, stopping at the first failure.

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gate-run.sh <run-dir> --chain 'lint=bun run lint' 'test=bun run test' 'build=bun run build'
```

`gate-run.sh` logs each step in full, stops at the first failure, and returns a verdict plus a
bounded excerpt; its exit status is the failing step's (`output-discipline.md`).

## When a step fails

The verdict already carries the failing step, its exit code, the grepped failures and the tail
— which is usually the whole diagnosis, and a compiler error is always its own.

**Delegate only when that is genuinely not enough**: a long suite whose failures do not name a
cause, or a failure you cannot attribute to the change. Hand a subagent the log path (it is on
disk), the step, the exit code and the changed-file list; it reads the log and only the sources
needed, and **returns at most 15 lines**: what failed by name, probable root cause in a
sentence or two, caused-by-this-change or pre-existing, a concrete suggested fix. **Read-only**
— triage diagnoses, never fixes: what to do about a red gate is the caller's decision, and for
`finish`/`pr` the user's.

## Rules

- Open the run directory before the first step — `facts.sh` did it; `gate-run.sh` refuses a
  path that does not exist rather than writing a log to `/`.
- Run the gate from the repo root of the current worktree.
- Report exactly which step failed and its exit code. Never silently continue past a failure.
- For a **draft** PR or an explicit override, the user may proceed past a failing gate: surface
  it, then respect the decision.
- Never fabricate a passing result. If a check was skipped — no such script, too slow, user
  opted out — say so, and say which.
