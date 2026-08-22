# AGENTS.md

This file provides guidance to agents when working with code in this repository.

**mkit** — a **Claude Code plugin** packaging a cohesive kit of agent coding-workflow
skills (`commit`, `review`, `finish`, `pr`, `note`) plus the shared
`_shared/references/` bundle. Composition over replacement: the skills orchestrate `git`,
`gh`, `wt`, and code-review tools — no new git logic.

## Rules

- When reporting information, be _extremely concise_ — prioritize brevity over grammar or style.
- When writing documentation, be _clear and complete_, but prioritize concision over polished grammar.
- When creating plans, be thorough and actionable; describe *what* to do, not *how*, and omit code
  unless essential for clarity.

## Layout
- `.claude-plugin/plugin.json` — plugin manifest (skills auto-discovered from `skills/`).
- `.claude-plugin/marketplace.json` — marketplace entry (`source: "./"`).
- `hooks/hooks.json` — hook registration, at the **plugin root** (not under `.claude-plugin/`):
  `Stop` + `SubagentStop` → `scripts/hooks/journal-nudge.sh`. Auto-discovered, so the manifest
  carries **no `hooks` key** — don't add one.
- `skills/<name>/SKILL.md` — the five triggerable skills. `note` is the only one that isn't part of
  the edit → commit → review → integrate line: it records intent mid-implementation.
- `skills/_shared/` — shared references (no `SKILL.md`); the skills link into it via
  `../_shared/references/…`. **Keep those relative paths intact** — they're what makes the
  bundle portable.
- `scripts/` — the mechanical steps, called as `${CLAUDE_PLUGIN_ROOT}/scripts/<name>`:
  `facts.sh <skill>` (opens the run directory under `<git-dir>/mkit/` **and** returns every starting
  fact — every skill's first call), `run-open.sh` (the directory alone, plus `--prune`),
  `gate-detect.sh` / `gate-run.sh` (the quality gate), `findings.mjs` (reconcile/group/report over a
  review's findings), `journal.sh` (the commit journal: `add` a record of *why* a unit exists, then
  `status` / `uncovered` classify those records against the current tree — plus `drop`, `compact`,
  `enable`/`disable`/`enabled`/`path`), `lib/common.sh` (sourced helpers).
- `scripts/hooks/journal-nudge.sh` — the `Stop` / `SubagentStop` hook: names the dirty paths no
  journal entry covers and hands them back to the model. Gated (git repo, journaling enabled,
  `stop_hook_active` false, one nudge per `prompt_id` + `agent_id`, uncovered > 0) and **always
  exits 0**. It never authors a record.
- `<git-dir>/mkit/` — the scripts' scratch root: per-run directories (`run-open.sh`), plus
  `journal.jsonl` (append-only records) and the `journal.enabled` opt-in marker. Never committed,
  never in `git status`; a linked worktree gets its own, so entries die with the worktree.
- `PREREQUISITES.md` — required tooling (`git`, `bash`, `node`, `jq`), recommended (`rg`, `gh`, `wt`),
  and the permission allowlist that stops the scripts prompting.
- `tests/` — the script layer's own test suite, dev-only (never shipped as part of a skill run):
  `tests/run.sh` runs both — `node --test tests/findings.test.mjs` (built-in runner, no deps) and
  `bats tests/bats/` (one `.bats` file per shell script, each against a throwaway git repo).

## Conventions
- Skills are Markdown — no build step, no runtime. Scripts are bash (POSIX-ish; macOS ships bash 3.2
  and BSD `sed`/`date`, so no GNU-only flags) plus one dependency-free `.mjs`; never a compiled binary,
  since installing the plugin is a clone.
- Add a script only for a mechanical invariant (see `concept.md`), never for a decision. Where the line
  is unclear, report candidates and let the skill choose. The hook is held to the same line one step
  further out: it may compute the gap, never fill it.
- Scripts report and run; they never stage, merge, push or edit. They parse stable machine output
  (`--porcelain`, `--shortstat`/`--name-only`, `--format=json`) and never call `rtk`, which
  reshapes output for reading.
- Nothing project-specific is hardcoded: quality-gate commands, commit scopes, and reviewers
  are discovered from the target repo.

- Direction & roadmap: [`concept.md`](concept.md)
