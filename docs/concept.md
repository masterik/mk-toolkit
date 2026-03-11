# ForgeZ — Concept

## Summary
`forge-zed` (ForgeZ) is a personal workflow helper that wraps git, GitHub CLI, and Claude Code into a single fast CLI. The goal: zero friction from idea to branch to PR, with support for the ways real work actually happens — commits on main before a branch exists, stacked features, parallel worktrees.

## Design Principles
- **Personal-first utility:** optimize for your workflow before generalization.
- **Composition over replacement:** orchestrate strong existing tools instead of rebuilding them.
- **Real workflows first:** support escape hatches and retroactive corrections, not just the happy path.
- **Local and parallel by default:** multiple active worktrees should be a first-class workflow.
- **Progressive enhancement:** shell wrapper first, then richer UI/runtime layers.

---

## Core Workflow Model

ForgeZ models work as **features** — named units of work that map to a branch, an optional worktree, and optionally a PR. A feature has a lifecycle:

```
start → work (commit, sync) → finish (local merge or PR) → close
```

The key design challenge is that features don't always start cleanly. ForgeZ handles real-world entry points: starting fresh, moving commits from main retroactively, parallel worktrees, and stacked branches.

---

## Phases

### Phase 0 — Personal CLI (no worktrees)
Single-workspace workflow: branches, AI commits, AI PRs, stacked features, visibility. No worktree management. See [phase-0.md](./phase-0.md).

### Phase 1 — Worktree support
Adds parallel worktrees as an opt-in per feature. See [phase-1.md](./phase-1.md).

### Phase 2 — Local App (Desktop or TUI)
Expands from helper to workspace control plane: multiple projects and sessions, visual branch/stack/PR overview, one-click commit/push/PR, diff view, built-in terminal.

### Phase 3 — Integrated Runtime Surface
Deeper feedback loops: integrated browser preview, mobile simulator, and other surfaces driven by daily workflow bottlenecks.

---

See [use-cases.md](./use-cases.md) for the full use case reference.
