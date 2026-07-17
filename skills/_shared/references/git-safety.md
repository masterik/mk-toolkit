# Git safety protocol

Shared by every mkit workflow skill. When a skill references "the git safety protocol", it means this.

- **NEVER** push directly to `main`/`master`, or to the repo's default branch.
- **NEVER** force-push (`--force`, `--force-with-lease`) unless the user explicitly asks.
- **NEVER** run destructive commands (`reset --hard`, `clean -fd`, `branch -D`, `worktree remove --force`) without an explicit request or a clear, stated reason.
- **NEVER** update git config.
- **NEVER** skip hooks (`--no-verify`, `wt ... --no-hooks`) unless the user asks.
- **NEVER** add AI/Claude co-authorship or attribution to commits, PR titles, or PR bodies (`Co-Authored-By: Claude`, "Generated with…", etc.).
- If a commit fails because of a hook, **fix the problem and make a NEW commit** — do not `--amend` a pushed commit or bypass the hook.
- Before any irreversible step (merge, branch delete, worktree removal), confirm the working tree is in the state you expect (`git status`) and that you are on the branch you think you are (`git branch --show-current`).
- When in doubt about a destructive or outward-facing action (push, merge, delete), state what you are about to do and proceed only if the user has authorized it — either now or as a standing instruction for this task.
