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

## Conventions
- Skills are Markdown only — no build step, no runtime.
- Nothing project-specific is hardcoded: quality-gate commands, commit scopes, and reviewers
  are discovered from the target repo.

- Direction & roadmap: [`concept.md`](concept.md)
