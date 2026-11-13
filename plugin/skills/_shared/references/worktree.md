# Worktrees: merge back and tear down

Read by `finish` (local merge) and `cleanup`. `facts.sh` already says where you are:

```
linked=yes                 not the primary checkout
worktree_origin=claude-code | linked | primary
cleanup_path=exit-worktree | git-worktree | none
primary=/path/to/primary/checkout
```

| `cleanup_path` | Meaning | Teardown |
| --- | --- | --- |
| `exit-worktree` | the harness's own (`.claude/worktrees/`) | merge, then the **ExitWorktree** tool (`action: "remove"`) — never `git worktree remove` it from inside itself |
| `git-worktree` | any other linked worktree | `wt merge` / `wt remove` when worktrunk is available; plain git otherwise |
| `none` | the primary checkout | nothing to tear down |

worktrunk is optional: check `command -v wt` (or `type wt` — it may be a shell function) in your own shell.
Every path is fully capable; none is a degraded fallback. Your run directory is under `$TMPDIR`, outside
every worktree.

## Update the base first

When a remote exists: `git fetch <remote>`, and `git pull --ff-only <remote> <base>` where the base is
checked out, so you merge onto the current base.

## Local merge, by `cleanup_path`

**`git-worktree`, worktrunk available** — `wt merge <base>` merges, fast-forwards the base, removes the
worktree and deletes the branch, firing the user's hooks. Add `-y` only when non-interactive completion is
authorized; `--no-remove` only on request. The repo's style (`docs/agents/merge.md`) goes on the command
line where a flag can carry it:

| style in `merge.md` | run |
| --- | --- |
| no file | `wt merge <base>` — worktrunk's own config, unchallenged |
| `squash` | `wt merge <base>` — squash is worktrunk's default; no flag forces it back on |
| `rebase` | `wt merge <base> --no-squash` |
| `merge` | `wt merge <base> --no-squash --no-ff` |

`wt merge`'s flags only turn defaults off, and `--config-set` accepts unknown keys silently — never use it,
never edit the user's worktrunk config. If `wt merge`'s output shows it did not do what `merge.md` asked (the
user's config overrode it), say so in one line and leave it. A repo `.config/wt.toml` with `merge.*` keys that
contradicts `merge.md` → report the contradiction, let the user settle it.

**`exit-worktree`** — merge into the base from `primary=` (below), then **ExitWorktree** (`action:
"remove"`).

**`git-worktree`, no worktrunk:**

```bash
git -C "<primary>" merge --ff-only <feature-branch>   # a real merge if it can't fast-forward
git worktree remove "<toplevel>"
git branch -d <feature-branch>
git worktree prune
```

**`none`:**

```bash
git switch <base>
git merge --no-ff <feature-branch>   # or --ff-only for linear history, if it fast-forwards
git branch -d <feature-branch>
```

On the plain-git paths the style decides the merge command: `merge` → `git merge --no-ff` · `rebase` →
rebase the feature branch onto `<base>`, then `--ff-only` · `squash` → `git merge --squash`, then one
commit. No file → the commands as written. Say which you used.

**Merging from inside a linked worktree.** `git switch <base>` fails there when the base is checked out in
the primary. Merge from the primary (`git -C "<primary>" merge …`), or, when `ff_from_base=yes`, without a
checkout: `git fetch . <feature-branch>:<base>` (fails safely when not a fast-forward).

## Rules

- Never remove a worktree or delete a branch with **uncommitted changes** or **unmerged commits** without an
  explicit request. `git worktree remove` failing on a dirty tree is correct: investigate before `--force`.
- `-d`, never `-D`, without a stated reason (a confirmed merge is one).
- An `origin=claude-code` worktree that is not this session's may be another session's: ask before
  touching it.
- After removal verify: `git worktree list` no longer shows it, `git branch` no longer lists it.
