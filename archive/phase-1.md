# ForgeZ — Phase 1

## Overview
Phase 1 adds worktree support to the Phase 0 CLI. Every feature can now optionally live in a parallel filesystem worktree, enabling true multi-feature parallelism without stashing or switching branches. Phase 0 commands continue to work unchanged; worktrees are opt-in via `--worktree`.

---

## New Use Cases

See [`docs/use-cases.md`](use-cases.md) — Phase 1 enables the worktree variants: **UC3b** and **UC4** with `--worktree`, and extends UC5 with `fz start --on <branch> --worktree`.

---

## Command Changes

### Modified Commands

| Command | Phase 0 | Phase 1 addition |
|---------|---------|-----------------|
| `fz start <name>` | Creates branch | `--worktree` flag: also creates a parallel worktree and switches to it |
| `fz merge [<name>]` | Merges branch, deletes it | Also removes worktree if present |
| `fz close [<name>]` | Deletes branch | Also removes worktree if present |
| `fz drop [<name>]` | Deletes branch | Also removes worktree if present |
| `fz list` | Lists branches + PR status | Also shows worktree path |

No new top-level commands are introduced in Phase 1.

---

## Detailed Behaviour Changes

### `fz start --worktree`
1. All Phase 0 `fz start` steps apply.
2. Additionally: use `wt` (Worktrunk) to create a worktree at `../<repo>-<name>`.
3. Switch terminal to the new worktree directory.
4. Store worktree path in feature metadata.

### `fz merge` (worktree-aware)
After the merge succeeds:
- If a worktree is registered for the feature, remove it (`wt remove` or `git worktree remove`).
- Then delete the branch as before.

### `fz close` (worktree-aware)
After branch deletion:
- If a worktree is registered, remove it.

### `fz drop` (worktree-aware)
After branch deletion:
- If a worktree is registered, remove it.

---

## Feature Metadata

Phase 1 adds a `worktree` field to the existing metadata schema:

```json
{
  "features": [
    {
      "name": "feature-b",
      "branch": "feature-b",
      "base": "feature-a",
      "worktree": "../project-feature-b",
      "pr": 42
    }
  ]
}
```

`worktree` is omitted (or `null`) for features created without `--worktree`.

---

## Tool Integrations Added

| Tool | Used for |
|------|----------|
| **Worktrunk (`wt`)** | Worktree creation and removal via `fz start --worktree` |

All Phase 0 integrations (git, gh, Claude Code, serie) remain unchanged.

---

## Phased delivery
1. `fz start --worktree` — worktree creation
2. `fz merge` / `fz close` / `fz drop` worktree cleanup
3. `fz list` worktree path column
