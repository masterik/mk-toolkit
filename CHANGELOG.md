# Changelog

Reconstructed from git history — this project kept no changelog while the versions below
were cut, so entries were derived from commit messages and `docs/`. Versions are the
`plugin.json` manifest version, set by the `chore(plugin|release): …` commit that closes
each block of work.

**Tags vs. versions.** `v0.12.0` and `v0.13.0` are tagged and pushed; `0.12.1` exists as a
manifest version only, so GoReleaser never built a cask for it. A version with no tag ships
nothing — `brew install masterik/tap/mkit` delivers whatever the newest tag built.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project
predates any semver commitment and is pre-1.0, so minor bumps carry breaking changes.

## [0.14.0] — 2026-09-09

The release that makes the payload usable on a machine with write boundaries. Breaking, because
mkit's state moved — but there is nothing to migrate; see below.

### Fixed
- **BREAKING — the payload works under a sandboxed, auto-mode, worktree-isolated session.**
  Three independent boundaries were refusing writes the payload assumed it could make
  ([#1](https://github.com/masterik/mk-toolkit/issues/1),
  [ADR 0002](docs/adr/0002-state-locations-under-a-sandbox.md)).
  - **Two template-less `mktemp` calls were failing 49 of the 190 shell tests.** On macOS
    `mktemp` with no template — bare or `-t` — resolves the Darwin per-user temp directory and
    ignores `$TMPDIR`, so the sandbox denies it. The gate fingerprint collapsed to no
    fingerprint (the cache was off, not slow) and the branch classifier *aborted* after
    `fetch=ok`, emitting no `gh=` line and no branch rows at all. Everything ephemeral now goes
    through `mkit_tmpfile`, which always passes a template.
  - **The branch classifier's PR cache is no longer load-bearing.** A cache it cannot create is
    `gh=no-cache`, with every branch and worktree row still emitted and the PR column reporting
    its own absence — like `gh-missing` and `gh-unauthenticated` already did.
  - **The `SessionStart` hook enforces the stamp-before-emit ordering its own comment
    documents.** It was `|| true`, so on an unwritable state directory a sentence specified to
    be said once per tool was said every session forever. A message whose stamp cannot be
    written is now dropped and the hook stays silent.

### Changed
- **BREAKING — state moved to two directories a grant can actually reach.** Repo scope is
  `<toplevel>/.mkit/` (was `<git-dir>/mkit/`), which from a linked worktree resolved into the
  main checkout, where the worktree-isolation guard refuses every write — and `facts.sh` opens
  the run directory as every skill's first call, so the toolkit was unusable in the sessions it
  is driven from. User scope is `~/.mkit/` (was `~/.claude/mkit/`), which was inside the
  sandbox's *protected* region where an allowlist entry is inert: the remedy the docs implied
  could not work. One `permissions.additionalDirectories` entry now genuinely opens it.
  Neither file is migrated, and nothing in the payload reads the old path. For
  `bootstrap.state` that costs nothing — it only records what has already been said. For the
  `--uninstall` tombstone it means a machine that silenced the hook before this release
  **warns once more per missing tool**, then stamps into the new location and is silent again;
  `install.sh --uninstall` re-silences it immediately. Only machines outside the sandbox are
  affected: inside it, `~/.claude/mkit` was unwritable, so `--uninstall` failed loudly there
  and there was never a tombstone to orphan.
- `run-open.sh` adds `.mkit/` to the common dir's `.git/info/exclude` **before** the first
  write. Not tidiness: unignored, `git worktree remove` refuses, `git add -A` would commit run
  artefacts, and the gate fingerprint sees a directory that changes while the gate runs.
- `facts.sh` reports four new starting facts — `tmp=`, `run_ignored=`, `user_dir=` /
  `user_dir_writable=`, `git_bin=` — plus a trailing `notes:` block carrying any cause that
  needs a sentence, and a remedy that works.
- `install.sh --status` gained a `state:` section (locations, and whether the user-scoped one is
  writable). Its exit status is still the prerequisite verdict alone. `--uninstall` now says the
  hook is *not* silenced when the tombstone cannot be written, with the remedy.
- `commit` gained a **non-interactive patch-staging recipe** — per-file diff to `$TMPDIR`, drop
  hunks by editing it, `git apply --cached`, verify with a staged stat — replacing the
  `git add -p` instruction, which needs a terminal there isn't one of. It states that hunks
  sharing lines are not split, because intermediate commits must build.
- Git calls whose output a skill parses or judges are pinned to `git_bin -C <toplevel>`: an
  output-reshaping hook can hand a skill a summarized status that reads exactly like the tree,
  and the isolation guard refuses any wrapper it cannot read a git target through.
- The delegation contract gained a coverage requirement (every changed path in exactly one
  proposed commit, a hunk assignment per mixed file), verification by set comparison rather
  than a re-read, repair by asking the same subagent about the gap, an exclusion for trees with
  more than ~3 mixed files, and a rule against polling or scheduling a wakeup on a subagent.
- Enabled the `skill-creator` and `plugin-dev` plugins in the repo's Claude config.

### Added
- Two payload invariants, asserted statically in `tests/bats/payload.bats` because neither has a
  behavioral seam: no shipped script uses a template-less `mktemp`, and every write target is a
  shell parameter on a reviewed allowlist — so the writable set is a property of the payload
  rather than a habit.

### Documentation
- [ADR 0002](docs/adr/0002-state-locations-under-a-sandbox.md) supersedes ADR 0001's
  state-location table (its other four decisions stand) and records why the protected-path
  region made relocation the only fix rather than one option among several.
- `docs/prerequisites.md` gained a sandbox section: the one grant mkit needs, the composed-tool
  grants (`~/.cache/gh`, the Go cache redirection, `~/.codex`, `~/.coderabbit`), the hosts the
  remote-facing skills reach, and the two facts no script may forget — `ps`/`pgrep` cannot list
  processes at all, and a template-less `mktemp` ignores `$TMPDIR`.
- `docs/ideas/`: recorded post-journal ideas (journal replacement, a native statusline
  provider), one file per idea.

### Migration

**One required step, and it is a permission, not a move:** grant `~/.mkit` by adding it to
`permissions.additionalDirectories`. Without it the user-scoped files cannot be written, and
`install.sh --status` reports the directory as `NOT WRITABLE` with that same remedy.
`sandbox.filesystem.allowWrite` is *not* a substitute — it grants the OS sandbox only and leaves
the auto-mode classifier refusing.

**Nothing needs copying.** Both old locations hold only regenerable state, so the honest migration
is to delete them:

- `~/.claude/mkit/` — `bootstrap.state` records which one-time prerequisite messages have already
  been said. Dropping it costs at most one repeated sentence per missing tool, after which the new
  location takes over permanently. If `bootstrap.disabled` is there, the hook was silenced: it will
  speak again once per missing tool until you re-run `install.sh --uninstall`, which writes the
  tombstone to the new location. Nothing reads the old path, deliberately —
  `tests/bats/payload.bats` asserts that no shipped script names it.
- `<git-dir>/mkit/` in each repo — per-run directories (scratch, already consumed) plus
  `gate.jsonl`. Losing the ledger costs wall-clock, never correctness: an unrecognised command
  classifies `none` and the gate step simply runs.

Machines inside the OS sandbox have nothing to clean up either way: `~/.claude/mkit` was unwritable
there, so no state was ever created and `--uninstall` failed loudly rather than leaving a tombstone.

## [0.13.0] — 2026-09-05

Two removals. The release that stops the plugin owning state it can't maintain.

### Removed
- **BREAKING — the commit journal.** Dropped `journal.sh`, the `note` skill, the
  `Stop`/`SubagentStop` journal-nudge hook, and their docs and tests. The journal was a
  second producer of commit intent (hook state, wrapper generation, user-scoped defaults)
  whose upkeep cost more than the diff read it saved. `commit` now always reads the diff
  directly. `install.sh` and `session-bootstrap.sh` are pared down to their remaining job:
  reporting missing prerequisites and the one opt-out tombstone.
- **BREAKING — the gate's fast tier.** `commit` no longer runs `gate-detect.sh` /
  `gate-run.sh` before committing; `review` no longer gates before its fix step or
  re-gates after it. `quality-gate.md` documents one tier (full), consumed only by `pr`
  (before opening) and `finish` (before merge) — the two points where a red tree is
  actually unsafe to act on. Skill output changes accordingly: no gate pass/fail line, no
  `fast_cache` reporting, no re-check step. A fixed tree is left unverified until
  `pr`/`finish` runs.

### Added
- `tools/purge-journal-state.sh` — removes the state mkit ≤ 0.12.1 left behind: the
  user-scoped marker, the `mkit-journal` wrapper, the stale `notice/v1` key, this repo's
  `journal.*` files. Dry-run by default (`--apply` to act); never removes a wrapper
  lacking mkit's generation marker, never follows a symlink, never deletes a directory.

### Documentation
- AGENTS.md, README, `backlog.md` and `concept.md` realigned with the journal removal.

## [0.12.1] — 2026-09-04

Fallout from the M1 reorg, plus the fix that made the marketplace installable.

### Added
- **`mkit storage prune`** (M2) — the first script ported to Go: read-only `Scan`,
  home-containment-guarded `Apply`, and a size-sorted Bubble Tea tick-list for `--apply`
  on a TTY. `tools/storage-prune.sh` deleted in the same commit.
- `justfile` task runner (`just ci` = build, vet, test, lint, shtest).

### Fixed
- `marketplace.json` moved to the **repo-root** `.claude-plugin/`. `/plugin marketplace
  add owner/repo` only ever looks there — nested under `plugin/` it failed with
  "Marketplace file not found". Its plugin entry now points at `./plugin`.
- The `Stop` nudge collapsed to one line — `additionalContext` renders verbatim in the
  transcript every turn and `suppressOutput` doesn't hide it, so the five-line nudge (plus
  the prose it provoked) pushed the user's own answer off screen in every journaling repo.
  (Moot as of 0.13.0, which deleted the hook.)
