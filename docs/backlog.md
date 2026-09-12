# mkit — Backlog

Migration from a shell-script plugin to a **Go binary + plugin**, over **two deliberately
independent distribution channels**: the binary via Homebrew, the plugin payload via the GitHub
marketplace ([ADR 0003](adr/0003-two-distribution-channels.md)).
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
- **Two channels; Homebrew carries only the binary.** The plugin payload ships from the GitHub
  marketplace — `/plugin marketplace add masterik/mk-toolkit`, which works today — and the cask
  installs one executable and nothing else. This **dissolved** both packaging blockers M3 was
  stuck on rather than solving them: no `files:` entry in the archive, no cask-vs-formula
  rework, no version-pinned Caskroom path to register, and no write to
  `~/.claude/settings.json`, which is inside the sandbox's protected region *and* explicitly
  denied, so no allowlist entry could ever have lifted it. The cost is that the two artifacts
  version independently — see M4, which settled what that costs: presence only, no declared
  range on either side
  ([ADR 0003](adr/0003-two-distribution-channels.md)).
- **Installation is manual in this phase.** No `mkit install`, and no `install.sh`. Adding a
  marketplace and enabling a plugin are two lines a human runs once; a command that wraps them
  buys nothing while it cannot write the file it would need to write. Silencing the
  `SessionStart` hook is likewise manual — the tombstone is a file, and creating it is the whole
  operation. `install`/`uninstall` get re-specified later, from whatever the binary actually
  needs by then rather than from what the shell script happened to do.

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
7. **Skills stay as files.** Authored as Markdown in this repo and served from the marketplace
   checkout Claude Code maintains. Not embedded via `embed.FS`, and not carried by the cask —
   they must stay diffable and reviewable, and the binary must never be what makes a skill
   available.
