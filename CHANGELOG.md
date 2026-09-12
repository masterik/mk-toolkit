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

## [0.18.0] — 2026-09-12

M6: the per-branch worklog, and the four existing skills start recording into it.

### Added
- **`mkit work show | append`** — `<toplevel>/.mkit/work/<branch>.jsonl`, append-only, one record
  per finished step, never committed, per-worktree. A record carries step, timestamp, content
  fingerprint, head, artifact pointer, a one-line gist and the assumptions the step made.
  `--json` on both; no TUI, because there is nothing here to render interactively.
- **The worklog in all four skills** — a `mkit work show --json --limit 20` beside each skill's
  `facts.sh` call, and one `mkit work append` in its final-report step. `workflow-contract.md`,
  which already shipped, is now linked from every one of them.
- **A goal order in `review` step 1**: worklog gist (matching fingerprint) → user → branch name →
  commit messages → ticket. A gist whose fingerprint no longer matches is still used, and the
  downgrade is named in step 6.

### Changed
- **Rotation matches the gate ledger exactly** — `Keep = 200`, trim only past `Keep*2`, dead heads
  dropped first (one batched `cat-file`), mkdir lock with the 60-minute staleness break, and its
  hardest rule: a rotation that cannot read the file cleanly does not rotate.
- **`work append` errors on a failed write**, unlike the ledger's best-effort appends: it is a
  command someone invoked. The skills are where it is best-effort — they append after their report
  is produced and treat a failure as one line of note.

### Notes
- **The fingerprint is delegated, not ported.** `mkit_tree_fingerprint` stays the single producer,
  reached through `pluginroot.CommonFunc`, until M5 ports it. An unavailable one is `""` plus a
  named cause, never a failed append.
- **These calls are optional, unlike M4's probe.** `review` stops without `mkit findings`; nothing
  stops without the worklog. A log the binary cannot answer for costs a step one input — a recorded
  fact is an input, never a permission.
- **`finish` usually does not record.** It removes the worktree its log lives in and deletes the
  branch the log is keyed on, so it appends only where the run stopped short of the cleanup.
- **No ignore rule was added.** `.mkit/*` already covers `work/`; a second rule would be a second
  thing to keep true.

## [0.17.0] — 2026-09-12

M4: the review-run arithmetic moves into the binary, and `review` becomes the first skill that
needs it.

### Added
- **`mkit findings schema | validate | reconcile | group | report`** — the port of
  `plugin/scripts/findings.mjs`. Human text by default, `--json` for the skill-facing path; flags
  `--sources-expected --sim --band --window --max-groups --min-per-group`, each strict: absent is
  the default, present-with-no-value exits 2. Exit codes unchanged — 0 ok, 1 malformed input,
  2 bad usage.
- **`facts.sh` reports `mkit=` and `mkit_bin=`** — raw starting facts, `none` when the binary is
  off `PATH`. Nothing is compared: neither the payload nor the binary declares a version range.
- **A presence probe at `review` step 0** — `command -v mkit && mkit findings schema --json`. A
  subcommand that does not exist *is* the too-old signal, surfaced before a reviewer runs rather
  than at step 3 with a full run behind it.

### Changed
- **`review` requires the `mkit` binary** and stops at step 0 without it, naming
  `brew install masterik/tap/mkit` / `brew upgrade mkit`. This is the one place a skill is
  deliberately not entry-capable: without the arithmetic there is no reconcile, no groups and no
  ids for verdicts to reference. Every other skill still runs with no binary installed.
- **The binary's judgement sentences moved to Markdown.** `--json` carries structured facts only
  (`low_sim`, `review_pairs`, `suggest`, `unverified`); the sentences that told an agent what to
  make of them now live in `triage-reconcile.md` and `review/SKILL.md`. The binary owns mechanical
  invariants; judgement stays in Markdown.
- `tests/run.sh` is the shell suite only. The binary's tests are Go's, beside their packages — all
  21 cases of the node suite ported, plus the JS→Go parity hazards the port introduced.

### Fixed
- **A failing `mkit version` no longer ends `facts.sh`.** Under `set -o pipefail` the probe's
  nonzero exit failed the assignment and `set -e` took every later fact with it — an optional tool
  that merely would not answer, killing the run. It reports `mkit=unknown` now.
- **Caller mistakes exit 2, not 1.** An unknown subcommand, an unknown flag or an extra argument
  came back through cobra as a plain error and landed on 1, the status reserved for input that is
  present but malformed. Non-finite tunables (`--sim NaN`, `--window Inf`) are rejected the way the
  script's `num()` rejected them: NaN makes every comparison false, silently disabling merging,
  LOW-SIM flagging and the drop rule.
- **Trailing content is rejected the way `JSON.parse` rejected it.** The decoder's `More()` does
  not see a stray `]` or `}`, so a reviewer file that lost its enclosing array parsed clean and was
  half-read.

