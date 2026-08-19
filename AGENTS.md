# AGENTS.md

This file provides guidance to agents when working with code in this repository.

**mkit** — a **Claude Code plugin** packaging a cohesive kit of agent coding-workflow
skills (`commit`, `review-changes`, `finish-feature`, `create-pr`) plus the shared
`_shared/references/` bundle. Composition over replacement: the skills orchestrate `git`,
`gh`, `wt`, and code-review tools — no new git logic.

## Layout
- `.claude-plugin/plugin.json` — plugin manifest (skills auto-discovered from `skills/`).
- `.claude-plugin/marketplace.json` — marketplace entry (`source: "./"`).
- `skills/<name>/SKILL.md` — the four triggerable skills.
- `skills/_shared/` — shared references (no `SKILL.md`); the skills link into it via
  `../_shared/references/…`. **Keep those relative paths intact** — they're what makes the
  bundle portable.
- `scripts/run-open.sh` — opens this run's directory under `<git-dir>/mkit/`; every skill calls it
  before it logs anything, via `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh <skill>`.

## Conventions
- Skills are Markdown — no build step, no runtime. The lone executable is `scripts/run-open.sh`;
  add a script only for a mechanical invariant (see `concept.md`), never for a decision.
- Nothing project-specific is hardcoded: quality-gate commands, commit scopes, and reviewers
  are discovered from the target repo.

- Direction & roadmap: [`concept.md`](concept.md)
