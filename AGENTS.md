# AGENTS.md

This file provides guidance to agents when working with code in this repository.

**mkit** — a **Go binary (`mkit`) plus a Claude Code plugin**, shipped together by Homebrew
(`brew install masterik/tap/mkit`). The plugin packages the agent coding-workflow skills
(`commit`, `review`, `finish`, `pr`, `cleanup`) plus the shared `_shared/references/`
bundle; the binary owns every mechanical step those skills take. Composition over replacement:
the skills orchestrate `git`, `gh`, `wt`, and code-review tools — no new git logic.

**Current phase: the Go port.** M1 (scaffold + release chain) done at `v0.12.0`; M2
(`mkit storage prune`) done; **M3 withdrawn** ([ADR 0003](docs/adr/0003-two-distribution-channels.md));
**M7 (`mkit repo profile`/`init`/`doctor`) done** — repo config is `<toplevel>/.mkit/config.toml`,
committed ([ADR 0001's config-path amendment](docs/adr/0001-per-repo-config-and-init.md#amendment-the-config-path)).
**M4 (`mkit findings`) done** — `internal/core/findings/` + `internal/cli/findings.go`.
**M6 (`mkit work`) done** — the per-branch worklog under `<toplevel>/.mkit/work/`, read and written
by all four skills; what it reports is **one fewer input, never a stop**.
**M5 (the `jq` consumers) done** — `mkit facts`, `mkit gate detect|run`, `mkit branch scan` and
`mkit run open|prune` replaced the last five scripts, and **the payload is Markdown only**: no
`plugin/scripts/`, no `lib/common.sh`, no `tests/`. That makes `mkit` a **hard requirement for
every skill** — each one's first call is `mkit facts <skill>` (`review` alone runs a one-line
compatibility probe before it, because its later steps need `mkit findings` too; `sandbox-audit`,
which is not repo-scoped and opens no run directory, starts with `mkit audit sessions` instead), and a
binary that is absent *or too old* is its stop condition with a `brew` remedy: `command not found` and
`unknown command "facts"` are the same answer. Presence only, no declared minimum on either side: a subcommand
that does not exist *is* the too-old signal. Milestones and the full invariant list:
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
just ci                          # build, vet, test, lint — what CI runs, in one shot
just build / vet / test          # go build|vet|test ./...
just lint                        # golangci-lint run (CI pins v2.12, brew install golangci-lint)
just run version --json          # exercise the front-end contract
just run doctor                  # prerequisites, sandbox writability, plugin state
just run work show --json        # this branch's worklog: what ran, and what it concluded
just run repo profile --json     # how this repo works: discovered|pinned|unavailable
just run facts commit --no-run   # every starting fact, without opening a run dir
just run audit sessions --days 14 # sandbox/permission-gate events across every session
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
- **Every command reachable non-interactively**, and **`--json` on every command**. Two contracts,
  both live: the fact-reporting commands are read as `key=value` human text, which is byte-for-byte
  what the scripts printed and what every skill still parses; `mkit findings` is read as `--json`,
  by `review` at steps 3, 4 and 6. `--json` is where the rest migrate; nothing else reads it yet.
- **Judgement stays in Markdown.** The binary owns mechanical invariants only.
- **Skills stay as files** — shipped by the package, never `embed.FS`; they must stay diffable.

## The port, and what it left behind

**The shell layer is gone** (M5). The rules that got it there are kept because they govern the
milestones that are left (M6 `mkit work`, M8 `mkit plan`) and because they explain why the code
reads as it does:

- The deleted `.bats` file **was the spec** for each script — the `go test` beside each package is
  a port of it, not a re-derivation from the shell. Do not "simplify" an assertion whose comment
  names a measured failure.
- **One implementation of one invariant.** Each script was deleted in the commit that landed its
  replacement, and no Go package may grow a second wording of a degradation sentence.
- Deleting degradation branches was part of the win — a binary is never half-capable, so
  `jq-missing`, `no-hash`, `gate_cache=no-jq`, `gh=no-cache` and `scripts_state=no-jq` died with
  the forks and temp files they existed for.

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
    **one producer** of the shadowed-config sentence, and the shell counterpart it was kept in
    parity with went with `lib/common.sh` in M5. `mkit doctor` and `mkit facts` both read it;
    neither re-words it.
  - `profile/` (M7): merges discovered with pinned, tagging every value. Gate discovery —
    and the pinned-over-discovered merge — belong to `gate.Detect` since M5; the profile
    consumes the tagged result rather than redoing it.
  - `pluginroot/` (M7): locates the payload — `CLAUDE_PLUGIN_ROOT`, `MKIT_PLUGIN_ROOT`, a
    `plugin/` beside the work tree, then the marketplace checkout. Every *searched* candidate is
    identified by the **manifest's own name**, never by path: the checkout is named after the
    marketplace *owner* (`marketplaces/masterik/plugin`), so a path test for the repo name
    matches nothing. The two environment variables are explicit overrides and are trusted as
    given. **The work tree comes before the installed copy** — in a payload checkout the tree
    being edited is what a report is about, and an installed 0.14.0 answering for a 0.16.0 work
    tree is a wrong answer that looks right.
  - `doctor/` (M7): the checks. Reports; fixes nothing; exit status stays 0 with findings.
  - `scratch/` (M5): `<toplevel>/.mkit/` — the scratch root, and the **only** package that writes
    *runtime state* inside a user's work tree. The one other writer there is `repoconfig`, which
    writes the committed `.mkit/config.toml`; both are on `writes_test.go`'s reviewed allowlist,
    which counts the write sites per file so adding one to a listed file still fails the test. `EnsureIgnored` puts the `.mkit/*` + `!.mkit/config.toml`
    pair in the common dir's `info/exclude` before the first create; `Ignored` probes **two**
    paths, because an unrelated `*.jsonl` rule hides the ledger while leaving every run
    directory untracked. `TestWriteSitesAreOnTheReviewedAllowlist` is the Go half of what
    `payload.bats` asserted over the shell: three write locations, chosen by lifetime, asserted
    by shape against a list a human reviewed. It also owns `~/.mkit` (`MKIT_HOME`) and the one
    remedy sentence for an unwritable one — `mkit doctor` and `mkit facts` both read that producer
    rather than wording it twice.
  - `facts/` (M5): every starting fact a skill reads, gathered in one call. `cd` to the toplevel
    first, so the pathspec'd file lists and the `--shortstat` beside them cannot disagree; **both**
    `unstaged_stat` and `staged_stat`, always, because a bare `git diff --shortstat` on fully-staged
    work reads exactly like a clean tree; `untracked_file_list` as its own block, because `git diff`
    never lists an untracked file; `:(exclude).mkit` on every enumeration of the user's work; and an
    unresolvable `--base` prints `base_state=unresolvable` **and** exits 1 — as does an
    unresolvable `--range`, which used to print an empty range and exit 0, a result a skill
    cannot tell from a range with nothing in it. A git query that fails is never reported as
    an empty answer: a `git status` that cannot run is an error, not `clean=yes`.
  - `branchscan/` (M5): `cleanup`'s classifier — every local branch's merge/upstream/PR
    state and every worktree's origin/cleanliness. One batched `gh` call, never a per-branch
    round trip. `--default` is never re-derived, `$default`/`$develop` are tested directly
    rather than by splitting a joined string (a branch name may contain a comma), and a
    `merged` PR match is refused when its `headRefOid` is not the branch tip or an ancestor.
    A worktree whose `git status` fails is `clean=error`, never collapsed into `yes`.
  - `gate/` (M5): `Fingerprint` (the staging- and commit-invariant content hash — symlinks
    hashed as their target path, a tracked file replaced by a directory leaving the mapping,
    `.mkit` dropped from the HEAD mapping as well as the overlays), `Ledger` (append, classify,
    rotate) and `Run` (step execution, full log, bounded excerpt). The two `gate run` call forms
    execute the same string but **normalize the ledger key differently**, and that seam is what
    makes a `review` → `finish` cache hit possible at all.
  - `sessionaudit/`: `mkit audit sessions` — reads `<claude home>/projects/**/*.jsonl` (the
    storage package's `CLAUDE_HOME` rule, not a second one) and classifies every tool result the
    sandbox or the permission gate had a say in. **Each result is matched against its own call**, paired
    by `tool_use_id`, never by substring over a transcript: that counted every `cat` of a doc quoting
    the markers. The three refusals are **anchored at the start of the result**, where Claude Code puts
    them. An EPERM is found by **the line's shape**, not the exit status — `git push … | tail` exits 0
    whatever push did, and a `grep` over docs that quote the error exits 1 — so a backticked, table,
    diff, comment or numbered-listing line is a file being read, not a command failing. An override is
    **preemptive** when no block had preceded it in its transcript by the time it was *called* — calls issued
    in one turn return in any order. The window is each event's own timestamp, the file's mtime only a
    prefilter: a session resumed today still holds last month's events. Its own runs are skipped, and so
    are queries of the report the skill saves (`sandbox-audit.*`), or each scan would re-count the last
    one's output. Read-only; the `sandbox-audit` skill holds the judgement.
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
    about, and the symptom is an empty log rather than an error. It escapes to `%XX` and then
    appends the **full** SHA-256 of the exact branch to **every** name: macOS is
    case-insensitive, so `JIRA-123` and `jira-123` would otherwise be one file; a digest added
    only to the uppercase ones is still ordinary branch text that another branch could spell; and
    a truncated one turns "same string" into "same string, or unlucky". Since git allows ref names
    longer than a filename may be, the name is budgeted against `NAME_MAX` and the **readable
    half** is what gets cut to fit, never the digest — injectivity was never carried by the
    readable half, and an unbudgeted name would not degrade but fail outright, `open` returning
    ENAMETOOLONG so the branch could not record at all. Rotation mirrors `gate-run.sh`'s
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
  (`brainstorm` → `spec` → `implement` → `commit` → `review` → `pr`/`finish`) plus two outside the
  line: `cleanup` (repo-wide branch/worktree gardening) and `sandbox-audit` (user-wide: every
  session's sandbox and permission-gate events, turned into a proposed settings diff — report-only,
  never edits a settings file). **Six exist today** — `commit`, `review`, `pr`, `finish`, `cleanup`,
  `sandbox-audit`; the front half is designed and unbuilt (`backlog.md`,
  M6–M8), so don't describe `brainstorm`/`spec`/`implement` as shipping.
  The steps are **composable, not sequential**: each is entry-capable, runs alone in any order with
  any subset skipped, derives the thin version of what it can't find, and names what it assumed.
  Never write a skill that tells the user to run another skill first, or that runs a step
  downstream of itself. The contract is `_shared/references/workflow-contract.md`.
- `plugin/skills/_shared/` — shared references (no `SKILL.md`); skills link in via
  `../_shared/references/…`. **Keep those relative paths intact** — they're what makes the bundle
  portable.
- **The payload ships no executable code at all** (M5). `plugin/` is the manifest, the skills and
  `_shared/`; there is no `scripts/`, no `lib/common.sh`, and nothing in it is run. `mkit facts
  <skill>` is every repo-scoped skill's first call — it opens the run directory under `<toplevel>/.mkit/` and
  returns every starting fact, including the three that say what this machine will let a skill do:
  `run_ignored=`, `user_dir_writable=` and `git_bin=` (the absolute git path, for any call whose
  output a skill parses — an output-reshaping hook can hand it a summarized status that reads
  exactly like the tree). A cause needing a sentence goes in the trailing `notes:` block, never on
  a `key=value` line, since several of those pack more than one pair.
- **No prerequisite reporting in the payload.** `session-bootstrap.sh` and `install.sh` are both
  gone (0.15.0). **`mkit doctor` is the report now** (M7) — human-run, on demand. What it does
  not restore, deliberately: it cannot run unprompted at session start, and cannot report that
  `mkit` itself is absent. Both were the hook's job and both stay accepted losses; don't re-add a
  reporter for them. `mkit facts`' `user_dir_writable=` and `config_state=` starting facts
  are what a *skill* reads, since `doctor` is for a human.
- `<toplevel>/.mkit/` — the scratch root, owned by `internal/core/scratch`: per-run directories plus `gate.jsonl` (the gate
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
  refuses every write, and `mkit facts` opens it as every skill's first call. `--show-toplevel`, so a
  linked worktree still gets its own. Scratch is never committed — `scratch.EnsureIgnored` puts the
  rule in the common dir's `info/exclude` **before** the first write, which is load-bearing rather
  than tidy: unignored, `git worktree remove` refuses, `git add -A` would commit run artefacts, and
  the gate fingerprint sees a directory that changes while the gate runs. `mkit facts` reports
  `run_ignored=` because an isolated session cannot write that file. `mkit run prune` only removes
  `<skill>-*` **directories**, which keeps `gate.jsonl` and `work/` out of its range. The worklog
  needs **no ignore rule of its own** — `.mkit/*` already covers it, and a second rule would be a
  second thing to keep true.
- `~/.mkit/` — the declared home for state outside a repo, overridable with `MKIT_HOME` (the tests
  set it so a developer's real state cannot affect a run). **The binary writes nothing there today**:
  its two files, `bootstrap.state` and `bootstrap.disabled`, went with the hook in 0.15.0. The one
  file in it is `sandbox-audit.md`, the `sandbox-audit` skill's ledger, written by the agent rather
  than by `mkit`. It keeps its definition because it is where the binary's user-scoped state will
  land, and `mkit facts` still probes it so
  an unwritable one is a starting fact rather than a later surprise. **Not `~/.claude/mkit/`**: that
  region is sandbox-*protected*, where an allowlist entry is inert, so it was a path no remedy could
  point at; here, one `permissions.additionalDirectories` entry works.
- `$TMPDIR` for anything that dies with the command. The division is by lifetime, not by caller.
  Go's own temp-file API honours `$TMPDIR`; the shell forms that did not (`mktemp` bare or `-t`,
  which resolve the Darwin per-user temp directory and fail outright under the sandbox — that is
  what took out 49 of 190 shell tests) are gone with the shell.
  `internal/core/scratch`'s `TestWriteSitesAreOnTheReviewedAllowlist` asserts the write set
  statically, since it has no behavioral seam: every file that writes is on a list a human
  reviewed.

### Docs and tests
- `docs/` — `concept.md` (direction/roadmap), `backlog.md` (ordered work list + invariants),
  `prerequisites.md` (required tooling, setup, permission allowlist, and the sandbox grants the
  toolkit and its composed tools need), `adr/` (decisions that were hard to reverse, one file per
  decision — `0001` reverses "no setup step" for repo scope; `0002` supersedes its state-location
  table and records why the sandbox's protected-path region made relocation the only fix), `ideas/`
  (researched but
  unscheduled, one file per idea — evidence parked so a later decision doesn't re-derive it;
  nothing in it is on the milestone line). Doc-only; nothing here ships in the cask.
- **Tests are Go's, beside their packages** — `tests/` and its bats suites went with the shell in
  M5, and each `.bats` was ported into the `_test.go` next to the code that replaced it. Every test
  that touches state builds a throwaway repo under `$TMPDIR` and points `MKIT_HOME` inside it; no
  test touches `HOME`, which is the whole containment story. `internal/cli`'s tests drive the real
  cobra root — argv in, stdout, exit code out — because that is the interface the skills call.

## Conventions

- **macOS-only.** Nothing detects or branches on an OS. `.goreleaser.yaml` builds `darwin` only —
  amd64 + arm64 is the whole matrix, and a Homebrew **cask** cannot install on Linux regardless.
  Go's cross-compilation stays available if that changes; it is not a requirement today
  (`backlog.md`, Later). Prefer `path/filepath` and stdlib over shelling out — for testability, not
  portability. The one deliberate shell dependency left is `mkit gate run`, which executes a step as
  `bash -c '<command>'` so `-- sh -c 'a && b'` keeps meaning what it says.
- **The payload runs nothing.** All mechanical work is in Go; `plugin/` is Markdown.
- Add a command only for a mechanical invariant, never for a decision. Where the line is unclear,
  report candidates and let the skill choose. Hooks are held one step further out: they may compute
  the gap, never fill it.
- Commands report and run; they never stage, merge, push or edit. They parse stable machine output
  (`--porcelain`, `--shortstat`/`--name-only`, `--format=json`) and never call `rtk`, which
  reshapes output for reading.
- **Three write locations, chosen by lifetime, and nowhere else.** `$TMPDIR` for anything that dies
  with the command; `<toplevel>/.mkit/` for anything a later step or session reads; `~/.mkit/` for
  user-scoped state. Plus one named exception, the common dir's `info/exclude`. Never the user's own
  files — `.mkit/`, ignored, is the only thing mkit puts in a working tree — never `/tmp`, never
  `~/.claude`. `internal/core/scratch`'s `TestWriteSitesAreOnTheReviewedAllowlist` asserts it by
  shape — every file that writes is on a list a human reviewed — because the boundaries that
  enforce it cannot be created inside a test.
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
