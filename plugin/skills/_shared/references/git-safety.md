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

## Pin the git calls whose output you parse or judge

`mkit facts` reports **`git_bin=`** — the absolute path of the real git binary. Any git call whose output
a skill *parses or judges* uses it, with the repository root spelled out:

```
"$git_bin" -C "$toplevel" diff --cached -- "<path>"
"$git_bin" -C "$toplevel" status --porcelain
"$git_bin" -C "$toplevel" log --oneline <base>..HEAD
```

**Keep the quotes when you substitute.** These are recipes an agent fills in and runs literally, and
both values are arbitrary paths: a checkout under `~/My Projects/thing` splits `-C` across two operands,
and git then fails with `cannot change to '/Users/you/My'` or — worse — succeeds against the wrong root.
Same for every `<path>` and every scratch file. `mkit facts` emits one pair per line precisely so a value
containing a space survives being read; dropping the quotes here throws that away at the point of use.

Both sides of this are measured, and both bite.

- **A hook can reshape what you read.** A `PreToolUse` hook may rewrite `git status --short` into a
  wrapper that compacts output for reading — it strips leading spaces from `git diff --stat`, for one.
  A summarized status reads exactly like the tree it summarizes, so you can approve a commit having
  read only a digest of it. That is the failure this rule exists to prevent, and it is why the rule is
  about *parsed* output and not about git in general.
- **A wrapper is refused outright in an isolated session.** The worktree-isolation guard refuses any
  launcher it cannot read a git target through — *"runs rtk with a git command among its operands:
  what runs it, and from which directory or root, cannot be read here"* — regardless of operand order.
  An absolute binary path is **not rewritten by the hook at all**, so the command text reaches the
  guard as plain git and is allowed.

**Convert:** the staged-diff reads, `status`, and `log`. **Leave readable:** illustrative examples in
skill prose, and calls whose output you neither parse nor judge. A payload whose every example is an
absolute invocation stops being reviewable, and reviewability is the reason the skills are files.

## Never assemble a program at runtime

`python3 - <<'PY'`, `node -e`, `bash -c` over a generated string: the isolation guard refuses these for
the same reason it refuses a wrapper — *"feeds python a program assembled at runtime … so what it runs
cannot be shown not to be git"*. Any recipe built on a heredoc'd interpreter is dead on arrival in an
isolated session, which is exactly the kind of session these skills are driven from.

Use `mkit`'s own commands, plain git plumbing, or a file written to `tmp=` and then read by a named
tool. Never improvise a helper into the target repository's working tree — see `output-discipline.md`.
