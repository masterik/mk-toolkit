---
name: finish
description: >-
  Commit, merge a feature branch into base (usually main), delete the branch, remove the worktree. Merges an
  existing open PR on GitHub if one exists for the branch, otherwise merges locally. Trigger on "finish this
  feature", "merge back and clean up", "merge into main and clean up", "done with this feature". For opening a
  new PR for review, use pr instead.
model: sonnet
---

# Finish a feature (merge + cleanup)

Part of the **mkit** bundle. The "wrap this branch up" path: commit, integrate into the base branch, tear down
the branch/worktree. If an open PR already exists for this branch, merge *that* — on GitHub, respecting
whatever checks/reviews it requires — rather than merging locally and leaving the PR stranded. With no PR, it
merges locally, no remote round trip. Either way this skill never *opens* a PR for review — that's `pr`.

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
${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh finish --base <base> --gh
```

Keep the `run=` literal; this file writes it as `<run-dir>`. There is no `$RUN_DIR` — a shell variable does
not survive to the next Bash call — and re-running the script opens a second directory instead of returning
the first. Check four things in what it printed:

1. **Current branch** — `branch=` must be a feature/bugfix branch, not `default_branch=`. On `main`/`master`:
   stop, nothing to finish.
2. **Base branch** — the branch this feature was cut from, usually `default_branch=`. Ask if ambiguous;
   `../_shared/references/branching.md` covers detecting a repo's own model. Pass it as `--base` so
   `commits:`, `base_stat` and `ff_from_base` come back with everything else.
3. **Cleanup path** — `cleanup_path=` is `exit-worktree` (the harness's own worktree) · `wt` (linked,
   worktrunk present) · `git-worktree` (linked, no worktrunk) · `none` (primary checkout). It decides step 4.
   Read `../_shared/references/worktree.md` there, not now.
4. **Existing PR** — `pr=` is a URL only when one exists, alongside `pr_state=OPEN|CLOSED|MERGED` and
   `pr_draft=true|false`. Sentinels (`none`, `gh-missing`, `jq-missing`, `gh-unauthenticated`, `no-remote`)
   mean there is nothing to merge remotely — take the **local merge path**. `pr_state=OPEN` (and not
   draft) redirects step 4 to the **PR merge path** instead: this branch is merged on GitHub, not with a
   local `git merge`. Treat `CLOSED`/`MERGED` like no PR for routing purposes, but mention it — a merged
   PR with the branch still around usually just needs cleanup; a closed one may mean the user changed
   their mind, worth a check before merging anything. `pr_draft=true` also routes to the local path unless
   the user asks to mark it ready first (`gh pr ready <url>`) — GitHub refuses to merge a draft.

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
commits the merge actually carries is worse than asking twice.

**Local path:**

```
Finish feature:  <feature-branch>
Merge into:       <base-branch>
Worktree:         <cleanup_path> @ <toplevel>
Commits to merge: <the commits: block, re-read if step 1 committed>
After merge:      delete branch <feature-branch> + remove worktree (if any)
```

**PR path** — step 4's first two items (push, then pick the merge method) run *before* this: fill this
block in with the method just picked, get the go-ahead, and only then continue with the rest of step 4:

```
Finish feature:  <feature-branch>
Merge via:        GitHub PR <pr-url> (<squash|merge|rebase>)
Base:             <base-branch>
Worktree:         <cleanup_path> @ <toplevel>
Commits to merge: <the commits: block, re-read if step 1 committed>
After merge:      delete branch <feature-branch> (local + remote) + remove worktree (if any);
                   switch to <base-branch> and pull
```

### 4. Merge back + clean up

**PR path** (`pr=<url>`, `pr_state=OPEN`, not draft) — merge on GitHub, then sync locally:

1. **Push anything step 1 committed.** `gh pr merge` merges what's on GitHub, not local state:
   `pushed=no` → `git push -u origin <feature-branch>`; otherwise plain `git push` if `ahead=` > 0.
2. **Pick the merge method.** `gh repo view --json mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed`
   — if exactly one is allowed, use it. If more than one, ask (default suggestion: squash). Never pass
   `--admin` (bypasses branch protection / required reviews) unless the user explicitly asks for it.

   **Stop here and show step 3's PR-path confirmation block, filled in with the method just picked. Get
   the go-ahead before continuing** — nothing below this point runs without it.
3. **Merge:**
   ```bash
   gh pr merge <pr-url> --squash --delete-branch   # or --merge / --rebase, whichever the previous item picked
   ```
   `--delete-branch` deletes the **local branch too, not just remote** — and if the feature branch is
   checked out right here with nothing else pinning it, `gh` switches this checkout to `<base>` itself
   first. Item 5 below checks before deleting for exactly this reason. If `gh pr merge` refuses (failing
   checks, missing required review, merge conflict), report exactly what it said — do not fall back to a
   local merge to route around a block GitHub is enforcing on purpose.
4. **Sync the local base.** If `<base>` is already checked out elsewhere (primary worktree, from a linked
   one), work from `primary=` instead — same rule as the local path's worktree.md note:
   ```bash
   git switch <base>
   git pull        # or: git fetch <remote> && git merge --ff-only <remote>/<base>
   ```
5. **Confirm the merge actually landed, then remove the local vestiges.** `gh pr merge` returning success
   does not always mean *merged*: on a branch that requires a merge queue it means "enqueued" (or
   "auto-merge enabled" if required checks hadn't passed yet), and the real merge can still fail later — a
   check regresses, a conflict, a dequeue. Force-deleting the only copy of this branch on that signal alone
   risks losing the work. Confirm first:
   ```bash
   gh pr view <pr-url> --json state,mergeStateStatus
   ```
   Proceed only on `state=MERGED`. Anything else (still `OPEN`, queued or auto-merge-enabled) — stop, tell
   the user the PR hasn't actually merged yet, and leave the branch/worktree in place; re-run this item once
   it lands.

   Once confirmed, remove by `cleanup_path`. Every branch below uses a *forced* delete: a squash or rebase
   merge produces a commit GitHub knows is merged but whose hash your local branch never reaches, so the
   ordinary ancestry check (`-d`, `wt remove`'s own test) reads a genuinely merged branch as unmerged —
   `state=MERGED`, just confirmed, is the "clear, stated reason" `../_shared/references/git-safety.md`
   requires for that. And because item 3's `--delete-branch` may already have deleted the local branch
   (whenever it wasn't pinned by another worktree), check before you delete — "already gone" means item 3
   did this already, not a failure:
   - `wt`: `wt remove <feature-branch>` if it's still there; add `-D` if it reports the branch unmerged.
   - `exit-worktree`: hand back via the **ExitWorktree** tool (`action: "remove"`, `discard_changes: true`
     — a squash/rebase merge leaves the worktree's commits unreachable from the branch it was cut from, and
     the tool refuses removal without it). Never `git worktree remove` the harness's own worktree from
     inside it.
   - `git-worktree`: `git worktree remove <toplevel>` then, if `git branch --list <feature-branch>` still
     shows it, `git branch -D <feature-branch>`, then `git worktree prune`.
   - `none`: if `git branch --list <feature-branch>` still shows it, `git branch -D <feature-branch>`.

**Local path** (no open PR) — by `cleanup_path`

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
- `git branch` — the feature branch is gone (and, PR path, `git ls-remote --heads <remote> <feature-branch>`
  is empty).
- `git branch --show-current` / `pwd` — you are on the base branch (or back in the primary checkout).
- `git log --oneline -5` — the base contains the feature commits (PR path: the squash/merge/rebase commit
  `gh pr merge` produced).

## Deliverable

Prune with `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh --prune` on the way out, folded into step 5's
verification call.

- Which path ran — local merge, or GitHub PR merge (name the PR URL and method used).
- The gate verdict, naming any step served from the ledger as `cached` and how old that proof was.
- What merged into what, the resulting base HEAD, and that branch + worktree were removed.
- Anything left in place on purpose (unmerged commits, dirty tree, a delete the user declined) — say so
  explicitly.

## Git safety

Follow `../_shared/references/git-safety.md`: never delete an unmerged branch or force-remove a dirty worktree
without an explicit request, never push to or force-update the base, confirm branch/tree state before each
irreversible step.
