# AGENTS.md

This file provides guidance to agents when working with code in this repository.

**mkit** — a **Go binary (`mkit`) plus a Claude Code plugin**, shipped together by Homebrew
(`brew install masterik/tap/mkit`). The plugin packages the agent coding-workflow skills
(`commit`, `review`, `finish`, `pr`, `cleanup`) plus the shared `_shared/references/`
bundle; the binary is absorbing the plugin's shell script layer one script at a time.
Composition over replacement: the skills orchestrate `git`, `gh`, `wt`, and code-review tools —
no new git logic.

**Current phase: the Go port.** M1 (scaffold + release chain) done at `v0.12.0`; M2
(`mkit storage prune`) done; **M3 (`mkit install`/`status`/`uninstall`) is next.** Milestones and
the full invariant list:
[`backlog.md`](docs/backlog.md). Direction and rationale: [`concept.md`](docs/concept.md) — the
place for *why*, so this file can stay operative.

## Rules

- When reporting information, be _extremely concise_ — prioritize brevity over grammar or style.
- When writing documentation, be _clear and complete_, but prioritize concision over polished grammar.
- When creating plans, be thorough and actionable; describe *what* to do, not *how*, and omit code
  unless essential for clarity.

## Commands

`just` (`brew install just`) wraps these; `just --list` shows all recipes.

```bash
just ci                          # build, vet, test, lint, shtest — what CI runs, in one shot
just build / vet / test          # go build|vet|test ./...
just lint                        # golangci-lint run (CI pins v2.12, brew install golangci-lint)
just run version --json          # exercise the front-end contract
just shtest                      # shell layer: node --test + bats (brew install bats-core)
```

Release is tag-driven: push `vX.Y.Z` → GoReleaser builds darwin × amd64/arm64 and commits
the Homebrew **cask** to `masterik/homebrew-tap`. `homebrew_casks`, not `brews` (deprecated in
GoReleaser v2).

## Binary invariants

Not preferences — breaking one is a design error, not a trade-off. Full list: `backlog.md`.

- **Layering.** `core` returns data · `cmd/` formats · `tui/` renders. Logic never lives in a
  Bubble Tea `Update`.
- **No TUI off a TTY.** stdout not a terminal → no ANSI, no alt-screen. Skills pipe this binary;
  a TUI on a pipe is corruption, not cosmetics.
- **Every command reachable non-interactively**, and **`--json` on every command** — the
  skill-facing contract replacing the shell layer's `key=value` parsing. Human text stays default.
- **Judgement stays in Markdown.** The binary owns mechanical invariants only.
- **Skills stay as files** — shipped by the package, never `embed.FS`; they must stay diffable.

## Porting a script

- A script's `.bats` file **is the spec** — port it to `go test` beside the code; don't re-derive
  the behavior from the script.
- One script per milestone, merged green. No big-bang rewrite: the bash is tested and load-bearing.
- **Delete the shell script in the same commit that lands its replacement.** Two implementations
  of one invariant is the failure the script layer exists to prevent.
- Deleting degradation branches is part of the win — a binary is never half-capable, so
  `jq-missing` / `no-hash` / `gate_cache=no-jq` die with their script.

## Layout

### The binary
- `cmd/mkit/main.go` — entrypoint only: build the root, print the error, exit 1. No logic.
- `internal/cli/` — the cobra tree. `root.go` owns the **front-end contract**: `--json`,
  `--no-tui`, `--yes` resolved once in `PersistentPreRun` into an `Options` on the command
  context. Read it via `cli.FromContext(cmd)` — never re-check a flag or call `term.IsTerminal`
  inside a command.
- `internal/core/` — data-returning logic. Never prints, never assumes a terminal.
  `internal/core/storage/` (M2): `provider.go` (the provider/category table), `scan.go`
  (read-only), `apply.go` (deletes only what `Scan` named, home-containment guarded), `size.go`
  (`HumanBytes`).
- `internal/tui/` — Bubble Tea rendering over `core`, one subpackage per command.
  `internal/tui/storageprune/` (M2): the size-sorted tick-list `storage prune --apply` opens on a
  TTY. `Update` holds no prune logic — it only toggles selection.