- `golangci-lint` errcheck/staticcheck findings; stale `plugin/` reorg paths in the shell
  test suite.

### Changed
- **Build matrix narrowed to macOS.** The Go port had quietly widened to darwin+linux
  because cross-compilation is free, but no linux target was ever tested and the tap
  publishes a **cask**, which Homebrew refuses to install on Linux. `goos: darwin` only;
  the one-line reversal is recorded under Later in `backlog.md`.

### Documentation
- Docs conformed to the Go binary + plugin design; M1 marked done; M3's cask packaging
  blockers recorded (no `files:` in `.goreleaser.yaml`, and a cask has no stable path to
  register — no `opt/` symlink, Caskroom is version-pinned).

## [0.12.0] — 2026-08-27 — `v0.12.0`

**The Go port begins (M1).** First tagged release; first cask on `masterik/homebrew-tap`.

### Added
- Go module `github.com/masterik/mk-toolkit` (go 1.26) with the layering invariant baked
  in: `cmd/mkit` (entrypoint only), `internal/{cli,core,tui,buildinfo}`.
- Cobra root owning the **front-end contract** — `--json` / `--no-tui` / `--yes` and TTY
  detection resolved once in `PersistentPreRun` into an `Options` on the command context —
  plus a `version` command exercising both output paths.
