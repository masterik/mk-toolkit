# ForgeZ — Concept

## Summary
`forge-zed` (ForgeZ) is a personal workflow helper that wraps Worktrunk, GitHub CLI, git, and Claude Code into a single fast CLI. The goal: zero friction from idea to branch to PR, with escape hatches for the ways real work actually happens (commits on main before a branch exists, stacked features, parallel worktrees).

## Design Principles
- **Personal-first utility:** optimize for your workflow before generalization.
- **Composition over replacement:** orchestrate strong existing tools instead of rebuilding them.
- **Real workflows first:** support escape hatches and retroactive corrections, not just the happy path.
- **Local and parallel by default:** multiple active worktrees should be normal.
- **Progressive enhancement:** shell wrapper first, then richer UI/runtime layers.
- **Fast execution loops:** minimize friction from idea to branch to PR.

---

## Core Workflow Model

ForgeZ models work as **features** — named units of work that map to a branch, an optional worktree, and optionally a PR. A feature has a lifecycle:

```
start → work (commit, sync) → finish (local merge or PR) → archive
```

The key design challenge is that features don't always start cleanly. ForgeZ handles these real-world entry points:

### Entry Points

| Scenario | How it works |
|----------|--------------|
| Start fresh on a new branch | `fz start <name>` — creates branch, stays in current workspace |
| Start with a parallel worktree | `fz start <name> --worktree` — creates branch + worktree |
| Already on main with unpushed commits | `fz start <name> --here` — moves commits to a new branch retroactively (with confirmation) |
| Already on main, just want to push | Work directly, `fz push`, no branch needed |
| Create a branch stacked on current | `fz start <name> --on <parent>` |

### Finish Options

| Scenario | How it works |
|----------|--------------|
| Local merge (no PR) | `fz merge` — rebases and merges locally, removes branch |
| GitHub PR | `fz pr` → merge on GitHub → `fz close` |
| Drop feature entirely | `fz drop` — discards branch and worktree without merging |

---

## Supported Branching Strategies

ForgeZ does not enforce a strategy — it supports all of them:

| Strategy | Key commands |
|----------|-------------|
| **Trunk-Based / GitHub Flow** | Work on main or short-lived branches, `fz push` or quick `fz start` → `fz pr` |
| **Feature Branching (local merge)** | `fz start` → `fz finish` (no PR) |
| **Feature Branching (with PR)** | `fz start` → `fz pr` → `fz archive` |
| **Stacked PRs** | `fz start --on <parent>` for each layer; `fz restack` keeps stack in sync |
| **GitFlow** | `fz start` from develop/release; `fz finish` targets correct base |
| **Rescue / retroactive** | `fz rescue` moves commits from main to a new branch |

---

## Stack Awareness

Stacked features are first-class. When a parent branch changes (e.g., merged or rebased), ForgeZ can sync all children in dependency order via `fz sync`. The feature graph is maintained locally alongside branch metadata.

---

## Phase 1: Personal CLI Wrapper

Phase 1 is intentionally narrow — a wrapper/helper for a single developer's workflow. No server, no UI, just orchestration.

**Tooling baseline:**
- **git** — base operations
- **Worktrunk (`wt`)** — branch and worktree lifecycle
- **GitHub CLI (`gh`)** — remote operations (push, PR, status)
- **Claude Code** — AI-assisted commits and PR descriptions
- **serie / git-graph** — visual log output

See [phase-1.md](./phase-1.md) for command specification.

---

## Phase 2: Local App (Desktop or TUI)

Phase 2 expands from helper to workspace control plane, similar in spirit to the Codex macOS app but centered on Claude Code.

**Core capabilities:**
- Multiple projects and sessions.
- Worktree-aware session management.
- Visual branch/stack/PR overview.
- One-click commit, push, and create PR.
- Diff/changes view and built-in terminal.
- Local execution and parallel runs.

**Key architectural decision:** Terminal-driven UX vs. API/SDK-driven UX (Codex-style).

---

## Phase 3: Integrated Runtime Surface

Phase 3 adds deeper tooling for development feedback loops:
- Integrated browser preview.
- Integrated mobile preview/simulator.
- Additional surfaces driven by daily workflow bottlenecks.
