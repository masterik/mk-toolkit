---
name: cleanup
description: >-
  Sweep every local branch and worktree in the repo: auto-delete what's merged (locally or via a closed PR),
  remove the worktrees that go with them, keep only the default branch (main/master/trunk) and a develop-like
  branch if one exists locally, then switch to one of those and pull it up to date with the remote. Trigger on
  "cleanup branches", "clean up my branches", "prune stale branches", "remove merged branches", "clean up
  worktrees", "tidy up local branches", "get rid of old branches". Local-only: it never deletes, force-pushes
  to, or otherwise touches a branch on the remote — only this checkout's own local branches and worktrees.
model: sonnet
---

# Clean up local branches and worktrees

Part of the **mkit** bundle, but outside the edit → commit → review → finish/pr line: this is repo
gardening, not feature work. `finish` tears down the *one* branch/worktree it just merged; `cleanup` sweeps
*every* local branch in the repo, whatever their state.

References, read the ones a step calls for: `../_shared/references/worktree.md`,
`../_shared/references/git-safety.md`, `../_shared/references/branching.md`,
`../_shared/references/output-discipline.md`.

## What this does and does not touch

- Deletes local branches and removes the worktrees attached to them.
- Never touches a remote branch, never `git push --delete`s, never closes or merges a PR, never force-pushes.
  `git fetch --prune` is the only network call, and it only updates this repo's own remote-tracking refs
  (`refs/remotes/<remote>/*`) — that is what makes `upstream=gone` mean anything, and it cannot delete anything
  on the remote itself.
- Keeps exactly the **default branch** (`main`/`master`/`trunk`, whichever the repo resolves to) and, if one
  exists **as a local branch**, the first of `develop`/`development`/`dev`. A `develop` that exists only as
  `origin/develop` and was never checked out locally is not kept — nothing here creates a local branch, only
  removes them.
- Ends by switching to one of the kept branches and pulling it up to date with the remote `branch-scan.sh`
  discovers (never assumed to be named `origin`).

## Preconditions

**Two calls.** The first opens this run's directory and resolves the default branch
(`../_shared/references/output-discipline.md`); the second needs that branch name, so it runs after:

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh cleanup
${CLAUDE_PLUGIN_ROOT}/scripts/branch-scan.sh --default <default_branch from facts.sh>
```

Keep the `run=` literal; this file writes it as `<run-dir>`. There is no `$RUN_DIR` — a shell variable does
not survive to the next Bash call.

From `facts.sh`: `branch=` (current branch — cannot be deleted while checked out), `clean=` (is the *current*
worktree dirty right now), `cleanup_path=` (only relevant if this session's own worktree turns out to be one
of the ones in play — see step 3).

From `branch-scan.sh`: `protected=`, `develop=`, `remote=`, `fetch=` (say if it came back `failed` — classify
on what you have and note it), `gh=` (say if it is anything but `ok` — some classes below then rest on git
alone), the `branches:` table and the `worktrees:` table. Full column meaning is in the script's own header
comment; the short version:

| `class` | means | default handling |
| --- | --- | --- |
| `protected` | the default/develop branch | never a candidate |
| `current` | whatever is checked out right now | never *directly* touched — see step 1's current-branch note before assuming it's out of scope entirely |
| `merged` | git proves it: an ancestor of a protected branch (`merged_into` names which) | auto-delete |
| `merged-pr` | not an ancestor locally (squash/rebase merge changes the SHAs), but GitHub says the PR merged, verified against this branch's own content | auto-delete, say how you know |
| `open-pr` | an open PR exists for this branch | ask |
| `closed-pr` | a PR exists and was closed without merging | ask |
| `gone` | had an upstream, it was deleted remotely, no PR trail found | ask |
| `unpushed` | never had an upstream at all | ask |
| `tracking` | a live upstream, not merged, no conclusive PR info | ask |

## Workflow

### 1. Classify

Walk the `branches:` table. Every `protected` row is out of scope, full stop — never propose deleting either
of the two kept branches. For everything else, sort into:

- **auto-delete**: `merged` or `merged-pr`, **and** (no attached worktree, or its `worktrees:` row is
  `clean=yes`). A `merged`/`merged-pr` branch whose worktree is `clean=no` **or `clean=error`** moves to
  **ask** instead — an unreadable worktree is not a proven-clean one, and neither is touched without an
  explicit go-ahead, merged or not (`../_shared/references/git-safety.md`).
- **ask**: `open-pr`, `closed-pr`, `gone`, `unpushed`, `tracking` — and any `merged`/`merged-pr` branch that
  landed here because its worktree is dirty or unreadable.
- **cannot act automatically**: a `worktrees:` row with `origin=claude-code` (some Claude Code session's own
  worktree — possibly still in use right now) always moves to **ask**, whatever its branch class, and a row
  with `clean=missing` (the worktree's directory is already gone; `git worktree prune` is the fix, not a
  removal) is noted but never a delete target.

**The `current` row needs one extra step, not a skip.** `branch-scan.sh` reports `current` in the `class`
column *instead of* what the branch would otherwise classify as — the branch you happen to be standing on is
never exempt from cleanup just because you started there. Before deciding it is out of scope, derive its real
disposition from its own `upstream` and `merged_into` columns, using the same priority order the script's own
header documents (`merged` > `merged-pr` > `open-pr`/`closed-pr` > `gone` > `unpushed` > `tracking` — `pr`'s
column tells you which of the PR-based ones apply). Sort that derived class into auto-delete/ask exactly like
any other row, but flag it separately as **"current — switch away first"**: step 3 has to leave it before it
can act on it, which is the one thing that makes this branch different from the rest of the table.

### 2. Show the plan and confirm

One screen, grouped exactly like the classification above — this mirrors `finish`'s "show the plan before
touching anything":

```
Cleanup plan:  <n> branch(es) reviewed, keeping <protected list>
Currently on:  <branch> — <its derived class, if not protected> (will switch away before deleting it, if in scope)