- `.goreleaser.yaml` using `homebrew_casks` (not the deprecated `brews`), publishing to
  `masterik/homebrew-tap` via a cross-repo token; CI (build/vet/test/lint) and a release
  workflow on `v*` tags.

### Changed
- **Repo reorganized.** The plugin payload (`.claude-plugin/`, `hooks/`, `skills/`,
  `scripts/`, `install.sh`) moved under `plugin/`; `concept.md`, `backlog.md` and
  `PREREQUISITES.md` moved under `docs/` (lowercased). Repository renamed
  `workflow_tool` → `mk-toolkit`; every cross-link and install snippet repointed.

## [0.11.0] — 2026-08-26

### Added
- `finish` merges an **existing open PR on GitHub** instead of merging locally.
  `facts.sh finish --gh` detects an open PR on the branch and routes step 4 through push →
  merge-method choice → `gh pr merge` → remote+local branch cleanup, so a branch under
  review is no longer orphaned by `finish`.

## [0.10.0] — 2026-08-25

*(`0.9.1` was tagged in the manifest one minute earlier for the same work and immediately
superseded — a new skill isn't a patch release. It never stood on its own.)*

### Added
- **The `cleanup` skill** — repo-wide branch/worktree gardening: delete merged/gone
  branches, remove their worktrees, keep the default branch plus a develop-like one, then
  switch and pull. Local-only; it never touches a remote branch.
- `branch-scan.sh` — the mechanical classifier behind it (merge/upstream/PR state per
  branch, origin/cleanliness per worktree) in one batched `gh` call, cached, never a
  per-branch round trip. Covered by bats for every classification plus the gh / no-gh /
  no-remote paths.

### Fixed
- `run-open.sh --prune` now removes `cleanup-*` run directories, not just the original
  four skills'.

### Documentation
- `review` forces `--wait` on codex-rescue, which was reporting false idles.

## [0.9.0] — 2026-08-24

### Changed
- The journal `Stop`-hook nudge collapsed to a count plus an `uncovered` pointer, and
  shortened again — the first pass at a cost that 0.12.1 and then 0.13.0 finished paying.

## [0.8.0] — 2026-08-24

Journaling on by default, and the bootstrap hook that made it reachable.

### Added
- **`SessionStart` bootstrap hook** (`session-bootstrap.sh`) — performs the user-scoped
  setup itself, because `install.sh` was a setup step nobody ran, and idempotently
  re-points a stale wrapper after a plugin move or version bump. Emits zero bytes on every
  later session, honours the `bootstrap.disabled` tombstone, never touches a repo, never
  calls git. Covered by bats for silence, idempotence and wrapper ownership.
- **`install.sh`** — the two jobs a re-asserting hook can't do: `--status` as the
  diagnostic surface and `--uninstall`, which writes the tombstone so a deliberate opt-out
  survives the next session.
- Journal enablement resolved **repo-first over a user-scoped default** (repo tombstone >
  repo marker > user default); `enabled --why` reports which scope decided, and `disable`
  only writes a tombstone when a user default is actually in play, so a pristine repo
  stays byte-identical to a never-enabled one.

### Changed
- `lib/common.sh` gained the shared prereq table, wrapper generator and jq-free JSON
  escape (`mkit_json_escape`) so the degradation sentences have exactly one producer.
- **Non-macOS accommodations dropped** — untested surface implying support that was never
  verified: the `sha256sum` fallback (`shasum` alone now), the Windows `Thumbs.db` ignore
  rule, the Debian install hint.

## [0.7.0] — 2026-08-22

**The gate ledger** — making a `pr` → `finish` cache hit possible at all.

### Added
- `mkit_tree_fingerprint` in `lib/common.sh` — a canonical path→blob mapping over the
  committed tree overlaid with the worktree, hashed to 16 hex chars. Deliberately
  invariant under staging and committing, because the flow worth caching is `review`
  (gates a dirty tree) then `finish` (commits, then gates the same content); a key built
  from HEAD plus the dirty set would drift the instant the commit landed. Hashing is
  batched into one `git hash-object --stdin-paths` (~55× faster than a per-path loop at
  1000 dirty files). Plus `mkit_sha256`, `mkit_have_hash`, `mkit_gate_ledger_path`,
  `mkit_age_human`.
- `gate-run.sh` appends one JSONL record **per finished step** to
  `<git-dir>/mkit/gate.jsonl` — what was proven, over which content. Strictly a side
  effect: never changes the exit code, the output, or where a chain stops; a failed append
  is silent; it never skips a step. `--no-ledger` turns it off.
- `gate-detect.sh` reads the ledger back and annotates the commands it already proposes:
  `fast_cache=`, `full_cache=`, `full_cache_exit=`, `full_cache_age=`, `gate_fingerprint=`,
  `gate_max_age_min=`. Classes: fresh, failed, drifted, stale, unknown-head, none.
  `failed` is checked before the age bound — it means "this tree was red on exactly this
  content", worth saying however old. Annotating a proposal is all this script has ever
  done; it still decides nothing. The skip decision stays the skill's, and a skipped step
  must be reported as `cached`.

### Testing
- `run-open.bats` asserts `--prune` removes only skill run directories.

## [0.6.0] — 2026-08-22

**The commit journal** — added here, removed in 0.13.0.

### Added
- `journal.sh` — an add/status/uncovered/drop/compact/enable/disable engine over an
  append-only JSONL journal, recording why a unit of work exists and classifying entries
  (fresh/drifted/committed/orphaned/unknown-head) against the current tree.
- The `journal-nudge` `Stop`/`SubagentStop` hook — names dirty paths with no recorded
  intent. Gated on repo/opt-in/`stop_hook_active`/budget/uncovered-count, always exits 0,
  never authors a record itself.
- The `note` skill — a manual path for recording intent when the hook can't fire.
- `commit` reads the journal before staging: a usable `journal_uncovered=0` + all-fresh
  state skips the diff read entirely, drift/uncovered narrows it to affected paths, and an
  absent journal falls back to a full read. Also proposes commit count from recorded
  units and credits journal `why` in the message body.
- `facts.sh --journal`.

### Changed
- `mkit_dir_or_die` hoisted into `lib/common.sh`.

## [0.5.0] — 2026-08-21

### Added
- **`review` quick mode** — CodeRabbit + Codex on bugs/impl only (no Claude craft
  subagent, no adversarial lens) as a lighter alternative to the default full
  three-reviewer pipeline. Selectable via `/mkit:review quick` or inferred from an
  explicit signal, and threaded through the roster, `--sources-expected` and the final
  report so a quick run is never mistaken for a degraded full one.

## [0.4.0] — 2026-08-20

**The script layer** — the mechanical steps leave Markdown.

### Added
- `scripts/`: `facts.sh` (opens the run directory *and* returns every starting fact in one
  call), `gate-detect.sh` + `gate-run.sh` (detect the repo's quality gate, then run it
  logged and bounded), `findings.mjs` (reconcile/group/report over a review's findings),
  and `lib/common.sh`. `run-open.sh` adopts common.sh and gains `--prune`.
- **Test suite** (`tests/run.sh`): `node --test` for the dependency-free `findings.mjs`
  (zero new deps) and bats-core for the bash scripts + `lib/common.sh`, each against a
  disposable git repo fixture.
- `PREREQUISITES.md`.

### Changed
- All four skills rewired onto the scripts. The references now describe what each script
  leaves to the caller — which merges to check, which verdicts to render, what gates a fix
  — rather than restating work the script does.
- `review/SKILL.md` states the reviewer completion contract.

## [0.3.0] — 2026-08-20

### Changed
- **Skills renamed**: `review-changes` → `review`, `create-pr` → `pr`, `finish-feature` →
  `finish`. Shorter trigger names, same skills.
- Startup calls batched and optional round trips gated — each skill had opened its run
  directory in a turn of its own, then spent two to five more on read-only probes that
  depended on nothing. Three unconditional delegations are now gated on being worth it.
- Skills stopped preloading references their subagents read; lenses and triage split by
  consumer.
- Writing rules added to AGENTS.md; the four skill files and the shared reference bundle
  tightened to them.

## [0.2.0] — 2026-08-19

**The three-reviewer pipeline, and the context strategy that makes it affordable.**

### Added
- `review-changes` runs **Claude as a third independent reviewer** alongside CodeRabbit
  and Codex, each carrying a distinct lens, with a verify stage — and falls back to
  redistributing lenses across two Claude subagents when a tool is missing. Previously two
  reviewers, no independent verification, gate run *after* fixing (so reviewers wasted
  findings on lint the gate would have caught first), and no context strategy at all.
- **Subagent delegation** where the read is the expensive line:
  - `create-pr` drafts title and description in a subagent that runs the full diff itself
    and returns ≤25 lines; the calling session sees only the commit log and `--stat`.
  - `commit` delegates the diff read above ~10 files or ~400 changed lines and gets back a
    commit plan under 20 lines — it's the front end `finish-feature` and `create-pr` both
    call, so that cost used to land three times.
  - A failing quality gate is triaged in a subagent that reads the log from disk and
    returns ≤15 lines: which checks failed, probable cause, caused-by-this-change or
    pre-existing, and a recommendation.
- Shared references: review severity bar, review lenses, finding triage, agent delegation,
  output discipline.

### Fixed
- Skills open the run directory via a script, not an inline snippet.

## [0.1.0] — 2026-07-17

**mkit becomes a Claude Code plugin.** No CLI, no runtime — a cohesive set of git
feature-workflow skills orchestrating `git`, `gh` and `wt` directly.

### Added
- `.claude-plugin/plugin.json` + `marketplace.json`.
- Four skills — `commit`, `review-changes`, `finish-feature`, `create-pr` — sharing one
  references bundle (moved in from `~/.agents/skills`, relative links preserved).
- `commit` and `create-pr` require a final commit/PR report.
- LICENSE.

### Changed
- Renamed twice: **ForgeZ → flowkit → mkit**. Shared bundle renamed `git-flow` → `_shared`.
- `concept.md` moved to the repo root; the previous fz-CLI/TUI concept archived, then
  removed from the workspace.

---

## Prehistory (pre-0.1.0, 2026-01 → 2026-07)

Not released; kept for provenance. The repo began as **ForgeZ**, a TUI-first tool, and the
plugin restructure at 0.1.0 discarded that direction wholesale.

- **2026-07-02** — `apps/cli` moved to `poc/bun-opentui-react`.
- **2026-04-06** — `.agents` and `.claude` untracked.
- **2026-03-17** — dev environment set up; the shell prototype archived.
- **2026-03-11 → 03-15** — POC A–E TUI stack prototypes and a stack evaluation; Bun
  workspace scaffolded with openTUI; project specs and use-case docs.
- **2026-01-27** — ForgeZ bootstrapped as a shell workflow helper.

[Unreleased]: https://github.com/masterik/mk-toolkit/compare/3d6c905...HEAD
[0.13.0]: https://github.com/masterik/mk-toolkit/compare/53dbda0...3d6c905
[0.12.1]: https://github.com/masterik/mk-toolkit/compare/v0.12.0...53dbda0
[0.12.0]: https://github.com/masterik/mk-toolkit/compare/1a0d1d2...v0.12.0
[0.11.0]: https://github.com/masterik/mk-toolkit/compare/a18f5df...1a0d1d2
[0.10.0]: https://github.com/masterik/mk-toolkit/compare/f19ea89...a18f5df
[0.9.0]: https://github.com/masterik/mk-toolkit/compare/379429b...f19ea89
[0.8.0]: https://github.com/masterik/mk-toolkit/compare/d93f0b1...379429b
[0.7.0]: https://github.com/masterik/mk-toolkit/compare/33be7b8...d93f0b1
[0.6.0]: https://github.com/masterik/mk-toolkit/compare/af5865d...33be7b8
[0.5.0]: https://github.com/masterik/mk-toolkit/compare/3c0c7fd...af5865d
[0.4.0]: https://github.com/masterik/mk-toolkit/compare/2e544c9...3c0c7fd
[0.3.0]: https://github.com/masterik/mk-toolkit/compare/3896c07...2e544c9
[0.2.0]: https://github.com/masterik/mk-toolkit/compare/c704d73...3896c07
[0.1.0]: https://github.com/masterik/mk-toolkit/commits/c704d73
