# mkit: concept

## What it is

A **personal toolkit for agentic coding**: the skills, conventions and small tools one developer uses
every day with a coding agent. It targets Claude Code, on macOS, in GitHub-hosted git repos, and is
published as-is.

It has two halves, which ship separately:

- **Plugin** (`plugin/`): Markdown skills that tell the agent *when* a task applies and *how* to do it
  safely.
- **Binary** (`mkit`, Go): the mechanics beneath the skills, plus environment utilities (`cache
  prune`, `audit sandbox`, `doctor`).

## Why

Agentic coding repeats the same small procedures many times a day: splitting commits, running checks,
multi-tool review, opening PRs with the right reviewers, tidying worktrees. Left alone, the agent
re-derives a fragile `git`/`gh`/`wt` sequence each time and sometimes breaks an invariant. The toolkit
writes each procedure down once, with its safety rules, and moves the deterministic parts into a
tested binary.

## Design principles

- **Judgement in Markdown, mechanics in the binary.** Skills decide commit boundaries, severity, and
  whether a fix is safe. `mkit` gathers facts, runs and records the gate, classifies branches, and
  does the arithmetic on review findings. When the split is unclear, the binary reports candidates
  and the skill picks.
- **Compose, don't replace.** Orchestrate `git`, `gh`, `wt`, CodeRabbit and Codex. The binary never
  stages, merges, pushes or edits your files. Its writes are limited to:
  - its own state: run dirs, the gate ledger, the worklog;
  - the `.mkit/*` + `!.mkit/config.toml` pair in `info/exclude`;
  - remote-tracking refs, via `git fetch --prune`;
  - `.mkit/config.toml`, written only by `mkit init`.

  The single exception is `mkit cache prune --apply`, which deletes stale Claude Code and Codex
  storage when you ask it to.
- **A recorded fact is an input, never a permission.** A skill may skip work because the gate ledger or
  worklog shows it was already done, and it says so (`cached`). A missing record is never a reason to
  stop.
- **Composable steps, not a pipeline.** Each skill runs alone, in any order. It derives what is missing
  and names what it assumed. It never sends you to another skill first
  ([workflow contract](../plugin/skills/_shared/references/workflow-contract.md)).
- **Safe by default.** Force-push, branch delete, history rewrite and skipping hooks all go through one
  shared safety protocol.
- **Sandbox-aware.** Repo and per-call state lives where the OS sandbox, the permission
  classifier and the worktree guard already allow writes (`<toplevel>/.mkit/`, `$TMPDIR`); `~/.mkit/`
  needs one grant. A path the sandbox would deny
  is reported up front, with a remedy that works ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).
- **Discover first, configure optionally.** Gate commands, scopes, reviewers and merge style are read
  from the repo. `mkit init` can pin them in a committed `.mkit/config.toml`; nothing requires it
  ([ADR 0001](adr/0001-per-repo-config-and-init.md)).
- **Narrow on purpose.** macOS and Claude Code only. Other platforms are a packaging question for when
  someone needs them.

## Skills

| Skill | Does |
| --- | --- |
| `commit` | Stage intentionally; split into logical Conventional Commits. |
| `review` | CodeRabbit, Codex and Claude (or `quick`: the first two) over the diff or recent commits; verify findings, fix the worthwhile ones. |
| `pr` | Commit, push, open a PR, request reviewers. |
| `finish` | Commit, merge into base (the open PR, or locally), delete the branch and worktree. |
| `cleanup` | Delete provably merged branches and worktrees; ask about the rest. |
| `sandbox-audit` | Turn sandbox and permission-gate events into a proposed settings diff. Report-only. |
| `explain` | Re-explain the last answer or one session item in plain English. |
| `recap` | Branch status in plain English: done, changed, left, waiting on you. |

Planned, not yet built: `brainstorm`, `spec` and `implement`. Together they take an idea to a task
graph and work through it slice by slice ([backlog](backlog.md)).

All skills share one reference bundle, [`_shared/`](../plugin/skills/_shared/README.md). It covers git
safety, Conventional Commits, gate detection, worktrees, review severity and lenses, triage, agent
delegation, output discipline and plain-English writing.

## How it fits

```
 Claude Code
   │  loads the plugin (no hooks)
   ▼
 skills/*/SKILL.md           when & how: judgement
   │  link into
   ▼
 _shared/references/*.md     safety · conventions · gate · worktrees · review
   +
 mkit (Go)                   mechanics, --json on every command
   facts                     run dir + all starting facts
   gate detect | run         the repo's checks; run, log, record
   findings                  validate · reconcile · group review findings
   branch status             classify branches/worktrees for cleanup
   worklog show | append     what each step concluded on this branch
   repo profile · init       discovered vs pinned config
   doctor                    prerequisites, sandbox, plugin state
   audit sandbox · cache prune · scratch prune     housekeeping
   │  drive
   ▼
 git · gh · wt · coderabbit · codex
```

Each repo-scoped skill starts with `mkit facts <skill>`. The exceptions:

- `review` first probes `mkit findings`.
- `sandbox-audit` starts with `mkit audit sandbox`.
- `explain` calls no `mkit` command.

A missing or too-old binary stops the skill with a `brew` remedy. Neither side declares a version
range: a subcommand that doesn't exist *is* the too-old signal.

## State

There are three write locations, chosen by lifetime:

| Where | What | Lifetime |
| --- | --- | --- |
| `$TMPDIR` | anything that dies with the command | one call |
| `<toplevel>/.mkit/` | run dirs, `gate.jsonl`, `worklog/` (all git-ignored); `config.toml` (committed) | across steps and sessions |
| `~/.mkit/` (`MKIT_HOME`) | user-scoped state; today only the `sandbox-audit` ledger | across repos |

mkit also adds the pair `.mkit/*` + `!.mkit/config.toml` to the common dir's `info/exclude`, so
scratch stays ignored and the config stays addable. It never stores
state in your files or in `~/.claude`, which is sandbox-protected, so no allowlist entry can open it.
`cache prune --apply` deletes stale storage in `~/.claude` and `~/.codex`, and only when you ask.

## Workflow

```
brainstorm → spec → implement → commit → review → pr ──┐
                                                        ├─→ merged
                                               finish ──┘
cleanup · sandbox-audit · explain · recap   (outside the line)
```

This is the common path, not a required one. A bug fix starts at `implement`, someone else's branch
starts at `review`, and most changes are just `commit`. The **worklog** keeps one file per branch under
`.mkit/worklog/`. It records what each step concluded and over which content, so the next step can reuse
a goal or a gate result instead of re-deriving it.

## Distribution

- **Plugin:** `/plugin marketplace add masterik/mk-toolkit`, then `/plugin install mkit@masterik`.
- **Binary:** `brew install masterik/tap/mkit` (darwin amd64 and arm64).

The two are versioned independently ([ADR 0003](adr/0003-two-distribution-channels.md)). In practice,
releases are tag-driven and ship together. Setup: [prerequisites](prerequisites.md).

## Considered and dropped

- **`SessionStart` hook** that reported missing prerequisites. Replaced by `mkit doctor`, so there is one
  implementation. The cost: nothing reports a missing tool unprompted, and a binary cannot report its
  own absence.
- **`Stop` hook** prompting the agent to record its intent. Its output shows in the transcript every
  turn and can't be suppressed. `commit` derives intent from the diff instead. A transcript-mining
  alternative is parked in [`ideas/`](ideas/README.md).
