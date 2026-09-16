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
`../_shared/references/output-discipline.md`, `../_shared/references/workflow-contract.md`.

## When NOT to use this

- Work needs review → `pr`.
- You only want to commit → `commit`.
- Base branch is protected, or the team merges only via PR → `pr`.

## Preconditions

**One call**, which also opens this run's directory (`../_shared/references/output-discipline.md`):

```bash
mkit facts finish --base <base> --gh
```

`mkit facts` **is** this skill's dependency check. If it fails with `command not found` **or**
`unknown command "facts"` — absent and too old are the same answer here — **stop** and say:

> This skill runs on the `mkit` binary. Install it with `brew install masterik/tap/mkit` (or upgrade
> with `brew upgrade mkit`), then run it again.

Presence only, no declared minimum on either side — a subcommand that does not exist *is* the too-old
signal.

Then read what ran on this branch before, and what each step concluded — never a stop:
`../_shared/references/workflow-contract.md`, "Reading it".

```bash
mkit work show --json --limit 20
```

A `review` record whose `fingerprint` matches the tree in front of you says a review already ran over this
exact content — worth naming in the deliverable. **Read its `assumptions` before believing it**: a review
that applied fixes records a fingerprint the reviewers never saw and says so there. It does not gate the
merge, and it never substitutes for the quality gate, which has its own ledger.

Keep the `run=` literal; this file writes it as `<run-dir>`. There is no `$RUN_DIR` — a shell variable does
not survive to the next Bash call — and re-running `mkit facts` opens a second directory instead of returning
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
   `pr_draft=true|false`. Sentinels (`none`, `gh-missing`, `gh-unauthenticated`, `no-remote`)
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
mkit gate detect
mkit gate run <run-dir> --chain 'lint=<cmd>' 'test=<cmd>' 'build=<cmd>'
```

One line per passing step; on failure the step, the exit code, the grepped failures and the tail
(`../_shared/references/output-discipline.md`). A failing suite is thousands of lines, none of which change
the decision ("fix it or get an explicit OK").

`mkit gate detect` also reports what the gate ledger already proved — each step's `cache=` in the
`full:` block. **This is the strictest consumer in the bundle**, because a local merge skips review and this gate
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
step 1 committed, re-read the log (`"$git_bin" -C "$toplevel" log --oneline <base>..HEAD` — pinned, per
`../_shared/references/git-safety.md`) first — approving a plan that omits the
commits the merge actually carries is worse than asking twice.

**Local path:**

```
Finish feature:  <feature-branch>
Merge into:       <base-branch>
Worktree:         <cleanup_path> @ <toplevel>
Commits to merge: <the commits: block, re-read if step 1 committed>
After merge:      delete branch <feature-branch> + remove worktree (if any)
```

**PR path** — step 4's opening profile call and its first two items (push, then pick the merge method) run
*before* this: fill this block in with the method just picked, get the go-ahead, and only then continue with
the rest of step 4:

```
Finish feature:  <feature-branch>
Merge via:        GitHub PR <pr-url> (<squash|merge|rebase>, <pinned|discovered|asked>)
Base:             <base-branch>
Worktree:         <cleanup_path> @ <toplevel>
Commits to merge: <the commits: block, re-read if step 1 committed>
After merge:      delete branch <feature-branch> (local + remote) + remove worktree (if any);
                   switch to <base-branch> and pull
