# mkit — Concept

## Summary
mkit is a **Claude Code plugin** — a cohesive **kit of agent coding-workflow skills**. It
gives Claude a safe, repeatable way to take work from **edits → committed → reviewed →
integrated**: stage and commit cleanly, review the diff, then either merge back locally or
open a PR — without re-deriving fragile `git` + `gh` + `wt` command sequences on every task.

It's a *workflow* toolkit, not just a git one: `review-changes` drives CodeRabbit/Codex/Claude,
`create-pr` drives GitHub, and `finish-feature` handles worktree cleanup — the parts of the
dev loop the agent runs, git-centric but not git-limited.

There is no CLI and nothing to install into the repo's toolchain. The plugin is essentially
**knowledge + procedure**: each skill tells Claude *when* it applies and *how* to drive the
underlying tools, with the safety rules that keep destructive steps from firing by accident.
The agent is the interface; the skills are the muscle memory.

The one exception is `scripts/run-open.sh`, which opens a run directory for logs and
intermediate files. It ships with the plugin (no `PATH`, no build, no install) and exists
because that step is *mechanical* — atomic creation, absolute path, inside the git dir — and
prose re-executed on every run kept getting one of the three wrong. Judgement stays in
Markdown; a shell invariant belongs in shell.

**Claude-only for now.** Other agents (Codex, opencode, …) are a later concern — the skills
are plain Markdown, so support for another agent is a thin packaging step, not a rewrite.

## Design Principles
- **Skills, not a binary:** the workflow lives in Markdown the agent reads, not in code it
  executes. Nothing to build, version, or keep on `PATH`. A script is warranted only for a
  *mechanical invariant* the agent would otherwise re-derive every run (see `scripts/`) —
  never for a decision, and never for a git operation.
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
`finish-feature` and `create-pr` both begin by committing, and you pick the finisher by
**destination** — merge it yourself locally, or push it for review.

| Skill | Does | Trigger examples |
|-------|------|------------------|
| **`commit`** | Inspect the tree, stage intentionally, split into logical Conventional Commits. | "commit", "split into commits" |
| **`review-changes`** | Review the local diff/commits with three independent reviewers (CodeRabbit + Codex + Claude), verify the findings, fix what's worth fixing, summarize. | "review my changes", "run codex and coderabbit" |
| **`finish-feature`** | Commit → merge the branch back into its base → delete branch / remove worktree. **Local**, no PR. | "finish this feature", "merge back and clean up" |
| **`create-pr`** | Commit → push → open a GitHub PR → assign reviewers. **Remote review** path. | "create a PR", "open a pull request", "submit for review" |

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
- `review-lenses.md` — the eight review lenses (`bugs`, `impl`, `quality`, `architecture`,
  `tests`, `docs`, `comments`, `adversarial`) and which reviewer carries which.
- `finding-triage.md` — reconcile (dedupe + corroboration), verify (five verdicts + the
  materiality test), and the three checks on every fix.
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
 commit · review-changes · finish-feature · create-pr   ← SKILL.md (when & how)
   │   all link into
   ▼
 _shared/references/*.md   (safety · conventions · quality gate · worktree · branching
                            severity bar · lenses · finding triage
                            agent delegation · output discipline)
   +
 scripts/run-open.sh       (one mechanical step: open this run's directory)
   │   drive
   ▼
 git   +   gh (GitHub CLI)   +   wt (Worktrunk)
```

The skills are the single source of truth for the *workflow*; the underlying tools remain
the source of truth for the *operations*. The plugin never re-encodes git logic.

## Workflow Model
Work is modeled as a **feature** — edits that become commits and then get integrated:

```
edit → commit → review → finish
                          ├── finish-feature   (local merge, delete branch / worktree)
                          └── create-pr         (push, open PR, review remotely)
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
  create-pr/SKILL.md
  review-changes/SKILL.md
  finish-feature/SKILL.md
  _shared/            # shared references (README + references/*.md) — no SKILL.md
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
