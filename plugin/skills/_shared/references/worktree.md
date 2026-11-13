# Worktree awareness

Shared by `finish`, `pr`, `commit`, `cleanup`. Git worktrees check out several branches at once in
sibling directories, and Claude Code agents commonly run inside one. The finishing skills must
clean up the right way for the one they are in.

`cleanup` is the one consumer that looks at every worktree in the repo rather than just the one it
is running in — `branch-status.sh` reports `origin=primary|claude-code|linked` per worktree,
the same three-way split as `facts.sh`'s `worktree_origin`, but as a table instead of a single answer.
The teardown below still applies row by row: never remove an `origin=claude-code` worktree with
plain `git worktree remove` if this session happens to be the one running inside it — hand it back
with **ExitWorktree** instead — and treat any other `claude-code` row (some other session's
worktree) as a reason to ask before touching it, not a reason to skip it silently.

## Ask `facts.sh`, not the layout

`facts.sh` already answered this (`output-discipline.md`):

```
linked=yes            git_dir != common_dir — this is not the primary checkout
worktree_origin=claude-code | linked | primary
cleanup_path=exit-worktree | git-worktree | none
primary=/Users/you/repo
```

There is deliberately no "is this a worktrunk worktree" answer: `wt list` enumerates *every* git
worktree in the repo and worktrunk's path template is configurable, so nothing can tell one from a
hand-made worktree — and nothing needs to. What differs is the teardown.

| `cleanup_path` | Meaning | Teardown |
| --- | --- | --- |
| `exit-worktree` | the harness's own (`/.claude/worktrees/`) | merge, then the **ExitWorktree** tool — never `git worktree remove` it from inside itself |
| `git-worktree` | any other linked worktree | `wt merge` when worktrunk is available (it respects the user's hooks and squash/rebase config); plain git, below, otherwise |
| `none` | the primary checkout | nothing to tear down |

worktrunk is optional. Check for it in the agent's own shell (`command -v wt`, or `type wt`):
its shell integration installs `wt` as a shell function, which a script does not inherit, so the
scripts never report it.

**Every one of these teardown paths stays fully capable.** `exit-worktree` in particular is not a
degraded fallback — it is the correct teardown for a supported environment, and a machine configured
differently from the author's depends on it.

## Writing from a worktree

**Your run directory is outside every worktree.** `facts.sh --run` makes it under `$TMPDIR`, so a
write of yours never targets the shared checkout. That is not a nicety: in a session Claude Code has
marked as worktree-isolated, every write to a path in the main checkout is refused — *"This session is
isolated in the worktree …; edit the worktree copy of this file instead of the shared-checkout path"*.

## worktrunk (`wt`)

Prefer worktrunk's own commands — they respect the user's hooks and config.

- Merge + clean up in one step: `wt merge [target]` — squash-rebases the current branch,
  fast-forwards the target (default = default branch), removes the worktree. Flags:
  `--no-squash`, `--no-commit`, `--no-rebase`, `--no-ff`, `--no-remove`, `--no-hooks`.
- Remove only: `wt remove [branch|path]` — removes the worktree, deletes the branch **if
  merged** (`--no-delete-branch` to keep, `-D` to force-delete unmerged, `-f` to discard a dirty
  worktree).
- `-y` skips approval prompts — only when the user authorized non-interactive completion.

### A pinned merge style vs. worktrunk's own config

`mkit repo profile --json` reports `merge_style` — pinned in `.mkit/config.toml`, or discovered
from this repo's git config. worktrunk carries its own `[merge]` block (`squash`, `commit`,
`rebase`, `remove`, `ff`, `verify`) in the user's `~/.config/worktrunk/config.toml`. The two can
disagree, so the decision is written down here rather than left to whichever ran last.

**A pinned style wins wherever a flag can carry it; worktrunk's config wins where none can, and
the skill says which happened.** `merge.style` is a committed, repo-wide fact about how this
project integrates a branch; worktrunk's `[merge]` block is one developer's preference. A personal
default quietly producing a history shape the repo does not use is the failure worth preventing.

| `merge_style` | what to run | why |
| --- | --- | --- |
| `squash` | `wt merge <base>` — no flag | squash-and-rebase **is** worktrunk's default; there is no `--squash` to force it back on |
| `rebase` | `wt merge <base> --no-squash` | keeps the individual commits, still rebases onto the target and fast-forwards it |
| `merge` | `wt merge <base> --no-squash --no-ff` | keeps the commits and asks for a merge commit instead of a fast-forward |

Two things make that the mapping rather than a fuller one:

- **`wt merge`'s flags are negative only** — `--no-squash`, `--no-commit`, `--no-rebase`,
  `--no-ff`, `--no-remove`, `--no-hooks` (checked against `wt merge --help`, worktrunk 0.77).
  They turn a default off; none turns one on. So a repo pinned to `squash` whose user set
  `merge.squash = false` cannot be corrected by a flag.
- **And not through the back door either.** `wt`'s global `--config-set <toml>` would set
  `merge.squash=true`, but it accepts a key that does not exist without a word of complaint
  (`wt list --config-set merge.bogus=true` exits 0) — a mistyped key is an override that looks
  applied and changes nothing. That is the one thing this bundle refuses to ship: a remedy that
  only looks like it worked.

So on `squash`, pass nothing and **read what `wt merge` reports**. If its output says it did not
squash, the user's worktrunk config overrode the repo's pin: say so in one line of the deliverable
and leave it. Do not re-run the merge to fight it, and never edit the user's worktrunk config.

If the repo itself carries `.config/wt.toml` with `merge.*` keys, that is a second **repo-wide**
statement, not a personal one. It either agrees with the pin or the two genuinely contradict each
other — report the contradiction and let the user settle it; do not pick a winner.

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