```

### 4. Merge back + clean up

**Both paths start here: ask the repo how it merges.** One call, and it is **optional enrichment,
never a prerequisite** — it answers the question this step used to ask on every run, and when it does not
answer, this step behaves exactly as it did before the call existed:

```bash
mkit repo profile --json
```

`merge_style` is `{"value": "merge|squash|rebase", "source": "pinned|discovered|unavailable"}`.
Take the value on **`pinned`** (a human wrote it in `.mkit/config.toml`) or **`discovered`** (read from this
repo's own git config) and stop asking. On `unavailable` — or if the command fails at all, which an older
binary that knows `facts` but not `repo` will do — there is **no style and nothing to report**: fall through
to asking, exactly as below. Unlike `mkit facts`, this call never stops the run and is never mentioned when
it has nothing to say. It is one fewer question, not a dependency.

**PR path** (`pr=<url>`, `pr_state=OPEN`, not draft) — merge on GitHub, then sync locally:

1. **Push anything step 1 committed.** `gh pr merge` merges what's on GitHub, not local state:
   `pushed=no` → `git push -u origin <feature-branch>`; otherwise plain `git push` if `ahead=` > 0.
2. **Pick the merge method.** `gh repo view --json mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed`
   tells you what the remote will accept; `merge_style` above tells you what this repo wants.
   `merge`→`--merge`/`mergeCommitAllowed`, `squash`→`--squash`/`squashMergeAllowed`,
   `rebase`→`--rebase`/`rebaseMergeAllowed`. In order:

   - **Exactly one method allowed** → use it, whatever `merge_style` says. GitHub decides what it will
     accept; a pin cannot widen that. If the pin named a different one, say so in one line.
   - **A style, and the remote allows it** → use it and **do not ask**. Name where it came from — "squash,
     pinned in `.mkit/config.toml`" or "squash, discovered from this repo's git config" — so a wrong pin is
     visible the first time it is used rather than on the merge it produced.
   - **A style the remote does not allow** → **report it, never substitute.** Say which value was pinned,
     what the remote actually allows, and let the user pick; do not quietly fall back to squash. A pinned
     value cheap to verify gets verified, and the point of verifying is to surface the mismatch, not to
     paper over it. A pin that is wrong for the repo is worth one interruption; a silent substitution on
     every run for the rest of the repo's life is not.
   - **No style** (`unavailable`, or no profile at all) and more than one method allowed → ask, default
     suggestion squash, as before.

   Never pass `--admin` (bypasses branch protection / required reviews) unless the user explicitly asks
   for it.

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
   - `none`: if `git branch --list <feature-branch>` still shows it, `git branch -D <feature-branch>`,
     then retire the worklog per **Retiring the worklog** below.

**Local path** (no open PR) — by `cleanup_path`

**`wt`** — delegate; it squash-rebases, fast-forwards the base and removes the worktree in one step, firing
the user's hooks:

```bash
wt merge <base>        # add -y only if non-interactive completion is authorized
```

`wt merge` removes the worktree by default and deletes the branch as part of the flow.

**A `merge_style` from the profile goes on this command line.** worktrunk carries the user's own
squash/rebase config, and a repo-wide pin outranks a personal default — but `wt merge`'s flags only turn
defaults *off*, so one direction cannot be forced and is reported instead of fought.
`../_shared/references/worktree.md`, "A pinned merge style vs. worktrunk's own config", has the mapping
(`squash` → no flag · `rebase` → `--no-squash` · `merge` → `--no-squash --no-ff`) and the reasoning. No
style → plain `wt merge <base>`, worktrunk's config unchallenged, as today. Use `--no-remove` only to
override the user's config on request.

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

On the three plain-git paths a `merge_style` decides the shape of that merge, with none of worktrunk's
config to contend with: `merge` → `git merge --no-ff` · `rebase` → rebase the feature branch onto `<base>`
first, then fast-forward · `squash` → `git merge --squash` followed by one commit. No style → the command
as written above. Say which one you used, the same as on the other paths.

#### Retiring the worklog

**Both paths, and only where `cleanup_path=none`.** The worklog lives in the work tree, so every other
cleanup path carries it off with the worktree it removes. `none` removes no worktree, so the log outlives
the branch it is keyed on — and a branch name is reusable, which hands a later `review` on a recreated
`feat/x` the *old* `feat/x`'s gists, a goal for work that no longer exists. Ask the binary for the path
rather than deriving the filename, which carries a digest:

```bash
p=$(mkit work show --branch <feature-branch> --path) && rm -f "$p"
```

Best effort, like every other `mkit work` call here: a path it cannot produce is one line of note. And skip
the append on this path — the record would go straight into the file being retired.

### 5. Verify the cleanup

**`$toplevel` is stale here whenever step 4 removed a worktree — and so may the shell's cwd be**, if it was
inside that worktree when `git worktree remove` ran. `$toplevel` is the root `mkit facts` resolved at step
1 — the *feature* worktree on any linked path — and that directory no longer exists, so a call pinned to it,
or one run from a cwd still inside it, fails instead of verifying anything. Resolve `<surviving-root>` once
(`primary=` from `mkit facts`, or re-resolve the root) and put every command below against it — `cd
"<surviving-root>"` first, since `pwd` and `git branch --show-current` have no `-C` equivalent, and pass
`-C "<surviving-root>"` explicitly to the rest, including the pinned log check. On `linked=no` there was
nothing to remove and `$toplevel`/the cwd are still correct.

- `git -C "<surviving-root>" worktree list` — the feature worktree is gone (if there was one).
- `git -C "<surviving-root>" branch` — the feature branch is gone (and, PR path, `git ls-remote --heads
  <remote> <feature-branch>` is empty).
- after `cd "<surviving-root>"`: `git branch --show-current` / `pwd` — you are on the base branch (or back
  in the primary checkout).
- `"$git_bin" -C "<surviving-root>" log --oneline -5` — the base contains the feature commits (PR path: the
  squash/merge/rebase commit `gh pr merge` produced).

## Deliverable

Prune with `mkit run prune` on the way out, folded into step 5's
verification call.

- Which path ran — local merge, or GitHub PR merge (name the PR URL and method used), and **where the
  method came from**: pinned, discovered, or asked. Plus, only when there was something to say: a pinned
  style the remote refused, or a `wt merge` whose output shows the user's worktrunk config overrode the
  pin. Silence when the profile had no answer — that is not a degradation.
- The gate verdict, naming any step served from the ledger as `cached` and how old that proof was.
- What merged into what, the resulting base HEAD, and that branch + worktree were removed.
- Anything left in place on purpose (unmerged commits, dirty tree, a delete the user declined) — say so
  explicitly.

Then record the run — from wherever this session ends up, and **only if that is still a work tree with
this branch's log in it**. `finish` is the one step that usually destroys its own log: the worklog lives in
the worktree it removes, and the branch it is keyed on is deleted a moment later. So the append is worth
doing where the run stopped short — a declined merge, a failed gate, a PR not ready — and worth skipping
where the cleanup actually ran. Skipping it is not a degradation to report:

```bash
mkit work append --step finish --gist '<one line: what this run concluded>' \
  [--artifact '<merge-sha>'] [--assume '<what this run derived rather than found>']...
```

`--artifact` only where a merge happened; a run that stopped short has no sha to name, and the gist is
where the reason goes.

After the report is produced, and it never changes the report: a failed append is one line of note, not a
failed run. A run that merged nothing still records where the log survives — the reason is exactly what the
next `finish` wants to know.

## Git safety

Follow `../_shared/references/git-safety.md`: never delete an unmerged branch or force-remove a dirty worktree
without an explicit request, never push to or force-update the base, confirm branch/tree state before each
irreversible step.
