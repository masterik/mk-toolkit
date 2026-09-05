# mkit — Backlog

Migration from a shell-script plugin to a **Go binary + plugin**, distributed via Homebrew.
Direction and rationale: [`concept.md`](concept.md). This file is the ordered work list.
Researched but unscheduled ideas — deliberately off this list —
live in [`ideas/`](ideas/README.md).

## Decision
- **Language: Go.** Chosen for a single static binary with no C toolchain, a stdlib that covers
  the entire workload (`os/exec`, `encoding/json`, `crypto/sha256`, `filepath.WalkDir`), the Charm
  TUI stack, and GoReleaser's Homebrew tap generation. Rejected: Rust (slower edit→test loop, for
  no gain on subprocess-orchestration work), Zig (pre-1.0, breaking releases, no TUI ecosystem).
  Cross-compilation is a property Go gives away, **not** a requirement here — see the platform
  decision below.
- **Platform: macOS only.** `.goreleaser.yaml` builds `darwin` × amd64/arm64 and nothing else.
  This matches the script layer rather than diverging from it, and the tap publishes a *cask*,
  which Homebrew refuses to install on Linux — so a linux archive would have had no `brew` path
  to reach a user through. Adding a `goos` back is one line whenever someone needs it; carrying
  an untested OS in the matrix is a support claim nobody verifies.
- **One binary, subcommand tree.** `mkit storage prune`, `mkit gate run`, `mkit status`.
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

### M1 — Repo reorg + scaffold + release chain — done
Repo renamed to `mk-toolkit`, tree reorganized (payload under `plugin/`, docs under `docs/`),
Go module (`github.com/masterik/mk-toolkit`), cobra root with the front-end contract, GoReleaser
config, `masterik/homebrew-tap`. No behavior beyond `mkit version`.
Step-by-step plan (local, gitignored): `.claude/plans/2026-08-26-mkit-m1-go-scaffold.md`.
Tagged `v0.12.0` → GitHub Release with an archive per `goos`/`goarch` in the config, checksums,
and an auto-committed Homebrew cask formula (`brews` is deprecated in GoReleaser v2; used
`homebrew_casks` instead). `brew install masterik/tap/mkit` verified end to end. The repo had to
be flipped from private to public — an unauthenticated `brew install` can't reach private-repo
release assets.

