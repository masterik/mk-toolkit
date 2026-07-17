# Worktree awareness

Shared by `finish-feature`, `create-pr`, and `commit`. Git worktrees let several branches be checked out at once in sibling directories. Claude Code agents commonly run inside one. The finishing skills must detect **which kind** of worktree they're in and clean up the right way.

## Detect the context

```bash
git rev-parse --is-inside-work-tree           # sanity: in a repo?
git branch --show-current                      # current branch
git rev-parse --show-toplevel                  # this checkout's root
git rev-parse --git-common-dir                 # shared .git dir — differs from --git-dir when in a linked worktree
git worktree list                              # all worktrees + their branches
```

You are in a **linked worktree** (not the primary checkout) when `git rev-parse --git-dir` and `git rev-parse --git-common-dir` resolve to different paths.

Identify the **primary worktree** (the one holding the default branch) from `git worktree list` — it's the entry whose path is the main repo root, usually on `main`/`master`.

## Three worktree origins — cleanup differs

### 1. worktrunk (`wt`)

Signals: `command -v wt` succeeds **and** a `.config/wt.toml` (or `~/.config/worktrunk/config.toml`) exists, or `git worktree list` paths match worktrunk's layout.

Prefer worktrunk's own commands — they respect the user's hooks and squash/rebase config:

- Merge + clean up in one step: `wt merge [target]` — squash-rebases the current branch, fast-forwards the target (default = default branch), then removes the worktree. Flags: `--no-squash`, `--no-ff`, `--no-remove` (keep worktree), `--no-hooks`.
- Remove only: `wt remove [branch|path]` — removes the worktree and deletes the branch **if merged** (`--no-delete-branch` to keep, `-D` to force-delete unmerged, `-f` to discard a dirty worktree).
- Pass `-y` to skip approval prompts only when the user has authorized non-interactive completion.

### 2. Claude Code agent worktree (`.claude/worktrees/`)

Signals: the worktree root path contains `/.claude/worktrees/`.

This is the harness's own isolation. Do **not** `git worktree remove` it by hand from inside itself. Instead:

- Merge the branch into the base (see "Merge back" below), then
- Hand control back with the **ExitWorktree** tool: `action: "remove"` to delete it after a successful merge, or `action: "keep"` to leave it. If a code change should ship as a PR instead, keep the worktree and use `create-pr`.

### 3. Plain `git worktree`

Manual git worktrees anywhere else. Clean up with git directly (see below).

## Merge back (plain git, when not delegating to `wt merge`)

You cannot `git checkout <base>` inside a worktree if the base branch is already checked out in the primary worktree. Two safe options:

- **Merge from the primary worktree** (preferred): run the merge in the primary checkout, targeting the feature branch —
  ```bash
  git -C <primary-worktree-path> merge --ff-only <feature-branch>   # or a real merge if ff isn't possible
  ```
- **Merge without checkout** using a ref update when fast-forward is valid:
  ```bash
  git fetch . <feature-branch>:<base>        # fast-forwards <base> to <feature> if <base> is an ancestor
  ```
  This fails (safely) if it isn't a fast-forward — fall back to a merge from the primary worktree.

Always fetch/update the base first so the merge is against current `origin/<base>` when a remote exists.

## Remove a plain git worktree

```bash
git worktree remove <path>          # fails if dirty — good; investigate before forcing
git branch -d <feature-branch>      # -d only deletes if merged; never -D without a reason
git worktree prune                  # tidy stale metadata
```

## Rules

- Determine the **base branch** before finishing: it's the branch the feature was cut from (often `main`). Confirm with the user if ambiguous.
- Never remove a worktree or delete a branch that still has **uncommitted changes** or **unmerged commits** without an explicit request.
- After removal, verify: `git worktree list` no longer shows it and `git branch` no longer lists the deleted branch.
- Respect the git safety protocol at all times.