- `internal/buildinfo/` — version/commit/date, injected by `-X` ldflags at release.
- `tools/` — shell that is not part of the plugin payload; staging for a port, and the home
  for one-shot maintenance scripts. `purge-journal-state.sh` removes the state mkit <= 0.12.1
  left behind when journaling was deleted (the user-scoped marker, the `mkit-journal` wrapper,
  the stale `notice/v1` key, this repo's `journal.*` files). Dry-run by default, `--apply` to
  act; it never removes a wrapper without mkit's generation marker, never follows a symlink,
  and never deletes a directory. Delete it once the machines that need it have run it.

### The plugin payload
- `plugin/` — the plugin payload: what Homebrew is *meant* to ship so M3 can register it as a
  `directory` marketplace. **It does not ship yet** — `.goreleaser.yaml` declares no `files:`, so
  the `v0.12.0` cask contains only the binary, `LICENSE` and `README.md`. M3 is blocked on that
  and on the fact that a cask has no stable path to register (no `opt/` symlink; Caskroom is
  version-pinned). Don't repeat "Homebrew ships the payload" as fact — see M3 in `backlog.md`.
- `plugin/.claude-plugin/plugin.json` — manifest (skills auto-discovered from `skills/`).
  The marketplace entry, `marketplace.json` (`source: "./plugin"`), lives at the **repo root**
  `.claude-plugin/` — not nested under `plugin/` — because `/plugin marketplace add owner/repo`
  always looks for `.claude-plugin/marketplace.json` at the repository root; there is no
  subdirectory syntax for the GitHub-shorthand or git-URL forms.
- `plugin/hooks/hooks.json` — hook registration, at the **plugin root** (not `.claude-plugin/`):
  `SessionStart` → `scripts/hooks/session-bootstrap.sh`, and nothing else. Auto-discovered, so the
  manifest carries **no `hooks` key** — don't add one. No `matcher`: a mistyped matcher is a hook
  that silently never runs. **No `Stop` / `SubagentStop` hook, deliberately** — see
  `concept.md`'s "considered and dropped": that event's `additionalContext` is rendered verbatim
  in the transcript every turn and cannot be suppressed.
- `plugin/skills/<name>/SKILL.md` — the five triggerable skills. `cleanup` (repo-wide
  branch/worktree gardening) sits outside the edit → commit → review → integrate line.
- `plugin/skills/_shared/` — shared references (no `SKILL.md`); skills link in via
  `../_shared/references/…`. **Keep those relative paths intact** — they're what makes the bundle
  portable.
- `plugin/scripts/` — the mechanical steps, called as `${CLAUDE_PLUGIN_ROOT}/scripts/<name>`:
  - `facts.sh <skill>` — opens the run directory under `<git-dir>/mkit/` **and** returns every
    starting fact; every skill's first call. `run-open.sh` — the directory alone, plus `--prune`.
  - `gate-detect.sh` / `gate-run.sh` — the quality gate. `gate-run.sh` **writes** the ledger (one
    record per finished step); `gate-detect.sh` **reads** it back, annotating the commands it
    proposes with `fast_cache=` / `full_cache=` / `gate_fingerprint=`, or one
    `gate_cache=off|empty|no-hash|no-jq` cause. Neither ever skips a step — the skill owns that
    trade-off and must label a skipped step `cached`. Escape hatches: `--no-ledger` / `--no-cache`.
  - `findings.mjs` — reconcile/group/report over a review's findings. M4 port target; `node`
    leaves `prerequisites.md` with it.
  - `branch-scan.sh` — `cleanup`'s classifier: every local branch's merge/upstream/PR state and
    every worktree's origin/cleanliness. One batched `gh` call, cached, never a per-branch round trip.
  - `lib/common.sh` — sourced helpers, including `mkit_tree_fingerprint`, the staging- and
    commit-invariant hash of the content a gate command reads — what makes a `review` → `finish`
    cache hit possible at all.
- `plugin/scripts/hooks/session-bootstrap.sh` — `SessionStart`: names, **once per tool**, any
  prerequisite this machine lacks, then emits **zero bytes** on every later session. Installs
  nothing and writes nothing but `bootstrap.state`. Gated (absolute user dir, no
  `bootstrap.disabled` tombstone, an unsaid message), **always exits 0**, never to stderr. Never
  parses its stdin — that would need `jq`, the very tool it must be able to report as missing
  (hence `mkit_json_escape`). Never touches a repo or calls git. **Stays in bash permanently** —
  it cannot depend on a binary whose absence it may have to report.
- `plugin/install.sh` — **installs nothing; there is no setup step.** Two jobs a hook cannot do:
  `--status` (the diagnostic surface — prerequisites, hook state, gate ledger — which exists
  precisely so the hook never has to be one) and `--uninstall [--purge]` (writes the tombstone
  that silences the hook for good). No arguments does what `--status` does, and its exit status
  is the prerequisite verdict. Sources `lib/common.sh` so the degradation sentences have exactly
  one producer. Never edits a shell rc, never touches a repo. **M3 absorbs it.**
- `<git-dir>/mkit/` — the scripts' scratch root: per-run directories plus `gate.jsonl` (the gate
  ledger, append-only, rotated back to the newest 200 records once it passes 400). Never
  committed, never in `git status`; a linked worktree gets its own. `--prune` only removes
  `<skill>-*` **directories**, which keeps `gate.jsonl` out of its range.
