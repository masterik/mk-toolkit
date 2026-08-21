# mkit — Concept

## Summary
mkit is a **Claude Code plugin** — a cohesive **kit of agent coding-workflow skills**. It
gives Claude a safe, repeatable way to take work from **edits → committed → reviewed →
integrated**: stage and commit cleanly, review the diff, then either merge back locally or
open a PR — without re-deriving fragile `git` + `gh` + `wt` command sequences on every task.

It's a *workflow* toolkit, not just a git one: `review` drives CodeRabbit/Codex/Claude,
`pr` drives GitHub, and `finish` handles worktree cleanup — the parts of the
dev loop the agent runs, git-centric but not git-limited.

There is no CLI and nothing to install into the repo's toolchain. The plugin is essentially
**knowledge + procedure**: each skill tells Claude *when* it applies and *how* to drive the
underlying tools, with the safety rules that keep destructive steps from firing by accident.
The agent is the interface; the skills are the muscle memory.

Alongside the Markdown sits a thin layer of **helper scripts** (`scripts/`, five of them) for
the steps that are identical every run and fail silently when hand-rolled: opening the run
directory, gathering the starting facts, detecting and running the quality gate, and the
arithmetic over a review's findings. They ship with the plugin — no `PATH`, no build, no
install — and they exist for reliability more than for tokens: prose re-executed every run kept
getting one invariant of three wrong. **Judgement stays in Markdown; a mechanical invariant
belongs in a script.** Prerequisites: [`PREREQUISITES.md`](PREREQUISITES.md).

**Claude-only for now.** Other agents (Codex, opencode, …) are a later concern — the skills
are plain Markdown, so support for another agent is a thin packaging step, not a rewrite.

## Design Principles
- **Skills, not a binary:** the workflow lives in Markdown the agent reads, not in code it
  executes. Nothing to build, version, or keep on `PATH` — which is also why the scripts are
  shell plus one dependency-free Node file, and never a compiled binary: a plugin install is a
  clone, and the glue is ~4 ms of a ~10 s agent turn, so a faster language would optimize
  nothing and cost a release pipeline.
- **A script for a mechanical invariant, never for a decision:** `scripts/` may open a
  directory, run a logged command, classify a worktree or do confidence arithmetic. It may not
  choose commit boundaries, assign severity, judge materiality, or decide that a fix is safe.
  Where the line is genuinely unclear the script reports candidates and the skill picks —
  `gate-detect.sh` proposing `fast=` beside `docs_candidates:` is the shape to copy.
- **Composition over replacement:** orchestrate `git`, GitHub CLI (`gh`), and Worktrunk
  (`wt`); never reimplement what they already do well.
- **Safe by default:** irreversible actions (force-push, branch delete, history rewrite,
  hook-skipping) are gated by an explicit safety protocol the skills share.
- **DRY via shared references:** the skills link into one `_shared/references/` bundle
  instead of each restating the same safety and convention rules.
- **Portable:** nothing project-specific is hardcoded — quality-gate commands, commit
  scopes, and reviewers are all *discovered* from the target repo.

## The Skills
A cohesive bundle that moves work through its lifecycle. `commit` is the shared front-end;
`finish` and `pr` both begin by committing, and you pick the finisher by
**destination** — merge it yourself locally, or push it for review.

| Skill | Does | Trigger examples |
|-------|------|------------------|
| **`commit`** | Inspect the tree, stage intentionally, split into logical Conventional Commits. | "commit", "split into commits" |
| **`review`** | Review the local diff/commits — full (CodeRabbit + Codex + Claude, all lenses) or quick (CodeRabbit + Codex, bugs/impl only) — verify the findings, fix what's worth fixing, summarize. | "review my changes", "quick review", "run codex and coderabbit" |
| **`finish`** | Commit → merge the branch back into its base → delete branch / remove worktree. **Local**, no PR. | "finish this feature", "merge back and clean up" |
| **`pr`** | Commit → push → open a GitHub PR → assign reviewers. **Remote review** path. | "create a PR", "open a pull request", "submit for review" |

### Shared references — `skills/_shared/`
`_shared/` is **not** a triggerable skill (it has no `SKILL.md`); it is the shared library
the four skills link into via `../_shared/references/…`:

