# AGENTS.md

This file provides guidance to agents when working with code in this repository.

**mkit** — a **Go binary (`mkit`) plus a Claude Code plugin**, shipped together by Homebrew
(`brew install masterik/tap/mkit`). The plugin packages the agent coding-workflow skills
(`commit`, `review`, `finish`, `pr`, `cleanup`) plus the shared `_shared/references/`
bundle; the binary is absorbing the plugin's shell script layer one script at a time.
Composition over replacement: the skills orchestrate `git`, `gh`, `wt`, and code-review tools —
no new git logic.

**Current phase: the Go port.** M1 (scaffold + release chain) done at `v0.12.0`; M2
(`mkit storage prune`) done; **M3 withdrawn** ([ADR 0003](docs/adr/0003-two-distribution-channels.md));
**M7 (`mkit repo profile`/`init`/`doctor`) done** — repo config is `<toplevel>/.mkit/config.toml`,
committed ([ADR 0001's config-path amendment](docs/adr/0001-per-repo-config-and-init.md#amendment-the-config-path)).
**M4 (`mkit findings`) done** — `internal/core/findings/` + `internal/cli/findings.go`, and
`review` now calls the binary. That makes `mkit` a **hard requirement for `review`** and only for
`review`: the skill probes `command -v mkit && mkit findings schema --json` at step 0 and stops
with a `brew` remedy if either half fails. Presence only, no declared minimum on either side — a
subcommand that does not exist *is* the too-old signal ([backlog](docs/backlog.md), Resolved).
**M6 (`mkit work`) done** — the per-branch worklog, read and written by all four skills. Its calls
are **optional**, unlike `review`'s findings probe: a worklog `mkit` cannot answer for costs a step
one input and never stops it.
**M5 (the `jq` consumers) is next.** Milestones and the full invariant list:
[`backlog.md`](docs/backlog.md). Direction and rationale: [`concept.md`](docs/concept.md) — the
place for *why*, so this file can stay operative.

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

`just` (`brew install just`) wraps these; `just --list` shows all recipes.

```bash
just ci                          # build, vet, test, lint, shtest — what CI runs, in one shot
just build / vet / test          # go build|vet|test ./...
just lint                        # golangci-lint run (CI pins v2.12, brew install golangci-lint)
just run version --json          # exercise the front-end contract
just run doctor                  # prerequisites, sandbox writability, plugin state
just run work show --json        # this branch's worklog: what ran, and what it concluded
just run repo profile --json     # how this repo works: discovered|pinned|unavailable
just shtest                      # shell layer: bats (brew install bats-core)
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
  - `storage/` (M2): `provider.go` (the provider/category table), `scan.go` (read-only),
    `apply.go` (deletes only what `Scan` named, home-containment guarded), `size.go`
    (`HumanBytes`).
  - `gitrepo/` (M7): the few git questions the config surfaces ask. **`Ignored` runs
    `check-ignore` twice on purpose** — `-v` exits 0 and prints the pattern for a *negated* path
    too, so it answers "which rule decided this", not "is it ignored"; `-q` is the boolean and
    `-v` runs only afterwards, to name the file a remedy must edit.
  - `repoconfig/` (M7): `<toplevel>/.mkit/config.toml`. `Stat` returns
    `tracked|untracked|shadowed|absent`; **shadowed** is a repo carrying the legacy
    directory-only `.mkit/` rule, where a written config would silently never travel.
    `Write` renders a **commented template** rather than marshalling — the file is committed and
    read in a diff, and no Go TOML marshaller preserves comments. `ShadowedRemedy` is the one
    **deliberate second producer** of a degradation sentence (`mkit_config_ignored_remedy` is the
    other): `init` must refuse before it has located a payload. `TestRemedyParityWithShell`
    compares the two byte for byte over every rule shape — keep it that way, or delete one.
  - `profile/` (M7): merges discovered with pinned, tagging every value. Gate discovery is
    **delegated to `gate-detect.sh`**, never reimplemented, until M5 ports it.
  - `pluginroot/` (M7): locates the payload — `CLAUDE_PLUGIN_ROOT`, `MKIT_PLUGIN_ROOT`, a
    `plugin/` beside the work tree, then the marketplace checkout. Every *searched* candidate is
    identified by the **manifest's own name**, never by path: the checkout is named after the
    marketplace *owner* (`marketplaces/masterik/plugin`), so a path test for the repo name
    matches nothing. The two environment variables are explicit overrides and are trusted as
    given. **The work tree comes before the installed copy** — in a payload checkout the tree
    being edited is what a report is about, and an installed 0.14.0 answering for a 0.16.0 work
    tree is a wrong answer that looks right.
    `CommonFunc` is how the binary calls a `lib/common.sh` helper instead of re-wording it.
  - `doctor/` (M7): the checks. Reports; fixes nothing; exit status stays 0 with findings.
  - `findings/` (M4): the review-run arithmetic — validate, similarity, reconcile, group, report.
    Records are an **order-preserving `Record`**, not a struct: `reconciled.jsonl` and `final.jsonl`
    re-serialize wholesale, and a struct would silently drop `fix`, `also` or anything a reviewer
    added. Numbers stay `json.Number` so `line: 42.5` is still not an integer. `toFixed2` rounds
    half **away from zero** on the exact binary value, matching JS — Go's own `FormatFloat` rounds
    half to even, and `sim` is compared against `--sim`/`--band`, so 0.125 decides whether a merge
    is flagged as thin or an unmerged pair comes back for review — location decides the merge
    itself. Every order-bearing sort is `sort.SliceStable`; ids come from a sort with ties. Writes the run
    directory's artefacts, prints nothing.
  - `worklog/` (M6): the per-branch record of what each step concluded,
    `<toplevel>/.mkit/work/<branch>.jsonl`. `FileName` is the branch→file mapping **both verbs go
    through** — one that `show` and `append` derive separately is one they eventually disagree
    about, and the symptom is an empty log rather than an error. Rotation mirrors `gate-run.sh`'s
    `ledger_trim` down to the constant (`Keep = 200`, trim past `Keep*2`, dead heads first, mkdir
    lock with the 60-minute staleness break) — including its hardest rule: **a rotation that cannot
    read the file cleanly does not rotate.** The fingerprint is **delegated to
    `mkit_tree_fingerprint`** via `pluginroot.CommonFunc`, never reimplemented, until M5 ports it.
- `internal/tui/` — Bubble Tea rendering over `core`, one subpackage per command.
  `internal/tui/storageprune/` (M2): the size-sorted tick-list `storage prune --apply` opens on a
  TTY. `internal/tui/repoinit/` (M7): the `mkit init` form. Neither `Update` holds command logic —
  one toggles selection, the other edits strings.
- `internal/buildinfo/` — version/commit/date, injected by `-X` ldflags at release.
- `tools/` — shell that is not part of the plugin payload; staging for a port, and the home
  for one-shot maintenance scripts. Empty today — `purge-journal-state.sh` and
  `migrate-state-layout.sh` both did their jobs (every machine ran them) and were deleted.

### The plugin payload
- `plugin/` — the plugin payload, shipped from the **GitHub marketplace**
  (`/plugin marketplace add masterik/mk-toolkit`, resolving the root `.claude-plugin/marketplace.json`).
  Homebrew ships the **binary only**: the cask carries one executable, `.goreleaser.yaml`
  deliberately declares no `files:`, and there is nothing to register — a cask has no stable path
  anyway (no `opt/` symlink; Caskroom is version-pinned), and `~/.claude/settings.json` is
  sandbox-denied besides. The two artifacts version independently, which is accepted rather than
  worked around ([ADR 0003](docs/adr/0003-two-distribution-channels.md), which withdrew M3).
- `plugin/.claude-plugin/plugin.json` — manifest (skills auto-discovered from `skills/`).
  The marketplace entry, `marketplace.json` (`source: "./plugin"`), lives at the **repo root**
  `.claude-plugin/` — not nested under `plugin/` — because `/plugin marketplace add owner/repo`
  always looks for `.claude-plugin/marketplace.json` at the repository root; there is no
  subdirectory syntax for the GitHub-shorthand or git-URL forms.
- **The plugin ships no hooks.** `hooks/hooks.json` and its one `SessionStart` script were
  removed in 0.15.0 — prerequisite reporting belongs to the binary (M7's `doctor`), and a hook
  that could not depend on the binary was the only reason it stayed in bash. Don't add a `hooks`
  key to the manifest either. **No `Stop` / `SubagentStop` hook, deliberately** — see
  `concept.md`'s "considered and dropped": that event's `additionalContext` is rendered verbatim
  in the transcript every turn and cannot be suppressed. If a hook is ever reintroduced, it goes
  at the **plugin root** in `hooks/hooks.json` (not `.claude-plugin/`), is auto-discovered, and
  takes no `matcher` — a mistyped matcher is a hook that silently never runs.
- `plugin/skills/<name>/SKILL.md` — the triggerable skills. The workflow is **seven steps**
  (`brainstorm` → `spec` → `implement` → `commit` → `review` → `pr`/`finish`) plus `cleanup`
  (repo-wide branch/worktree gardening, outside the line). **Five exist today** — `commit`,
  `review`, `pr`, `finish`, `cleanup`; the front half is designed and unbuilt (`backlog.md`,
  M6–M8), so don't describe `brainstorm`/`spec`/`implement` as shipping.
  The steps are **composable, not sequential**: each is entry-capable, runs alone in any order with
  any subset skipped, derives the thin version of what it can't find, and names what it assumed.
  Never write a skill that tells the user to run another skill first, or that runs a step
  downstream of itself. The contract is `_shared/references/workflow-contract.md`.
- `plugin/skills/_shared/` — shared references (no `SKILL.md`); skills link in via
  `../_shared/references/…`. **Keep those relative paths intact** — they're what makes the bundle
  portable.
- `plugin/scripts/` — the mechanical steps, called as `${CLAUDE_PLUGIN_ROOT}/scripts/<name>`:
  - `facts.sh <skill>` — opens the run directory under `<toplevel>/.mkit/` **and** returns every
    starting fact; every skill's first call. Among them the three that say what this machine will
    let a skill do: `run_ignored=`, `user_dir_writable=` and `git_bin=` (the absolute git path, for
    any call whose output a skill parses — an output-reshaping hook can hand it a summarized status
    that reads exactly like the tree). A cause needing a sentence goes in the trailing `notes:`
    block, never on a `key=value` line, since several of those pack more than one pair.
    `run-open.sh` — the directory alone, plus `--prune`.
  - `gate-detect.sh` / `gate-run.sh` — the quality gate. `gate-run.sh` **writes** the ledger (one
    record per finished step); `gate-detect.sh` **reads** it back, annotating the commands it
    proposes with `fast_cache=` / `full_cache=` / `gate_fingerprint=`, or one
    `gate_cache=off|empty|no-hash|no-jq` cause. Neither ever skips a step — the skill owns that
    trade-off and must label a skipped step `cached`. Escape hatches: `--no-ledger` / `--no-cache`.
  - `branch-scan.sh` — `cleanup`'s classifier: every local branch's merge/upstream/PR state and
    every worktree's origin/cleanliness. One batched `gh` call, cached, never a per-branch round
    trip. The cache is **not** load-bearing: a cache it cannot create is `gh=no-cache`, one row per
    branch and per worktree still emitted, the PR column reporting its own absence. That asymmetry
    with the run directory is deliberate — the classifier's contract is met without its cache, no
    skill's contract is met without its run directory.
  - `lib/common.sh` — sourced helpers, including `mkit_tree_fingerprint`, the staging- and
    commit-invariant hash of the content a gate command reads — what makes a `pr` → `finish`
    cache hit possible at all.
- **No prerequisite reporting in the payload.** `session-bootstrap.sh` and `install.sh` are both
  gone (0.15.0), and with them `mkit_prereq_rows`, `mkit_state_*` and `mkit_json_escape` from
  `lib/common.sh`. **`mkit doctor` is the report now** (M7) — human-run, on demand. What it does
  not restore, deliberately: it cannot run unprompted at session start, and cannot report that
  `mkit` itself is absent. Both were the hook's job and both stay accepted losses; don't re-add a
  shell reporter for them. `facts.sh`'s `user_dir_writable=` and `config_state=` starting facts
  are what a *skill* reads, since `doctor` is for a human.
- `<toplevel>/.mkit/` — the scripts' scratch root: per-run directories plus `gate.jsonl` (the gate
  ledger, append-only, rotated back to the newest 200 records once it passes 400) and `work/`
  (M6's worklog, one `<branch>.jsonl` per branch, same append-only shape and same rotation) — **and one
  committed file, `config.toml`**, the repo config `mkit init` writes
  ([ADR 0001](docs/adr/0001-per-repo-config-and-init.md#amendment-the-config-path)). That is why
  the ignore rule is the **pair** `.mkit/*` + `!.mkit/config.toml` and not a directory-only line:
  git cannot re-include a file whose parent directory is excluded. Both lines go in together —
  `.mkit/*` alone would hide repo config from `git add`. `.gitignore` **outranks** the common
  dir's `info/exclude`, so a negation in the exclude cannot lift a `.mkit/` rule in a committed
  `.gitignore`; the remedy names whichever file git reported. **Inside the
  working directory, not `<git-dir>/mkit`** ([ADR 0002](docs/adr/0002-state-locations-under-a-sandbox.md)):
  under a shared `.git` it resolved into the main checkout, where the worktree-isolation guard
  refuses every write, and `facts.sh` opens it as every skill's first call. `--show-toplevel`, so a
  linked worktree still gets its own. Scratch is never committed — `run-open.sh` puts the rule in
  the common dir's `info/exclude` **before** the first write, which is load-bearing rather than tidy: unignored,
  `git worktree remove` refuses, `git add -A` would commit run artefacts, and the gate fingerprint
  sees a directory that changes while the gate runs. `facts.sh` reports `run_ignored=` because an
  isolated session cannot write that file. `--prune` only removes `<skill>-*` **directories**, which
  keeps `gate.jsonl` and `work/` out of its range. The worklog needs **no ignore rule of its own** —
  `.mkit/*` already covers it, and a second rule would be a second thing to keep true.
- `~/.mkit/` — the declared home for state outside a repo, overridable with `MKIT_HOME` (the bats
  suite sets it so a developer's real state cannot affect a run). **Empty today**: its two files,
  `bootstrap.state` and `bootstrap.disabled`, went with the hook in 0.15.0. It keeps its definition
  because it is where the binary's user-scoped state will land, and `facts.sh` still probes it so an
  unwritable one is a starting fact rather than a later surprise. **Not `~/.claude/mkit/`**: that
  region is sandbox-*protected*, where an allowlist entry is inert, so it was a path no remedy could
  point at; here, one `permissions.additionalDirectories` entry works.
- `$TMPDIR`, with an explicit `mktemp` template, for anything that dies with the command. The
  division is by lifetime, not by caller. The bare and `-t` forms are banned: on macOS they resolve
  the Darwin per-user temp directory and ignore `$TMPDIR`, so under the sandbox they fail outright —
  which is what took out 49 of 190 shell tests. `tests/bats/payload.bats` asserts both this and the
  writable set statically, since neither has a behavioral seam.

### Docs and tests
- `docs/` — `concept.md` (direction/roadmap), `backlog.md` (ordered work list + invariants),
  `prerequisites.md` (required tooling, setup, permission allowlist, and the sandbox grants the
  toolkit and its composed tools need), `adr/` (decisions that were hard to reverse, one file per
  decision — `0001` reverses "no setup step" for repo scope; `0002` supersedes its state-location
  table and records why the sandbox's protected-path region made relocation the only fix), `ideas/`
  (researched but
  unscheduled, one file per idea — evidence parked so a later decision doesn't re-derive it;
  nothing in it is on the milestone line). Doc-only; nothing here ships in the cask.
- `tests/` — dev-only, deliberately kept at repo root rather than under `plugin/` so `plugin/`
  stays exactly the payload and nothing else. `tests/run.sh` runs `bats tests/bats/` (one `.bats`
  per shell script, each against a throwaway git repo, plus `payload.bats` — static assertions over
  the shipped shell, for the invariants with no behavioral seam). The binary's tests are Go's,
  beside their packages. `helpers.bash` sandboxes `MKIT_HOME` for every suite, which
  is the whole containment story now that nothing writes outside it — no suite touches `HOME`.
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
- Payload runtime is bash only, since M4 took the last `.mjs`. **New mechanical work goes in Go**,
  and the payload shrinks as milestones land.
- Add a script or a command only for a mechanical invariant, never for a decision. Where the line
  is unclear, report candidates and let the skill choose. Hooks are held one step further out:
  they may compute the gap, never fill it.
- Scripts report and run; they never stage, merge, push or edit. They parse stable machine output
  (`--porcelain`, `--shortstat`/`--name-only`, `--format=json`) and never call `rtk`, which
  reshapes output for reading.
- **Three write locations, chosen by lifetime, and nowhere else.** `$TMPDIR` (explicit `mktemp`
  template, always) for anything that dies with the command; `<toplevel>/.mkit/` for anything a
  later step or session reads; `~/.mkit/` for the two user-scoped files. Plus one named exception,
  the common dir's `info/exclude`. Never the user's own files — `.mkit/`, ignored, is the only
  thing mkit puts in a working tree — never `/tmp`, never `~/.claude`.
  `tests/bats/payload.bats` asserts it by shape — every write target is a parameter on a reviewed
  allowlist — because the boundaries that enforce it cannot be created inside a test.
- **A degradation sentence names a remedy that works, or it says the command is human-run.** The
  sandbox's protected-path region is why: telling a user to allowlist `~/.claude/mkit` produced a
  configuration that looked right and changed nothing. Detect at the first call and turn it into a
  starting fact; never add a "reduced" mode.
- Nothing project-specific is hardcoded: quality-gate commands, commit scopes, and reviewers are
  discovered from the target repo.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for `masterik/mk-toolkit`, operated via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` and `docs/adr/` at the repo root, created lazily. See `docs/agents/domain.md`.
