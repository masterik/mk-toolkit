---
name: cleanup
disable-model-invocation: true
description: >-
  Delete merged local branches and their worktrees, keep the `protected=` ones, then switch to one and pull.
  Run it as /mkit:cleanup. Local-only: never touches remote branches.
model: sonnet
---

# Clean up local branches and worktrees

Repo gardening, outside the feature line: `finish` tears down the one branch it merged; `cleanup` sweeps
*every* local branch and its worktree.

- Deletes local branches and their worktrees. **Never touches the remote**: no `push --delete`, no PR
  changes, no force-push. `git fetch --prune` (inside `branch-status.sh`) only updates remote-tracking refs.
- **Keeps exactly the `protected` rows** `branch-status.sh` reports — the default branch (always, whatever
  the keep list says), a local `develop`/`development`/`dev` (or the one `docs/agents/cleanup.md` names), and
  every kept name that exists locally. Never re-derive or second-guess that set.
- Ends on a kept branch, pulled from the resolved remote.

## Preconditions

Two calls; paths are relative to this skill's directory — resolve and run them absolute
(`../_shared/references/output-discipline.md`):

```bash
bash ../_shared/scripts/facts.sh cleanup
bash ../_shared/scripts/branch-status.sh --default <default_branch> [--develop <name>] [--keep <name>]...
```

**Kept branches** (`../_shared/references/repo-settings.md`): if `docs/agents/cleanup.md` exists, each
branch it keeps goes to the second call as one `--keep <name>` (names, not patterns; added to what is
protected, never substituted), and a develop branch it names as `--develop <name>`. No file → no `--keep`.

Either call fails → stop, show its stderr. `default_branch=unknown` → ask for it; pass `--default <name>`.

**Remote**: `remote_choice=ask` → ask *before* the second call, re-run the first with `--remote <answer>`, and
pass `--remote <answer>` to the second (`../_shared/references/remote.md`).

From `facts.sh`: `branch=`, `clean=`, `default_branch=`, `cleanup_path=` (this session's own worktree). From
`branch-status.sh`: `protected=` (for reading; the per-row `class` is exact), `develop=`, `keep=`,
`keep_unknown=` (kept names with no local branch — normal, mention once), `remote=`, `fetch=` (`failed` →
classify on what you have, say so), `gh=` (anything but `ok` → say so; PR-based classes rest on git alone),
the `branches:` and `worktrees:` tables, and `notes:` (report verbatim).

| `class` | means | handling |
| --- | --- | --- |
| `protected` | in `protected=` | never a candidate |
| `current` | checked out now | derive its real class (below) |
| `merged` | ancestor of the default or develop branch (`merged_into`); a kept branch is never a merge target | auto-delete |
| `merged-pr` | GitHub says its PR merged, verified against this branch's content | auto-delete, say how you know |
| `open-pr` / `closed-pr` | a PR open / closed unmerged | ask |
| `gone` | upstream deleted, no PR trail | ask |
| `unpushed` | never had an upstream | ask |
| `tracking` | live upstream, not merged, no conclusive PR | ask |

## Workflow

### 1. Classify

- **auto-delete**: `merged`/`merged-pr` with no worktree, or a `worktrees:` row with `clean=yes`.
- **ask**: `open-pr`, `closed-pr`, `gone`, `unpushed`, `tracking`, and any `merged`/`merged-pr` whose
  worktree is `clean=no` **or `clean=error`** (unreadable is not clean).
- **ask, whatever the class**: a worktree with `origin=claude-code` — some session's own, maybe in use.
- **never a delete target**: `clean=missing` (directory already gone; `git worktree prune` fixes it).

**`current`** is reported *instead of* its real class. Derive that from its `upstream`, `merged_into` and
`pr` columns in priority `merged` > `merged-pr` > `open-pr`/`closed-pr` > `gone` > `unpushed` > `tracking`,
sort it like any row, and flag it **"current — switch away first"**.

### 2. Plan and confirm

One screen:

```
Cleanup plan:  <n> branch(es) reviewed, keeping <protected list>
Currently on:  <branch> — <derived class> (will switch away first, if in scope)

Auto-delete (merged):
  <branch>  (merged into <target>[, worktree removed])

Needs your OK:
  <branch>  open PR #<n>, worktree at <path>
  <branch>  unpushed — 1 commit not on any remote, recoverable via reflog for a while

Left alone:
  <branch>  worktree is dirty — <path>
  <branch>  origin=claude-code worktree — may still be in use
```

Auto-delete needs no further confirmation. For **ask**: the host's question tool, one question per branch (or per
cluster sharing a reason), header = the branch cut to 12 characters, options `keep` (first, recommended
unless the reason is conclusive) and `delete`; more than four → further calls. Numbered-list fallback per
`../_shared/references/summary-format.md`. No answer = keep, named in the report.

