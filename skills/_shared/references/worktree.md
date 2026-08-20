# Worktree awareness

Shared by `finish`, `pr`, `commit`. Git worktrees check out several branches at once in sibling
directories, and Claude Code agents commonly run inside one. The finishing skills must clean up
the right way for the one they are in.

## Ask the script, not the layout

`facts.sh` already answered this (`output-discipline.md`):

```
linked=yes            git_dir != common_dir — this is not the primary checkout
worktree_origin=claude-code | linked | primary
cleanup_path=exit-worktree | wt | git-worktree | none
wt_lists_this=yes     worktrunk can see this repo
primary=/Users/you/repo
```

There is deliberately no "is this a worktrunk worktree" answer: `wt list` enumerates *every* git
worktree in the repo and worktrunk's path template is configurable, so nothing can tell one from a
hand-made worktree — and nothing needs to. What differs is the teardown.

| `cleanup_path` | Meaning | Teardown |
| --- | --- | --- |
| `exit-worktree` | the harness's own (`/.claude/worktrees/`) | merge, then the **ExitWorktree** tool — never `git worktree remove` it from inside itself |
| `wt` | a linked worktree, worktrunk present | `wt merge` — it respects the user's hooks and squash/rebase config |
| `git-worktree` | a linked worktree, no worktrunk | plain git, below |
| `none` | the primary checkout | nothing to tear down |

`wt_bin=none` is **advisory**: worktrunk's shell integration installs `wt` as a shell function,
which a script may not inherit even though the agent's own shell has it. Treat a missing `wt`
as "check before concluding", not as proof.

## worktrunk (`wt`)

Prefer worktrunk's own commands — they respect the user's hooks and config.

- Merge + clean up in one step: `wt merge [target]` — squash-rebases the current branch,
  fast-forwards the target (default = default branch), removes the worktree. Flags:
  `--no-squash`, `--no-ff`, `--no-remove`, `--no-hooks`.
- Remove only: `wt remove [branch|path]` — removes the worktree, deletes the branch **if
  merged** (`--no-delete-branch` to keep, `-D` to force-delete unmerged, `-f` to discard a dirty
  worktree).
- `-y` skips approval prompts — only when the user authorized non-interactive completion.

## Merge back with plain git

`git checkout <base>` fails inside a worktree when the base is checked out in the primary one.
Two safe options, and `facts.sh` already gave you `primary=`:

- **Merge from the primary worktree** (preferred):
  ```bash
  git -C <primary> merge --ff-only <feature-branch>   # or a real merge if ff isn't possible
  ```
- **Merge without checkout**, when a fast-forward is valid (`ff_from_base=yes`):
  ```bash
  git fetch . <feature-branch>:<base>
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

- Determine the **base branch** before finishing — the branch the feature was cut from (often
  `default_branch`). Confirm with the user if ambiguous.
- Never remove a worktree or delete a branch with **uncommitted changes** or **unmerged commits**
  without an explicit request. `facts.sh` reports `clean=` and `commits_ahead_of_base=`.
- After removal verify: `git worktree list` no longer shows it, `git branch` no longer lists it.
- Respect `git-safety.md` throughout.
