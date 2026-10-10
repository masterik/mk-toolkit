# mkit: backlog

This file holds the ordered work list and the rules every change must keep. Rationale is in
[`concept.md`](concept.md), unscheduled ideas in [`ideas/`](ideas/README.md), and open work in
[Issues](https://github.com/masterik/mk-toolkit/issues).

## Decisions

- **Go, one binary, subcommand tree.** It ships as a single static binary. The stdlib covers the
  workload (`os/exec`, `encoding/json`, `crypto/sha256`), and Go has the Charm TUI stack and GoReleaser's
  Homebrew support. Rust and Zig were rejected for subprocess orchestration.
- **macOS only.** Builds are darwin amd64 and arm64, and a cask can't install on Linux anyway. Adding a
  platform means a `goos` line plus a distribution channel, done when someone needs it.
- **Two channels.** Homebrew ships the binary and the marketplace ships the plugin
  ([ADR 0003](adr/0003-two-distribution-channels.md)). There is no installer: each channel is a
  one-time human step.
- **Presence, not versions.** Neither side declares a compatible range. `unknown command` is the too-old
  signal, and the remedy is `brew upgrade mkit`.

## Invariants

Breaking one is a design error, not a trade-off.

1. **Layering.** `core` returns data and never prints or assumes a terminal. `cli` formats; `tui`
   renders. There is no logic in a Bubble Tea `Update`.
2. **No TUI off a TTY.** If stdout is not a terminal, emit no ANSI and no alt-screen.
3. **Every command is non-interactive-capable and takes `--json`.** Two output contracts are live:
   skills parse the `key=value` text of the fact-reporting commands, and `review` reads
   `mkit findings --json`. The other commands migrate to `--json` over time.
4. **Commands report and run; they never integrate.** No staging, merging, pushing, or editing your
   files. Each command's writes are bounded:

   | Command | Writes |
   | --- | --- |
   | `scratch prune` | removes old run dirs |
   | `gate run` | appends to `.mkit/gate.jsonl` |
   | `worklog append` | appends to the worklog |
   | `branch status` | runs `git fetch --prune` |
   | `init` | writes `.mkit/config.toml` (the only config writer) |
   | `cache prune --apply` | deletes stale Claude Code / Codex storage, and only with `--apply` |

5. **Judgement stays in Markdown.** The binary owns mechanical invariants only. When the split is
   unclear, the binary reports candidates and the skill chooses.
6. **Skills stay as files.** They are Markdown in this repo, served from the marketplace, and never
   embedded in the binary.
7. **Every workflow step is entry-capable.** A step runs alone, in any order, derives what's missing,
   names its assumptions, and never sends the user to another step
   ([workflow contract](../plugin/skills/_shared/references/workflow-contract.md)).
8. **Three write locations, chosen by lifetime:** `$TMPDIR`, `<toplevel>/.mkit/` and `~/.mkit/`. The one
   exception is the common dir's `info/exclude`. No state is kept in `~/.claude` or your files
   (`cache prune --apply` only deletes there). `TestWriteSitesAreOnTheReviewedAllowlist` asserts this
   ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).
9. **Degradation is named, never hit.** A path the sandbox would deny is reported with a working
   remedy, or the command says it is human-run. Each such sentence has one producer.
10. **Config is an input, never a permission.** Everything runs without `.mkit/config.toml`. A problem
    in the config is reported, never fatal ([ADR 0001](adr/0001-per-repo-config-and-init.md)).

## Next

### M8: `mkit plan` + the `spec` and `implement` skills ([#9](https://github.com/masterik/mk-toolkit/issues/9))

This is the workflow's front half. `brainstorm` needs no binary support, so it can land at any time.

- **`mkit plan frontier|blocked|validate --json`**: pure graph arithmetic. It reports unblocked slices,
  blockers, cycles and dangling edges. It never picks or sizes a slice.
- **`spec`**: turns the discussion so far into a spec plus a task graph, without re-interviewing the
  user. It publishes to the repo profile's store, falling back to the run dir.
- **`implement`**: works the frontier one slice at a time, running the full gate between slices
  (unchanged steps come back `cached`). It runs sequentially in place by default, for two reasons:
  `wt` is fragile under the sandbox (it was seen failing to `mktemp`), and two editors in one tree
  collide. Parallel worktrees need `[implement] worktrees = true` pinned in config.

**Done when** `implement`, in a fresh session, can work a spec from `spec` using only the artifact and
the worklog.

### Also open

- **Worktree helpers in the binary**: [ADR 0004](adr/0004-worktree-helpers-in-the-binary.md),
  [#37](https://github.com/masterik/mk-toolkit/issues/37).
- **Signing and notarization**: [#28](https://github.com/masterik/mk-toolkit/issues/28). Until then,
  the cask strips the quarantine flag on install
  ([fallback](prerequisites.md#gatekeeper-blocks-the-binary)).

## Later

- **`mkit stage hunks`**: replaces `commit`'s Markdown patch-staging recipe
  ([#11](https://github.com/masterik/mk-toolkit/issues/11)).
- **`mkit session show`**: reads earlier transcript turns back, so `explain` can cover compacted
  context.
- **TUIs for `cleanup` and `review`**: [#12](https://github.com/masterik/mk-toolkit/issues/12),
  [#13](https://github.com/masterik/mk-toolkit/issues/13).
- **`install` / `uninstall`**: only if a job appears that manual steps can't do
  ([#10](https://github.com/masterik/mk-toolkit/issues/10)).
- **Other agents and platforms**: [#14](https://github.com/masterik/mk-toolkit/issues/14),
  [#15](https://github.com/masterik/mk-toolkit/issues/15).

## Done

Milestone numbers are stable IDs, referenced from code comments and `AGENTS.md`. Details are in the
linked issues, the [Releases](https://github.com/masterik/mk-toolkit/releases) and
[`CHANGELOG.md`](../CHANGELOG.md) (frozen at 0.19.0).

| | What | Notes |
| --- | --- | --- |
| M1 | Scaffold + release chain | Go module, cobra, GoReleaser, Homebrew cask (`v0.12.0`). |
| M2 | `mkit cache prune` | Ported from shell; TUI tick-list on `--apply`. |
| M3 | ~~`install`/`status`/`uninstall`~~ | Withdrawn ([ADR 0003](adr/0003-two-distribution-channels.md)); `status` folded into `doctor`. |
| M4 | `mkit findings` ([#6](https://github.com/masterik/mk-toolkit/issues/6)) | Review arithmetic ported from JS. |
| M5 | The `jq` consumers ([#7](https://github.com/masterik/mk-toolkit/issues/7)) | `facts`, `gate`, `branch status`, `scratch`; the payload became Markdown only. |
| M6 | `mkit worklog` + workflow contract ([#8](https://github.com/masterik/mk-toolkit/issues/8)) | Written by `commit`, `review`, `pr`, `finish`. |
| M7 | `repo profile`, `init`, `doctor` ([#3](https://github.com/masterik/mk-toolkit/issues/3)) | Committed `.mkit/config.toml`; consumers in #17–#20. |
| — | `audit sandbox` + `sandbox-audit` ([#36](https://github.com/masterik/mk-toolkit/issues/36)) | |
| — | `explain` + `recap` ([#43](https://github.com/masterik/mk-toolkit/issues/43)) | |

## Accepted gaps

- **Nothing reports a missing tool unprompted.** The `SessionStart` hook that did this was removed in
  0.15.0. `mkit doctor` is human-run, and a binary can't report its own absence. Re-adding a hook is
  fine if this gap proves costly, but do it deliberately.
