# ADR 0002 — Where mkit's state lives, under a sandbox

**Status:** accepted · **Date:** 2026-09-09 ·
**Supersedes:** the state-location table in [ADR 0001](0001-per-repo-config-and-init.md) (its
"Context" table and decision 5's premise). ADR 0001's other four decisions stand unchanged.

## Context

ADR 0001 measured three paths and drew a table. The symptoms were right; the conclusion it implies —
that `~/.claude/mkit/` is a path the user can be told to grant — is wrong, and the run-directory row
was measured in a main checkout and does not generalise.

Three independent boundaries can refuse a write on this machine. Keeping them apart is what makes
the decision reviewable, because they are configurable in different places and one of them is not
configurable at all:

| layer | enforced by | message shape | configurable |
| --- | --- | --- | --- |
| **OS sandbox** (Seatbelt) | the kernel, over the command's whole process tree | `Operation not permitted`, `EPERM` | `sandbox.*` — **except protected paths** |
| **Auto-mode classifier** | Claude Code, before the tool runs | `Permission for this action was denied by the Claude Code auto mode classifier` | `permissions.*`, `autoMode.*` |
| **Worktree isolation guard** | Claude Code, before the tool runs | `This session is isolated in the worktree …` | `worktree.bgIsolation` — except the command-shape check |

Measured 2026-09-09 (Claude Code 2.1.266, macOS, sandbox on):

| path | in the user's `allowWrite`? | agent-invoked write |
| --- | --- | --- |
| `~/.claude/mkit/probe` | no | `Operation not permitted` |
| `~/.claude/probe` | no | `Operation not permitted` |
| `~/.claude/plugins/data/<plugin>/probe` | **yes** | `Operation not permitted` |
| `~/.codex/probe` | yes | ok |
| `~/.coderabbit/probe` | yes | ok |

Row three is the finding. An allowlist entry works (rows four and five) and is **inert** inside the
protected region (row three). The documentation is explicit: in `~/.claude`, or whatever
`CLAUDE_CONFIG_DIR` points at, *"there is no way to exempt one of these paths: an `allowWrite` entry
or an `Edit` allow rule that covers the path doesn't lift the protection. The only way to turn the
protection off is `filesystem.disabled`"* — which disables filesystem isolation for every path.

So ADR 0001's remedy shape — name the path, tell the user to allowlist it — produces a configuration
that looks correct and changes nothing.

The second half is the run directory. `<git-dir>/mkit/` resolves, from a linked worktree, into the
**main checkout**. The sandbox deliberately permits that (it allows writes to the shared `.git` when
the cwd is a linked worktree), but the isolation guard's file-edit check does not: 28 writes in the
transcripts were refused there, `Write` and `cat >` alike, with *"This session is isolated in the
worktree …; edit the worktree copy of this file instead of the shared-checkout path"*. Since
`facts.sh` opens the run directory as **every skill's first call**, the toolkit was unusable in
exactly the sessions it is driven from. ADR 0001's "permitted" for that row was measured in a main
checkout.

## Decision

**1. Repo scope becomes `<toplevel>/.mkit/`.** Run directories and `gate.jsonl`. Inside the working
directory, which is what makes all three layers permit it *with no configuration at all*: the
sandbox writes the cwd by default, the guard only blocks the main checkout, and the classifier's
out-of-working-directory rules do not apply. `--show-toplevel`, so a linked worktree still gets its
own.

**2. User scope becomes `~/.mkit/`**, holding the two files ADR 0001 assigned to user scope. Outside
the protected region, so one `permissions.additionalDirectories` entry genuinely opens it — and that
entry grants the sandbox write *and* makes the path a working directory, satisfying the classifier's
rule in the same line. `sandbox.filesystem.allowWrite` is the narrower alternative that leaves that
rule biting. `MKIT_HOME` still overrides, which is how the bats suite stays off a developer's real
state.

**3. Ephemeral files go to `$TMPDIR`, with an explicit `mktemp` template, never to the run
directory.** The division is by *lifetime*, not by caller: a file that dies with the command uses
`$TMPDIR`, a file a later step or a later session reads uses the run directory. This is the seam that
makes this ADR safe as one change — the fingerprint fix does not wait on the relocation, and the
relocation cannot regress it. The bare and `-t` forms of `mktemp` leave the payload entirely: on
macOS they resolve the Darwin per-user temp directory and **ignore `$TMPDIR`**, so they are not
fixable by environment.

**4. `$TMPDIR` is never used for anything the hook and a skill share.** Sandboxed and unsandboxed
commands resolve it to different directories, and the SessionStart hook runs unsandboxed. That is why
user scope is a real directory and not a temp one.

**5. `.mkit/` is ignored before the first write, and that is load-bearing.** Measured in a throwaway
repo with a linked worktree: unignored, `git status --porcelain` reports `?? .mkit/` and
`git worktree remove` refuses with *"contains modified or untracked files, use --force"*; with
`.mkit/` in the common-dir `.git/info/exclude`, the worktree inherits the rule, porcelain is clean,
and removal succeeds. Three things break without it — worktree teardown for `finish`/`cleanup` and
for Claude Code's own sweep, which keeps any worktree holding changed or untracked files; `git add
-A`, which would commit run artefacts; and the gate cache, since the fingerprint enumerates with
`git ls-files --others --exclude-standard` and an unignored scratch directory enters the fingerprint
and then changes while the gate runs. The rule goes in the **common dir's** `info/exclude` —
uncommitted, shared by every worktree, no diff noise. A committed `.gitignore` line is the variant
for repos whose teammates run mkit from fresh clones. Neither can be written from a worktree-isolated
session, so `facts.sh` reports `run_ignored=yes|no` as a starting fact and no staging step runs while
it is `no`.

**6. Every remedy sentence names a remedy that works.** No surface may say `~/.claude/mkit` can be
allowlisted, because it cannot. Where a write is genuinely impossible, the surface says the command
is human-run (`! mkit uninstall`) rather than offering configuration.

**7. Detection happens at the first call, and nothing gains a reduced mode.** `facts.sh` reports
writability, `run_ignored` and the pinned git invocation as starting facts. A skill either has what
it needs or reports cause and remedy and stops. The one exception is the branch classifier's PR
cache, whose absence is an already-specified column state (`gh=no-cache`, beside `gh-missing` and
`gh-unauthenticated`) — the classifier's contract is met without it, and no skill's contract is met
without its run directory.

## Consequences

- **The cost of leaving the git dir is stated, not discovered.** `.git/mkit` was invisible to
  everything that walks a working tree. `<toplevel>/.mkit` is invisible only to tools that honour
  `.gitignore` — `rg`, git itself, `prettier --ignore-path`. Plain `grep -r`, `find`, a `tar` or
  `docker build` context, and any gate command that globs the tree will see it. Go tooling ignores
  dot-directories, so this repo's own gate is unaffected.
- **The `.bats` files move with the paths**, deliberately: they are the spec, and their diff is where
  the relocation is reviewed.
- **A bats test can assert resolution, not permission.** The isolation guard is a permission check on
  tool calls and does not apply to a git repo a test creates. Its behavior is verified by the refusal
  messages in the transcripts; the suite asserts that the run root resolves inside the worktree, that
  porcelain stays clean, and that `git worktree remove` succeeds without `--force`.
- **Two writable-set invariants join the list** (`backlog.md`): no payload script uses a
  template-less `mktemp`, and no payload script writes outside the run directory, `$TMPDIR` and the
  user-scoped root. Both are asserted statically, because neither has a behavioral seam — a bats run
  cannot create an OS sandbox.
- **M3 is unblocked on the question ADR 0001 left open.** `mkit install` / `uninstall` can write user
  scope on a machine with one `additionalDirectories` entry, and can say exactly what to add when
  there is none. The tombstone does not have to move to repo scope.
- **ADR 0001's decision 5 survives with a corrected premise.** "Sandbox degradation is named, never
  hit" still holds; what changes is that naming a path is not enough — the sentence has to name a
  path that can actually be granted.

## Alternatives rejected

- **Keep `~/.claude/mkit` and tell users to allowlist it.** The measurement above is the whole
  argument: the entry is inert. The only lever is `filesystem.disabled`, which is not a remedy, it is
  a decision to run unsandboxed.
- **Move user scope to `$TMPDIR`.** The hook runs unsandboxed and a skill does not, so they would
  resolve it to different directories and the say-it-once ledger would never be read back.
- **Keep the run directory in `<git-dir>` and special-case a linked worktree.** Two code paths for
  one invariant, and the one that fires in an isolated session is the untested one. `--show-toplevel`
  gives the same per-worktree property from the side of the boundary the session is on.
- **Write the ignore rule into a committed `.gitignore` by default.** It is a diff in the user's
  repo that mkit did not ask permission for. The common-dir exclude is uncommitted and invisible;
  `.gitignore` stays the documented variant for teams who want it.
- **Fix the temp files without moving state.** The fingerprint is called from gate detection, so
  rooting its working files in the run directory would recreate the same failure one layer down in
  any session where the run directory is unreachable. Doing the work twice.
