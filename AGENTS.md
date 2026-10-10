# AGENTS.md

This file provides guidance to agents when working with code in this repository.

**mkit** — personal agentic-coding toolkit: **Go binary (`mkit`, Homebrew `masterik/tap/mkit`) +
Claude Code plugin** (`/plugin marketplace add masterik/mk-toolkit`). Plugin = skills + `_shared/references/`; binary owns the
mechanical invariants they rely on. Skills orchestrate `git`, `gh`, `wt`, review tools — no new git logic.

**Next: M8** (`mkit plan`, `spec`/`implement` skills); M1–M7 done (M3 withdrawn). `mkit` is **required
by every skill but `explain`**; first call `mkit facts <skill>` (`review` probes `mkit findings` first;
`sandbox-audit` starts with `mkit audit sandbox`). Binary absent *or too old* (`command not found` ≡
`unknown command "facts"`) = stop with a `brew` remedy; presence only, no declared minimum. Worklog
(`.mkit/worklog/`): written by `commit`/`review`/`pr`/`finish`, also read by `recap`; missing = **one
fewer input, never a stop**. Milestones: [`backlog.md`](docs/backlog.md); the *why*: [`concept.md`](docs/concept.md).

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
just ci                          # build, vet, test, lint — what CI runs
just build / vet / test          # go build|vet|test ./...
just lint                        # golangci-lint (CI pins v2.12)
just run version --json          # front-end contract
just run doctor                  # prerequisites, sandbox writability, plugin state
just run worklog show --json     # this branch's worklog
just run repo profile --json     # discovered|pinned|unavailable
just run facts commit --no-run   # starting facts, no run dir
just run audit sandbox --days 14 # sandbox/gate events, all sessions, vs current settings
```

**Release**: `just release` (`tools/release.sh`), needs a terminal. Picks version (auto: breaking/`feat` → minor pre-1.0, else patch; or `patch|minor|major|X.Y.Z`), shows PRs
since last tag, **asks before pushing**, bumps `plugin.json`, commits `chore(release): X.Y.Z`, tags,
pushes. Tag → GoReleaser: darwin amd64/arm64, notes from PR titles (**the Release is the
changelog**; `CHANGELOG.md` frozen at 0.19.0), cask to `masterik/homebrew-tap`
(`homebrew_casks`, not deprecated `brews`). Lockstep: plugin-only changes still get a tag.

## Binary invariants

Breaking one is a design error (full list: `backlog.md`).

- **Layering.** `core` returns data · `cmd/` formats · `tui/` renders. No logic in a Bubble Tea `Update`.
- **No TUI off a TTY** — no ANSI/alt-screen on a pipe; skills pipe the binary.
- **Every command non-interactive-capable, `--json` on all.** Live contracts: fact commands' `key=value`
  text (byte-for-byte the old scripts', parsed by every skill); `mkit findings --json` (`review` steps
  3, 4, 6). Others migrate to `--json`; nothing reads it yet.
- **Judgement stays in Markdown**; binary does mechanical invariants only.
- **Skills stay as files**, never `embed.FS` — diffable.

## The port, and what it left behind

Shell layer gone (M5); its rules still apply:

- Deleted `.bats` **was the spec**; each `go test` ports it. Don't "simplify" an assertion whose
  comment names a measured failure.
- **One implementation per invariant**; never a second wording of a degradation sentence.
- Never half-capable: `jq-missing`, `no-hash`, `gate_cache=no-jq`, `gh=no-cache`, `scripts_state=no-jq` stay dead.

## Layout

### The binary
- `cmd/mkit/main.go` — entrypoint only; exits with `cli.Fail`'s status (1; 2 = usage).
- `internal/cli/` — cobra. `root.go` = **front-end contract**: `--json`/`--no-tui`/`--yes` resolved once
  in `PersistentPreRun` into `Options`; read via `cli.FromContext(cmd)`, never re-check a flag or
  `term.IsTerminal`.
- `internal/core/` — returns data; never prints or assumes a terminal.
  - `cache/` (M2): read-only `Scan`; `apply.go` deletes only what `Scan` named, home-containment guarded.
  - `gitrepo/` (M7): **`Ignored` runs `check-ignore` twice**: `-v` exits 0 for a *negated* path too
    (answers "which rule", not "ignored?"); `-q` is the boolean, then `-v` names the file to edit.
  - `repoconfig/` (M7): `.mkit/config.toml`. `Stat` → `tracked|untracked|shadowed|absent`
    (**shadowed** = legacy dir-only `.mkit/` rule; config would never travel). `Write` renders a
    **commented template** (no Go TOML marshaller keeps comments). `ShadowedRemedy` = **sole producer**
    of that sentence; `doctor`/`facts` never re-word it.
  - `initplan/` (#31): `mkit init` form as data. `Build` (pinned config + `profile.Discover` without
    config, tiers apart + `Gather` candidates → pages); `Apply` (answers → config). Pre-select
    **pinned → discovered → default**; discovered gets pinned (gate, scopes, spec store, merge style)
    except reviewers (pinned list replaces `pr`'s CODEOWNERS match). Defaults only `merge.style = merge`,
    `review.mode = full`. Pages Spec → Commit → Review → Merge; Gate/Cleanup `Optional`, pre-selected,
    opened from review. Custom text uses `repoconfig`'s flag rules.
  - `profile/` (M7): merges discovered + pinned, tags each value; consumes `gate.Detect`'s gate
    result. `gate.Detect` overlays task-runner recipes (`runners.go`: just, make, Taskfile, deno) on
    the ecosystem chain **per step** — runner `test` replaces `go test ./...`, missing step keeps the
    ecosystem's, extras (`lint`) join in `GateSteps` order.
  - `pluginroot/` (M7): `CLAUDE_PLUGIN_ROOT`, `MKIT_PLUGIN_ROOT` (trusted as given), `plugin/` beside
    the work tree, marketplace checkout. Searched ones matched by **manifest name, never path**
    (checkout is `marketplaces/masterik/plugin`). **Work tree before installed copy** — else an old
    install answers for a newer tree, wrong but plausible.
  - `doctor/` (M7): reports, fixes nothing, exit 0. `orphans.go`: closed table of old mkit paths
    (`.mkit/work/`, `<git-dir>/mkit`, `bootstrap.*`) → `leftovers` warning + `rm`; drop an entry once
    every machine is past the release that stopped writing it.
  - `scratch/` (M5): owns `.mkit/`; **sole writer of runtime state** in a work tree (`repoconfig` writes
    `config.toml`). `writes_test.go` counts write sites per file. `EnsureIgnored` writes the ignore
    pair to `info/exclude` before first create; `Ignored` probes **two** paths (a `*.jsonl` rule hides
    the ledger, run dirs untracked). Owns `~/.mkit` (`MKIT_HOME`) and its one unwritable remedy sentence.
  - `facts/` (M5): all starting facts, one call. `cd` toplevel first (pathspec lists and `--shortstat`
    agree). **Both** `unstaged_stat` and `staged_stat` (fully-staged looks clean otherwise);
    separate `untracked_file_list`; `:(exclude).mkit` everywhere. Unresolvable `--base` →
    `base_state=unresolvable` **+ exit 1**; same for `--range` (empty + exit 0 looks like an empty range).
    Failed git query = error, never empty (`git status` failing ≠ `clean=yes`).
  - `branchstatus/` (M5): `cleanup`'s classifier (branch merge/upstream/PR, worktree origin/clean).
    **One batched `gh` call.** `--default` never re-derived; `$default`/`$develop` tested directly
    (names may contain commas). `merged` match refused unless `headRefOid` is tip or ancestor. Failed
    worktree `git status` → `clean=error`.
  - `gate/` (M5): `Fingerprint` (staging-/commit-invariant; symlink = target path; file→dir leaves
    mapping; `.mkit` dropped from HEAD and overlays), `Ledger` (append/classify/rotate), `Run` (full
    log, bounded excerpt). The two `gate run` forms **normalize the ledger key differently** — that
    seam enables the `review` → `finish` cache hit.
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
  - `findings/` (M4): validate, similarity, reconcile, group, report. **Order-preserving `Record`**, not
    a struct (re-serialized wholesale; a struct drops `fix`, `also`, extras). `json.Number` (`42.5`
    stays non-integer). `toFixed2` rounds half **away from zero** on the exact binary value, like JS
    (Go is half-to-even); at 0.125 `sim` vs `--sim`/`--band` decides thin-merge flag or re-review —
    location decides the merge. `sort.SliceStable` throughout; ids from a sort with ties. Writes
    artefacts, prints nothing.
  - `worklog/` (M6): `.mkit/worklog/<branch>.jsonl`. **`FileName` is the one mapping both verbs use**
    (drift shows as an empty log). `%XX`-escape + **full** SHA-256 of the exact branch on **every**
    name (macOS case-insensitive; partial or truncated digest still collides). Budgeted to `NAME_MAX`,
    cutting the **readable half, never the digest** (else ENAMETOOLONG, no record). Rotation = gate
    ledger's (`Keep = 200`, trim past `Keep*2`, dead heads first, mkdir lock, 60-min stale break);
    **can't read cleanly → don't rotate.** Uses **`gate.Fingerprint`**.
- `internal/tui/` — Bubble Tea over `core`. `ui/`: shared lipgloss for human forms (`doctor`, `repo
  profile`, `branch status`, `cache prune`, `audit sandbox`, `init`, `version`), only when
  `Options.Pretty` (TTY, no `--json`/`--no-tui`). `cacheprune/`: tick-list for `cache prune --apply`.
  `repoinit/`: `init` wizard over `initplan.Plan`, clack style. Plain Bubble Tea, not `huh`: Esc backs
  up (aborts on first prompt only), locked branch cursor-unreachable, Back from review → last prompt,
  scroll only when cursor leaves window. No command logic in either.
- `internal/buildinfo/` — version/commit/date via `-X` ldflags.
- `tools/` — non-payload maintainer shell; only `release.sh`.

### The plugin payload
- `plugin/` — **Markdown only** (manifest, skills, `_shared/`): no scripts, hooks, `tests/`; nothing
  runs. Homebrew ships **binary only** (no `files:` in `.goreleaser.yaml`; casks have no stable path;
  `~/.claude/settings.json` sandbox-denied). Versions independently, accepted ([ADR 0003](docs/adr/0003-two-distribution-channels.md)).
- `plugin/.claude-plugin/plugin.json` — manifest, skills auto-discovered. `marketplace.json`
  (`source: "./plugin"`) at **repo-root** `.claude-plugin/`: `/plugin marketplace add owner/repo`
  looks only there.
- **No hooks**, no `hooks` key (removed 0.15.0). **No `Stop`/`SubagentStop` hook** — its
  `additionalContext` renders in the transcript every turn (`concept.md`). If ever reintroduced:
  plugin-root `hooks/hooks.json`, **no `matcher`** (mistyped = silently never runs).
- `plugin/skills/<name>/SKILL.md` — **seven-step** workflow (`brainstorm` → `spec` → `implement` →
  `commit` → `review` → `pr`/`finish`) plus `cleanup` (branches/worktrees), `sandbox-audit` (user-wide
  → proposed settings diff, never edits), `explain` and `recap` (plain English, share
  `_shared/references/plain-english.md`; `explain` has **no binary dependency**; `recap` read-only,
  `mkit facts recap --no-run`, no worklog record). **Eight exist**: `commit`, `review`, `pr`, `finish`,
  `cleanup`, `sandbox-audit`, `explain`, `recap`; `brainstorm`/`spec`/`implement` unbuilt (M8) — never
  describe as shipping. Steps **composable, not sequential**: entry-capable, any order/subset, derive
  a thin version of what's missing, name assumptions. Never tell the user to run another skill first
  or run a downstream step. Contract: `_shared/references/workflow-contract.md`.
- `plugin/skills/_shared/` — references, no `SKILL.md`; **keep `../_shared/references/…` paths relative**.
- `mkit facts <skill>` opens a run dir and reports what the machine allows: `scratch_ignored=`,
  `user_dir_writable=`, `config_state=`, `git_bin=` (absolute git for parsed calls — a reshaping hook
  can summarize output). Causes go in trailing `notes:`, never on a `key=value` line (some pack two pairs).
- **`mkit doctor` is the prereq report** (human-run). Accepted losses — don't re-add a reporter: no
  session-start run, can't report `mkit` absent. Skills read `facts`, not `doctor`.
- `<toplevel>/.mkit/` — run dirs, `gate.jsonl` (append-only, back to 200 past 400), `worklog/` (same),
  **plus committed `config.toml`** ([ADR 0001](docs/adr/0001-per-repo-config-and-init.md#amendment-the-config-path)).
  - Ignore **pair** `.mkit/*` + `!.mkit/config.toml`, always together (git can't re-include under an
    excluded dir; `.mkit/*` alone hides config from `git add`). `.gitignore` **outranks**
    `info/exclude`; remedy names the file git reported.
  - **Work tree, not `<git-dir>/mkit`** ([ADR 0002](docs/adr/0002-state-locations-under-a-sandbox.md)):
    shared `.git` → main checkout, refused by worktree isolation. `--show-toplevel`: one per worktree.
  - Rule written **before** first write: else `git worktree remove` refuses, `git add -A` commits
    artefacts, fingerprint sees a changing dir. `scratch_ignored=` exists because isolated sessions
    can't write `info/exclude`.
  - `mkit scratch prune` removes only `<skill>-*` dirs. Worklog needs no rule of its own.
- `~/.mkit/` (`MKIT_HOME`) — future user-scoped binary state; binary writes nothing now. Holds
  `sandbox-audit.md` (agent-written ledger). `facts` probes it. **Not `~/.claude/mkit/`**: protected
  region, allowlists inert; here one `permissions.additionalDirectories` entry works.
- `$TMPDIR` — dies with the command. Go honours it; the Darwin per-user temp dir fails under sandbox.

### Docs and tests
- `README.md` (users) and `CONTRIBUTING.md` (contributors) — short, pointing here.
- `docs/` (not shipped) — `concept.md` (direction), `backlog.md` (work + invariants),
  `prerequisites.md` (tooling, allowlist, sandbox grants), `adr/` (one per decision; `0001` adds a repo
  setup step, `0002` supersedes its state locations because of the protected region), `ideas/`
  (researched, unscheduled).
- **Go tests beside packages** (ported `.bats`). Stateful tests use a throwaway `$TMPDIR` repo with
  `MKIT_HOME` inside; never `HOME`. `internal/cli` tests drive the real cobra root (argv → stdout,
  exit code).

## Conventions

- **macOS-only**; never branch on OS. darwin amd64+arm64 only (casks don't install on Linux).
  Prefer stdlib over shelling out (testability). Only shell dependency: `mkit gate run` runs
  `bash -c '<command>'`, so `-- sh -c 'a && b'` keeps its meaning.
- Add a command only for a mechanical invariant, never a decision; unclear → report candidates, skill
  chooses. Hooks may compute the gap, never fill it.
- Commands report and run; never stage, merge, push, edit. Parse stable output (`--porcelain`,
  `--shortstat`/`--name-only`, `--format=json`); never call `rtk`.
- **Three write locations, by lifetime:** `$TMPDIR`, `<toplevel>/.mkit/`, `~/.mkit/`; one exception,
  the common dir's `info/exclude`. Never user files, `/tmp` or `~/.claude`.
  `TestWriteSitesAreOnTheReviewedAllowlist` (`internal/core/scratch`) asserts it against a
  human-reviewed list — the real boundaries can't exist in a test.
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
