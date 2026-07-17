# ForgeZ — Phase 0

## Overview
Phase 0 delivers a personal CLI (`fz`) that orchestrates git, GitHub CLI, and Claude Code into a single fast workflow helper — covering the full feature lifecycle without worktrees. All work happens in a single workspace (the main checkout). Worktree support is Phase 1.

---

## Use Cases

See [`docs/use-cases.md`](use-cases.md) for the full use case reference. Phase 0 covers UC1–UC5 excluding worktree variants (UC3b, UC4 with `--worktree`).

---

## Command Reference

### Feature Lifecycle

| Command | Description |
|---------|-------------|
| `fz start <name> [--on <branch>] [--here]` | Create feature branch, stay in current workspace. `--on` stacks onto a parent branch. `--here` moves unpushed commits from current branch to this new feature (prompts for confirmation). |
| `fz merge [<name>]` | Rebase onto base, merge locally, delete branch. Defaults to current feature. |
| `fz close [<name>]` | Clean up after a remote PR merge: switch to base, pull, delete local branch. Guards against unmerged branches. |
| `fz drop [<name>]` | Abandon feature — delete branch without merging. Prompts for confirmation. |

### Daily Workflow

| Command | Description |
|---------|-------------|
| `fz commit` | AI-assisted commit (delegates to Claude Code) |
| `fz push` | Push current branch; sets upstream on first push |
| `fz pr [--base <branch>]` | AI-assisted PR creation (delegates to Claude Code + `gh`). `--base` overrides default base branch. |
| `fz restack [--all]` | Rebase current branch onto its updated upstream base. `--all` walks the entire stack in dependency order. |

### Visibility

| Command | Description |
|---------|-------------|
| `fz list` | List all active features: branch name, remote tracking, PR number + status |
| `fz status` | Current feature: uncommitted changes, ahead/behind upstream, PR link and status |
| `fz log [--graph] [--all]` | Commit history. `--graph` uses serie/git-graph for visual tree. `--all` includes other branches. |

---

## Closing a Feature — Decision Guide

```
Is the branch merged remotely (PR merged on GitHub)?
  YES → fz close      (clean up after remote merge)
  NO  → Do you want to merge it now?
          YES, locally → fz merge   (rebase + merge into base, clean up)
          NO, abandon  → fz drop    (delete without merging, destructive)
```

`merge` = you are doing the merge now, locally.
`close` = the merge already happened (remotely), you are cleaning up.
`drop`  = throw it away without merging.

---

## Detailed Behaviour

### `fz start`
1. Determine base: `--on <branch>` if provided, else `main` (or configured trunk).
2. If `--here`: collect unpushed commits, create branch at HEAD, reset current branch to `origin/<current>`, prompt for confirmation before reset.
3. Create branch with `git checkout -b`.
4. Register feature in local metadata (name, base).

### `fz merge`
1. Guard: working tree must be clean.
2. `git fetch` + `git rebase origin/<base>` (or `<base>` if local-only).
3. Switch to base branch, fast-forward merge.
4. Delete feature branch.
5. Deregister feature from local metadata.

### `fz close`
1. Guard: verify branch is merged — checks `gh pr view --json state` if PR exists, else `git merge-base --is-ancestor`.
2. Switch to base branch; `git pull`.
3. Delete local branch.
4. Deregister feature.

### `fz drop`
1. Prompt: "This will delete branch <name> without merging. Continue? [y/N]"
2. Switch to base branch.
3. Delete branch.
4. Deregister feature.

### `fz restack`
- Single branch: `git fetch && git rebase origin/<base>` (or `<base>` if local).
- `--all`: walk the feature stack in topological order (parents before children when fetching, children before parents when rebasing), rebasing each onto its updated parent.

### `fz pr`
1. Guard: clean tree.
2. Auto-push if branch has no upstream yet.
3. Delegate to Claude Code for PR title/body generation.
4. `gh pr create` with generated content.

---

## Feature Metadata

ForgeZ maintains a lightweight local metadata file (`.fz/features.json` in the repo root, gitignored) to track:

```json
{
  "features": [
    {
      "name": "feature-b",
      "branch": "feature-b",
      "base": "feature-a",
      "pr": 42
    }
  ]
}
```

This enables `fz list`, stack ordering for `fz restack --all`, and correct base detection for `fz merge`/`fz close`.

---

## Implementation

### Stack
**Bun + openTUI.** The `list`, `status`, and `log` commands require structured output formatting and `gh` JSON parsing that becomes awkward in bash. openTUI provides the rendering layer for these views and establishes the foundation for Phase 1's worktree support without requiring a rewrite.

### Tool Integrations

| Tool | Used for |
|------|----------|
| **git** | All local branch/commit/rebase operations |
| **GitHub CLI (`gh`)** | Push, PR create, PR status, remote branch info |
| **Claude Code** | `fz commit` and `fz pr` content generation |
| **serie / git-graph** | `fz log --graph` visual tree output |

### Phased delivery
1. `fz commit`, `fz push`, `fz pr` — daily workflow core (delegates to existing tools)
2. `fz start`, `fz merge`, `fz close`, `fz drop` — feature lifecycle
3. `fz start --here` — retroactive branching
4. `fz list`, `fz status`, `fz log` — visibility layer
5. `fz restack`, `fz start --on` — stack support