Auto-delete (merged):
  <branch>  (merged into <target>[, worktree removed])
  ...

Needs your OK:
  <branch>  open PR #<n>, worktree at <path>
  <branch>  unpushed — 1 commit not on any remote, recoverable via reflog for a while
  ...

Left alone:
  <branch>  worktree is dirty — <path>
  <branch>  origin=claude-code worktree — may still be in use
```

Auto-delete branches need no further confirmation beyond this plan (unless the user already gave standing
authorization to skip even this). For the **ask** group: with four or fewer distinct decisions, use
**AskUserQuestion** — one option per branch or per small cluster of branches sharing the same reason, framed
as "delete" vs "keep". With more than that, ask once in plain text for the whole group rather than forcing
everything through a 4-option tool. Never assume "ask" means "delete" — no response, or an unclear one, means
that branch is left alone and named in the final report.

### 3. Leave the current branch first, if it's in scope

Do this **before** anything else in this section. If step 1 flagged the branch you started on as
"current — switch away first" (it fell into auto-delete or was approved in the ask group), `git switch
<default>` now — you cannot delete the branch you are standing on, and there is nothing left to special-case
once you aren't standing on it anymore. If it was never in scope, skip this and stay put; you'll land somewhere
in step 5 either way.

`git switch` refuses when the target is already checked out in another worktree (relevant if `facts.sh`'s own
`cleanup_path=` said this session is itself inside a linked or `.claude/worktrees/` checkout, and the default
branch is checked out in the primary one). Try `develop` instead if it exists and isn't taken either; if both
protected branches are unavailable, stop and say so rather than forcing anything — this is the one place in
the workflow where "switch away" can fail through no fault of the branch being deleted.

### 4. Delete — worktree first, then branch, always verified, never a blind `-d`

For every branch approved in step 1, step 2, or just vacated in step 3, in this order:

1. **Remove the worktree, if any.** Prefer `wt remove <branch>` when `wt` is on `PATH` — it respects the
   user's hooks and config (`../_shared/references/worktree.md`). Otherwise `git worktree remove <path>` (add
   `--force` only for a `clean=no` worktree the user explicitly approved removing anyway — that discards
   uncommitted work, so say so plainly before doing it). Never do this for an `origin=claude-code` worktree
   unless the user explicitly approved it knowing what it is. **`wt remove` already deletes a merged branch
   itself** — after it runs, check `git show-ref --verify --quiet refs/heads/<branch>` before step 4.2; if it's
   already gone, you're done with this branch, and running a delete on it anyway is a redundant, avoidable
   error, not a real failure worth reporting as one.
2. **Delete the branch, if it still exists.** Always `git branch -D <branch>` here, never plain `-d` — `-d`
   only checks whether the branch is merged into whatever you currently have checked out, which is not what
   any of this skill's own evidence is measured against (a `merged` branch may be an ancestor of `develop`
   while you're standing on `main`; a `merged-pr` branch is by definition *not* a local ancestor of anything,
   that's what makes it `merged-pr` instead of `merged`). State which proof licenses the `-D` as you run it:
   - `merged` → "verified: an ancestor of `<merged_into>`" (the column already named it — cite the actual
     target, not just the label).
   - `merged-pr` → "verified: GitHub PR `#<n>` merged" (`branch-scan.sh` already checked the PR's head commit
     against this branch's own content before reporting the class, so there is no separate check to redo here).
   - anything from the **ask** bucket → cite the user's approval itself as the reason, same as before.

