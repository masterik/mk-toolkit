# flowkit

A **Claude Code plugin** — a cohesive kit of agent coding-workflow skills that take work from
**edits → committed → reviewed → integrated**. Composition over replacement: the skills
orchestrate `git`, GitHub CLI (`gh`), Worktrunk (`wt`), and code-review tools
(CodeRabbit/Codex); they don't reimplement them.

Claude-only for now; other agents (Codex, opencode, …) are a later, thin packaging step.

## Skills

| Skill | Does |
|-------|------|
| `commit` | Inspect the tree, stage intentionally, split into logical Conventional Commits. |
| `review-changes` | Review the local diff/commits with CodeRabbit + Codex, fix what's worth fixing, summarize. |
| `finish-feature` | Commit → merge the branch back into its base → delete branch / remove worktree (local, no PR). |
| `create-pr` | Commit → push → open a GitHub PR → assign reviewers (remote review path). |

`git-flow/` is a shared **references** bundle (git safety, Conventional Commits, quality
gate, worktree detection, branching) that the four skills link into — not a triggerable
skill.

## Install

```
/plugin marketplace add masterik/workflow_tool
/plugin install flowkit@masterik
```

## Docs

- [Concept](docs/concept.md)