- `git-safety.md` — the non-negotiable git safety protocol (no force-push, no config edits,
  no AI attribution, don't skip hooks, …).
- `conventional-commits.md` — commit message format, type table, scope detection.
- `quality-gate.md` — how to **detect** (not hardcode) the repo's fast check + full
  lint/test/build gate, and how to triage a failing step.
- `worktree.md` — detect the worktree origin (Worktrunk `wt` / Claude Code
  `.claude/worktrees/` / plain `git worktree`) and clean up correctly.
- `branching.md` — the default branch model to assume when a repo doesn't document its own.
- `review-severity.md` — the severity bar (`critical`/`major`/`minor`), the `[surface, severity]`
  tag, the read-only reviewer contract, what not to report, and the partial-review rule.
- `lenses-correctness.md` / `lenses-craft.md` — the eight review lenses, split along the reviewer
  that carries each set: `bugs`/`impl`/`adversarial` (Codex) and
  `architecture`/`quality`/`tests`/`docs`/`comments` (the Claude subagent).
- `triage-reconcile.md` / `triage-verify.md` / `fix-checks.md` — reconcile (dedupe +
  corroboration), verify (five verdicts + the materiality test), and the three checks on every
  fix — one file per stage, so each stage loads only its own.
- `agent-delegation.md` — context discipline: the per-run directory inside the git dir as the
  transport between stages, subagent return budgets, resolved reference paths, parallel-vs-
  sequential rules, and which model each stage shape wants.
- `output-discipline.md` — bounding command output: gate logs written to a file and read by
  their tail, `--stat` before any diff, never a full branch diff to write prose — plus what
  must never be capped.

## Architecture
```
 Claude Code
   │   loads plugin skills (via .claude-plugin/plugin.json)
   ▼
 commit · review · finish · pr   ← SKILL.md (when & how)
   │   all link into
   ▼
 _shared/references/*.md   (safety · conventions · quality gate · worktree · branching
                            severity bar · lenses · finding triage
                            agent delegation · output discipline)
   +
 scripts/                  the mechanical steps, one call each
   run-open.sh             open a run directory · --prune old ones
   facts.sh                run dir + refs path + branch/status/worktree/stats, in one call
   gate-detect.sh          what this repo's fast + full checks are
   gate-run.sh             run a gate step: log it, bound it, stop at the first failure
   findings.mjs            reconcile · group · report over a review's findings (JSONL)
   │   drive
   ▼
 git   +   gh (GitHub CLI)   +   wt (Worktrunk)   +   rg   +   jq
```

The scripts never act: no staging, no merging, no `wt merge`, no edits. They report facts and
run commands the skill named. `rtk` is deliberately not among them — it reshapes output for an
agent to read, which is exactly what a parser must not tolerate; it stays on the agent's own
direct commands.

The skills are the single source of truth for the *workflow*; the underlying tools remain
the source of truth for the *operations*. The plugin never re-encodes git logic.

## Workflow Model
Work is modeled as a **feature** — edits that become commits and then get integrated:

```
edit → commit → review → finish
                          ├── finish  (local merge, delete branch / worktree)
                          └── pr      (push, open PR, review remotely)
```

Worktree awareness is built in: the finishing skills detect whether they're in a Worktrunk
worktree, a Claude Code agent worktree, or a plain checkout, and use the matching cleanup
path.

## Distribution
mkit ships as a standard Claude Code plugin:

```
.claude-plugin/
  plugin.json          # plugin manifest (name, skills discovered from skills/)
  marketplace.json     # marketplace entry — source "./"
skills/
  commit/SKILL.md
  pr/SKILL.md
  review/SKILL.md
  finish/SKILL.md
  _shared/             # shared references (README + references/*.md) — no SKILL.md
scripts/
  lib/common.sh        # sourced helpers: plugin root, refs path, rg-or-grep, wt binary
  run-open.sh  facts.sh  gate-detect.sh  gate-run.sh  findings.mjs
tests/                  # dev-only: the script layer's own test suite (tests/run.sh)
PREREQUISITES.md       # required + recommended tooling, setup, permission allowlist
```

Install it with:

```
/plugin marketplace add masterik/workflow_tool
/plugin install mkit@masterik
```

Plugin skills are namespaced (`mkit:commit`), which avoids clashing with any repo-local
skills of the same name.

## Roadmap
- **Now — Claude Code plugin.** The four skills + the shared reference bundle, packaged and
  installable. This is the whole product.
- **Later — other agents.** Codex, opencode, and others are plain-Markdown consumers of the
  same skill content; supporting one is a packaging step, added only if needed, with no
  change to the skills themselves.
