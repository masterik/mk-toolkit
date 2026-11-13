---
name: finish
description: >-
  Commit, merge a feature branch into base, delete the branch, remove the worktree. Merges the open PR if
  one exists, else locally. Trigger on "finish this feature", "merge back and clean up", "merge into main
  and clean up", "done with this feature". To open a PR, use pr.
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
`../_shared/references/remote.md`,
`../_shared/references/output-discipline.md`, `../_shared/references/workflow-contract.md`,
`../_shared/references/stacked-prs.md` (only when the PR is part of a stack).

## When NOT to use this

- Work needs review → `pr`.
- You only want to commit → `commit`.
- Base branch is protected, or the team merges only via PR → `pr`.

## Preconditions

**One call**, which also opens this run's directory (`../_shared/references/output-discipline.md`):

```bash
bash ../_shared/scripts/facts.sh finish --run --base <base> --gh
```

The path is relative to this skill's directory (`../_shared/references/output-discipline.md` says how to
resolve it).

What ran on this branch before comes from the session and `commits:` (`<base>..HEAD`),
plus the PR when one is open — `../_shared/references/workflow-contract.md`, rule 4. A `review` this session
ran over the content now being merged is worth naming in the deliverable — but only for the content it
actually read: fixes it applied afterwards were not reviewed. It does not gate the merge, and it never
substitutes for the quality gate.

Keep the `run_dir=` literal; this file writes it as `<run-dir>`, its `toplevel=` as `<toplevel>`, and the
absolute git binary as `<git>` (`../_shared/references/git-safety.md`). There is no `$RUN_DIR` — a shell
variable does not survive to the next Bash call — and re-running with `--run` opens a second directory
instead of returning the first. Check four things in what it printed:

1. **Current branch** — `branch=` must be a feature/bugfix branch, not `default_branch=`. On `main`/`master`:
   stop, nothing to finish.
2. **Base branch** — the branch this feature was cut from, usually `default_branch=`. Ask if ambiguous;
   `../_shared/references/branching.md` covers detecting a repo's own model. Pass it as `--base` so
   `commits:`, `base_stat` and `ff_from_base` come back with everything else.
