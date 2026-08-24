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
  `SessionStart` → `scripts/hooks/session-bootstrap.sh`, and `Stop` + `SubagentStop` →
  `scripts/hooks/journal-nudge.sh`. Auto-discovered, so the manifest carries **no `hooks` key** —
  don't add one. No `matcher` on any of them: a mistyped matcher is a hook that silently never
  runs, and both hooks are cheap and re-entrant enough that filtering buys nothing.
- `skills/<name>/SKILL.md` — the five triggerable skills. `note` is the only one that isn't part of
  the edit → commit → review → integrate line: it records intent mid-implementation.
- `skills/_shared/` — shared references (no `SKILL.md`); the skills link into it via
  `../_shared/references/…`. **Keep those relative paths intact** — they're what makes the
  bundle portable.
- `scripts/` — the mechanical steps, called as `${CLAUDE_PLUGIN_ROOT}/scripts/<name>`:
  `facts.sh <skill>` (opens the run directory under `<git-dir>/mkit/` **and** returns every starting
  fact — every skill's first call), `run-open.sh` (the directory alone, plus `--prune`),
  `gate-detect.sh` / `gate-run.sh` (the quality gate — `gate-run.sh` also **writes** the gate ledger, one
  record per finished step; `gate-detect.sh` **reads** it back and annotates the commands it proposes with
  `fast_cache=` / `full_cache=` / `gate_fingerprint=`, or one `gate_cache=off|empty|no-hash|no-jq` cause.
  Neither ever skips a step: the skill owns that trade-off and must label a skipped step `cached`.
  `--no-ledger` / `--no-cache` are the escape hatches), `findings.mjs` (reconcile/group/report over a
  review's findings), `journal.sh` (the commit journal: `add` a record of *why* a unit exists, then
  `status` / `uncovered` classify those records against the current tree — plus `drop`, `compact`,
  `enable`/`disable`/`enabled [--why]`/`path`, resolved repo-marker > repo-tombstone > user default), `lib/common.sh` (sourced helpers, including
  `mkit_tree_fingerprint` — the staging- and commit-invariant hash of the content a gate command reads,
  which is what makes a `review` → `finish` cache hit possible at all).
- `scripts/hooks/journal-nudge.sh` — the `Stop` / `SubagentStop` hook: names the *count* of dirty
  paths no journal entry covers and points the model at `journal.sh uncovered` for the list,
  rather than inlining it — that list is additionalContext, which the transcript always renders in
  full. Gated (git repo, journaling enabled, `stop_hook_active` false, one nudge per `prompt_id` +
  `agent_id`, uncovered > 0) and **always exits 0**. It never authors a record.
- `scripts/hooks/session-bootstrap.sh` — the `SessionStart` hook: makes install.sh's setup happen
  by itself. Writes `journal.default` + the `mkit-journal` wrapper, idempotently, then emits
  **zero bytes** on every later session. Gated (absolute user dir, no `bootstrap.disabled`
  tombstone, a pending write or an unsaid message) and **always exits 0**, never to stderr.
  Deliberately: it does not parse its stdin (nothing in the payload is relevant, and parsing
  would need `jq` — the tool it must be able to report as missing, hence `mkit_json_escape`
  instead), never touches a repo or calls git, does not refuse to write when a prerequisite is
  missing (it withholds the *notice* instead), and never touches a `mkit-journal` it did not
  generate. A stale wrapper — the plugin moved or a marketplace version bumped — is rewritten
  silently; this hook is the only component that runs from the new plugin root.
- `<git-dir>/mkit/` — the scripts' scratch root: per-run directories (`run-open.sh`), plus
  `journal.jsonl` (append-only records), the `journal.enabled` / `journal.disabled` opt-in markers, and `gate.jsonl` (the gate
  ledger, append-only, rotated back to the newest 200 records once it passes 400). Never committed, never in `git status`; a linked
  worktree gets its own, so entries die with the worktree. `--prune` only ever removes `<skill>-*`
  **directories**, which is what keeps both `.jsonl` files out of its range.
- `install.sh` — plugin-root, user-scoped setup. **Not needed for setup any more**: the
  `SessionStart` hook writes the same two files itself. It survives for the three jobs a hook
  cannot do — `--status` (the diagnostic surface, which exists precisely so the hook never has to
  be one), `--uninstall` (the only global opt-out, and the thing that writes the tombstone), and
  `--bin <dir>` (a location the hook will not guess). Plus `--no-bin`, `--force`, and
  `--uninstall --purge` (drop the tombstone too, accepting that the next session re-creates
  everything). It sources `lib/common.sh` — the wrapper generator especially must have exactly
  one producer, or the hook's "is this wrapper mine?" test rots; `tests/bats/install.bats` has a
  byte-identical-wrapper test guarding that. It never edits a shell rc and never touches a repo.
  The **gate ledger needs nothing**: `gate-run.sh` already writes `gate.jsonl` everywhere, so
  install.sh only reports whether `jq` + sha256 are present.
- `~/.claude/mkit/` — the one piece of state outside a repo, overridable with `MKIT_HOME` (which the bats
  suite sets, so a developer who ran install.sh doesn't fail the "pristine repo is disabled" assertions).
  Holds `journal.default` (the global on-switch), `bootstrap.disabled` (the tombstone that makes an
  uninstall outlive the session — absent files carry no provenance, so "never set up" and
  "deliberately removed" are otherwise indistinguishable) and `bootstrap.state` (one key per line:
  which one-time messages have been said. Self-heals — a `prereq/` key is dropped once the tool is
  back, so a later removal warns again. No prune block: the key space is fixed, unlike
  journal-nudge's). `MKIT_BIN` overrides the wrapper's directory, for the same test-isolation
  reason as `MKIT_HOME`.
- `PREREQUISITES.md` — required tooling (`git`, `bash`, `node`, `jq`), recommended (`rg`, `gh`, `wt`),
  and the permission allowlist that stops the scripts prompting.
- `tests/` — the script layer's own test suite, dev-only (never shipped as part of a skill run):
  `tests/run.sh` runs both — `node --test tests/findings.test.mjs` (built-in runner, no deps) and
  `bats tests/bats/` (one `.bats` file per shell script, each against a throwaway git repo).
  `helpers.bash` sandboxes `MKIT_HOME` for every suite; the two setup suites also call
  `mkit_sandbox_home` (which redirects `HOME` and `MKIT_BIN`) because they write an executable,
  and `mkit_fake_path <tool>…` builds a PATH missing only the named tools — it must include
  `bash`, or `env PATH=… bash -c` exits 127 with empty output, which reads exactly like a hook
  correctly staying silent.

## Conventions
- **macOS only.** No script detects or branches on an OS; they are written to what macOS provides,
  which is the narrower target. Scripts are bash (POSIX-ish; macOS ships bash 3.2 and BSD
  `sed`/`date`, so no GNU-only flags, and no `flock`) plus one dependency-free `.mjs`; never a
  compiled binary, since installing the plugin is a clone. The user's interactive shell being zsh
  is irrelevant — every script carries `#!/usr/bin/env bash`. Don't add a GNU fallback "for
  portability": it is untested surface for a platform that isn't supported.
- Add a script only for a mechanical invariant (see `concept.md`), never for a decision. Where the line
  is unclear, report candidates and let the skill choose. The hook is held to the same line one step
  further out: it may compute the gap, never fill it.
- Scripts report and run; they never stage, merge, push or edit. They parse stable machine output
  (`--porcelain`, `--shortstat`/`--name-only`, `--format=json`) and never call `rtk`, which
  reshapes output for reading.
- Nothing project-specific is hardcoded: quality-gate commands, commit scopes, and reviewers
  are discovered from the target repo.

- Direction & roadmap: [`concept.md`](concept.md)
