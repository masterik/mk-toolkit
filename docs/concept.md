# mkit — Concept

## What it is

mk-toolkit is a **personal toolkit for agentic coding**: the skills, conventions and small
mechanical tools one developer uses to work with a coding agent day to day. It is built for that
workflow first — Claude Code, on macOS, in git repos hosted on GitHub — and published as-is in case
parts of it are useful to someone else.

It has two halves that ship separately:

- **A Claude Code plugin** (`plugin/`) — Markdown skills that tell the agent *when* a task applies
  and *how* to do it safely: commit, review, open a PR, merge back, clean up branches, audit the
  sandbox, explain or recap the session.
- **A Go binary, `mkit`** — the mechanical layer beneath the skills, plus a few standalone
  utilities for the agent's environment (cache pruning, sandbox audit, `doctor`).

The agent is the interface; the skills are the muscle memory; the binary does the parts that must
be identical every run.

## Why it exists

Agentic coding repeats the same small procedures many times a day — stage and split commits, run
the repo's checks, review a diff with several tools, open a PR with the right reviewers, tidy up
worktrees. Left to the agent, each run re-derives a fragile `git` + `gh` + `wt` command sequence
from scratch and gets some invariant wrong some of the time. The toolkit writes each procedure down
once, with the safety rules attached, and moves the deterministic parts into a tested binary.

## Design principles

- **Judgement in Markdown, mechanics in the binary.** A skill decides commit boundaries, severity,
  whether a fix is safe. `mkit` opens directories, gathers facts, runs and records the quality
  gate, classifies branches, does arithmetic over review findings. Where the line is unclear the
  binary reports candidates and the skill picks.
- **Composition over replacement.** Orchestrate `git`, `gh`, `wt`, CodeRabbit and Codex; never
  reimplement them. The binary reports and runs; it never integrates — no staging, merging,
  pushing or editing your files. Its writes are bounded to its own state (run directories, the gate
  ledger, the worklog), one `info/exclude` line that keeps `.mkit/` ignored, remote-tracking refs
  (`git fetch --prune`), and `.mkit/config.toml`, which only `mkit init` writes. The one exception
  is `mkit cache prune --apply`, which deletes stale Claude Code / Codex storage on request.
- **A recorded fact is an input, never a permission.** The gate ledger and the worklog remember
  what earlier runs proved. A skill may use that to skip work, but always says so (`cached`), and
  never treats a missing record as a reason to stop.
