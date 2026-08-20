# Worktree awareness

Shared by `finish-feature`, `create-pr`, `commit`. Git worktrees check out several branches at once in
sibling directories, and Claude Code agents commonly run inside one. The finishing skills must detect
**which kind** and clean up accordingly.

## Detect the context

```bash
git rev-parse --is-inside-work-tree   # in a repo?
git branch --show-current             # current branch
git rev-parse --show-toplevel         # this checkout's root
git rev-parse --git-common-dir        # shared .git dir — differs from --git-dir inside a linked worktree
git worktree list                     # all worktrees + branches
```

- **Linked worktree** (not the primary checkout): `--git-dir` and `--git-common-dir` resolve differently.
- **Primary worktree**: the `git worktree list` entry whose path is the main repo root, usually on
  `main`/`master`. It holds the default branch.

## Three origins — cleanup differs

### 1. worktrunk (`wt`)

Signals: `command -v wt` succeeds **and** `.config/wt.toml` (or `~/.config/worktrunk/config.toml`) exists, or
`git worktree list` paths match worktrunk's layout.

Prefer worktrunk's own commands — they respect the user's hooks and squash/rebase config.

- Merge + clean up in one step: `wt merge [target]` — squash-rebases the current branch, fast-forwards the
  target (default = default branch), removes the worktree. Flags: `--no-squash`, `--no-ff`, `--no-remove`,
  `--no-hooks`.
- Remove only: `wt remove [branch|path]` — removes the worktree, deletes the branch **if merged**
  (`--no-delete-branch` to keep, `-D` to force-delete unmerged, `-f` to discard a dirty worktree).
- `-y` skips approval prompts — only when the user authorized non-interactive completion.

### 2. Claude Code agent worktree (`.claude/worktrees/`)

Signal: the worktree root path contains `/.claude/worktrees/`. This is the harness's own isolation.

Do **not** `git worktree remove` it by hand from inside itself. Merge into the base (below), then hand back
with the **ExitWorktree** tool: `action: "remove"` after a successful merge, `action: "keep"` to leave it. If
the change should ship as a PR instead, keep the worktree and use `create-pr`.

### 3. Plain `git worktree`

Manual worktrees anywhere else. Clean up with git directly (below).

## Merge back (plain git, not delegating to `wt merge`)

`git checkout <base>` fails inside a worktree when the base is checked out in the primary one. Two safe
options:

- **Merge from the primary worktree** (preferred):
  ```bash
  git -C <primary-worktree-path> merge --ff-only <feature-branch>   # or a real merge if ff isn't possible
  ```
- **Merge without checkout**, when a fast-forward is valid:
  ```bash
  git fetch . <feature-branch>:<base>        # ff's <base> to <feature> if <base> is an ancestor
  ```
  Fails safely when it isn't a fast-forward — fall back to the primary-worktree merge.

Fetch/update the base first, so the merge is against current `origin/<base>` when a remote exists.

## Remove a plain git worktree

```bash
git worktree remove <path>          # fails if dirty — good; investigate before forcing
git branch -d <feature-branch>      # -d only deletes if merged; never -D without a reason
git worktree prune                  # tidy stale metadata
```

## Rules

- Determine the **base branch** before finishing — the branch the feature was cut from (often `main`).
  Confirm with the user if ambiguous.
- Never remove a worktree or delete a branch with **uncommitted changes** or **unmerged commits** without an
  explicit request.
- After removal verify: `git worktree list` no longer shows it, `git branch` no longer lists the branch.
- Respect `git-safety.md` throughout.