8. **Every workflow step is entry-capable.** Any of the seven runs as the only thing in a session,
   in any order, with any subset of the others skipped. A step discovers what it needs, derives the
   thin version of what is missing, names what it assumed, and completes. It never sends the user
   to another step, and never runs one downstream of itself. Full contract:
   [`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md).
9. **State is repo-scoped.** `<toplevel>/.mkit/` by default — run directories, the gate ledger, the
   worklog; inside the working directory, so the sandbox, the auto-mode classifier and the
   worktree-isolation guard all permit it with no configuration. One file in there is **committed**
   and not scratch: `config.toml`, the repo config, which is why the ignore rule is the pair
   `.mkit/*` + `!.mkit/config.toml` rather than a directory-only line. User scope (`~/.mkit/`, overridable
   with `MKIT_HOME`) holds only what must outlive every repo — **empty today**: its two files, the
   hook tombstone and its once-per-tool messages, went with the hook in 0.15.0. Not `~/.claude/…`: that is a *protected* region where an allowlist entry
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

## Near-term, outside the port line

Neither of these is a port, and neither waits on a milestone. Both follow from
[ADR 0002](adr/0002-state-locations-under-a-sandbox.md) landing and
[ADR 0003](adr/0003-two-distribution-channels.md) being taken.

- ~~**Delete `plugin/install.sh`.**~~ **Done (0.15.0)**, together with the `SessionStart` hook —
  `hooks/hooks.json`, `scripts/hooks/session-bootstrap.sh`, both `.bats` suites, and the
  `mkit_prereq_rows` / `mkit_state_*` / `mkit_json_escape` helpers in `lib/common.sh`. Prerequisite
  reporting moves to the binary rather than being reimplemented in shell.
  - **The tombstone is gone, not manual.** With no hook to silence, `~/.mkit/bootstrap.disabled`
    signals nothing; `prerequisites.md` says to delete a leftover one.
  - **Unprompted prerequisite detection stays lost after M7.** A missing tool surfaces only as a
    thinner `facts.sh` block or a `gate_cache=no-hash` annotation until someone runs `doctor`.
    `doctor` restored the human-run report and cannot restore the unprompted one, and cannot
    report a missing `mkit` at all — see the note under "Staying in bash, permanently" below. Do
    not re-add a shell reporter for either gap.
  - **`facts.sh`'s `user_dir_writable=` survives**, so no skill lost information. `~/.mkit/` is
    empty but still the declared home for user-scoped state.

## Milestones

**Order.** `mkit init` was the priority, so **M7 went first**, ahead of the remaining ports; it is
done, and **M4 followed**, then **M6** — the worklog is what the front half will read, so it was
worth having before any of the new skills exist. **M5 is next**, then M8 as written. M3 is
withdrawn. The M-numbers are stable
identities referenced from `concept.md` and `AGENTS.md`, so nothing is renumbered when the order
changes.

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

### M3 — withdrawn

Was `mkit install` / `status` / `uninstall`, and it never started. It sat blocked on two
packaging gaps found by inspecting the shipped `v0.12.0` cask — the payload was not in the
archive, and a cask has no stable path to register (Caskroom is version-pinned and there is no
`opt/` symlink). [ADR 0003](adr/0003-two-distribution-channels.md) removed the premise instead
of the blockers: the plugin ships from GitHub, so there is no Homebrew-provided payload to
register and no marketplace entry for a command to write.

What became of its three jobs:

- **`install`** — nothing to do. Adding the marketplace and enabling the plugin is manual, and
  `~/.claude/settings.json` is sandbox-denied besides.
- **`status`** — folded into M7's `mkit doctor`, which was already specified to overlap it.
- **`uninstall`** — nothing to undo. The tombstone existed only to silence the hook, and both
  are gone (0.15.0).

Re-specified later if the binary turns out to need either verb (see Later). Do not resurrect
this entry as written — it is scoped against a distribution model the project no longer has.

### M4 — `mkit findings` — done
Ported `scripts/findings.mjs` (507 lines) to `internal/core/findings/` + `internal/cli/findings.go`.
Pure data transformation, so parity was testable: all 21 cases of `tests/findings.test.mjs` are now
`go test` beside the package, plus the JS→Go parity hazards the port introduced —
`Number(x.toFixed(2))` rounds half away from zero where Go's `FormatFloat` rounds half to even
(`sim` is compared against `--sim` and `--band`, so 0.125 decides a merge), `sort.SliceStable`
everywhere because ES2019's sort is stable and ids come from a sort with ties, `||=` rather than
`??=` on `source`, and an order-preserving record type so an unknown field a reviewer added
round-trips into `final.jsonl` instead of being dropped by a struct.

**The version-skew guard, and why it points nowhere.** This is the milestone that created the
problem: it is the first port that makes a *skill* call the binary, so a plugin from GitHub can
now meet a binary from Homebrew too old for it, with no shared release to keep them in step. The
answer is that **neither side declares a range**. No comparable tool does — worktrunk ships CLI
and plugin from one repo and states compatibility in free-text frontmatter; coderabbit, the exact
analogue, checks `coderabbit --version || echo NOT_INSTALLED` in Markdown at step 1 of its skill
and carries its one per-feature minimum as untested prose; codegraph declares capability and names
a fallback. So `facts.sh` reports `mkit=` and `mkit_bin=` as raw starting facts and compares
nothing, and the check is **presence only**: `review` runs `command -v mkit && mkit findings schema
--json` at step 0, where a subcommand that does not exist *is* the too-old signal. Absent or too
old → **stop**, with `brew install masterik/tap/mkit` / `brew upgrade mkit`. That costs invariant 8
for `review` specifically, deliberately: without the arithmetic there is no reconcile, no groups
and no ids for verdicts to reference, and the alternative puts back into judgement exactly what
this stage exists to remove.

**The probe lives in the skill, not in `facts.sh`.** The guard's original home here — "named once
by the `SessionStart` hook the way a missing `jq` already is" — stopped existing in 0.15.0 when the
hook and all payload prerequisite reporting were deleted. M5 closes the other door: it folds
`facts.sh` into the binary, after which the call that would report `mkit=<version>` *is* `mkit` and
cannot report its own absence. A probe in the skill's own Markdown survives both.

**Done:** `node` is gone from [`prerequisites.md`](prerequisites.md) and `tests/run.sh`; `review`
consumes `mkit findings … --json` at steps 3, 4 and 6 with the merge and verdict judgement prose in
Markdown; with `mkit` off `PATH`, `review` says so at step 0 with a remedy that works instead of
failing at step 3.

### M5 — the `jq` consumers
Port `branch-scan.sh`, `gate-run.sh`, `facts.sh` and `gate-detect.sh`. Deletes a whole family
of degradation branches — `pr=jq-missing`, `gate_cache=no-jq`, `no-hash` — because a binary is
never half-capable. The last milestone in the port: the shell payload after it is `run-open.sh`
alone. **It also removes the last place a shell script could report the binary's absence** — after
this, `facts.sh` *is* `mkit` — which is why M4 put `review`'s presence probe in the skill's own
Markdown rather than here.
Drop the fast tier in the same port. Since the tier was removed from `commit` and `review`, no
skill consumes `fast=` or `fast_cache=`, yet `gate-detect.sh` still derives both for every
ecosystem and the ledger still classifies them. Dead output is not a compatibility surface: the
Go command proposes the full tier only.
**Done when:** `jq` and `shasum` are gone from [`prerequisites.md`](prerequisites.md).

### M6 — `mkit work` + the workflow contract — done
The substrate the seven steps stand on, landed before any of the new skills, so the back half
starts recording immediately and the front half has something to read.

**One deviation from the plan:**
[`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md) already shipped,
ahead of the command it documents — so this milestone made the contract true rather than writing
it. What was missing was the link from the four skills, and the command itself.
- `mkit work show|append`, `--json`. `<toplevel>/.mkit/work/<branch>.jsonl`, append-only, rotated
  like `gate.jsonl`, never committed, per-worktree. A record carries step, timestamp, content
  fingerprint (reusing `mkit_tree_fingerprint`'s successor), artifact pointer, one-line gist, and
  assumptions. Appending is bookkeeping; **reading it is judgement and stays in the skills.**
  The fingerprint is reached through `pluginroot`'s `CommonFunc` — one producer until M5 ports it,
  the same delegation M7 used for gate discovery. An unavailable one is `""` plus a named cause,
  never a failed append.
  `work append` **errors** on a failed write, unlike the gate ledger's best-effort appends: it is a
  command someone invoked. The best-effort half lives in the skills, which append after their report
  and treat a failure as one line of note — rule 4 says a recorded fact is an input, never a
  permission. Exit codes follow M4's vocabulary rather than adding one: `usageErr` (2) for a
  mistake at the command line — an unknown `--step`, a missing `--gist` — and a plain error (1) for
  no work tree, which is the environment and is what `repo profile` already returns there.
- Ship [`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md) and link
  it from all four existing skills.
- **The worklog calls are optional, and that is the difference from M4's probe.** M4 settled the
  skew question as presence-only, and made `review` *stop* when `mkit findings` is missing, because
  without the arithmetic there is no reconcile. Nothing here is load-bearing that way: a worklog
  `mkit` cannot answer for costs a step one input and never stops it, which is rule 4 again — a
  recorded fact is an input, never a permission. So these calls read `facts.sh`'s `mkit=` /
  `mkit_bin=` starting facts and carry on either way, and no skill grew a second probe.
- Retrofit the back half: `commit`, `review`, `pr`, `finish` each append one record and each read
  the log for a goal before deriving one. `review`'s step 1 goal derivation is the model — it
  already degrades correctly, so this generalises an existing behaviour rather than inventing one.
**Done when:** a branch that ran `commit` then `review` shows both in `mkit work show --json`, and
`review` invoked cold on that branch takes its goal from the log instead of the branch name.
`finish` is the exception worth naming: it destroys the log it would write to, so it records only
where the run stopped short of the cleanup.

### M7 — `mkit repo profile` + `init` + `doctor` — done
The configuration surface ([ADR 0001](adr/0001-per-repo-config-and-init.md)). Independent of M6
and of every remaining port, which is what let it come first: `mkit init` in each project was the
priority, and nothing in the ports blocked it.

**The blocking decision is settled:** repo config is `<toplevel>/.mkit/config.toml`, committed —
one mkit directory in a working tree, not two. Recorded as
[ADR 0001's config-path amendment](adr/0001-per-repo-config-and-init.md#amendment-the-config-path),
which also records why a root-level `mkit.toml`, a path under `.claude/`, and `.agents/mkit.toml`
were each rejected. The ignore rule becomes a **pair** — `.mkit/*` plus `!.mkit/config.toml` —
because git cannot re-include a file whose parent directory is excluded.
- `mkit repo profile --json` — gate commands, spec store, scopes, reviewers, merge style, each
  tagged `discovered` or `pinned` (or `unavailable`, with a cause — an empty value is never
  presented as an answer). Discovery reads `docs/agents/issue-tracker.md` where present; scopes
  come from history, reviewers from CODEOWNERS, the spec ref from the remote.
  **Gate discovery is delegated to `gate-detect.sh`, not reimplemented** — that script is the
  single implementation of the invariant until M5 ports it, and a second one in Go is the failure
  the porting rules name. `internal/core/pluginroot` is what locates it, and the same mechanism
  fetches the remedy sentences below.
- `mkit init` — writes the pinned remainder, committed. Interactive TUI on a TTY, flags otherwise
  (invariants 2 and 3). Writes nothing outside the repo. Refuses on a shadowed path rather than
  writing a config that never travels.
- `mkit doctor` — prerequisites, permission-allowlist gaps, hook registration, plugin enablement,
  and the **sandbox writable set**. Reports; fixes nothing. Exit status stays 0 with findings: a
  report that answered is a report that succeeded.
  **It is the diagnostic surface, not an addition to one** — `install.sh --status` and the
  `SessionStart` hook were both deleted in 0.15.0, so between then and this the only report of an
  unwritable user directory was `facts.sh`'s `user_dir_writable=` starting fact, and there was no
  report of a missing tool at all. **It does not restore all of it**: `doctor` cannot run
  unprompted at session start, and cannot report that `mkit` itself is absent. Both were the
  hook's job; both remain accepted losses.
- The degradation sentences keep exactly one producer. That is `lib/common.sh` until M5 ports
  `facts.sh`, and `doctor` **calls** it — `mkit_user_dir_writable` for the probe and
  `mkit_user_dir_remedy` for the sentence, through `pluginroot.CommonFunc`. Where the payload
  cannot be found, the affected checks report `unknown` and say why, rather than wording their own.
**Done — all four met:** the committed config path is decided and recorded; `mkit doctor` names an
unwritable `~/.mkit/` without failing, with a remedy naming **both** halves (create, then grant —
the grant alone cannot create it); `mkit repo profile --json` distinguishes discovered from pinned
on this repo; and `mkit init` is a no-op on a repo it has already configured.

Two measured facts the implementation turned up, both now pinned by tests:
- **`git check-ignore -v` exits 0 for a *negated* path too**, printing the `!` pattern that
  re-included it. It answers "which rule decided this", not "is it ignored" — so truth comes from
  `-q` and `-v` runs only afterwards, to name the file a remedy must edit. Reading truth off `-v`
  reports every deliberately re-included file as excluded.
- **`.gitignore` outranks the common dir's `info/exclude`.** A negation written into the exclude
  cannot lift a `.mkit/` rule that lives in a committed `.gitignore`, which is why the remedy names
  whichever file git actually reported.

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
- **Re-specify `install` / `uninstall`, if the binary earns them.** Withdrawn from M3, not
  refuted. The bar is a job manual steps genuinely cannot do: `~/.claude/settings.json` stays
  sandbox-denied, so registration will never be one of them, but detecting a plugin/binary skew
  (M4's guard), reporting an unregistered marketplace, or creating `~/.mkit` outside a sandboxed
  session all plausibly are. Specify from what the binary needs then, not from what `install.sh`
  did.
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
- Any hook that must run before setup completes — it cannot depend on a binary whose presence it
  may have to report as missing.

  This entry used to name `scripts/hooks/session-bootstrap.sh`, which was exactly that case. It
  was **removed rather than kept** in 0.15.0: the reasoning still holds — `mkit doctor` genuinely
  cannot report a missing `mkit` — but one implementation with a known gap was preferred over two
  implementations of one invariant. Reintroducing a bash hook here is a real option if the gap
  proves expensive; do it deliberately, not by reflex.

## Porting rules
- Each shell script's `.bats` file is a ready-made spec — port it to `go test` alongside the
  code, do not re-derive the behavior. (M4's spec was `tests/findings.test.mjs`, same rule.)
- One script per milestone, merged green. No big-bang rewrite: the bash is commented,
  tested and load-bearing, and a mass rewrite is pure regression risk.
- Delete the shell script in the same commit that lands its replacement. Two implementations
  of one invariant is the failure mode the script layer exists to prevent.

## Open questions

None open.

Resolved:
- **Which way the version-skew guard points** — **dissolved** at M4, not decided. The question
  assumed one side must declare a machine-comparable range; no comparable tool does (worktrunk,
  coderabbit and codegraph all checked), and neither does mkit. `facts.sh` reports `mkit=` and
  `mkit_bin=` without comparing them, the payload declares no minimum and the binary declares no
  payload range, and the check is **presence only**: the skill asks for the subcommand it needs
  (`mkit findings schema --json`) and a subcommand that does not exist is what "too old" looks
  like. The probe lives in the skill's Markdown, at step 0 — the only place that survives M5,
  since after it `facts.sh` *is* the binary and a binary cannot report its own absence. Skew
  stays a standing condition to report ([ADR 0003](adr/0003-two-distribution-channels.md)),
  never a state to eliminate.
- **Whether repo config belongs to mkit or to the harness** — neither, as posed. It is the
  *repo's* agent configuration and mkit is one consumer, and it lives at
  `<toplevel>/.mkit/config.toml`, committed, with `.mkit/*` + `!.mkit/config.toml` as the ignore
  rule ([ADR 0001's config-path amendment](adr/0001-per-repo-config-and-init.md#amendment-the-config-path)).
  `.agents/mkit.toml` was the closest rejected alternative and the one to revisit if `.agents/`
  ever specifies a config slot: it is already generated and reconciled by a skills installer
  against `skills-lock.json`, so writing there means writing into a tree another tool owns.
- **Two distribution channels, deliberately independent** — the binary via Homebrew, the payload
  via the GitHub marketplace ([ADR 0003](adr/0003-two-distribution-channels.md)). Neither
  `curl | sh` nor `go install` for the binary; no clone for the payload. This also settles what
  the old skew question was really asking: the two versions never "begin moving together",
  because nothing brings them together.
- **The `SessionStart` hook still self-heals** and still runs before any binary is on PATH,
  which is exactly why it stays in bash permanently.
