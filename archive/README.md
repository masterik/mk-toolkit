# Archive — previous ForgeZ concept

Everything here belongs to the **earlier ForgeZ direction** (a standalone `fz` CLI +
TUI app) and its exploratory prototypes. None of it is wired into the current project.
ForgeZ is now a **Claude Code plugin — a set of skills**; see
[`../docs/concept.md`](../docs/concept.md) for the current direction.

Kept in git history for reference; this whole folder is removed from the working tree in a
follow-up commit (recover with `git checkout <sha> -- archive/`).

## Contents
- `initial/` — earliest notes: `commit.md` and the `fz-function.sh` shell prototype.
- `phase-0.md`, `phase-1.md`, `use-cases.md` — the `fz` CLI specs (lifecycle commands,
  worktree support, use cases).
- `poc/` — TUI stack prototypes (Bun + openTUI core & React, bun-rezi, Go + charm,
  Rust + ratatui).
- `poc-tech-stack-eval.md` — the write-up comparing those stacks.
- `plans/` — earlier Bun-workspace / openTUI CLI and stack-eval plans.
- `package.json`, `tsconfig.json`, `skills-lock.json` — Bun/TypeScript build config and
  external-skill lockfile from the TUI stack (unused by a Markdown-only plugin).
