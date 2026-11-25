# mkit: concept

## What it is

A **personal toolkit for agentic coding**: the skills, conventions and small tools one developer uses
every day with a coding agent, in GitHub-hosted git repos, published as-is.

- **Plugin** (`plugin/`) — the product: Markdown skills that tell the agent *when* a task applies and
  *how* to do it safely, plus two bundled bash scripts that gather facts in one call. It stands alone.
- **Binary** (`mkit`, Go) — optional: macOS + Claude Code power tools (`doctor`, `audit sandbox`,
  `cache prune`). Only `sandbox-audit` needs it
  ([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decisions 1–2).

## Why

Agentic coding repeats the same small procedures many times a day: splitting commits, running checks,
multi-tool review, opening PRs with the right reviewers, tidying worktrees. Left alone, the agent
re-derives a fragile `git`/`gh`/`wt` sequence each time and sometimes breaks an invariant. The toolkit
writes each procedure down once, with its safety rules, and moves the parts that would otherwise take
many tool calls into two small, tested scripts.

## Design principles

- **Judgement in Markdown, mechanics in code only where it saves calls.** Skills decide commit
  boundaries, severity, and whether a fix is safe. Two bundled scripts gather facts and classify
  branches; skills find and run the gate themselves; `review` dedupes its findings in prose. When the
  split is unclear, a script reports candidates and the skill picks.
- **Compose, don't replace.** Orchestrate `git`, `gh`, CodeRabbit, Codex and (optionally) `wt`. The
  scripts and the binary never stage, merge, push or edit your files. Their writes are limited to
  remote-tracking refs (`branch-status.sh`'s `git fetch --prune`), `facts.sh --run`'s run dir under
  `$TMPDIR`, the binary's net-zero write probes, and `mkit cache prune --apply`, which deletes stale
  Claude Code and Codex storage when you ask it to.
- **A recorded fact is an input, never a permission.** A skill may skip work because the commit
  history, the PR, or (for the gate) a pass earlier in the same session shows it was already done, and it says so. A missing record is never a reason to
  stop.
- **Composable steps, not a pipeline.** Each skill runs alone, in any order. It derives what is missing
  and names what it assumed. It never sends you to another skill first
  ([workflow contract](../plugin/skills/_shared/references/workflow-contract.md)).
- **Safe by default.** Force-push, branch delete, history rewrite and skipping hooks all go through one
  shared safety protocol.
- **Sandbox-aware.** Per-run files live in `$TMPDIR`, where the OS sandbox, the permission
  classifier and the worktree guard already allow writes; nothing goes in the work tree. `~/.mkit/`
  needs one grant. A path the sandbox would deny
  is reported up front, with a remedy that works ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).
- **Discover first, configure optionally.** Gate commands, scopes and reviewers are read from the repo;
  the rest has a default. A repo changes one with a short prose file in `docs/agents/` (written by the
  `setup` skill or by hand); nothing requires it
  ([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 7).
- **Plugin first, portable next.** The skills install in Claude Code, Codex and Copilot; Windows
  (Git Bash) is next ([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 9).
  The binary stays macOS + Claude Code on purpose.

## Skills

| Skill | Does |
| --- | --- |
| `commit` | Stage intentionally; split into logical Conventional Commits. |
| `review` | CodeRabbit, Codex and Claude (or `quick`: the first two) over the diff or recent commits; verify findings, fix the worthwhile ones. |
| `pr` | Commit, push, open a PR, request reviewers. |
| `finish` | Commit, merge into base (the open PR, or locally), delete the branch and worktree. |
| `cleanup` | Delete provably merged branches and worktrees; ask about the rest. |
| `setup` | Optional, once per repo: record non-default settings as prose in `docs/agents/`. |
| `sandbox-audit` | Turn sandbox and permission-gate events into a proposed settings diff. Report-only; needs the binary. |
| `explain` | Re-explain the last answer or one session item in plain English. |
| `recap` | Branch status in plain English: done, changed, left, waiting on you. |
| `shorten` | Rewrite text shorter (returns it) or a doc file (edits it in place); input always named. |

Every skill but `shorten` shares one reference bundle, [`_shared/`](../plugin/skills/_shared/README.md). It covers git
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
 _shared/references/*.md     safety · facts · gate · worktrees · review
   +
 _shared/scripts/            bash, bundled with the plugin
   facts.sh                  all starting facts (+ a run dir under $TMPDIR)
   branch-status.sh          classify branches/worktrees for cleanup
     │  drive
   ▼
 git · gh · coderabbit · codex · wt (optional)

 mkit (Go, optional)         macOS + Claude Code power tools, --json on every command
   doctor                    prerequisites, plugin state, sandbox grants, old-release leftovers
   audit sandbox             what sandbox-audit reads
   cache prune               housekeeping
```

Each repo-scoped skill starts with the bundled `facts.sh <skill>`. The exceptions:

- `sandbox-audit` starts with `mkit audit sandbox`.
- `explain` and `shorten` run no commands (`shorten` may edit the one file named).
- `setup` explores with plain `git` calls and the repo's files.

No skill checks for the binary but `sandbox-audit`, which stops with a `brew` remedy when
`mkit audit sandbox` is missing or too old.

## State

Two write locations, chosen by lifetime:

| Where | What | Lifetime |
| --- | --- | --- |
| `$TMPDIR` | per-run files (`facts.sh --run`'s run dir) | one session |
| `$TMPDIR` | anything that dies with the command | one call |
| `~/.mkit/` (`MKIT_HOME`) | user-scoped state; today only the `sandbox-audit` ledger | across repos |

Nothing goes in the work tree, so there is no ignore rule to write and nothing to prune
([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 5). mkit never stores state in
your files or in `~/.claude`, which is sandbox-protected, so no allowlist entry can open it.
`cache prune --apply` deletes stale storage in `~/.claude` and `~/.codex`, and only when you ask.

## Workflow

```
commit → review → pr ──┐
                        ├─→ merged
               finish ──┘
setup · cleanup · sandbox-audit · explain · recap · shorten   (outside the line)
```

This is the common path, not a required one. Someone else's branch starts at `review`, and most changes are just `commit`. A step learns what came before from the branch's
commits and its PR (subjects, bodies, review state), so the next step can reuse a goal instead of
re-deriving it.

## Distribution

- **Plugin:** `/plugin marketplace add masterik/mk-toolkit`, then `/plugin install mkit@masterik`.
- **Binary (optional):** `brew install masterik/tap/mkit` (darwin amd64 and arm64).

Two channels ([ADR 0003](adr/0003-two-distribution-channels.md)), one tag: every release bumps each
agent manifest and builds the binary together. Setup: [prerequisites](prerequisites.md).

## Considered and dropped

- **`SessionStart` hook** that reported missing prerequisites. Replaced by `mkit doctor`, so there is one
  implementation. The cost: nothing reports a missing tool unprompted.
- **`mkit init` and TOML pinning** (`.mkit/config.toml`). Replaced by prose files in `docs/agents/`
  that the `setup` skill writes on any agent; the binary could only ever run on macOS
  ([ADR 0006](adr/0006-standalone-skills-optional-binary.md)).
- **`Stop` hook** prompting the agent to record its intent. Its output shows in the transcript every
  turn and can't be suppressed. `commit` derives intent from the diff instead. A transcript-mining
  alternative is parked in [`ideas/`](ideas/README.md).