### Removed
- **`plugin/scripts/findings.mjs` and `tests/findings.test.mjs`**, in the commit that lands the
  replacement. The payload is bash only again.
- **`node` as a prerequisite** — gone from `prerequisites.md`, `tests/run.sh`, `mkit doctor`'s
  table and the bats fake-PATH set. The allowlist entry
  `Bash(node */mkit/scripts/findings.mjs:*)` becomes `Bash(mkit findings:*)`.

### Notes
- **The version-skew question dissolved rather than resolved.** No comparable tool declares a
  machine-comparable range on either side — worktrunk states compatibility in free-text
  frontmatter, coderabbit checks `--version || echo NOT_INSTALLED` in Markdown and carries its one
  per-feature minimum as untested prose, codegraph declares capability and names a fallback. So
  neither side declares one here either, and the check is presence only.
- **The probe lives in the skill, not in `facts.sh`.** The guard's original home, the `SessionStart`
  hook, was deleted in 0.15.0, and M5 folds `facts.sh` into the binary — after which the call that
  would report `mkit=<version>` *is* `mkit`, and a binary cannot report its own absence.

## [0.16.0] — 2026-09-10

M7: the configuration surface, and the diagnostic surface 0.15.0 removed.

### Added
- **`mkit init`** — writes `<toplevel>/.mkit/config.toml`, committed, so a colleague and a fresh
  clone inherit it. Interactive form on a TTY, every field also a flag. Pins only what inspection
  cannot establish; a no-op on a repo that already has a config (`--force` rewrites). **Config is
  an input, never a permission** — every command still runs with the file absent.
- **`mkit repo profile [--json]`** — gate commands, spec store, commit scopes, reviewers and merge
  style, each tagged `discovered`, `pinned` or `unavailable` **with a cause**; an empty value is
  never presented as an answer. Scopes come from history ranked by use, reviewers from CODEOWNERS,
  the spec store from `docs/agents/issue-tracker.md` and the ref from the remote. Gate discovery is
  **delegated to `gate-detect.sh`**, not reimplemented — that script stays the single
  implementation until M5 ports it.
- **`mkit doctor`** — prerequisites, sandbox writability, the writable set, plugin payload and
  enablement, hook registration, and permission-allowlist gaps. Reports; fixes nothing; exit status
  stays 0 with findings. Where a sentence already has a shell producer — the `~/.mkit` probe and
  its remedy — it is read **from `lib/common.sh`** rather than re-worded; the checks with no shell
  counterpart word their own.
  - **One exception, deliberate:** the shadowed-config remedy exists in both languages
    (`repoconfig.ShadowedRemedy` and `mkit_config_ignored_remedy`), worded to match. `mkit init`
    must refuse a shadowed path *before* it has located a payload, and a remedy that is
    unavailable exactly when the payload is missing is not a remedy. A parity test compares the
    two **byte for byte** over every rule shape, so a reword on either side fails the build —
    a documented exception is one that drifts.
  - It does **not** restore everything the hook did: it cannot run unprompted at session start and
    cannot report that `mkit` itself is absent. Both remain accepted losses.
  - The allowlist check distinguishes `permissions.additionalDirectories` from
    `sandbox.filesystem.allowWrite`, because they are not equivalent — the latter grants the
    sandbox only and leaves the auto-mode classifier's rule biting.
- **`config=` and `config_state=` starting facts in `facts.sh`**
  (`tracked|untracked|shadowed|absent`), plus `mkit_config_path`, `mkit_config_committable` and
  `mkit_config_ignored_remedy` in `lib/common.sh`.

