# Git safety protocol

Shared by every mkit skill. "The git safety protocol" means this file.

- **NEVER** push directly to `main`/`master`/the repo's default branch.
- **NEVER** force-push (`--force`, `--force-with-lease`) unless the user explicitly asks.
- **NEVER** run destructive commands (`reset --hard`, `clean -fd`, `branch -D`, `worktree remove --force`)
  without an explicit request or a clear, stated reason.
- **NEVER** update git config.
- **NEVER** skip hooks (`--no-verify`, `wt … --no-hooks`) unless the user asks.
- **NEVER** add AI/Claude co-authorship or attribution to commits, PR titles or PR bodies
  (`Co-Authored-By: Claude`, "Generated with…").
- A commit that fails a hook: **fix the problem, make a NEW commit.** Never `--amend` a pushed commit, never
  bypass the hook.
- Before any irreversible step (merge, branch delete, worktree removal), confirm the tree is in the state you
  expect (`git status`) and you are on the branch you think (`git branch --show-current`).
- When in doubt about a destructive or outward-facing action (push, merge, delete), say what you are about to
  do and proceed only on authorization — given now or standing for this task.
