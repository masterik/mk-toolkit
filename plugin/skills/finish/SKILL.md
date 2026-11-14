---
name: finish
description: >-
  Commit, merge a feature branch into base, delete the branch, remove the worktree. Merges the open PR if
  one exists, else locally. Trigger on "finish this feature", "merge back and clean up", "merge into main
  and clean up", "done with this feature". To open a PR, use pr.
model: sonnet
---

# Finish a feature (merge + cleanup)

Commit, integrate into the base, tear down the branch and worktree. An open PR for this branch is merged *on
GitHub*, respecting its checks and reviews; with none, the merge is local. This skill never *opens* a PR —
work that needs review, a protected base, or a PR-only team → `pr`.

## Preconditions

**One call**, which also opens the run directory. The path is relative to this skill's directory: resolve it
and run it absolute. A key or exit not named here: `../_shared/references/output-discipline.md`.

```bash
bash ../_shared/scripts/facts.sh finish --run --base <base> --gh
```

Keep `run_dir=` as `<run-dir>` (no shell variable survives; re-running `--run` opens a second directory),
`toplevel=` as `<toplevel>`, and `command -v git` as `<git>` for calls whose output you parse
(`../_shared/references/git-safety.md`). Then check:

1. **Branch** — `branch=` is a feature branch, not `default_branch=`; on the base, stop: nothing to finish.
2. **Base** — the branch this one was cut from, usually `default_branch=`; ask if ambiguous.
3. **Remote** — with `remote_choice=no` and no `docs/agents/remote.md`, use `remote=`; otherwise follow
   `../_shared/references/remote.md` **before routing** (`pr=` was computed for the fallback remote).
4. **Route** — `pr=<url>` with `pr_state=OPEN` and `pr_draft=false` → the **PR path**. `gh-error` → stop,
   relay its `notes:` entry: an open PR may be hidden. Anything else → the **local path**:
   `none`/`gh-missing`/`gh-unauthenticated`/`no-remote`; a draft (unless the user asks to
   `gh pr ready <url>` first); `MERGED` (usually just needs cleanup — say so) or `CLOSED` (check with the user
   before merging anything — they may have changed their mind).

A `review` this session ran over the content being merged is worth naming in the report — only for what it
read, not fixes applied after. It never gates the merge and never replaces the gate.

## Workflow

### 1. Commit remaining work

Dirty tree → do `commit`'s work inline (`../commit/SKILL.md`). **Never merge with uncommitted changes.**

### 2. Gate

The **full quality gate** (`../_shared/references/quality-gate.md`): find the commands, run each to a log in
`<run-dir>`, stop at the first failure, one line per step. A local merge skips review, so this is the only
safety net: do not merge past a failure without the user's explicit OK. Skip only under its one reuse rule,
and say so — never `gate=ok` for a gate that did not run.

### 3. How this repo merges

Read `docs/agents/merge.md` if it exists (`../_shared/references/repo-settings.md`):

- **Names a style** (`merge`, `squash`, `rebase`) → the **repo's style**, used without asking; name where it
  came from.
- **No file, or silent on style** → the **default style**, `merge`. It is not a repo statement: it gives way
  without comment where the remote refuses it and never overrides the user's worktrunk config. Never mention
  the missing file.
- **A style `finish` doesn't know** (`squish`) → say so in one line, then behave as with no file.

### 4. Plan and go-ahead

PR path: first run `pr-merge.md` items 1–2 (push, pick the method) — [pr-merge.md](pr-merge.md). Then, unless
the user already said "finish and clean up" or gave standing authorization, show one screen and get a
go-ahead through `AskUserQuestion` — **merge and clean up** (recommended) / **stop here** — numbered-list
fallback per `../_shared/references/summary-format.md`. If step 1 committed, re-read the log first
(`"<git>" -C "<toplevel>" log --oneline <base>..HEAD`).

```
Finish feature:   <feature-branch>
Merge:            into <base> locally | via GitHub PR <url> (<method>, <from docs/agents/merge.md|default|asked>)
Worktree:         <cleanup_path> @ <toplevel>
Commits to merge: <the commits: block>
After merge:      delete branch <feature-branch> [local + remote] + remove worktree (if any)[; switch to <base> and pull]
```

A stacked PR adds a `Stack:` line naming every unmerged PR below that merges with it (`pr-merge.md`).

### 5. Merge back + clean up

- **PR path** → [pr-merge.md](pr-merge.md), items 3–5.
- **Local path** → `../_shared/references/worktree.md`, by `cleanup_path`: update the base, merge in the
  style from step 3 (the `wt merge` flag mapping is there), remove the worktree, delete the branch.

### 6. Verify the cleanup

When step 5 removed a worktree, `<toplevel>` — and possibly the shell's cwd — no longer exists. Resolve
`<surviving-root>` once (`primary=`), `cd "<surviving-root>"`, and point every check at it (`-C
"<surviving-root>"`). With `linked=no`, `<toplevel>` still stands.

- `git -C "<surviving-root>" worktree list` — the feature worktree is gone.
- `git -C "<surviving-root>" branch` — the feature branch is gone (PR path: `git ls-remote --heads <remote>
  <feature-branch>` is empty too).
- `git branch --show-current` and `pwd` — on the base, in the surviving checkout.
- `"<git>" -C "<surviving-root>" log --oneline -5` — the base holds the feature commits (or the PR's merge
  commit).

## Report

Bold verdict (`**Verdict:** merged into main, branch and worktree removed.` — or where and why it stopped);
form per `../_shared/references/summary-format.md`.

- The path that ran — local, or the PR (URL, method) — and **where the method came from**: `merge.md`, the
  default, or asked. Only when there is something to say: a file style the remote refused, a style `finish`
  doesn't know, a `wt merge` whose output shows the user's config overrode the file.
- Stacked: every PR merged together, the `gh stack merge` outcome, the PRs still open above.
- The gate verdict, or that it was skipped under the reuse rule and at which commit it passed.
- What merged into what, the resulting base HEAD, branch and worktree removed.
- Anything left in place on purpose — each with name or path and why — or "nothing left behind".
- Last line: what the user needs to do, or "nothing needed from you".

Follow `../_shared/references/git-safety.md`: never delete an unmerged branch or force-remove a dirty worktree
without an explicit request, never push to or force-update the base, confirm branch and tree state before each
irreversible step.