### Changed
- **BREAKING (repo setup) — the `.mkit/` ignore rule is a pair, not a line.** `.mkit/*` followed by
  `!.mkit/config.toml`, written together into the common dir's `info/exclude`, and shipped in this
  repo's `.gitignore`. Git cannot re-include a file whose parent directory is excluded, so a
  directory-only rule made repo config impossible
  ([ADR 0001's config-path amendment](docs/adr/0001-per-repo-config-and-init.md#amendment-the-config-path)).
  - **A repo carrying the old rule is detected, not broken.** `facts.sh` reports
    `config_state=shadowed` and `mkit doctor` fails that check, both naming the file to edit.
    Nothing migrates automatically: `.gitignore` is the user's file.
  - `mkit_run_ignored` now probes `.mkit/gate.jsonl` rather than `.mkit/`. The directory form still
    works — `check-ignore .mkit/` *with the trailing slash* is matched by `.mkit/*` — but only via
    a subtlety already load-bearing here for an unrelated reason, and a concrete path turns on none
    of it.
- **The unwritable-user-directory remedy names both halves.** `! mkdir -p ~/.mkit` first, then the
  `additionalDirectories` grant: the grant covers the directory's *interior*, so it cannot create
  it, and the `mkdir` is a write to `$HOME` that no sandboxed session can perform. The
  grant-only wording had survived in `lib/common.sh`, `docs/prerequisites.md` and ADR 0002's
  decision 2 — all three corrected ([#3](https://github.com/masterik/mk-toolkit/issues/3)).

### Fixed
- **`git check-ignore -v` was being read as a boolean.** It exits 0 and prints the matching pattern
  for a **negated** path too, so it answers "which rule decided this", not "is it ignored" — which
  reported every deliberately re-included file as excluded, `.mkit/config.toml` first among them.
  Truth now comes from `-q`; `-v` runs only afterwards, to name the file a remedy must edit.
- **The payload is identified by its manifest name, never by path.** A marketplace checkout is
  named after the marketplace *owner* (`marketplaces/masterik/plugin`), so a path test for the repo
  name matched nothing.

### Dependencies
- `github.com/pelletier/go-toml/v2` — parsing the repo config. The file is *rendered* from a
  commented template rather than marshalled: it is committed and read in a diff, and no Go TOML
  marshaller preserves comments.

## [0.15.0] — 2026-09-10

### Removed
- **BREAKING — the plugin ships no hooks and no installer.** `hooks/hooks.json`,
  `scripts/hooks/session-bootstrap.sh` and `install.sh` are all deleted, along with
  `tests/bats/session-bootstrap.bats`, `tests/bats/install.bats`, and the helpers that existed
  only for them: `mkit_have`, `mkit_prereq_rows`, `mkit_state_has/add/drop/missing_keys` and
  `mkit_json_escape` in `lib/common.sh`. Prerequisite reporting belongs to the binary — M7's
  `mkit doctor` — and keeping a shell implementation alive until then meant two implementations
  of one invariant. Completes the deletion planned in
  [ADR 0003](docs/adr/0003-two-distribution-channels.md); every remaining script in the payload
  is called by a skill.
- **`~/.mkit/bootstrap.disabled` and `~/.mkit/bootstrap.state` are no longer written or read.**
  The tombstone silenced a hook that no longer exists. Delete a leftover one; nothing migrates.

### Changed
- **Nothing reports a missing prerequisite unprompted any more.** A missing `jq`, `gh` or `node`
  now surfaces where it bites — a thinner `facts.sh` block, a `gate_cache=no-hash` annotation —
  instead of once at session start. This is an **accepted regression** until `mkit doctor`, and
  `doctor` will not fully close it: a binary cannot report its own absence and cannot speak at
  session start. `docs/backlog.md` records the reasoning under "Staying in bash, permanently",
  where the hook used to be listed as permanent.
- **`~/.mkit/` is empty but still declared.** It remains the home for user-scoped state and what
  `MKIT_HOME` redirects, and `facts.sh` still emits `user_dir=` / `user_dir_writable=`, so no
  skill lost a fact and an unwritable directory is still a starting fact rather than a later
  failed write. Its `notes:` sentence no longer names `install.sh --uninstall`.

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
- **`cleanup`'s `--force` step could discard a tracked `.mkit/`.** Before treating an unignored
  `.mkit/` as disposable scratch, it checked only the index (`git ls-files`) for tracked content —
  missing a file `git rm --cached` had staged for deletion while it still sat in `HEAD`. Now checks
  both the index and `git ls-tree -r HEAD -- .mkit`.
- `mkit_tree_fingerprint`'s `EXIT` cleanup trap now installs before the first `mkit_tmpfile`
  allocation, so a failure on `tmp_d`/`tmp_s`/`tmp_h` still removes `tmp_p` (the local declaration
  was also missing `tmp_s`/`tmp_h`).
- `finish`'s step 5 teardown verification now runs every command — not just the log check —
  against the resolved surviving root, since the shell's own cwd can itself be inside the worktree
  step 4 just removed.

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
- `output-discipline.md`'s `mktemp` example now falls back to `/tmp` when `$TMPDIR` is unset,
  matching `mkit_tmpfile`'s own fallback.
- [ADR 0003](docs/adr/0003-two-distribution-channels.md) splits distribution into two independent
  channels: Homebrew ships the binary only, the plugin payload ships from the GitHub marketplace
  as it already does today, and the two version independently. Withdraws M3 (`mkit
  install`/`status`/`uninstall`) rather than re-scoping it — inspecting the shipped `v0.12.0` cask
  found no Homebrew-provided payload to register and no stable path to register it at (Caskroom is
  version-pinned, no `opt/` symlink) — and promotes M7 (`mkit init`/`repo profile`/`doctor`) to
  next, since nothing in the remaining ports blocks it.

### Removed
- `tools/purge-journal-state.sh` — every machine that needed it has run it; the 0.12.1 journal
  state it cleared no longer exists to clean up.
- `tools/migrate-state-layout.sh` — the one-time script that moved a machine to this release's
  state layout (above). Every machine has run it, so `tools/` is empty again.

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
