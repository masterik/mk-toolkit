# ForgeZ

A **Claude Code plugin** — a cohesive set of git feature-workflow skills that take work from
**edits → committed → integrated**. Composition over replacement: the skills orchestrate
`git`, GitHub CLI (`gh`), and Worktrunk (`wt`); they don't reimplement git.

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
/plugin install forgez@forgez
```

## Docs

- [Concept](docs/concept.md)