Never run step 4.2 on a branch nobody approved and whose class isn't `merged`/`merged-pr` — the two per-class
justifications above are what make `-D` here a **verified** delete rather than a blind force, not a license to
force-delete anything you merely suspect is fine.

### 5. Switch and pull

Bring both kept branches up to date with the **discovered** remote — the one `branch-scan.sh` reported as
`remote=`, never a hardcoded name, since nothing here is safe to assume about a repo you didn't set up:

```bash
git switch <default-or-develop>                           # land on whichever you're ending on
git pull --ff-only                                        # only if remote != none
git fetch <remote> <the-other-one>:<the-other-one>         # update the other one without checking it out
```

`remote=none` (no remote configured at all): skip both network calls entirely and say so — there is nothing to
pull from, and that is not a failure.

The `fetch ref:ref` form only succeeds as a fast-forward and only when that branch is not checked out
elsewhere (`../_shared/references/worktree.md`, "merge without checkout") — if it fails, say the branch is
ahead/diverged locally and needs a manual look, don't force it.

Prefer switching to the **default** branch as the place to land, unless the user's request or standing habit
points at `develop` instead — say which one you picked and why it was a judgement call, not a fact this script
handed you.

### 6. Verify

- `git branch -vv` — only the protected branches remain, and both (if two) show no `ahead`/`behind` against
  their upstream.
- `git worktree list` — every removed worktree is gone; `git worktree prune` first if step 1 saw any
  `clean=missing` stale metadata.

## Final report (always)

Prune with `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh --prune` folded into step 6's verification call.

```
Cleanup done — <n> local branch(es) removed, only <protected list> remain.

Deleted (merged into <target>):
<branch>[, <branch> (#PR if merged-pr)], ...

Deleted (<reason>, per your choice):
<branch> — <one-line why: open PR #n / unpushed, 1 commit, recoverable via reflog / ...>
...

Left alone:
<branch> — <why: dirty worktree / declined / claude-code worktree>
...

Note: <branch> is N commits behind origin/<branch> — <why it couldn't be fast-forwarded, if it couldn't>
```

Omit any section with nothing in it. Never report a branch as deleted that the user did not either fall into
the merged auto-delete bucket or explicitly approve.

## Git safety

Follow `../_shared/references/git-safety.md`: never delete a branch or force-remove a worktree without either
a clean merge or an explicit go-ahead, never touch anything on the remote, never update git config. `-D` and
`--force` are the two commands in this skill capable of losing work. Every `-D` needs a stated reason before it
runs — for `merged`/`merged-pr` that reason is the git/GitHub proof step 2's plan already showed the user, not
a fresh ask; for anything from the ask bucket it is the user's actual approval, and nothing licenses it without
one. No exceptions for "it looked stale."