- **Composable steps, not a pipeline.** Every skill runs alone, in any order, with any subset
  skipped. It discovers what it needs, derives the thin version of what's missing, and names what
  it assumed. It never sends you to another skill first and never runs one downstream of itself.
  Contract: [`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md).
- **Safe by default.** Irreversible actions (force-push, branch delete, history rewrite, skipping
  hooks) are gated by one shared safety protocol.
- **Sandbox-aware.** The agent usually runs under an OS sandbox, a permission classifier and a
  worktree-isolation guard. State goes where all three already allow writes (`<toplevel>/.mkit/`,
  `$TMPDIR`), and anything that would hit a denied path is reported up front with a remedy that
  actually works ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).
- **Discover first, configure optionally.** Gate commands, commit scopes, reviewers and merge style
  are discovered from the repo. `mkit init` can pin what discovery can't establish into a committed
  `.mkit/config.toml`, but nothing ever requires it
  ([ADR 0001](adr/0001-per-repo-config-and-init.md)).
- **Narrow on purpose.** macOS only, Claude Code only. Supporting more is a packaging question for
  when someone needs it, not a matrix to carry untested.

## The skills

| Skill | Does |
| --- | --- |
| `commit` | Inspect the tree, stage intentionally, split into logical Conventional Commits. |
| `review` | Review the local diff or recent commits with CodeRabbit, Codex and Claude (or a quick mode with the first two), verify findings, fix what's worth fixing. |
| `pr` | Commit → push → open a GitHub PR → request reviewers. |
| `finish` | Commit → merge into the base branch locally (or merge the open PR) → delete the branch and worktree. |
| `cleanup` | Sweep every local branch and worktree; delete what's provably merged, ask about the rest. |
| `sandbox-audit` | Read recent sessions' sandbox and permission-gate events and propose a settings diff. Report-only. |
| `explain` | Re-say the last answer, or one item from the session, in plain English. |
| `recap` | Plain-English status of the branch: done, changed, left, waiting on you. |

Planned, not built: `brainstorm`, `spec` and `implement` — the front half that takes an idea to a
task graph and works it slice by slice ([backlog](backlog.md)).

Skills share one reference bundle, [`plugin/skills/_shared/`](../plugin/skills/_shared/README.md):
git safety, Conventional Commits, quality-gate detection, worktree handling, review severity and
lenses, finding triage, agent delegation, output discipline, plain-English writing.

## How it fits together

```
 Claude Code
   │  loads the plugin (no hooks)
   ▼
 skills/*/SKILL.md           when & how — judgement
   │  link into
   ▼
 _shared/references/*.md     safety · conventions · gate · worktrees · review
   +
 mkit (Go)                   mechanics — one call each, --json on every command
   facts                     run directory + every starting fact a skill needs
   gate detect | run         what this repo's checks are; run them, logged and recorded
   findings                  validate · reconcile · group a review's findings
   branch status             classify every local branch/worktree for cleanup
   worklog show | append     what each step concluded on this branch
   repo profile · init       discovered vs pinned repo config
   doctor                    prerequisites, sandbox writability, plugin state
   audit sandbox · cache prune · scratch prune     environment housekeeping
   │  drive
   ▼
 git · gh · wt · coderabbit · codex
```

Every repo-scoped skill starts with `mkit facts <skill>` — `review` first probes `mkit findings`,
`sandbox-audit` (user-wide) starts with `mkit audit sandbox` instead, and `explain` calls no `mkit`
at all. A missing or too-old binary stops the skill with a `brew` remedy. There is no version range on either side — a subcommand that doesn't exist
*is* the too-old signal.

## State

Three write locations, chosen by lifetime, and nowhere else:

| Where | What | Lifetime |
| --- | --- | --- |
| `$TMPDIR` | anything that dies with the command | one call |
| `<toplevel>/.mkit/` | run directories, the gate ledger (`gate.jsonl`), the per-branch worklog — git-ignored; plus `config.toml`, committed | across steps and sessions |
| `~/.mkit/` (`MKIT_HOME`) | user-scoped state — today only the `sandbox-audit` ledger | across repos |

Plus one line in the common dir's `info/exclude`, so `.mkit/` stays ignored. State is never kept in
`~/.claude` (sandbox-protected, so no allowlist entry can open it) or the user's own files;
`mkit cache prune --apply` is the one command that reaches into `~/.claude` and `~/.codex`, and only
to delete stale storage you asked it to.

## Workflow model

```
brainstorm → spec → implement → commit → review → pr ──┐
                                                        ├─→ merged
                                               finish ──┘
cleanup · sandbox-audit · explain · recap   (outside the line)
```

The arrows are the common path, never a required one. Real work enters in the middle — a bug fix
starts at `implement`, someone else's branch starts at `review`, most changes are just `commit`.
The **worklog** (one file per branch under `.mkit/worklog/`) is what makes that cheap: each step appends what it
concluded and over which content, so the next step can reuse a goal or a gate result instead of
re-deriving it.

## Distribution

- **Plugin** — from the GitHub marketplace: `/plugin marketplace add masterik/mk-toolkit`, then
  `/plugin install mkit@masterik`.
- **Binary** — from Homebrew: `brew install masterik/tap/mkit` (darwin amd64 + arm64).

The two version independently ([ADR 0003](adr/0003-two-distribution-channels.md)); releases are
tag-driven and lockstep in practice. Setup details: [`prerequisites.md`](prerequisites.md).

## Considered and dropped

- **A `SessionStart` hook** that named missing prerequisites. Removed in favour of `mkit doctor` —
  one implementation instead of two. Accepted loss: nothing reports a missing tool unprompted, and a
  binary can't report its own absence.
- **A `Stop` hook** nudging the agent to journal intent per unit of work. Its output is rendered in
  the transcript every turn and can't be suppressed; `commit` derives intent from the diff instead.
  A transcript-mining alternative is parked in [`ideas/`](ideas/README.md).