- `~/.claude/mkit/` — the only state outside a repo, overridable with `MKIT_HOME` (the bats suite
  sets it so a developer's real state cannot affect a run). Holds `bootstrap.disabled` (the
  tombstone that makes the hook's silencing outlive the session) and `bootstrap.state` (one key
  per line: which one-time messages have been said — self-heals, a `prereq/` key drops once the
  tool is back so a later removal warns again).

### Docs and tests
- `docs/` — `concept.md` (direction/roadmap), `backlog.md` (ordered work list + invariants),
  `prerequisites.md` (required tooling, setup, permission allowlist). Doc-only; nothing here ships
  in the cask.
- `tests/` — dev-only, deliberately kept at repo root rather than under `plugin/` so `plugin/`
  stays exactly the payload and nothing else. `tests/run.sh` runs both `node --test
  tests/findings.test.mjs` and `bats tests/bats/` (one `.bats` per shell script, each against a
  throwaway git repo). `helpers.bash` sandboxes `MKIT_HOME` for every suite, which is the whole
  containment story now that nothing writes outside it — no suite touches `HOME`.
  `mkit_fake_path <tool>…` builds a PATH missing only the named tools — **it must
  include `bash`**, or `env PATH=… bash -c` exits 127 with empty output, which reads exactly like
  a hook correctly staying silent. Go tests live beside their package.

## Conventions

- **macOS-only — shell and binary alike.** Nothing detects or branches on an OS. The scripts are
  written to what macOS provides (bash 3.2, BSD `sed`/`date`, no GNU-only flags, no `flock`); a GNU
  fallback "for portability" is untested surface for an unsupported platform. `.goreleaser.yaml`
  builds `darwin` only — amd64 + arm64 is the whole matrix, and a Homebrew **cask** cannot install
  on Linux regardless. Go's cross-compilation stays available if that changes; it is not a
  requirement today (`backlog.md`, Later). Still prefer `path/filepath` and stdlib over shelling
  out — for testability, not portability. Every script carries `#!/usr/bin/env bash` — the user's
  interactive zsh is irrelevant.
- Payload runtime stays bash plus one dependency-free `.mjs`. **New mechanical work goes in Go**,
  and the payload shrinks as milestones land.
- Add a script or a command only for a mechanical invariant, never for a decision. Where the line
  is unclear, report candidates and let the skill choose. Hooks are held one step further out:
  they may compute the gap, never fill it.
- Scripts report and run; they never stage, merge, push or edit. They parse stable machine output
  (`--porcelain`, `--shortstat`/`--name-only`, `--format=json`) and never call `rtk`, which
  reshapes output for reading.
- Nothing project-specific is hardcoded: quality-gate commands, commit scopes, and reviewers are
  discovered from the target repo.
