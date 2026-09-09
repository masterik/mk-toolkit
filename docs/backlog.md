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
8. **Every workflow step is entry-capable.** Any of the seven runs as the only thing in a session,
   in any order, with any subset of the others skipped. A step discovers what it needs, derives the
   thin version of what is missing, names what it assumed, and completes. It never sends the user
   to another step, and never runs one downstream of itself. Full contract:
   [`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md).
9. **State is repo-scoped.** `<toplevel>/.mkit/` by default — run directories, the gate ledger, the
   worklog; inside the working directory, so the sandbox, the auto-mode classifier and the
   worktree-isolation guard all permit it with no configuration. User scope (`~/.mkit/`, overridable
   with `MKIT_HOME`) holds only what must outlive every repo: the hook tombstone and its
   once-per-tool messages. Not `~/.claude/…`: that is a *protected* region where an allowlist entry
   is inert, so it is a path no remedy sentence can point at
   ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).
10. **Sandbox degradation is named, never hit.** A command that would write a sandbox-denied path
    reports which path *and a remedy that works*, rather than surfacing `Operation not permitted`.
    `mkit doctor` reports the writable set as a first-class fact. Same rule as a missing
    prerequisite, one axis over.
11. **Ephemeral files go to `$TMPDIR`, always with an explicit `mktemp` template.** The division is
    by lifetime, not by caller: a file that dies with the command uses `$TMPDIR`, a file a later
    step or a later session reads uses the run directory. The bare and `-t` forms of `mktemp` are
    banned outright — on macOS they resolve the Darwin per-user temp directory and *ignore*
    `$TMPDIR`, so they are unfixable by environment. Asserted statically over the payload; it has no
    behavioral seam, since a test cannot create an OS sandbox.
12. **The writable set is a property of the payload.** No shipped script writes outside the run
    directory, `$TMPDIR` and the user-scoped root — plus one named exception, the common dir's
    `info/exclude`, one line, so `.mkit/` stays out of `git status`. Nothing is ever written to the
    user's own files: `.mkit/` is inside a working tree by design, and is the only thing that is.
    Asserted statically, by shape: every write target is a shell parameter on a reviewed allowlist,
    never a literal path.
13. **Config is an input, never a permission.** Every command and every step runs with no config
    present. `mkit init` removes repeated discovery and captures what inspection cannot establish;
    it never becomes a precondition, and a pinned value cheap to verify gets verified
    ([ADR 0001](adr/0001-per-repo-config-and-init.md)).

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
3. **Registration in `~/.claude/settings.json` is a human-run step. The tombstone is not, any more.**
   Settled by [ADR 0002](adr/0002-state-locations-under-a-sandbox.md), which resolved the choice this
   entry used to pose. The tombstone moved to `~/.mkit/` — outside the sandbox's protected region, so
   one `permissions.additionalDirectories` entry genuinely opens it, and `mkit uninstall` is
   skill-invocable on a machine that has the entry and reports the exact remedy on one that does not.
   Repo-scoping the tombstone is no longer needed and was rejected: silencing that the user set once
   should not have to be re-set per repo.

   `~/.claude/settings.json` has no such escape — it is inside the protected region and explicitly
   denied besides, and no allowlist entry lifts that. So **`mkit install`'s marketplace registration
   is a human-run step** (`! mkit install`) and this milestone's "Done when" says so. Per invariant
   10, it names the denied path *and* the fact that the command is human-run, rather than offering
   configuration that cannot work.

**Done when:** `brew install mkit && ! mkit install` yields a working plugin with no clone (the
registration step named as human-run), the registered path still resolves after a `brew upgrade`, and
`mkit status` reports what today's `install.sh --status` does — the prerequisite table, the state
locations and whether the user-scoped one is writable, the `SessionStart` hook's state, and the gate
ledger's — with its exit status still the prerequisite verdict alone.

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

### M6 — `mkit work` + the workflow contract
The substrate the seven steps stand on, landed before any of the new skills, so the back half
starts recording immediately and the front half has something to read.
- `mkit work show|append`, `--json`. `<toplevel>/.mkit/work/<branch>.jsonl`, append-only, rotated
  like `gate.jsonl`, never committed, per-worktree. A record carries step, timestamp, content
  fingerprint (reusing `mkit_tree_fingerprint`'s successor), artifact pointer, one-line gist, and
  assumptions. Appending is bookkeeping; **reading it is judgement and stays in the skills.**
- Ship [`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md) and link
  it from all four existing skills.
