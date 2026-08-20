# mkit

A **Claude Code plugin** — a cohesive kit of agent coding-workflow skills that take work from
**edits → committed → reviewed → integrated**. Composition over replacement: the skills
orchestrate `git`, GitHub CLI (`gh`), Worktrunk (`wt`), and code-review tools
(CodeRabbit/Codex); they don't reimplement them.

Claude-only for now; other agents (Codex, opencode, …) are a later, thin packaging step.

## Skills

| Skill | Does |
|-------|------|
| `commit` | Inspect the tree, stage intentionally, split into logical Conventional Commits. |
| `review` | Review the local diff/commits with three independent reviewers (CodeRabbit + Codex + Claude), verify the findings, fix what's worth fixing, summarize. |
| `finish` | Commit → merge the branch back into its base → delete branch / remove worktree (local, no PR). |
| `pr` | Commit → push → open a GitHub PR → assign reviewers (remote review path). |

`_shared/` is the shared **references** bundle (git safety, Conventional Commits, quality
gate, worktree detection, branching) that the four skills link into — not a triggerable
skill. `scripts/` holds five helpers the skills call for the mechanical steps: opening a run
directory, gathering the starting facts, detecting and running the quality gate, and the
arithmetic over a review's findings.

## Install

```
/plugin marketplace add masterik/workflow_tool
/plugin install mkit@masterik
```

Nothing to build. The scripts need `git`, `bash`, `node` and `jq`; `rg`, `gh` and `wt` are
recommended — see [Prerequisites](PREREQUISITES.md).

## Testing the scripts

`tests/run.sh` runs the script layer's own suite: `node --test` over `findings.mjs`, `bats`
over the five shell scripts. Dev-only — see [Prerequisites](PREREQUISITES.md#dev-only--running-tests).

## Docs

- [Concept](concept.md) — direction and design principles
- [Prerequisites](PREREQUISITES.md) — required tooling, setup, permission allowlist
