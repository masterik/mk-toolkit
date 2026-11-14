# ADR 0006 — Standalone skills, optional binary

**Status:** accepted · **Date:** 2026-11-13 · **Epic:** [#59](https://github.com/masterik/mk-toolkit/issues/59)
**Supersedes** [ADR 0001](0001-per-repo-config-and-init.md) (per-repo config, `mkit init`) and
[ADR 0002](0002-state-locations-under-a-sandbox.md) (state locations).
**Amends** [ADR 0003](0003-two-distribution-channels.md): `plugin/` may now carry two scripts.
**Withdraws** [ADR 0004](0004-worktree-helpers-in-the-binary.md) (proposed, never built).
**Amends** [ADR 0005](0005-one-remote-per-repo.md): the `[remote] name` pin in `.mkit/config.toml` is
gone (an optional `docs/agents/remote.md` takes its place); the resolution order without it stands.

## Context

Since M4–M7 every skill but `explain` stops when `mkit` is absent. The binary carries the starting
facts, the gate ledger and fingerprint, the worklog, the findings pipeline, `.mkit/` and its ignore
pair, `mkit init` with `.mkit/config.toml`, and pinning. That costs three things:

1. **Tokens.** Skill text plus shared references run ~15K tokens a run (~133 KB of skills, ~120 KB
   of references), much of it explaining binary contracts the model then re-judges anyway.
2. **Reach.** The binary is darwin-only and Homebrew-only. Codex, Copilot and Windows users cannot
   run a single skill.
3. **Code that buys no determinism.** Gate (1,628 non-test lines), findings (1,560) and worklog
   (508) make steps deterministic that the model re-decides: a cached gate result is re-checked,
   findings are re-read before acting, the worklog is one input among several for `recap`.

The principle stays: **judgement in Markdown, mechanics in code**. What changes is the bar for code:
it stays only where it replaces many tool calls (`facts`, `branch-status`).

Research on what Codex and Copilot support (skill discovery, bundled scripts, subagents, structured
questions, sandbox, Windows) is in the [#54 comment](https://github.com/masterik/mk-toolkit/issues/54),
written up as [`other-agents.md`](../ideas/other-agents.md). The portability decisions below rest on it.

## Decision

**1. Skills stand alone.** All eight skills run without the binary. The "binary absent (or too old)
= stop with a `brew` remedy" rule is gone.

**2. The binary is optional.** A macOS + Claude Code power tool with only `doctor`, `audit sandbox`,
`cache prune` and `version`. It still releases in lockstep with the plugin (one tag for both).

**3. Two bundled scripts.** `facts.sh` and `branch-status.sh` live in
`plugin/skills/_shared/scripts/`, reached through paths relative to `SKILL.md`: no target agent sets a
skill-root env var in a skill body (`CLAUDE_PLUGIN_ROOT` and `PLUGIN_ROOT` reach hooks and MCP config
only). Installing means taking the whole plugin. Both target **bash 3.2**, are tested with bats, and
**never read config**.

**4. The script contract is internal.** `facts` output may change freely, provided the script and
every skill that parses it change in the same commit. Byte-for-byte compatibility with the old
binary's `key=value` text is dropped.

**5. No per-repo runtime state.** `.mkit/` is gone: no run dirs under the work tree, no ignore pair,
no `info/exclude` write. Per-run files live in a temp dir that `facts.sh --run` creates.

**6. No gate ledger, no worklog, no findings pipeline.**
- Gate detection is prose (a precedence table); a gate result is reused only within a session.
- `recap` reads the session, git history and the PR.
- Findings are deduplicated in the main context, in prose.

**7. Config is prose, mattpocock style.**
- Lazy `docs/agents/{commit,review,merge,remote,gate,cleanup}.md`, listed under a `### mkit workflow`
  sub-block of `## Agent skills` in `AGENTS.md`, written by a prompt-driven `setup` skill.
- The spec store reuses `docs/agents/issue-tracker.md`; mattpocock's own sections are left alone.
- `.mkit/config.toml` pinning is gone. A committed `docs/agents/remote.md` is a repo setting like the
  others, read before `origin` → lone remote → ask; a missing file means "detect or ask". An answer
  given in a session is never persisted.
- The remote choice is applied by the skill, which re-runs `facts.sh --remote <name>`.

**8. Worktrees.** `wt` is optional; skills use `git worktree`. ADR 0004 is withdrawn.

**9. Portability.**
- Targets: Claude Code, Codex and Copilot, from one plugin tree with a manifest per agent.
- On Windows, skills call `bash <script>` (Git for Windows). Inline snippets are plain `git`/`gh`
  calls: no `$(…)`, pipes or heredocs, so PowerShell runs them unchanged.

## What `plugin/` may run (amends ADR 0003)

ADR 0003 and `AGENTS.md` said the plugin is **Markdown only; nothing runs**. That becomes: Markdown,
manifests, and **exactly two scripts**, `_shared/scripts/facts.sh` and `_shared/scripts/branch-status.sh`.
Still no hooks, no `hooks` key, no per-skill scripts, no other executables. A third script needs a
new ADR. ADR 0003's other decisions stand: Homebrew ships the binary only, the marketplace ships the
payload.

## Status of earlier ADRs

| ADR | Status | What survives |
| --- | --- | --- |
| [0001](0001-per-repo-config-and-init.md) | superseded | "Config is an input, never a permission" and "discovery recognises formats it did not write", now carried by decision 7 |
| [0002](0002-state-locations-under-a-sandbox.md) | superseded | `$TMPDIR` for per-run files; working remedy sentences; detect at first call |
| [0003](0003-two-distribution-channels.md) | amended | everything but "Markdown only" |
| [0004](0004-worktree-helpers-in-the-binary.md) | withdrawn | nothing |
| [0005](0005-one-remote-per-repo.md) | amended | one remote for push and PRs; `origin` → lone remote → ask; session answers passed, never persisted; `gh` told the repo |

## Consequences

- Each run reads ~5K tokens of skill text instead of ~15K (the prose diet, #47, finishes this).
- Skills run on Codex, Copilot and Windows (Git Bash), not only macOS + Claude Code.
- A Go-side invariant that skills relied on (gate cache hit `review` → `finish`, worklog across
  compaction, order-preserving findings records) is lost. Accepted: each is re-derived in session
  or from git, at the cost of a re-run gate in a new session.
- `AGENTS.md`'s binary invariants, payload rules and "three write locations" describe a shrinking
  binary; they point here, and each simplify ticket (#48–#53, #61, #47) rewrites its own section
  when it makes the change real.
- The write-sites allowlist test (`TestWriteSitesAreOnTheReviewedAllowlist`) is retired with the
  `scratch` package (#50). It guarded the work-tree writer; what is left writes only `~/.mkit` (a
  net-zero probe) and `cache prune --apply` (home-guarded, with its own tests), so a counted list
  of write sites no longer buys anything.
- An old `.mkit/` (run dirs, worklog, ledger, `config.toml`) and mkit's lines in `info/exclude` stay
  until the user removes them; `mkit doctor` names each with its remedy.
- Users with `.mkit/config.toml` need a one-time migration to `docs/agents/*.md`, owned by the
  `setup` ticket (#61).
- Codex's default sandbox has network off and `.git` read-only, so `gh` and `git commit` need
  approval there. Skills name that; they do not work around it.

## Alternatives rejected

- **A findings subagent** to run the dedupe. Adds a briefing and a hand-off for a step the main
  context already holds all inputs for; subagent support differs per agent (unverified in the Copilot
  coding agent).
- **A TOML reader in shell**, to keep `.mkit/config.toml`. bash 3.2 has no TOML parser; a hand-rolled
  one is a second implementation of a format, and the model reads prose config as well as TOML.
- **A PowerShell twin** of each script. Two implementations of one invariant, drifting by platform;
  Git for Windows already brings bash.
- **Per-skill script copies**, so a skill installs alone. Copies drift; installing takes the whole
  plugin anyway, so one shared copy reached by relative path costs nothing.
- **Keeping `mkit init`.** It lives in the now-optional binary, so Codex, Copilot and Windows users
  could never run it; a prompt-driven `setup` skill writes the prose config on every target.