### M2 — `mkit storage prune` — done
Ported `tools/storage-prune.sh` to `internal/core/storage/` + `internal/cli/storage*.go` +
`internal/tui/storageprune/`. Eliminates the per-file `stat` and per-category `find` *subprocess
forks* the shell version paid for `sum_size`/`prune_files`/`prune_stale_dirs` — not a syscall
saving: on macOS `readdir` carries no size, so `DirEntry.Info()` still issues an `lstat` per file,
same as the script's `stat -f%z`. The win is process elimination.
Differential check against the script (`.claude/plans/2026-08-27-mkit-m2-storage-prune.md`, step
4) showed a clean diff except at the retention-boundary days, exactly as the plan's single-cutoff
deviation (mtime strictly before `now - N*24h`, vs. the script's `-mtime +N`/`-mtime -N` split)
predicts.
**Done when:** dry-run output matches the shell version's categories and totals outside the
boundary case; `--apply` deletes the same set; a TUI mode offers a size-sorted tick-list before
applying. All met.

### M3 — `mkit install` / `status` / `uninstall`
Absorbs `install.sh`. The `SessionStart` hook is **not** in scope — it stays in bash
permanently (see "Staying in bash, permanently" below), because it cannot depend on a binary
whose absence it may have to report. Registers the Homebrew-installed plugin payload as a
`directory` marketplace in `~/.claude/settings.json`.
Installer targets are an interface from the start — `claude` now, `codex` later. M1 already
consolidated the payload under `plugin/`, so this milestone points at one path rather than
enumerating root directories.
**Blocked on two packaging gaps M1 left open — resolve these before writing any Go.** Both were
found by inspecting the shipped `v0.12.0` cask, not by reading the config:

1. **The payload is not in the archive.** `.goreleaser.yaml` declares `archives: [formats:
   [tar.gz]]` with no `files:`, so the tarball carries only the binary plus goreleaser's default
   `LICENSE` + `README.md`. The installed cask contains exactly those three entries — no
   `plugin/` at all. Nothing can be registered until the payload is added to the archive.
2. **A cask has no stable path to register.** `homebrew_casks` was chosen in M1 because `brews`
   is deprecated, but a cask is not a keg: it installs to
   `/opt/homebrew/Caskroom/mkit/<version>/` and **never creates `/opt/homebrew/opt/<name>`**.
   Verified — `/opt/homebrew/opt/mkit` does not exist, and `/opt/homebrew/bin/mkit` is a symlink
   straight into `Caskroom/mkit/0.12.0/mkit`. So the `opt` path this milestone was written
   against does not exist, and the only path that does is version-pinned — precisely the failure
   the bullet below is guarding against. Casks also have no artifact stanza for "install this
   directory into `share/`"; `binary` is the one that applies.

   Pick one, deliberately:
   - **Switch the tap entry to a formula.** Kegs get `opt/`, and `share/mkit/plugin` installs
     naturally, so the registration below works as originally written. Costs re-doing M1's
     release chain and understanding why `brews` was deprecated before depending on it.
   - **Keep the cask; register a path `mkit install` owns.** Copy the payload out of the
     versioned Caskroom directory into a stable location the binary controls (e.g.
     `$MKIT_HOME/plugin`) and register *that*. No tap rework, but `mkit install` becomes
     load-bearing for upgrades — it must re-copy after every `brew upgrade`, which the
     `SessionStart` hook is the natural thing to detect.

   Until this is settled, "Homebrew ships binary *and* plugin payload" in the Decision section
   above, and the same claim in `README.md`, `concept.md` and `AGENTS.md`, are statements of
   intent rather than fact.

- Register the payload by a path that survives `brew upgrade`. Never a versioned path
  (`Cellar/…`, `Caskroom/<version>/…`): it rots on the next upgrade.
- `autoUpdate: false` — a directory source's autoUpdate implies a git pull and the Homebrew
  payload is not a checkout. `brew upgrade mkit` is the update mechanism.
- Merge into existing settings, never overwrite. `--dry-run` prints the diff.
**Done when:** `brew install mkit && mkit install` yields a working plugin with no clone, the
registered path still resolves after a `brew upgrade`, and `mkit status` reports what today's
`install.sh --status` does — the prerequisite table, the `SessionStart` hook's state, and the
gate ledger's.

### M4 — `mkit findings`
Port `scripts/findings.mjs` (507 lines). Pure data transformation, so parity is testable.
**Done when:** `node` is gone from [`prerequisites.md`](prerequisites.md).

### M5 — the `jq` consumers
Port `branch-scan.sh`, `gate-run.sh`, `facts.sh` and `gate-detect.sh`. Deletes a whole family
of degradation branches — `pr=jq-missing`, `gate_cache=no-jq`, `no-hash` — because a binary is
never half-capable. The last milestone in the port: the shell payload after it is
`run-open.sh` and the `SessionStart` hook.
Drop the fast tier in the same port. Since the tier was removed from `commit` and `review`, no
skill consumes `fast=` or `fast_cache=`, yet `gate-detect.sh` still derives both for every
ecosystem and the ledger still classifies them. Dead output is not a compatibility surface: the
Go command proposes the full tier only.
**Done when:** `jq` and `shasum` are gone from [`prerequisites.md`](prerequisites.md).

### Later
- Delete `tools/purge-journal-state.sh`. It exists only to clear what mkit ≤ 0.12.1 left behind
  when journaling was removed, so it is finished the moment every machine that ran that version
  has run it once. Tracked here because nothing else will surface it.
- `mkit cleanup` TUI — multi-select over `branch-scan.sh`'s classification.
- `mkit review` TUI — live parallel reviewer progress.
- Codex installer target (`~/.codex/`).
- Other platforms. Deliberately out (see Decision). Reversing it means adding the `goos` entry,
  auditing the path/exec assumptions, and picking a distribution channel a cask can't serve —
  Linuxbrew needs a *formula*, Windows a Scoop manifest (GoReleaser emits one).

## Staying in bash, permanently
- `scripts/hooks/session-bootstrap.sh` — it cannot depend on a binary whose presence it may
  have to report as missing, and its whole job is reporting a missing tool. Already reduced to
  that one job; there is nothing left to port out of it.
- Any hook that must run before setup completes.

## Porting rules
- Each shell script's `.bats` file is a ready-made spec — port it to `go test` alongside the
  code, do not re-derive the behavior.
- One script per milestone, merged green. No big-bang rewrite: the bash is commented,
  tested and load-bearing, and a mass rewrite is pure regression risk.
- Delete the shell script in the same commit that lands its replacement. Two implementations
  of one invariant is the failure mode the script layer exists to prevent.

## Open questions
- **Version skew — the live state, no longer a hypothetical.** `plugin.json` is at `0.13.0`
  while the newest tag is `v0.12.0`, so the only binary Homebrew can install is a minor behind
  the payload — and because the cask carries no payload (M3), the plugin side is whatever
  checkout the machine has. Does the binary assert a minimum plugin version, or stay backward
  compatible? Settle it in M3: registering a Homebrew-provided payload is the point where the
  two versions begin moving together and a skew stops being visible.

Resolved: Homebrew is the only distribution channel (no `curl | sh`, no `go install`) — see the
M1 plan's decisions. The `SessionStart` hook still self-heals: it runs pre-install, before a
Homebrew-provided binary is on PATH.
