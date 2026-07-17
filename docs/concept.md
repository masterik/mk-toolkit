# mkit — Concept

## Summary
mkit is a **Claude Code plugin** — a cohesive **kit of agent coding-workflow skills**. It
gives Claude a safe, repeatable way to take work from **edits → committed → reviewed →
integrated**: stage and commit cleanly, review the diff, then either merge back locally or
open a PR — without re-deriving fragile `git` + `gh` + `wt` command sequences on every task.

It's a *workflow* toolkit, not just a git one: `review-changes` drives CodeRabbit/Codex,
`create-pr` drives GitHub, and `finish-feature` handles worktree cleanup — the parts of the
dev loop the agent runs, git-centric but not git-limited.

There is no CLI, no runtime, and nothing to install into the repo's toolchain. The plugin is
just **knowledge + procedure**: each skill tells Claude *when* it applies and *how* to drive
the underlying tools, with the safety rules that keep destructive steps from firing by
accident. The agent is the interface; the skills are the muscle memory.

**Claude-only for now.** Other agents (Codex, opencode, …) are a later concern — the skills
are plain Markdown, so support for another agent is a thin packaging step, not a rewrite.

## Design Principles
- **Skills, not a binary:** the workflow lives in Markdown the agent reads, not in code it
  executes. Nothing to build, version, or keep on `PATH`.
- **Composition over replacement:** orchestrate `git`, GitHub CLI (`gh`), and Worktrunk
  (`wt`); never reimplement what they already do well.
- **Safe by default:** irreversible actions (force-push, branch delete, history rewrite,
  hook-skipping) are gated by an explicit safety protocol the skills share.
- **DRY via shared references:** the skills link into one `git-flow/references/` bundle
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
| **`review-changes`** | Review the local diff/commits with CodeRabbit + Codex, fix what's worth fixing, summarize. | "review my changes", "run codex and coderabbit" |
| **`finish-feature`** | Commit → merge the branch back into its base → delete branch / remove worktree. **Local**, no PR. | "finish this feature", "merge back and clean up" |
| **`create-pr`** | Commit → push → open a GitHub PR → assign reviewers. **Remote review** path. | "create a PR", "open a pull request", "submit for review" |

### Shared references — `skills/git-flow/`
`git-flow/` is **not** a triggerable skill (it has no `SKILL.md`); it is the shared library
the four skills link into via `../git-flow/references/…`:

- `git-safety.md` — the non-negotiable git safety protocol (no force-push, no config edits,
  no AI attribution, don't skip hooks, …).
- `conventional-commits.md` — commit message format, type table, scope detection.
- `quality-gate.md` — how to **detect** (not hardcode) the repo's fast check + full
  lint/test/build gate.
- `worktree.md` — detect the worktree origin (Worktrunk `wt` / Claude Code
  `.claude/worktrees/` / plain `git worktree`) and clean up correctly.
- `branching.md` — the default branch model to assume when a repo doesn't document its own.

## Architecture
```
 Claude Code
   │   loads plugin skills (via .claude-plugin/plugin.json)
   ▼
 commit · review-changes · finish-feature · create-pr   ← SKILL.md (when & how)
   │   all link into
   ▼
 git-flow/references/*.md   (safety · conventions · quality gate · worktree · branching)
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
  git-flow/            # shared references (README + references/*.md) — no SKILL.md
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
