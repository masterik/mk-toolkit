# ADR 0004 — Worktree helpers live in the binary; `wt` stops being a dependency

**Status:** proposed · **Date:** 2026-10-01 ·
**Amends** "composition over replacement" for worktrees only; `git`, `gh` and the review tools stay composed.

## Context

Skills and docs compose `wt` (worktrunk) for worktree creation and removal. Three things make that
costly:

1. **Where worktrees live is decided twice.** `branchstatus` hardcodes `/.claude/worktrees/` to
   tell a Claude Code worktree from a linked one, while `wt`'s `worktree-path` and Claude Code's
   own location decide where they actually land. Moving them to `<repo>/.worktrees` (a repeated
   user wish) makes the classifier wrong with no error.
2. **`wt` is a second install and a second config.** It is a separate Rust tool with its own user
   config, hook approvals and Claude plugin. A skill that needs a worktree must first find out
   whether `wt` is present, configured and approved. `backlog.md` already records it as
   sandbox-fragile.
3. **Claude Code has no path setting.** The CLI has no `worktree.*` key for location; the only
   lever is a `WorktreeCreate`/`WorktreeRemove` hook, and today the one that fills it is `wt`'s
   plugin. The desktop app's own "Worktree location" setting does not cover CLI runs.

The need is small: a path rule and three mutations. It is not wt's hook, approval, merge, LLM-commit
or shell-integration surface.

## Decision

**1. A `mkit worktree` verb group, the only mutating group.**
`path <branch>`, `add <branch>`, `remove <branch>`, plus `hook create|remove` — a stdin/stdout
entrypoint for Claude Code's `WorktreeCreate`/`WorktreeRemove` hooks. Every verb has `--json`, no
TUI, no prompts. `add` and `remove` mutate; that is a deliberate exception to "commands report and
run, never edit", scoped to this group and recorded in `AGENTS.md`.

**2. One path rule, in repo config.** `[worktree] path` in `.mkit/config.toml`, a template over
`{branch}` (sanitized) and `{repo}`; default `.claude/worktrees/{branch}`. `branchstatus` derives
the `claude-code` origin from the same producer instead of a literal. mkit does **not** read
`wt`'s config: two sources would drift, and a user keeping `wt` simply does not enable its plugin.

**3. Safe by construction.** `add` calls `scratch.EnsureIgnored` and excludes the worktree
location before the first create. `remove` refuses a dirty tree, reusing `branchstatus`'s `Clean`
check (`error` is never `yes`), and never deletes a branch. No force flag in the first cut.

**4. Out of scope, permanently.** Hooks, approvals, template language beyond the two variables,
`merge`, `step`, LLM commit messages, CI status, statusline, shell integration, and any `wt`
config import. This is helpers, not parity.

**5. Skills drop `wt`.** `finish` and `cleanup` call `mkit worktree …`. `mkit doctor` stops
treating `wt` as a prerequisite and may report it as optional.

## Consequences

- One definition of "where worktrees live", read by the hook, the classifier and the skills.
- Claude Code's CLI worktrees can land in `.worktrees/` via a hook entry:
  `{ "hooks": { "WorktreeCreate": [ { "hooks": [ { "type": "command", "command": "mkit worktree hook create" } ] } ] } }`
  — written by the user, not by `mkit init` (ADR 0001/0003: mkit does not edit settings files).
- Users of `wt` lose nothing; the two coexist if the user does not enable `wt`'s Claude plugin.
- `mkit` now owns a mutation path; the write-site allowlist test and the dirty-tree guard carry the
  safety argument.
- Roughly one week including tests and the skill/doc pass.

## Open questions

- **Hook payload.** The `WorktreeCreate` stdin schema (assumed `base_path`, `branch`; Claude Code
  passes an agent id as `name` for `isolation: "worktree"`) must be verified against the hooks
  reference before the parser is written, and the agent-id naming decided.
- **Desktop app.** Whether the app's session-in-worktree path honours the hook is unverified; its
  separate "Worktree location" setting may bypass mkit.
- **Branch collision and nesting.** Branch names containing `/` and sanitization must be
  injective, as `worklog.FileName` already has to be.
