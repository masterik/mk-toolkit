# ForgeZ — Use Cases

This document describes the intended usage scenarios for `fz`. Use cases are divided into two categories: **feature lifecycle scenarios** (how a feature begins and ends) and **cross-cutting capabilities** (commands available throughout any workflow).

---

## Feature Lifecycle Scenarios

### UC1 — Commit directly to main, push (no branch, no PR)

The simplest case: small changes that don't warrant a branch. Work lands directly on `main` and is pushed without a PR.

```
fz commit          # AI-assisted commit (repeat as needed)
fz push            # push main
```

---

### UC2 — Retroactive branch: rescue commits from main

You've been committing on `main` and realize the work should be on a feature branch with a PR. `fz start --here` moves your unpushed commits onto a new branch and resets `main` back to `origin/main`.

```
fz start <name> --here   # move unpushed commits to new branch; prompts for confirmation
fz commit                # any additional commits
fz push
fz pr                    # open PR with AI-generated description
# after PR merges on GitHub:
fz close                 # switch to main, pull, delete feature branch
```

---

### UC3a — Feature branch in the main workspace (PR merge)

Standard feature branch workflow. Everything happens in the same directory; you switch branches as usual.

```
fz start <name>    # create branch, stay in current workspace
fz commit          # repeat as needed
fz push
fz pr
# after PR merges on GitHub:
fz close           # switch to main, pull, delete branch
```

---

### UC3b — Feature branch with worktree (PR merge)

Same as UC3a, but the feature lives in a parallel directory so you can context-switch between features without touching your main workspace.

```
fz start <name> --worktree   # create branch + worktree, cd into worktree
fz commit
fz push
fz pr
# after PR merges on GitHub:
fz close                     # remove worktree, switch to main, delete branch
```

---

### UC4 — Feature branch with worktree (local merge, no PR)

Like UC3b, but the merge happens locally — no PR is opened. Useful for personal or experimental work.

```
fz start <name> --worktree   # create branch + worktree, cd into worktree
fz commit
fz merge                     # rebase onto main, fast-forward merge locally,
                             # remove worktree and delete branch
```

---

### UC5 — Stacked features

A feature branch built on top of another feature branch (not main). Useful when feature B depends on feature A but you want to develop them in parallel.

The `--on` flag sets the parent; `fz restack` keeps the stack in sync when the parent changes.

```
# starting from feature-a (branch or worktree):
fz start feature-b --on feature-a          # stack feature-b on feature-a
fz start feature-b --on feature-a --worktree  # same, with a worktree

# if feature-a is updated (new commits, rebased):
fz restack                 # rebase current branch onto its updated parent
fz restack --all           # rebase the entire stack in dependency order

# close via PR (bottom-up):
fz pr --base feature-a     # PR for feature-b targeting feature-a
# after feature-a merges, feature-b's base becomes main automatically
fz close feature-b

# or close via local merge (bottom-up):
fz merge feature-a
fz merge feature-b
```

---

## Cross-Cutting Capabilities

These apply throughout any of the scenarios above.

### Sync with upstream
Keep the current branch up to date with its base (main or a parent feature branch).

```
fz restack             # rebase current branch onto origin/<base>
fz restack --all       # rebase entire stack bottom-up
```

### Merge — PR or local
Two paths to close a feature:

| Path | Command | When to use |
|------|---------|-------------|
| PR on GitHub | `fz pr` → merge on GitHub → `fz close` | Collaborative work, code review |
| Local merge | `fz merge` | Personal/solo work, no review needed |

### Drop a feature at any time
Abandon a branch (and worktree if present) without merging. Prompts for confirmation.

```
fz drop [<name>]
```

### List all active features
Shows branch name, worktree path (if any), upstream tracking status, and PR number + state.

```
fz list
```

### Current feature status
Shows uncommitted changes, ahead/behind upstream, and PR link + state.

```
fz status
```

### AI-assisted commit
Generates a conventional commit message from staged changes using Claude.

```
fz commit
```

### AI-assisted PR
Generates a PR title and body from commits and diff using Claude, then opens the PR via `gh`.

```
fz pr [--base <branch>]
```

### Commit log and history
View commit history, optionally across all branches with a visual graph.

```
fz log
fz log --graph     # visual branch tree (via serie / git-graph)
fz log --all       # include all branches
```