3. **Cleanup path** — `cleanup_path=` is `exit-worktree` (the harness's own worktree) · `git-worktree`
   (any other linked worktree; worktrunk, when `wt` is available, does its teardown) · `none` (primary
   checkout). It decides step 4. Read `../_shared/references/worktree.md` there, not now.
4. **Existing PR** — first the remote: when `../_shared/references/remote.md` says to (a remote named in
   `docs/agents/remote.md`, or `remote_choice=ask` and the user's answer), re-run this call without `--run`
   and with `--remote <name>` added, **before routing** — `pr=` was computed for the
   fallback remote (`../_shared/references/remote.md`). Then `pr=` is a URL only when one exists, alongside `pr_state=OPEN|CLOSED|MERGED` and
   `pr_draft=true|false`. Sentinels (`none`, `gh-missing`, `gh-unauthenticated`, `no-remote`)
   mean there is nothing to merge remotely — take the **local merge path**. `pr_state=OPEN` (and not
   draft) redirects step 4 to the **PR merge path** instead: this branch is merged on GitHub, not with a
   local `git merge`. Treat `CLOSED`/`MERGED` like no PR for routing purposes, but mention it — a merged
   PR with the branch still around usually just needs cleanup; a closed one may mean the user changed
   their mind, worth a check before merging anything. `pr_draft=true` also routes to the local path unless
   the user asks to mark it ready first (`gh pr ready <url>`) — GitHub refuses to merge a draft.
   **Stacked?** On the PR path, check whether the PR is part of a GitHub stack
   (`../_shared/references/stacked-prs.md`, "Detect"). Optional: a failed or empty probe means not stacked.
   A stacked PR changes step 4's merge call (**`gh stack merge`, never `gh pr merge`**) and what step 3
   must say: every unmerged PR below this one merges with it.

## Workflow

### 1. Commit remaining work

Dirty tree → run `commit` first (stage intentionally, logical commits, Conventional Commit messages). The
tree must be clean before merging. **Never merge with uncommitted changes.**

### 2. Verify before merging

Run the **full quality gate** (`../_shared/references/quality-gate.md`): lint → test → build or the repo's
equivalent, stop on first failure. A local merge skips review, so this gate is the only safety net. Report any
failure; do not merge past it without an explicit user OK.

The reference's table ("Find the gate") names the commands; run each yourself, its output to a log in the run directory. One line
per passing step; on failure the step, the exit code, the failure lines and the tail
(`../_shared/references/output-discipline.md`). A failing suite is thousands of lines, none of which change
the decision ("fix it or get an explicit OK"). Skip the gate only under the reference's one reuse rule, and say
so — `gate=ok` for a gate that did not run is not an acceptable report here.

Delegate the diagnosis only when that verdict is not enough (`../_shared/references/quality-gate.md`, "when a
step fails") and give the user what it returns — what failed, probable cause, whether the change caused it,
suggested fix. The log stays on disk.

### 3. Show the plan and confirm

Before merging or deleting anything, print a one-screen summary and get a go-ahead (unless the user already
said "finish and clean up" or gave standing authorization). Collect the go-ahead through `AskUserQuestion`
where the host has it — two options, **merge and clean up** (recommended) and **stop here** — with the
numbered-list fallback in `../_shared/references/summary-format.md`. Step 0's `commits:` block predates step 1, so if
step 1 committed, re-read the log (`"<git>" -C "<toplevel>" log --oneline <base>..HEAD` — pinned, per
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

**PR path** — step 4's opening read of the merge style and its first two items (push, then pick the merge method) run
*before* this: fill this block in with the method just picked, get the go-ahead, and only then continue with
the rest of step 4:

```
Finish feature:  <feature-branch>
Merge via:        GitHub PR <pr-url> (<squash|merge|rebase>, <from docs/agents/merge.md|default|asked>)
Base:             <base-branch>
Worktree:         <cleanup_path> @ <toplevel>
Commits to merge: <the commits: block, re-read if step 1 committed>
After merge:      delete branch <feature-branch> (local + remote) + remove worktree (if any);
                   switch to <base-branch> and pull
```

**Stacked PR** — add one line, and list the PRs, because the go-ahead covers all of them:

```
Stack:            #<n> of <size> — also merges (atomically) the <k> unmerged PRs below: #<a>, #<b>
                   PRs above stay open; GitHub rebases and retargets them
```

### 4. Merge back + clean up

**Both paths start here: how this repo merges.** Read `docs/agents/merge.md` if it exists
(`../_shared/references/repo-settings.md`, "Merge"):

- **It names a style** (`merge`, `squash` or `rebase`) → that is the **repo's style**, used on every path
  below without asking. Name where it came from — "squash, from `docs/agents/merge.md`".
- **No file, or one silent on style** → the **default style**, `merge`. A default is not a repo statement: it
  gives way without comment where the remote refuses it, and never overrides the user's worktrunk config.
  The missing file is never mentioned.
- **A style `finish` doesn't know** (`squish`) → say so in one line, then carry on as with no file. Falling
  silently back would leave the only symptom a merge shape the team thought they had changed.

**PR path** (`pr=<url>`, `pr_state=OPEN`, not draft) — merge on GitHub, then sync locally:

1. **Push anything step 1 committed.** `gh pr merge` merges what's on GitHub, not local state:
   Push explicitly, after step 1: `git push -u <remote> <feature-branch>` when `pushed=no`, else
   `git push <remote> <feature-branch>` (a no-op when up to date). Not a bare `git push`: `pushed=yes` only proves
   *some* upstream exists, possibly on another remote, and `ahead=` predates step 1's commits.
   `<remote>` is `remote=` — never a hardcoded `origin` — as settled in step 0.
2. **Pick the merge method.** `gh repo view [<remote_repo>] --json mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed`
   tells you what the remote will accept; the style above tells you what this repo wants.
   `merge`→`--merge`/`mergeCommitAllowed`, `squash`→`--squash`/`squashMergeAllowed`,
   `rebase`→`--rebase`/`rebaseMergeAllowed`. In order:

   - **Exactly one method allowed** → use it, whatever the style says. GitHub decides what it will
     accept; a file cannot widen that. If the repo's style named a different one, say so in one line.
   - **The style is allowed** → use it and **do not ask**. Name where it came from — the file or the
     default — so a wrong setting is visible the first time it is used rather than on the merge it produced.
   - **The repo's style is not allowed** → **report it, never substitute.** Say what `merge.md` says, what
     the remote actually allows, and let the user pick; do not quietly fall back. A setting that is wrong for
     the repo is worth one interruption; a silent substitution on every run for the rest of the repo's life
     is not.
   - **The default style is not allowed**, several others are → ask which, without reporting a mismatch:
     nobody asked for `merge`.

   Never pass `--admin` (bypasses branch protection / required reviews) unless the user explicitly asks
   for it.

   **Stop here and show step 3's PR-path confirmation block, filled in with the method just picked. Get
   the go-ahead before continuing** — nothing below this point runs without it.
3. **Merge.** **Stacked PR → `gh stack merge <pr-number> --yes --<method>` instead of `gh pr merge`**
   (`../_shared/references/stacked-prs.md`, "Merge"). It needs the `gh stack` extension: absent → stop
   with the install line from that file, nothing merged. **Signed commits first:** GitHub's rebase of the
   layers above is unsigned, so if the base branch requires signed commits, stop before merging and
   report that the stack needs a local `gh stack rebase` + `gh stack push` (stacked-prs.md, "Why the
   normal merge path breaks"). Success continues to item 4; a refusal or `enqueued` stops here (report it
   verbatim; no local-merge fallback; nothing deleted). The server does not delete branches, so item 5
   removes remote and local by hand — **for every PR the merge carried** (the lower layers' head
   branches too, listed in step 3's `Stack:` line), each only after its own `state=MERGED` check
   (`git push <remote> --delete <branch>`). Auto-merge is unsupported for stacks — never offer it.

   Not stacked:
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
   git pull --ff-only <remote> <base>   # explicit: a bare `git pull` follows the base's own upstream, possibly another remote
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
   - `git-worktree`, worktrunk available: `wt remove <feature-branch>` if it's still there; add `-D` if it
     reports the branch unmerged.
   - `exit-worktree`: hand back via the **ExitWorktree** tool (`action: "remove"`, `discard_changes: true`
     — a squash/rebase merge leaves the worktree's commits unreachable from the branch it was cut from, and
     the tool refuses removal without it). Never `git worktree remove` the harness's own worktree from
     inside it.
   - `git-worktree`, no worktrunk: `git worktree remove <toplevel>` then, if `git branch --list <feature-branch>` still
     shows it, `git branch -D <feature-branch>`, then `git worktree prune`.
   - `none`: if `git branch --list <feature-branch>` still shows it, `git branch -D <feature-branch>`.

   **Stacked: sync what is left.** The PRs above this one were rebased and retargeted on GitHub; the
   local branches for them are behind. If a local stack is tracked, ask before running `gh stack sync` (it force-pushes the rebased branches), then run it (from the surviving root, after the cleanup above); otherwise tell the user
   those branches need a fetch and rebase. Leave them and their worktrees in place — they are not this
   run's to remove. Name them in the deliverable under "left behind".

**Local path** (no open PR) — by `cleanup_path`

**`git-worktree`, worktrunk available** (`command -v wt`, or `type wt` for its shell function) — delegate; it squash-rebases, fast-forwards the base and removes the worktree in one step, firing
the user's hooks:

```bash
wt merge <base>        # add -y only if non-interactive completion is authorized
```

`wt merge` removes the worktree by default and deletes the branch as part of the flow.

**The repo's style from `docs/agents/merge.md` goes on this command line.** worktrunk carries the user's
own squash/rebase config, and a repo-wide setting outranks a personal default — but `wt merge`'s flags only
turn defaults *off*, so one direction cannot be forced and is reported instead of fought.
`../_shared/references/worktree.md`, "The repo's merge style vs. worktrunk's own config", has the mapping
(`squash` → no flag · `rebase` → `--no-squash` · `merge` → `--no-squash --no-ff`) and the reasoning. No
file → plain `wt merge <base>`, worktrunk's config unchallenged: the default `merge` is not a repo statement. Use `--no-remove` only to
override the user's config on request.

**`exit-worktree`** — merge into the base, then hand back via the **ExitWorktree** tool
(`action: "remove"`). Never `git worktree remove` the harness's own worktree from inside it. If the base
cannot be fast-forwarded (`ff_from_base=no`, or it is checked out in the primary worktree), merge from
`primary=` per `../_shared/references/worktree.md`.

**`git-worktree`, no worktrunk**:

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

Update the base against the remote first (`git fetch <remote>` / `git pull --ff-only <remote> <base>`) when one exists, so you
merge onto current base.

On the three plain-git paths the style decides the shape of that merge, with none of worktrunk's
config to contend with: `merge` → `git merge --no-ff` · `rebase` → rebase the feature branch onto `<base>`
first, then fast-forward · `squash` → `git merge --squash` followed by one commit. No file → the commands
as written above. Say which one you used, the same as on the other paths.

### 5. Verify the cleanup

**`<toplevel>` is stale here whenever step 4 removed a worktree — and so may the shell's cwd be**, if it was
inside that worktree when `git worktree remove` ran. `<toplevel>` is the root `facts.sh` resolved at step
1 — the *feature* worktree on any linked path — and that directory no longer exists, so a call pinned to it,
or one run from a cwd still inside it, fails instead of verifying anything. Resolve `<surviving-root>` once
(`primary=` from `facts.sh`, or re-resolve the root) and put every command below against it — `cd
"<surviving-root>"` first, since `pwd` and `git branch --show-current` have no `-C` equivalent, and pass
`-C "<surviving-root>"` explicitly to the rest, including the pinned log check. On `linked=no` there was
nothing to remove and `<toplevel>`/the cwd are still correct.

- `git -C "<surviving-root>" worktree list` — the feature worktree is gone (if there was one).
- `git -C "<surviving-root>" branch` — the feature branch is gone (and, PR path, `git ls-remote --heads
  <remote> <feature-branch>` is empty).
- after `cd "<surviving-root>"`: `git branch --show-current` / `pwd` — you are on the base branch (or back
  in the primary checkout).
- `"<git>" -C "<surviving-root>" log --oneline -5` — the base contains the feature commits (PR path: the
  squash/merge/rebase commit `gh pr merge` produced).

## Deliverable

Open with a one-line bold verdict (`**Verdict:** merged into main, branch and worktree
removed.` — or where and why it stopped short); form per `../_shared/references/summary-format.md`.

- Which path ran — local merge, or GitHub PR merge (name the PR URL and method used), and **where the
  method came from**: `docs/agents/merge.md`, the default, or asked. Plus, only when there was something to
  say: a style from the file the remote refused, a style in the file `finish` doesn't know, or a `wt merge`
  whose output shows the user's worktrunk config overrode the file. Silence when there is no file — that is
  not a degradation.
- Stacked PR: which PRs merged together (all of them, by number), the `gh stack merge` outcome, and the
  PRs still open above.
- The gate verdict, or that it was skipped under the reuse rule and at which commit it passed.
- What merged into what, the resulting base HEAD, and that branch + worktree were removed.
- Anything left in place on purpose (unmerged commits, dirty tree, a delete the user declined) — say so
  explicitly, each with its path or name and why, or "nothing left behind" so the absence is not ambiguous.
- What is needed from the user, as the last line: how to continue where the run stopped short, or
  "nothing needed from you" when it completed.

## Git safety

Follow `../_shared/references/git-safety.md`: never delete an unmerged branch or force-remove a dirty worktree
without an explicit request, never push to or force-update the base, confirm branch/tree state before each
irreversible step.