- Retrofit the back half: `commit`, `review`, `pr`, `finish` each append one record and each read
  the log for a goal before deriving one. `review`'s step 1 goal derivation is the model — it
  already degrades correctly, so this generalises an existing behaviour rather than inventing one.
**Done when:** a branch that ran `commit` then `review` shows both in `mkit work show --json`, and
`review` invoked cold on that branch takes its goal from the log instead of the branch name.

### M7 — `mkit repo profile` + `init` + `doctor`
The configuration surface ([ADR 0001](adr/0001-per-repo-config-and-init.md)). Independent of M6,
and worth landing near it: every new skill would otherwise rediscover the same facts apart.
- `mkit repo profile --json` — gate commands, spec store, scopes, reviewers, merge style, each
  tagged `discovered` or `pinned`. Discovery reads `docs/agents/issue-tracker.md` where present.
- `mkit init` — writes the pinned remainder, committed. Interactive TUI on a TTY, flags otherwise
  (invariants 2 and 3). Writes nothing outside the repo.
- `mkit doctor` — prerequisites, permission-allowlist gaps against what the skills invoke, hook
  registration, plugin enablement, and the **sandbox writable set**. Reports; fixes nothing.
- Fold `install.sh --status`'s degradation sentences in, so they keep one producer.
**Done when:** `mkit doctor` names an unwritable `~/.mkit/` on a sandboxed run without failing, with
the `additionalDirectories` remedy beside it,
`mkit repo profile --json` distinguishes discovered from pinned on this repo, and `mkit init` is a
no-op on a repo it has already configured.

### M8 — `mkit plan` + the `spec` and `implement` skills
The front half's mechanical core plus the two skills that consume it. `brainstorm` needs no binary
support and can land whenever.
- `mkit plan frontier|blocked|validate`, `--json` — pure arithmetic over the task graph: which
  slices are unblocked, which are gated by what, cycle and dangling-edge detection. It never
  chooses a slice, sizes one, or decides one is done.
- `spec` — synthesise, never re-interview; publish the spec and the graph to the store the profile
  names, falling back to the run directory when that store is unreachable.
- `implement` — work the frontier one slice at a time, gate between slices, full gate at the end.
  **Sequential in place is the default**, not a worktree per slice: `wt` is sandbox-fragile
  (observed failing to `mktemp`), and parallel editors over one tree is the collision `review`
  already avoids. Parallel worktrees stay an opt-in for a graph with genuinely independent slices.
- **Interaction with M5, settle it there or here:** M5 drops the fast tier because no skill consumes
  `fast=`. `implement` is exactly the consumer that wants one — a per-slice cheap check with the
  full gate held for the end. Either M5 keeps the fast tier for this milestone's sake, or
  `implement` runs the full gate every slice and leans on the ledger for the cache. Decide before
  M5 deletes it, because resurrecting it afterwards costs more than keeping it.
**Done when:** a spec written by `spec` can be implemented by `implement` on a fresh session with
no conversation context, working from the artifact and the worklog alone.

### Later
- Delete `tools/purge-journal-state.sh`. It exists only to clear what mkit ≤ 0.12.1 left behind
  when journaling was removed, so it is finished the moment every machine that ran that version
  has run it once. Tracked here because nothing else will surface it.
- `mkit stage hunks` — the eventual replacement for `commit`'s Markdown patch-staging recipe.
  Mechanical throughout: cut a per-file diff, drop named hunks, `git apply --cached`, verify with a
  staged stat, and refuse a split that would cut inside a hunk (intermediate commits must build).
  The recipe works and is the right thing to ship first; a command earns its place by removing the
  `@@`-block editing an agent currently does by hand, not by unbreaking anything. Separate from
  invariant 6, since which hunks go in which commit stays a judgement in the skill.
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
