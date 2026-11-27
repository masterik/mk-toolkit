# ADR 0007 — Themes in their own repo, listed in this marketplace

**Status:** accepted · **Date:** 2026-11-26

## Context

Claude Code's desktop and mobile apps take themes from a *mod*: a plugin whose hooks module calls
`$.theme.set`. A Catppuccin theme was built here first, as `catppuccin/` beside `plugin/`, released
with mkit's tag. mkit can't hold it in `plugin/` (no hooks since 0.15.0; Markdown + two scripts,
ADR 0006), and a theme has its own release cycle: lockstep forced an mkit tag for every color tweak
and a theme version jump to mkit's.

## Decision

1. **Own repo: `masterik/mk-mods`**, one plugin per top-level folder (`catppuccin/`), its own CI
   (`claude plugin validate` + `test` per theme), and one version for the whole repo (every theme's
   `plugin.json`, tag `vX.Y.Z`, independent of mkit's) with its own GitHub Releases. Its `AGENTS.md`
   holds its rules.
2. **Same marketplace.** This repo's `.claude-plugin/marketplace.json` lists each theme as a
   `git-subdir` source: `{"source": "git-subdir", "url": "masterik/mk-mods", "path": "<theme>"}`.
   Install stays `/plugin install catppuccin@masterik`. No `ref`: users get `mk-mods`' default
   branch, and a `plugin.json` version bump (a mk-mods release) triggers their update. Pinning a `ref`
   to a tag would gate delivery on releases but cost an mk-toolkit commit per release; not taken.
3. **Nothing here builds, tests or releases a theme.** `release.sh`, `just ci` and CI cover mkit only.
4. **Claude Code only.** No Codex entry (`.agents/plugins/marketplace.json` stays mkit-only). Copilot CLI
   reads the same `.claude-plugin/marketplace.json`, so it lists `catppuccin` too; it runs no hooks
   modules, so the theme does nothing there. Accepted: the listing is harmless, a Copilot-only file isn't worth it.

## Consequences

- The marketplace lists plugins from two repos; `pluginroot` matches mkit by manifest name, unaffected.
- A new theme = a folder in `mk-mods` + one entry here.
- `mk-mods` must exist and hold the folder before an entry here ships, or that install fails.
