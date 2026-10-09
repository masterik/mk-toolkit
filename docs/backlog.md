# mkit — Backlog

The ordered work list and the rules every change must hold. Direction and rationale:
[`concept.md`](concept.md). Researched-but-unscheduled ideas live in [`ideas/`](ideas/README.md);
open work is tracked in [GitHub Issues](https://github.com/masterik/mk-toolkit/issues).

## Decisions

- **Go, one binary, subcommand tree.** Single static binary, a stdlib that covers the workload
  (`os/exec`, `encoding/json`, `crypto/sha256`), the Charm TUI stack, GoReleaser's Homebrew
  support. Rust and Zig were considered and rejected for this kind of subprocess orchestration.
- **macOS only.** `.goreleaser.yaml` builds darwin amd64 + arm64; the Homebrew cask can't install
  on Linux anyway. Adding a platform is a `goos` line plus a distribution channel — done when
  someone needs it, not carried untested.
- **Two channels.** Homebrew ships the binary only; the GitHub marketplace ships the plugin
  ([ADR 0003](adr/0003-two-distribution-channels.md)). No installer command: `brew install` for the
  binary, and adding the marketplace plus enabling the plugin, are steps a human runs once.
- **Presence, not versions.** Neither side declares a compatible range. A skill calls the
  subcommand it needs; `unknown command` is the too-old signal, answered with `brew upgrade mkit`.

## Invariants

Breaking one is a design error, not a trade-off.

1. **Layering.** `internal/core` returns data and never prints or assumes a terminal; `internal/cli`
   formats; `internal/tui` renders. No logic in a Bubble Tea `Update`.
2. **No TUI off a TTY.** stdout not a terminal → no ANSI, no alt-screen. Skills pipe this binary.
3. **Every command reachable non-interactively**, and **`--json` on every command.** Two contracts
   are live: skills parse the fact-reporting commands' `key=value` text, and `review` reads
   `mkit findings --json`. `--json` is where the rest migrate.
4. **Commands report and run — they never integrate.** No staging, merging, pushing or editing the
   user's files. Their writes are bounded: `scratch prune` removes old run directories, `gate run`
   appends to `.mkit/gate.jsonl`, `worklog append` to the worklog, `branch status` runs
   `git fetch --prune`, `init` writes `.mkit/config.toml` (the one command that writes config), and
   `cache prune --apply` deletes stale Claude Code / Codex storage — only with `--apply`.
5. **Judgement stays in Markdown.** The binary owns mechanical invariants only; where the line is
   unclear it reports candidates and the skill chooses.
6. **Skills stay as files** — Markdown in this repo, served from the marketplace checkout, never
   embedded in the binary.
7. **Every workflow step is entry-capable** — runs alone, in any order, derives what's missing,
   names what it assumed, never sends the user to another step
   ([`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md)).
8. **Three write locations, by lifetime:** `$TMPDIR`, `<toplevel>/.mkit/`, `~/.mkit/` — plus one
   named exception, the common dir's `info/exclude`, so `.mkit/` stays ignored. State is never kept
   in `~/.claude` or the user's files; `cache prune --apply` only deletes there. Asserted by `TestWriteSitesAreOnTheReviewedAllowlist`
   ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).
9. **Degradation is named, never hit.** A path the sandbox would deny is reported with a remedy
   that works, or the command says it is human-run. Each such sentence has exactly one producer.
10. **Config is an input, never a permission.** Everything runs with no `.mkit/config.toml`; a
    problem in it is reported, never fatal ([ADR 0001](adr/0001-per-repo-config-and-init.md)).

## Next

### M8 — `mkit plan` + the `spec` and `implement` skills ([#9](https://github.com/masterik/mk-toolkit/issues/9))

The workflow's front half. `brainstorm` needs no binary support and can land any time.

- `mkit plan frontier|blocked|validate --json` — pure arithmetic over a task graph: unblocked
  slices, what blocks the rest, cycles, dangling edges. Never picks or sizes a slice.
- `spec` — synthesise what was discussed into a spec plus a task graph; never re-interview.
  Publish to the store the repo profile names, falling back to the run directory.
- `implement` — work the frontier one slice at a time, full gate between slices (the ledger makes
  unchanged steps `cached`). Sequential in place by default — `wt` is sandbox-fragile (observed failing to `mktemp`) and two
  editors over one tree collide; parallel worktrees behind a pinned
  `[implement] worktrees = true`.

**Done when** a spec written by `spec` can be implemented by `implement` in a fresh session from
the artifact and the worklog alone.

### Also open

- **Worktree helpers in the binary** — [ADR 0004](adr/0004-worktree-helpers-in-the-binary.md),
  [#37](https://github.com/masterik/mk-toolkit/issues/37).
- **Sign and notarize release binaries** — [#28](https://github.com/masterik/mk-toolkit/issues/28);
  the cask strips the quarantine flag on install meanwhile ([if Gatekeeper still blocks it](prerequisites.md#gatekeeper-blocks-the-binary)).

## Later

- `mkit stage hunks` — replace `commit`'s Markdown patch-staging recipe: cut a per-file diff, drop
  named hunks, `git apply --cached`, verify ([#11](https://github.com/masterik/mk-toolkit/issues/11)).
- `mkit session show` — read earlier transcript turns back so `explain` can cover compacted context.
- TUIs for `cleanup` and `review` ([#12](https://github.com/masterik/mk-toolkit/issues/12),
  [#13](https://github.com/masterik/mk-toolkit/issues/13)).
- `install`/`uninstall`, only if a job appears that manual steps can't do
  ([#10](https://github.com/masterik/mk-toolkit/issues/10)).
- Other agents (Codex, …) and other platforms ([#14](https://github.com/masterik/mk-toolkit/issues/14),
  [#15](https://github.com/masterik/mk-toolkit/issues/15)).

## Done

Milestone numbers are stable identities referenced from code comments and `AGENTS.md`. Details
live in the linked issues, the [Releases](https://github.com/masterik/mk-toolkit/releases) and
[`CHANGELOG.md`](../CHANGELOG.md) (frozen at 0.19.0).

| | What | Notes |
| --- | --- | --- |
| M1 | Scaffold + release chain | Go module, cobra root, GoReleaser, Homebrew cask (`v0.12.0`). |
| M2 | `mkit cache prune` | Ported from shell; TUI tick-list on `--apply`. |
| M3 | ~~`install`/`status`/`uninstall`~~ | Withdrawn by [ADR 0003](adr/0003-two-distribution-channels.md); `status` folded into `doctor`. |
| M4 | `mkit findings` ([#6](https://github.com/masterik/mk-toolkit/issues/6)) | Review arithmetic ported from JS; `review` stops without it. |
| M5 | The `jq` consumers ([#7](https://github.com/masterik/mk-toolkit/issues/7)) | `facts`, `gate detect/run`, `branch status`, `scratch` — the payload became Markdown only. |
| M6 | `mkit worklog` + workflow contract ([#8](https://github.com/masterik/mk-toolkit/issues/8)) | Per-branch record written by `commit`, `review`, `pr`, `finish`. |
| M7 | `repo profile`, `init`, `doctor` ([#3](https://github.com/masterik/mk-toolkit/issues/3)) | Committed `.mkit/config.toml`; consumers added in #17–#20. |
| — | `audit sandbox` + `sandbox-audit` skill ([#36](https://github.com/masterik/mk-toolkit/issues/36)) | |
| — | `explain` + `recap` skills ([#43](https://github.com/masterik/mk-toolkit/issues/43)) | |

## Accepted gaps

- **Nothing reports a missing tool unprompted.** The `SessionStart` hook that did was removed in
  0.15.0; `mkit doctor` is human-run, and a binary can't report its own absence. Re-adding a bash
  hook is a legitimate option if this proves expensive — do it deliberately, not by reflex.
