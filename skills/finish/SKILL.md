---
name: finish
description: >-
  Commit, merge a feature branch into base (usually main), delete the branch, remove the worktree. Trigger on
  "finish this feature", "merge back and clean up", "merge into main and clean up", "done with this feature".
  Local path — reviewed remote merge is pr.
model: sonnet
---

# Finish a feature (local merge + cleanup)

Part of the **mkit** bundle. The "merge it back myself" path: no remote PR, no reviewers — commit, integrate
into the base branch, tear down the branch/worktree. For the review path use `pr`.

References, read the ones a step calls for: `../_shared/references/worktree.md`,
`../_shared/references/quality-gate.md`, `../_shared/references/conventional-commits.md`,
`../_shared/references/git-safety.md`, `../_shared/references/branching.md`,
`../_shared/references/output-discipline.md`.

## When NOT to use this

- Work needs review → `pr`.
- You only want to commit → `commit`.
- Base branch is protected, or the team merges only via PR → `pr`.

## Preconditions

**One call**, which also opens this run's directory (`../_shared/references/output-discipline.md`):

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh finish --base <base>
```

Keep the `run=` literal; this file writes it as `<run-dir>`. There is no `$RUN_DIR` — a shell variable does
not survive to the next Bash call — and re-running the script opens a second directory instead of returning
the first. Check three things in what it printed:

1. **Current branch** — `branch=` must be a feature/bugfix branch, not `default_branch=`. On `main`/`master`:
   stop, nothing to finish.
2. **Base branch** — the branch this feature was cut from, usually `default_branch=`. Ask if ambiguous;
   `../_shared/references/branching.md` covers detecting a repo's own model. Pass it as `--base` so
   `commits:`, `base_stat` and `ff_from_base` come back with everything else.
3. **Cleanup path** — `cleanup_path=` is `exit-worktree` (the harness's own worktree) · `wt` (linked,
   worktrunk present) · `git-worktree` (linked, no worktrunk) · `none` (primary checkout). It decides step 4.
   Read `../_shared/references/worktree.md` there, not now.

## Workflow

### 1. Commit remaining work

Dirty tree → run `commit` first (stage intentionally, logical commits, Conventional Commit messages). The
tree must be clean before merging. **Never merge with uncommitted changes.**

### 2. Verify before merging

Run the **full quality gate** (`../_shared/references/quality-gate.md`): lint → test → build or the repo's
equivalent, stop on first failure. A local merge skips review, so this gate is the only safety net. Report any
failure; do not merge past it without an explicit user OK.

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gate-detect.sh
${CLAUDE_PLUGIN_ROOT}/scripts/gate-run.sh <run-dir> --chain 'lint=<cmd>' 'test=<cmd>' 'build=<cmd>'
```

One line per passing step; on failure the step, the exit code, the grepped failures and the tail
(`../_shared/references/output-discipline.md`). A failing suite is thousands of lines, none of which change
the decision ("fix it or get an explicit OK").

`gate-detect.sh` also reports what the gate ledger already proved (`full_cache=`, pipe-parallel with
`full=`). **This is the strictest consumer in the bundle**, because a local merge skips review and this gate
is the only safety net: consume a step only on an exact command match, a matching fingerprint and an age
inside the bound — per step, never a whole chain at once — and print `cached (Nm ago, exit=0)` on that
step's own line, with the verdict naming how many were cached. `gate=ok` for a step that did not run is not
an acceptable report here. `failed` means the tree was red on this exact content: say so before starting,
then run the step anyway — the environment may have moved since.

Delegate the diagnosis only when that verdict is not enough (`../_shared/references/quality-gate.md`, "when a
step fails") and give the user what it returns — what failed, probable cause, whether the change caused it,
suggested fix. The log stays on disk.

### 3. Show the plan and confirm

Before merging or deleting anything, print a one-screen summary and get a go-ahead (unless the user already
said "finish and clean up" or gave standing authorization). Step 0's `commits:` block predates step 1, so if
step 1 committed, re-read the log (`git log --oneline <base>..HEAD`) first — approving a plan that omits the
commits the merge actually carries is worse than asking twice:

```
Finish feature:  <feature-branch>
Merge into:       <base-branch>
Worktree:         <cleanup_path> @ <toplevel>
Commits to merge: <the commits: block, re-read if step 1 committed>
After merge:      delete branch <feature-branch> + remove worktree (if any)
```

### 4. Merge back + clean up — by `cleanup_path`

**`wt`** — delegate; it squash-rebases, fast-forwards the base and removes the worktree in one step, firing
the user's hooks:

```bash
wt merge <base>        # add -y only if non-interactive completion is authorized
```

`wt merge` removes the worktree by default and deletes the branch as part of the flow. Use `--no-remove` /
`--no-squash` / `--no-ff` only to override the user's config on request.

**`exit-worktree`** — merge into the base, then hand back via the **ExitWorktree** tool
(`action: "remove"`). Never `git worktree remove` the harness's own worktree from inside it. If the base
cannot be fast-forwarded (`ff_from_base=no`, or it is checked out in the primary worktree), merge from
`primary=` per `../_shared/references/worktree.md`.

**`git-worktree`**:

```bash
git -C <primary> merge --ff-only <feature-branch>   # or a real merge if ff isn't possible
git worktree remove <toplevel>
git branch -d <feature-branch>
git worktree prune
```

**`none` (single checkout)**:

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

Prune with `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh --prune` on the way out, folded into step 5's
verification call.

- The gate verdict, naming any step served from the ledger as `cached` and how old that proof was.
- What merged into what, the resulting base HEAD, and that branch + worktree were removed.
- Anything left in place on purpose (unmerged commits, dirty tree, a delete the user declined) — say so
  explicitly.

## Git safety

Follow `../_shared/references/git-safety.md`: never delete an unmerged branch or force-remove a dirty worktree
without an explicit request, never push to or force-update the base, confirm branch/tree state before each
irreversible step.
