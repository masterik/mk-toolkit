# ADR 0007 — A second plugin in the marketplace: `catppuccin`

**Status:** accepted · **Date:** 2026-11-26

## Context

Claude Code's desktop and mobile apps take themes from a *mod*: a plugin whose hooks module calls
`$.theme.set`. A Catppuccin theme (Latte light, Frappé/Macchiato/Mocha dark, 14 accents, like the
VS Code port) is wanted, shipped from this repo. `plugin/` (mkit) can't hold it: no hooks, no `hooks`
key (0.15.0), payload = Markdown + two scripts (ADR 0006), and a theme is unrelated to the workflow.

## Decision

1. **Own plugin, own folder.** `catppuccin/` at the repo root, listed second in the repo-root
   `.claude-plugin/marketplace.json` (`source: "./catppuccin"`). Install: `/plugin install catppuccin@masterik`.
   mkit's invariants (no hooks, two scripts, skills only) are untouched and keep applying to `plugin/` alone.
2. **Claude Code only.** No `.codex-plugin/`, no `.agents/plugins/marketplace.json` entry: Codex and
   Copilot run no hooks modules.
3. **Own version, outside the release.** `tools/release.sh` bumps `plugin/` only; `catppuccin`'s
   `plugin.json` version is bumped by hand. Tags stay mkit's (plugin + binary lockstep).
4. **Own tests, not in `just ci`.** `catppuccin/tests/*.test.ts` run with `claude plugin test catppuccin`
   (needs the `claude` CLI); `claude plugin validate catppuccin` checks the module. CI stays Go + bats.
5. **`make-colors.js` is vendored verbatim** from Claude Code's plugin-authoring kit; re-copy it when
   the build updates, then rerun the tests (contrast rules move with the design system).

## Consequences

- The marketplace is no longer single-plugin; `pluginroot` already matches by manifest name, not path.
- Engine-written `catppuccin/.claude-plugin/types/` is git-ignored.
