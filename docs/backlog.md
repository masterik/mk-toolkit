# mkit — Backlog

Migration from a shell-script plugin to a **Go binary + plugin**, distributed via Homebrew.
Direction and rationale: [`concept.md`](concept.md). This file is the ordered work list.

## Decision
- **Language: Go.** Chosen for cross-compilation (`GOOS`/`GOARCH`, no C toolchain), a stdlib
  that covers the entire workload (`os/exec`, `encoding/json`, `crypto/sha256`,
  `filepath.WalkDir`), the Charm TUI stack, and GoReleaser's Homebrew/Scoop tap generation.
  Rejected: Rust (cross-compile friction, slower edit→test loop, for no gain on
  subprocess-orchestration work), Zig (pre-1.0, breaking releases, no TUI ecosystem).
- **One binary, subcommand tree.** `mkit storage prune`, `mkit journal add`, `mkit gate run`.
- **Dual front-end.** Rich TUI when interactive; flags + `--json` when driven by a skill.
- **Homebrew ships binary *and* plugin payload.** `brew upgrade mkit` updates both.

## Invariants
Rules that must hold through every milestone. A change that breaks one is a design error,
not a trade-off.

1. **Layering.** `internal/core` returns data and never prints or assumes a terminal. `cmd/`
   formats. `internal/tui/` renders. Both front-ends stay thin; logic never lives in a
   Bubble Tea `Update`.
2. **No TUI off a TTY.** stdout not a terminal → non-interactive, no ANSI, no alt-screen.
   Skills shell out to this binary; a TUI on a pipe is corruption, not cosmetics.
3. **Every command reachable non-interactively.** Anything the TUI can do, flags can do.
4. **`--json` on every command.** Present output stays the default human form; `--json` is
   the skill-facing contract, replacing today's ad-hoc `key=value` parsing.
5. **Scripts still report and run.** No staging, merging, pushing or editing — carried over
   from the shell layer unchanged.
6. **Judgement stays in Markdown.** The binary owns mechanical invariants only. Where the
   line is unclear, report candidates and let the skill choose.
7. **Skills stay as files.** Authored as Markdown in this repo, copied by the formula. Not
   embedded via `embed.FS` — they must stay diffable and reviewable.

## Milestones

### M1 — Repo reorg + scaffold + release chain
Repo rename, tree reorganized (payload under `plugin/`, docs under `docs/`), Go module,
cobra root, GoReleaser config, Homebrew tap repo. No behavior.
Step-by-step plan (local, gitignored): `.claude/plans/2026-08-26-mkit-m1-go-scaffold.md`.
**Done when:** the shell suite still passes after the move, and a tagged commit produces a
GitHub Release with darwin/linux archives and an auto-committed tap formula, such that
`brew install` puts a working `mkit version` on PATH.

### M2 — `mkit storage prune`
Port `tools/storage-prune.sh`. Greenfield, no bats suite to preserve. CLI first, TUI view
second over the same core. Fixes the per-file `stat` fork in `sum_size` — `WalkDir` gets
size from the dirent already read.
**Done when:** dry-run output matches the shell version's categories and totals; `--apply`
deletes the same set; a TUI mode offers a size-sorted tick-list before applying.

### M3 — `mkit install` / `status` / `uninstall`
Absorbs `install.sh` and most of `scripts/hooks/session-bootstrap.sh`. Registers the
Homebrew-installed plugin payload as a `directory` marketplace in `~/.claude/settings.json`.
Installer targets are an interface from the start — `claude` now, `codex` later. M1 already
consolidated the payload under `plugin/`, so this milestone points at one path rather than
enumerating root directories.
- Register `/opt/homebrew/opt/mkit/share/mkit/plugin` — the **`opt`** symlink, stable across
  upgrades. Never a `Cellar` path: it is version-pinned and rots on the next `brew upgrade`.
- `autoUpdate: false` — a directory source's autoUpdate implies a git pull and the Homebrew
  payload is not a checkout. `brew upgrade mkit` is the update mechanism.
- Merge into existing settings, never overwrite. `--dry-run` prints the diff.
**Done when:** `brew install mkit && mkit install` yields a working plugin with no clone, and
`mkit status` reports what today's `install.sh --status` does.

### M4 — `mkit findings`
Port `scripts/findings.mjs` (507 lines). Pure data transformation, so parity is testable.
**Done when:** `node` is gone from `PREREQUISITES.md`.

### M5 — the `jq` consumers
Port `branch-scan.sh`, `gate-run.sh`, and `facts.sh`'s journal block. Deletes a whole family
of degradation branches — `pr=jq-missing`, `journal=jq-missing`, `gate_cache=no-jq`,
`no-hash` — because a binary is never half-capable.
**Done when:** `jq` and `shasum` are gone from `PREREQUISITES.md`.

### M6 — `mkit journal`
Port `journal.sh` (874 lines, the largest). Last, once the porting pattern is proven.
TUI: browse and edit records.

### Later
- `mkit cleanup` TUI — multi-select over `branch-scan.sh`'s classification.
- `mkit review` TUI — live parallel reviewer progress.
- Codex installer target (`~/.codex/`).
- Windows: Scoop manifest (GoReleaser emits it), plus the path/exec assumptions to audit.

## Staying in bash, permanently
- `scripts/hooks/session-bootstrap.sh` — reduced but not deleted. It cannot depend on a
  binary whose presence it may have to report as missing. Under Homebrew most of its job is
  gone: the binary is already on PATH.
- Any hook that must run before setup completes.

## Porting rules
- Each shell script's `.bats` file is a ready-made spec — port it to `go test` alongside the
  code, do not re-derive the behavior.
- One script per milestone, merged green. No big-bang rewrite: the bash is commented,
  tested and load-bearing, and a mass rewrite is pure regression risk.
- Delete the shell script in the same commit that lands its replacement. Two implementations
  of one invariant is the failure mode the script layer exists to prevent.

## Open questions
- **Version skew.** A skill from plugin v0.13 calling a binary from v0.12 — does the binary
  assert a minimum plugin version, or stay backward compatible? Only matters once someone
  installs outside Homebrew.

Resolved: Homebrew is the only distribution channel (no `curl | sh`, no `go install`) — see the
M1 plan's decisions. The `SessionStart` hook still self-heals: it runs pre-install, before a
Homebrew-provided binary is on PATH.
