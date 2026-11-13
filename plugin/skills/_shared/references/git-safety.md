# Git safety protocol

Shared by every mkit skill. "The git safety protocol" means this file.

- **NEVER** commit or push directly to `main`/`master`/the repo's default branch.
- **NEVER** force-push (`--force`, `--force-with-lease`) unless the user explicitly asks.
- **NEVER** run destructive commands (`reset --hard`, `clean -fd`, `branch -D`, `worktree remove --force`)
  without an explicit request or a clear, stated reason.
- **NEVER** update git config.
- **NEVER** skip hooks (`--no-verify`, `wt … --no-hooks`) unless the user asks.
- **NEVER** add AI/Claude co-authorship or attribution to commits, PR titles or PR bodies
  (`Co-Authored-By: Claude`, "Generated with…").
- A commit that fails a hook: **fix the problem, make a NEW commit.** Never `--amend` a pushed commit.
- Before any irreversible step (merge, branch delete, worktree removal), confirm the tree is in the state you
  expect (`git status`) and you are on the branch you think (`git branch --show-current`).
- A destructive or outward-facing action (push, merge, delete): say what you are about to do and proceed only
  on authorization — given now or standing for this task.

## Pin the git calls whose output you parse or judge

Any git call whose output you *parse or judge* — the staged-diff reads, `status`, `log` — uses the
**absolute path of the real git binary** with the repository root spelled out. Resolve it once per session
(`command -v git`, e.g. `/usr/bin/git`) and substitute it for `<git>`:

```
"<git>" -C "<toplevel>" diff --cached -- "<path>"
"<git>" -C "<toplevel>" status --porcelain
"<git>" -C "<toplevel>" log --oneline <base>..HEAD
```

**Keep the quotes when you substitute**: `<git>`, `<toplevel>` and `<path>` are arbitrary paths and may hold
spaces. Calls whose output you neither parse nor judge stay plain `git`.

## Shell shape

Inline commands are plain `git`/`gh` calls: no `$(…)`, no pipes, no heredocs, no `&&` chains — one command
per call, its output read from the result. Never assemble a program at runtime (`python3 - <<'PY'`,
`node -e`, `bash -c` over a generated string): an isolation guard refuses it. Use the bundled scripts, plain
git plumbing, or a file written to `$TMPDIR` and read by a named tool. Never write a helper into the target
repository's working tree.

## Branches

The default branch is production-ready: work happens on a feature branch. A repo's own convention
(`CONTRIBUTING.md`, the prevailing `git branch -a` pattern) wins; otherwise assume `feat/*`, `bugfix/*`,
`hotfix/*` off the default. A branch prefix can hint at the commit type, but **the diff decides** type and
scope.