### 3. Leave the current branch, if in scope

If the starting branch is flagged and approved (or auto-delete), `git switch <default>` first. `git switch`
refuses when the target is checked out in another worktree: try another protected branch; if none is free,
stop and say so — never force.

### 4. Delete — worktree first, then branch

Per approved branch:

1. **Worktree**, if any: this session's own with `cleanup_path=exit-worktree` → **ExitWorktree** (`action:
   "remove"`, `discard_changes: true`), never a remove from inside it; a host without it: `git -C "<primary>" worktree
   remove "<path>"`, then the rest from `<primary>`. Any other: `wt remove <branch>` when `wt` is available, else `git worktree remove <path>`.
   `--force` only for a `clean=no` worktree the user approved removing — say plainly it discards uncommitted
   work. Never an `origin=claude-code` worktree without explicit approval knowing what it is. `wt remove`
   deletes a merged branch itself: check `git show-ref --verify --quiet refs/heads/<branch>` before 4.2.
2. **Branch**, if it still exists: always `git branch -D <branch>` (`-d` checks against the current checkout,
   not this skill's evidence), stating the proof as you run it: `merged` → "verified: ancestor of
   `<merged_into>`"; `merged-pr` → "verified: PR `#<n>` merged"; ask bucket → the user's approval. Never `-D` a
   branch without one of these.

### 5. Switch and pull

Land on the default branch unless the user's request points at another kept branch (say which, and that it
was a judgement call). With `remote != none`:

```bash
git switch <landing-branch>
git pull --ff-only <remote> <landing-branch>
git fetch <remote> <other>:<other>
```

Pull and fetch only branches whose row has an `upstream` on `<remote>`; skip the rest silently (a kept branch
without a remote counterpart is normal — never report "couldn't find remote ref" as divergence). A fetch
`ref:ref` that fails (not a fast-forward, or checked out elsewhere) → the branch is ahead or diverged and
needs a manual look; never force. `remote=none` → skip both, say so; not a failure.

### 6. Verify

- `git branch -vv` — only `protected=` branches remain, none `ahead`/`behind` its upstream (no upstream is
  fine).
- `git worktree list` — every removed worktree gone; `git worktree prune` first if any `clean=missing`.

## Final report (always)

Form per `../_shared/references/summary-format.md`; the first line bold, as the verdict:

```
Cleanup done — <n> local branch(es) removed, only <protected list> remain.

Deleted (merged into <target>):
<branch>[, <branch> (#PR if merged-pr)], ...

Deleted (<reason>, per your choice):
<branch> — <open PR #n / unpushed, 1 commit, recoverable via reflog / ...>

Left alone:
<branch> — <dirty worktree / declined / claude-code worktree>

Note: <branch> is N commits behind <remote>/<branch> — <why it couldn't be fast-forwarded>
Note: the keep list names <name>, which is not a local branch in this checkout — nothing to protect here.
```

Omit empty sections except **Left alone** ("none"). End on what the user still has to decide, or "nothing
needed from you". Never report a branch deleted that was neither auto-delete nor approved.

Follow `../_shared/references/git-safety.md`: `-D` and `--force` are this skill's only work-losing commands,
and each needs a stated reason before it runs — a merge proof or the user's approval. Never touch the remote,
never update git config.
