---
name: finish-feature
description: >-
  Commit, merge a feature branch into base (usually main), delete the branch, remove the worktree. Trigger on
  "finish this feature", "merge back and clean up", "merge into main and clean up", "done with this feature".
  Local path — reviewed remote merge is create-pr.
model: sonnet
---

# Finish a feature (local merge + cleanup)

Part of the **mkit** bundle. The "merge it back myself" path: no remote PR, no reviewers — commit, integrate
into the base branch, tear down the branch/worktree. For the review path use `create-pr`.

References, read the ones a step calls for: `../_shared/references/worktree.md`,
`../_shared/references/quality-gate.md`, `../_shared/references/conventional-commits.md`,
`../_shared/references/git-safety.md`, `../_shared/references/branching.md`,
`../_shared/references/output-discipline.md`.

## When NOT to use this

- Work needs review → `create-pr`.
- You only want to commit → `commit`.
- Base branch is protected, or the team merges only via PR → `create-pr`.

## Preconditions

**One call** — these are independent read-only probes, and the run directory depends on none of them
(`../_shared/references/output-discipline.md`):

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh finish-feature
git branch --show-current && git status --short
git rev-parse --git-dir --git-common-dir --show-toplevel
git worktree list
command -v wt >/dev/null && echo "wt: yes" || echo "wt: no"
```

The script prints one absolute path. **Reuse that literal**; this file writes it as `<run-dir>`. There is no
`$RUN_DIR` — a shell variable does not survive to the next Bash call — and re-running it opens a second
directory instead of returning the first.

1. **Current branch** — a feature/bugfix branch, not the base. On `main`/`master`: stop, nothing to finish.
2. **Base branch** — default `main`; the branch this feature was cut from. Ask if ambiguous;
   `../_shared/references/branching.md` covers detecting a repo's own model.
3. **Worktree context** — worktrunk `wt` / Claude Code `.claude/worktrees/` / plain git / none. Decides the
   cleanup path. `--git-dir` ≠ `--git-common-dir` → a linked worktree; a root path containing
   `/.claude/worktrees/` → the harness's own; `wt` on `PATH` plus a worktrunk-shaped layout → worktrunk. Read
   `../_shared/references/worktree.md` at step 4, not now.

## Workflow

### 1. Commit remaining work

Dirty tree → run `commit` first (stage intentionally, logical commits, Conventional Commit messages). The
tree must be clean before merging. **Never merge with uncommitted changes.**

### 2. Verify before merging

Run the **full quality gate** (`../_shared/references/quality-gate.md`): lint → test → build or the repo's
equivalent, stop on first failure. A local merge skips review, so this gate is the only safety net. Report any
failure; do not merge past it without an explicit user OK.

Redirect each step to its own log and keep output out of context — one line per passing step; on failure the
step, the exit code and the tail (`../_shared/references/output-discipline.md`). A failing suite is thousands
of lines, none of which change the decision ("fix it or get an explicit OK").

On a long failure log, delegate the diagnosis (`../_shared/references/quality-gate.md`, "when a step fails")
and give the user its verdict — what failed, probable cause, whether the change caused it, suggested fix. The
log stays on disk.

### 3. Show the plan and confirm

Before merging or deleting anything, print a one-screen summary and get a go-ahead (unless the user already
said "finish and clean up" or gave standing authorization):

```
Finish feature:  <feature-branch>
Merge into:       <base-branch>
Worktree:         <origin: worktrunk | claude-code | plain-git | none> @ <path>
Commits to merge: <git log --oneline base..HEAD>
After merge:      delete branch <feature-branch> + remove worktree (if any)
```

### 4. Merge back + clean up — by worktree origin

**worktrunk (`wt`)** — delegate; it squash-rebases, fast-forwards the base and removes the worktree in one
step, firing the user's hooks:

```bash
wt merge <base>        # add -y only if non-interactive completion is authorized
```

`wt merge` removes the worktree by default and deletes the branch as part of the flow. Use `--no-remove` /
`--no-squash` / `--no-ff` only to override the user's config on request.

**Claude Code worktree (`.claude/worktrees/`)** — merge into the base, then hand back via the **ExitWorktree**
tool (`action: "remove"`). Never `git worktree remove` the harness's own worktree from inside it. If the base
cannot be fast-forwarded (checked out in the primary worktree), merge from there per
`../_shared/references/worktree.md`.

**Plain git worktree**:

```bash
git -C <primary-worktree-path> merge --ff-only <feature-branch>   # or a real merge if ff isn't possible
git worktree remove <feature-worktree-path>
git branch -d <feature-branch>
git worktree prune
```

**No worktree (single checkout)**:

```bash
git switch <base>
git merge --no-ff <feature-branch>      # or --ff-only for linear history, if it fast-forwards
git branch -d <feature-branch>
```

Update the base against the remote first (`git fetch` / `git pull --ff-only <base>`) when one exists, so you
merge onto current base.

### 5. Verify the cleanup

- `git worktree list` — the feature worktree is gone (if there was one).
- `git branch` — the feature branch is gone.
- `git branch --show-current` / `pwd` — you are on the base branch (or back in the primary checkout).
- `git log --oneline -5` — the base contains the feature commits.

## Deliverable

- What merged into what, the resulting base HEAD, and that branch + worktree were removed.
- Anything left in place on purpose (unmerged commits, dirty tree, a delete the user declined) — say so
  explicitly.

## Git safety

Follow `../_shared/references/git-safety.md`: never delete an unmerged branch or force-remove a dirty worktree
without an explicit request, never push to or force-update the base, confirm branch/tree state before each
irreversible step.
