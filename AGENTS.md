# AGENTS.md

This file provides guidance to agents when working with code in this repository.

**mkit** — personal agentic-coding toolkit: **plugin** (`/plugin marketplace add masterik/mk-toolkit`)
of standalone skills + `_shared/` (references, two bash 3.2 scripts `facts.sh`/`branch-status.sh`),
plus an **optional Go binary** (`mkit`, Homebrew `masterik/tap/mkit`): a macOS + Claude Code power tool
(`doctor`, `audit sandbox`, `cache prune`), released in lockstep. Skills orchestrate `git`, `gh`, review
tools — no new git logic; `wt` optional. Targets Claude Code, Codex, Copilot; Windows via Git Bash.

**Plugin = product, binary = optional tools** ([ADR 0006](docs/adr/0006-standalone-skills-optional-binary.md),
epic #59): **no skill needs `mkit` but `sandbox-audit`** (it stops with a `brew` remedy; absent ≡ too
old). No `.mkit/`, no gate ledger/worklog/findings pipeline, no `mkit init` or pinning; config is prose
in lazy `docs/agents/*.md`, written by `setup`. Still open in #59: Codex/Copilot
ports (#54–#57), Windows (#58). **Next:** the ports. Milestones:
[`backlog.md`](docs/backlog.md); the *why*: [`concept.md`](docs/concept.md).

## Rules

- When reporting information, be _extremely concise_ — prioritize brevity over grammar or style.
- When writing documentation, be _clear and complete_, but prioritize concision over polished grammar.
- When creating plans, be thorough and actionable; describe *what* to do, not *how*, and omit code
  unless essential for clarity.
- **Never run a destructive operation (create, edit, write, delete) against the real user (home)
  folder** while testing or implementing. If a test or implementation needs to simulate or mock
  the home folder, do it inside the project (e.g. a throwaway dir under this repo or `$TMPDIR`),
  never via an env var (`HOME`, `MKIT_HOME`, etc.) pointed at the real home directory or anywhere
  outside the project. Only the user may authorize a write in the real home folder.

## Commands

`just` wraps these (`just --list`).

```bash
just ci                          # build, vet, test, lint, bats — what CI runs
just build / vet / test          # go build|vet|test ./...
just lint                        # golangci-lint (CI pins v2.12)
just bats                        # bats tests/ — the bundled scripts
just run version --json          # front-end contract
just run doctor                  # prerequisites, plugin state, sandbox grants, leftovers
just run audit sandbox --days 14 # sandbox/gate events, all sessions, vs current settings
just run cache prune             # stale Claude Code / Codex storage (dry run)
bash plugin/skills/_shared/scripts/facts.sh commit   # starting facts, no run dir
bash plugin/skills/_shared/scripts/branch-status.sh --no-fetch   # cleanup's classifier
```

**Release**: `just release` (`tools/release.sh`), needs a terminal. Picks version (auto: breaking/`feat` → minor pre-1.0, else patch; or `patch|minor|major|X.Y.Z`), shows PRs
since last tag, **asks before pushing**, bumps every agent's `plugin.json` under `plugin/` that exists
(Claude's required), commits `chore(release): X.Y.Z`, tags,
pushes. Tag → GoReleaser: darwin amd64/arm64, notes from PR titles (**the Release is the
changelog**; `CHANGELOG.md` frozen at 0.19.0), cask to `masterik/homebrew-tap`
(`homebrew_casks`, not deprecated `brews`). Lockstep: plugin-only changes still get a tag.

## Binary invariants

Breaking one is a design error (full list: `backlog.md`). The binary is **`doctor`, `audit sandbox`,
`cache prune`, `version`** — nothing else; a removed verb stays an unknown command (exit 2,
`TestRemovedCommandsAreUnknown`).

- **Optional.** No skill may need it but `sandbox-audit`; never add a verb a skill must call.
- **Layering.** `core` returns data · `cmd/` formats · `tui/` renders. No logic in a Bubble Tea `Update`.
- **No TUI off a TTY** — no ANSI/alt-screen on a pipe.
- **Every command non-interactive-capable, `--json` on all.** Only `sandbox-audit` parses the binary
  (`audit sandbox`); the other skills' live contract is the bundled scripts' `key=value` text (below).
- **Judgement stays in Markdown**; binary does mechanical invariants only.
- **Skills stay as files**, never `embed.FS` — diffable.

## The port, and what it left behind

Shell layer gone (M5), two scripts back (#52, `facts`/`branch status` → `_shared/scripts/`); the rules
apply both ways:

- The old test **was the spec**: pre-M5 `.bats` → Go tests → `tests/*.bats` again. Don't "simplify" an
  assertion whose comment names a measured failure.
- **One implementation per invariant**; never a second wording of a degradation sentence.
- Never half-capable: `jq-missing`, `no-hash`, `gh=no-cache` stay dead.

## Layout

### The binary
- `cmd/mkit/main.go` — entrypoint only; exits with `cli.Fail`'s status (1; 2 = usage).
- `internal/cli/` — cobra. `root.go` = **front-end contract**: `--json`/`--no-tui`/`--yes` resolved once
  in `PersistentPreRun` into `Options`; read via `cli.FromContext(cmd)`, never re-check a flag or
  `term.IsTerminal`.
- `internal/core/` — returns data; never prints or assumes a terminal.
  - `cache/` (M2): read-only `Scan`; `apply.go` deletes only what `Scan` named, home-containment guarded.
  - `gitrepo/`: `Open` (toplevel) and `CommonDir` only — doctor's leftover anchors, `claudecfg`'s
    roots. Every other repo question is the plugin's scripts' (remote resolution: `facts.sh`,
    [ADR 0005](docs/adr/0005-one-remote-per-repo.md)).
  - `pluginroot/` (M7): `CLAUDE_PLUGIN_ROOT`, `MKIT_PLUGIN_ROOT` (trusted as given), `plugin/` beside
    the work tree, marketplace checkout. Searched ones matched by **manifest name, never path**
    (checkout is `marketplaces/masterik/plugin`). **Work tree before installed copy** — else an old
    install answers for a newer tree, wrong but plausible.
  - `doctor/` (M7): human-run, reports, fixes nothing, exit 0. Prerequisites the skills compose (`git`,
    `gh`, `bash` with its version — scripts need 3.2+ — `gh stack`, optional `rg`/`wt`/`codex`/
    `coderabbit`), plugin payload + enabled + no hooks, binary↔payload drift (Warn, "optional, own
    channel"), `~/.mkit` and `$TMPDIR` probes, allowlist grants. No config/profile/gate checks.
    `orphans.go`: closed table of old mkit paths (`.mkit/work/`, `.mkit/worklog/`, `.mkit/gate.jsonl`,
    `.mkit/config.toml`, `<git-dir>/mkit`, `bootstrap.*`) → `leftovers` warning + `rm` (`config.toml`:
    run `setup`, then `git rm`); plus `.mkit/<skill>-<UTC>-*` run dirs (matched by name shape, one
    finding) and mkit's exact lines in `info/exclude` (remedy: delete them). Drop an entry once every
    machine is past the release that stopped writing it.
  - `userdir/`: `~/.mkit` (`MKIT_HOME`), its net-zero write probe and its one unwritable remedy sentence.
  - `sessionaudit/`: `mkit audit sandbox` (hidden alias `sessions`, one release). Reads
    `<claude home>/projects/**/*.jsonl` (`cache`'s `CLAUDE_HOME` rule). Classifies results the
    sandbox/gate touched: **paired to their call by `tool_use_id`**, never transcript substring (that
    counted `cat`s of docs); three refusals **anchored at result start**; EPERM by **line shape**, not
    exit status (`git push … | tail` exits 0; doc `grep` exits 1) — backticked/table/diff/comment/
    numbered line = file read. Override **preemptive** if no block preceded it when *called* (same-turn
    calls return out of order). Window by event timestamp, mtime only prefilter. Skips own runs and
    `sandbox-audit.*` queries (no re-counting). Read-only; `AttachConfig` marks targets
    `covered`/`protected` via `claudecfg`.
  - `claudecfg/`: user `settings.json` + each root's `.claude/settings{,.local}.json`. **Absent = no
    entry; unreadable/unparseable = `unreadable`, never empty** (opposite advice). `Roots`: cwd → work
    tree top (linked worktree its own, removed skipped). `Judge` matches
    `allowedDomains`/`allowWrite`/`additionalDirectories`; protected paths (`settings*.json`, hooks,
    skills, plugins) are inert. `keys.go` flags unknown `sandbox`/`permissions`/`autoMode` keys; may lag docs — a hint, not a verdict.
- `internal/tui/` — Bubble Tea over `core`. `ui/`: shared lipgloss for human forms (`doctor`,
  `cache prune`, `audit sandbox`, `version`, help), only when `Options.Pretty` (TTY, no
  `--json`/`--no-tui`). `cacheprune/`: tick-list for `cache prune --apply`. No command logic in either.
- `internal/buildinfo/` — version/commit/date via `-X` ldflags.
- `tools/` — non-payload maintainer shell; only `release.sh`.

### The plugin payload
- `plugin/` — Markdown + manifest + **exactly two scripts** (`_shared/scripts/facts.sh`,
  `branch-status.sh`; a third needs an ADR): no hooks, no `tests/`. **Stands alone**: every skill but
  `sandbox-audit` runs with `mkit` off `PATH`; no binary guard, no "absent ≡ too old → stop" rule
  anywhere else. Homebrew ships **binary only** (no `files:` in `.goreleaser.yaml`; casks have no
  stable path; `~/.claude/settings.json` sandbox-denied); one tag releases both
  ([ADR 0003](docs/adr/0003-two-distribution-channels.md)).
- `plugin/.claude-plugin/plugin.json` — manifest, skills auto-discovered. `marketplace.json`
  (`source: "./plugin"`) at **repo-root** `.claude-plugin/`: `/plugin marketplace add owner/repo`
  looks only there.
- **No hooks**, no `hooks` key (removed 0.15.0). **No `Stop`/`SubagentStop` hook** — its
  `additionalContext` renders in the transcript every turn (`concept.md`). If ever reintroduced:
  plugin-root `hooks/hooks.json`, **no `matcher`** (mistyped = silently never runs).
- `plugin/skills/<name>/SKILL.md` — workflow (`commit` → `review` → `pr`/`finish`) plus `cleanup`
  (branches/worktrees), `sandbox-audit` (user-wide → proposed settings diff, never edits; **the one skill that needs the binary**: `mkit audit sandbox`
  is its first call and dependency check), `explain` and `recap` (plain English, share
  `_shared/references/plain-english.md`; `explain` runs nothing; `recap` read-only,
  `facts.sh recap --gh` + `git log <base>..HEAD` bodies + the PR, records nothing), `setup` (user-only;
  writes the repo-settings files below), `concise` (rewrites pasted text per the user's report/doc/plan
  concision rules; runs nothing). **Ten exist**: `commit`, `review`, `pr`, `finish`,
  `cleanup`, `sandbox-audit`, `explain`, `recap`, `concise`, `setup`. Steps **composable, not sequential**:
  entry-capable, any order/subset, derive a thin version of what's missing, name assumptions. Never tell the user to run another skill first
  or run a downstream step. Contract: `_shared/references/workflow-contract.md`.
- **Repo settings** (ADR 0006 decision 7) — lazy prose `docs/agents/{commit,review,merge,remote,gate,cleanup}.md`
  in the *target* repo, one per non-default choice, listed under `### mkit workflow` in `## Agent skills`.
  Skills read the file if present, else discover/default — reading rules + defaults in
  `_shared/references/repo-settings.md`, each area's detail in the skill that reads it; missing file never stops. Spec store = mattpocock's
  `issue-tracker.md`. `setup` writes them (templates beside its `SKILL.md`), converts `.mkit/config.toml`,
  never touches mattpocock's files. No script reads them.
- **Skill prose** (#47): `SKILL.md` ≤ ~8 KB of instructions; a happy-path `commit`/`pr`/`finish` loads
  ≤ ~6K tokens of skill text. No up-front reference list: each link at the step that uses it. A branch only
  one skill takes lives beside its `SKILL.md` (`finish/pr-merge.md`, `review/reviewers.md`). Inline shell =
  plain `git`/`gh` calls, no `$(…)`/pipes/heredocs/`&&`. Rationale and the invariant checklist:
  [`docs/skill-invariants.md`](docs/skill-invariants.md) — cutting or moving a rule updates its row.
- `plugin/skills/_shared/` — references and `scripts/`, no `SKILL.md`; **keep `../_shared/…` paths
  relative**. Skills run `bash ../_shared/scripts/<name>.sh`, resolved against their own `SKILL.md`
  directory (no agent sets a skill-root env var in a skill body).
- `facts.sh <skill> [--run] [--gh] [--base] [--range] [--remote] [--worktrees]` — every starting fact, one call;
  **never reads config**. `--run` → `run_dir=` under `$TMPDIR`. `cd` toplevel first (pathspec lists and
  `--shortstat` agree). **Both** `unstaged_stat` and `staged_stat` (fully-staged looks clean otherwise);
  separate `untracked_file_list`. Unresolvable `--base`/`--range` →
  `*_state=unresolvable` **+ exit 1**. Failed git query = error, never empty (`git status` failing ≠
  `clean=yes`; a failed ahead/behind count ≠ `0 0`; a failed `remote get-url` ≠ no GitHub repo).
  Remote: `--remote` → `origin` → lone → first; `remote_choice=ask` → the skill asks, re-runs
  with `--remote` (`_shared/references/remote.md`); a `--remote` naming nothing falls back + `notes:`.
  `user_dir_writable=` is a net-zero write probe; its `notes:` remedy is `userdir.Remedy()` verbatim
  (`userdir_test.go` compares). Causes go in trailing `notes:`, never on a `key=value` line (some pack
  two pairs): one `- <key>=<value> — …` entry per line, the format both scripts share.
- `branch-status.sh [--default] [--develop] [--keep]… [--remote] [--no-fetch] [--no-gh]` — `cleanup`'s
  classifier. Default, remote and each worktree's origin (`--worktrees`) come from `facts.sh` (one
  resolver), and its `--remote` note is relayed, not re-worded; a failed branch listing is exit 1, never
  an empty `branches:`; `--keep` is what
  `docs/agents/cleanup.md` keeps. **One batched `gh` call** (`--jq`, no `jq`). Default/develop tested by
  name (may contain commas); a kept branch is never a merge target. `merged` PR refused unless the branch
  tip is its `headRefOid` or an ancestor of it. Failed worktree `git status` → `clean=error`.
- Script contract is internal (ADR 0006 decision 4): change a key in the script and every parsing skill
  in one commit. bash 3.2 (`/bin/bash` on macOS) and Git Bash's 5: no assoc arrays, `${1+"$@"}`.
  Tests: `tests/*.bats`, throwaway repo under `$TMPDIR`, `commit_here` asserts the toplevel first.
- **`mkit doctor` is the prereq report** (human-run, optional). Accepted losses — don't re-add a
  reporter: no session-start run. Skills read `facts.sh`, not `doctor`.
- **Nothing writes into the work tree** (ADR 0006 decision 5): no `.mkit/`, no ignore pair, no
  `info/exclude` write; `git add -A` and `git worktree remove` never meet a run artefact (`setup`'s
  `docs/agents/*.md` are user-confirmed docs, not state). An old `.mkit/` (incl. `config.toml`) shows as
  untracked; `doctor` names the remedy.
- `<run-dir>` — `facts.sh --run` → `mktemp -d "$TMPDIR/mkit-<skill>-XXXXXX"` (`$TMPDIR` unset: bare
  `mktemp -d`, the system's choice; the script never names `/tmp`); dies with the session, nothing to prune.
- `~/.mkit/` (`MKIT_HOME`) — user-scoped state; the binary writes nothing there but its probe. Holds
  `sandbox-audit.md` (agent-written ledger). `facts.sh` probes it. **Not `~/.claude/mkit/`**: protected
  region, allowlists inert; here one `permissions.additionalDirectories` entry works.
- `$TMPDIR` — dies with the command. Go honours it; the Darwin per-user temp dir fails under sandbox.

### Docs and tests
- `README.md` (users) and `CONTRIBUTING.md` (contributors) — short, pointing here.
- `docs/` (not shipped) — `concept.md` (direction), `backlog.md` (work + invariants), `skill-invariants.md` (skill rules → enforcing line, and their why),
  `prerequisites.md` (tooling, allowlist, sandbox grants), `adr/` (one per decision; `0006` supersedes
  `0001`/`0002`, amends `0003`/`0005`, withdraws `0004`), `ideas/` (researched, unscheduled).
- `CONTEXT.md` — domain glossary (repo setting, discovered value, default, remote, plugin, binary).
- **Go tests beside packages**. Stateful tests use a throwaway `$TMPDIR` repo with
  `MKIT_HOME` inside; never `HOME`. `internal/cli` tests drive the real cobra root (argv → stdout,
  exit code).
- **`tests/*.bats`** — the bundled scripts (argv → stdout, exit status); CI runs them on Linux (bash 5)
  and macOS (`/bin/bash` 3.2).

## Conventions

- **Binary: macOS-only**; never branch on OS. darwin amd64+arm64 only (casks don't install on Linux).
  Prefer stdlib over shelling out (testability). The plugin is not macOS-only: scripts target bash 3.2
  and Git Bash.
- Add a binary command only for a macOS/Claude Code power tool no skill needs, never a decision;
  unclear → report candidates, skill chooses. Hooks may compute the gap, never fill it.
- Commands and bundled scripts report and run; never stage, merge, push, edit. Parse stable output
  (`--porcelain`, `--shortstat`/`--name-only`, `--format`, `--json`); never call `rtk`. Scripts: no OS
  branching except where Git Bash forces it (#58).
- **Write locations, by lifetime:** skills write only to `$TMPDIR` (per-run files in `<run-dir>`), except
  `setup` (user-confirmed `docs/agents/*.md`) and `sandbox-audit` (its agent-written `~/.mkit/` ledger); `branch-status.sh` runs `git fetch --prune`.
  The binary's writes: `cache prune --apply` deleting stale `~/.claude`/`~/.codex` storage
  (home-guarded, own tests) and the net-zero `~/.mkit`/`$TMPDIR` probes. State never goes in the work
  tree, user files, `/tmp` or `~/.claude`.
- **Degradation sentences name a working remedy or say "human-run".** (Allowlisting the protected
  `~/.claude/mkit` looked right, did nothing.) Detect at first call as a starting fact; no "reduced" mode.
- Nothing project-specific hardcoded: gate commands, scopes, reviewers are discovered.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for `masterik/mk-toolkit`, operated via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` and `docs/adr/` at the repo root, created lazily. See `docs/agents/domain.md`.

### mkit workflow

Settings for mkit's skills; a file not listed means the default, or what the skill discovers. Defaults
throughout: scopes from history, `full` review, `merge` style, `CODEOWNERS` reviewers (else ask), remote by `origin` →
lone remote → ask, gate detected (`just ci`), cleanup keeps the default and develop branches. Add a
`docs/agents/<area>.md` file (or re-run `setup`) to change one.

Spec store: `docs/agents/issue-tracker.md`.
