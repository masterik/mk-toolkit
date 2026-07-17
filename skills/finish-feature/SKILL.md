---
name: finish-feature
description: >-
  Finish a feature branch locally: commit any remaining work, merge the branch back into its base (usually main), then
  delete the feature branch and remove its worktree. Use when the user says "finish this feature", "wrap up and merge
  back", "merge into main and clean up", "finish the branch", "done with this feature", or "/finish-feature". This is the
  local merge-and-cleanup path — for a reviewed remote merge use create-pr instead.
model: sonnet
---

# Finish a feature (local merge + cleanup)

Part of the **git-flow** bundle. This is the "merge it back myself" path: no remote PR, no reviewers — commit, fast
integrate into the base branch, and tear down the feature branch/worktree. For the review path, use `create-pr`.

Shared references (read the ones you need): `../git-flow/references/worktree.md`,
`../git-flow/references/quality-gate.md`, `../git-flow/references/conventional-commits.md`,
`../git-flow/references/git-safety.md`, `../git-flow/references/branching.md`.

## When NOT to use this

- The work needs review → use `create-pr`.
- You only want to commit, not merge → use `commit`.
- The base branch is protected / the team merges only via PR → use `create-pr`.

## Preconditions

1. Determine the **current branch** and confirm it is a feature/bugfix branch, not the base itself
   (`git branch --show-current`). If it's `main`/`master`, stop — there is nothing to finish.
2. Determine the **base branch** to merge into — default `main`; the branch this feature was cut from. If ambiguous,
   ask. See `../git-flow/references/branching.md` for the common branch model and how to detect a repo's own.
3. Detect the **worktree context** (worktrunk `wt` / Claude Code `.claude/worktrees/` / plain git / none) per
   `../git-flow/references/worktree.md`. This decides the cleanup path.

## Workflow

### 1. Commit remaining work

If the tree is dirty, run the `commit` workflow first (stage intentionally, logical commits, Conventional Commit
messages). The tree must be clean before merging. Never merge with uncommitted changes.

### 2. Verify before merging

Run the project **full quality gate** (`../git-flow/references/quality-gate.md`) — lint → test → build, or the repo's
equivalent, stop on first failure. A local merge skips review, so this gate is the only safety net. Report any failure
and do not merge past it without an explicit user OK.

### 3. Show the plan and confirm

Before merging or deleting anything, print a one-screen summary and get a go-ahead (unless the user already said
"finish and clean up" / gave standing authorization):

```
Finish feature:  <feature-branch>
Merge into:       <base-branch>
Worktree:         <origin: worktrunk | claude-code | plain-git | none> @ <path>
Commits to merge: <git log --oneline base..HEAD>
After merge:      delete branch <feature-branch> + remove worktree (if any)
```

### 4. Merge back + clean up — by worktree origin

**worktrunk (`wt`)** — delegate; it squash-rebases, fast-forwards the base, and removes the worktree in one step, firing
the user's configured hooks:

```bash
wt merge <base>        # add -y only if non-interactive completion is authorized
```

`wt merge` removes the worktree by default; the branch is deleted as part of the flow. Use `--no-remove` /
`--no-squash` / `--no-ff` only if the user wants to override their config.

**Claude Code worktree (`.claude/worktrees/`)** — merge into the base, then hand back via the **ExitWorktree** tool
(`action: "remove"`). Do not `git worktree remove` the harness's own worktree from inside it. If you cannot fast-forward
the base (it is checked out in the primary worktree), merge from the primary worktree per the reference doc.

**Plain git worktree** — merge, then:

```bash
git -C <primary-worktree-path> merge --ff-only <feature-branch>   # or a real merge if ff isn't possible
git worktree remove <feature-worktree-path>
git branch -d <feature-branch>
git worktree prune
```

**No worktree (single checkout)** — standard local merge:

```bash
git switch <base>
git merge --no-ff <feature-branch>      # or --ff-only if you want a linear history and it fast-forwards
git branch -d <feature-branch>
```

Update the base against the remote first (`git fetch` / `git pull --ff-only <base>`) when a remote exists, so you merge
onto current base.

### 5. Verify the cleanup

- `git worktree list` — the feature worktree is gone (if there was one).
- `git branch` — the feature branch is gone.
- `git branch --show-current` / `pwd` — you're now on the base branch (or back in the primary checkout).
- Confirm the base branch contains the feature commits (`git log --oneline -5`).

## Deliverable

- Confirmation of what merged into what, the resulting base HEAD, and that the branch + worktree were removed.
- If anything was left in place on purpose (e.g. unmerged commits, dirty tree, user declined a delete), say so explicitly.

## Git safety

Follow `../git-flow/references/git-safety.md`. Specifically: never delete an unmerged branch or force-remove a dirty
worktree without an explicit request; never push to or force-update the base; confirm branch/tree state before each
irreversible step.
