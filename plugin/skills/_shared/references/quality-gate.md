# Quality gate

Shared by `finish` and `pr`, both full tier. `commit` and `review` don't gate — they work
directly on the diff. **Nothing is hardcoded** — the commands are detected from the target repo.

## Detect, then choose

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gate-detect.sh
# pm=bun
# ecosystem=node
# fast=bun run lint
# fast_cache=fresh exit=0 age=6m
# full=bun run lint|bun run test|bun run build
# full_cache=fresh|failed|none
# full_cache_exit=0|1|-
# full_cache_age=6m|6m|-
# gate_fingerprint=7c998da01a8a9aa8
# gate_max_age_min=60
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

## One tier, two consumers

`gate-detect.sh` still proposes both `fast=` and `full=`, but only the full tier is used now —
**by `finish` and `pr`, once each, before merge/PR**: the complete pre-integration sequence, in
order, stopping at the first failure. `commit` and `review` used to consume the fast tier; they
no longer gate at all.

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

## The gate ledger — what was already proven

`gate-run.sh` records every step it finishes into `<git-dir>/mkit/gate.jsonl`:
`(step, command, exit code, seconds, fingerprint of the content it ran over)`.
`gate-detect.sh` compares each command it proposes against the newest record for that
**exact command string** and annotates it. The `*_cache=` keys above are that annotation.

**What this saves is wall-clock, not tokens.** There are no token savings here by
construction — `gate-run.sh` already sends full output to a log so it never reaches
context. What it saves is a re-execution of a 90-second suite over a tree that stopped
changing — e.g. `pr` gates a step, then `finish` gates the same content again later.

### The governing rule

> The ledger records **what was proven, over which content**. It never decides whether a
> gate may be skipped, and a skipped step is always reported as skipped.

**A report may never show `gate=ok` for a step that did not run.** A cached step is named
`cached` with its age, in the same line that would have carried its verdict:

```
lint   cached (6m ago, exit=0)
test   ok 91s
gate=ok (1 step cached)
```

A run that prints `gate=ok` having executed nothing is worse than any amount of
re-running: it reports a safety net that was never deployed. Hence the script never
skips, and you must label.

### Classification

| Class | Means | Do |
|---|---|---|
| `fresh` | same content, `exit=0`, within the age bound | skip if you choose — and **label it `cached`** |
| `failed` | same content, `exit≠0` — **whatever its age** | **say so before running.** Surface the known-red tree, then run the step anyway: a stale environment can turn a real failure green, and the win here is the early warning, not the skip |
| `drifted` | the content changed | run |
| `stale` | same content, `exit=0`, older than the age bound | run — the environment may have moved |
| `unknown-head` | the recorded commit no longer resolves | run; never treat as `fresh` |
| `none` | no record for this exact command | run |

A step *name* is not identity — the command is. `bun run test` and
`bun run test --coverage` are different checks. A command you overrode (from `alt_fast`,
or `docs_candidates`) therefore classifies `none` and simply runs, which is the correct
default for anything unrecognized.

### What the fingerprint cannot see

It covers **tracked repo content only**: the committed tree overlaid with the worktree,
invariant under staging and committing. Invisible to it — a dependency install, a tool or
runtime version change, environment variables, generated artifacts under `.gitignore`,
and **file mode** (`chmod +x` does not change a blob sha; a known, documented gap).

So `fresh` means "the tracked content is identical", not "the environment is identical".
That gap is what the **age bound** is for: past 60 minutes a matching fingerprint
classifies `stale` and the step runs. Age is *reported* on every class so you can always
see how old a proof is; it is only *decisive* for `stale`.

One consequence worth recognizing in the wild: a gate step that *writes* into the tree — a
build emitting untracked output, a formatter rewriting files, a snapshot test updating
fixtures — changes the content its own record is keyed on, so the next lookup reads
`drifted` and everything runs. Correct, and safe; it just means the ledger never helps on
those repos. Ignore the output (`.gitignore`) and it becomes invisible to the fingerprint
again.

Note also that `--prune` deletes run directories, so a record can outlive the
`gate-<step>.log` it names. The ledger stores the verdict; if you need the log, re-run.

### Per-skill posture

Deliberately unequal — `pr` and `finish` do not carry the same risk.

| Skill | Step | Posture |
|---|---|---|
| `pr` | full tier | **may consume per step.** A draft PR is recoverable and CI runs remotely anyway |
| `finish` | full tier | **strictest.** Its gate is the only safety net before a local merge. Consume only on an exact command match, a matching fingerprint, and an age within bound — and **always** print `cached (Nm ago)`. Never cache a whole chain silently |

### When there is nothing to classify

One key, one cause — the same discipline as `scripts_state`:

- `gate_cache=off` — `--no-cache` was passed. This is the answer to "I don't trust it".
- `gate_cache=empty` — nothing to compare against: no ledger yet, or one with no records.
- `gate_cache=no-hash` — no `shasum` on the machine, so no fingerprint is possible.
- `gate_cache=no-fingerprint` — a hash tool *is* present but the fingerprint could not be
  computed anyway. A distinct cause on purpose: "install `shasum`" is the wrong advice for
  someone who already has it.
- `gate_cache=no-jq` — no `jq`.

Each degrades to today's behavior exactly: detect, then run everything.

The two escape hatches: `gate-detect.sh --no-cache` ignores the ledger for one call
(`gate_cache=off`), and `gate-run.sh --no-ledger` stops writing to it. Neither is needed in
normal use; reach for `--no-cache` when a `fresh` verdict looks wrong and you want the
question off the table.

## Rules

- Open the run directory before the first step — `facts.sh` did it; `gate-run.sh` refuses a
  path that does not exist rather than writing a log to `/`.
- Run the gate from the repo root of the current worktree.
- Report exactly which step failed and its exit code. Never silently continue past a failure.
- For a **draft** PR or an explicit override, the user may proceed past a failing gate: surface
  it, then respect the decision.
- Never fabricate a passing result. If a check was skipped — no such script, too slow, user
  opted out, or **served from the ledger** — say so, and say which.
