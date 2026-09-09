# ADR 0001 — Per-repo configuration, an `init` command, and repo-scoped state

**Status:** accepted · **Date:** 2026-09-09 ·
**Partly superseded by** [ADR 0002](0002-state-locations-under-a-sandbox.md): the state-location
table below, and decision 5's premise. Decisions 1–4 stand.

## Context

`plugin/install.sh` is documented as "**installs nothing; there is no setup step**". That held while
mkit was five skills that discovered everything they needed on every run: quality-gate commands,
commit scopes, reviewers, worktree origin. Discovery keeps the plugin portable across repos and
keeps config from going stale, and it stays the default.

Two things change the picture.

**The workflow grew a front half.** `brainstorm` → `spec` → `implement` produce and consume
artifacts (a spec, a task graph) whose *home* is genuinely project-specific: GitHub Issues here, a
GitLab project elsewhere, plain repo files in a repo with no remote. That is not discoverable with
confidence — a `gh` binary and a GitHub remote do not tell you the team tracks work there — and it
is not a per-run decision either, because every step of the workflow must agree on the same answer.
The same is true of the parts of the gate that discovery can only guess at: which of three test
commands is the cheap one, whether this repo squash-merges, who reviews what.

**The sandbox forbids the state model M3 was written against.** Claude Code runs Bash tool calls in
a sandbox whose write allowlist covers the project directory and `$TMPDIR`. Measured on
2026-09-09:

| path | agent-invoked write |
| --- | --- |
| `<git-dir>/mkit/` (run dirs, `gate.jsonl`) | permitted |
| `$TMPDIR` | permitted |
| `~/.claude/mkit/` (`MKIT_HOME`) | **`Operation not permitted`** |

> **Superseded by [ADR 0002](0002-state-locations-under-a-sandbox.md).** The symptoms are right and
> the conclusion is not: `~/.claude/mkit` cannot be allowlisted at all (it is a *protected* path, so
> an `allowWrite` entry covering it is inert), and the run-directory row was measured in a main
> checkout — from a linked worktree that path resolves into the main checkout, where the
> worktree-isolation guard refuses every write. State has since moved to `<toplevel>/.mkit/` and
> `~/.mkit/`.

`bootstrap.state` exists in `~/.claude/mkit/` only because the harness executes `SessionStart`
hooks outside the sandbox. Anything a *skill* invokes that writes user scope fails: `install.sh
--uninstall` writing the tombstone today, and `mkit install` / `mkit uninstall` as M3 specifies
them. Composed tools are exposed too — `wt` failed under the sandbox with `mktemp: mkstemp failed
on /var/folders/…/T/…: Operation not permitted`.

## Decision

**1. Configuration is per-repo by default; user scope holds only what must outlive every repo.**
Repo config lives in the working tree, committed, so a colleague and a fresh clone inherit it. User
scope keeps exactly two things, both of which are about silencing rather than behaviour: the hook
tombstone and the record of which one-time prerequisite messages have been said.

**2. `mkit init` exists, and it is the only command that writes repo config.** This reverses
"there is no setup step" for the repo, and only for the repo. It stays true of the *machine*:
nothing is installed, no shell rc is edited, no global state is seeded.

**3. Config is an input, never a permission.** Every step still runs with no config present,
discovering what it can and recording what it assumed. `mkit init` removes repeated discovery and
captures the undiscoverable; it never becomes a precondition. A skill that refuses to run without
config has broken this, and so has one that trusts a pinned value it can cheaply verify is stale.

**4. Discovery recognises formats it did not write.** `docs/agents/issue-tracker.md` is read where
it exists. mkit ships none of the skills that produce that file and depends on none of them; it
reads the file because it is already in repos on this machine, and a store the user has already
declared is better evidence than a heuristic over `git remote`.

**5. Sandbox degradation is named, never hit.** A command that would write a path the sandbox
denies says so and names the path, rather than failing with `Operation not permitted`. `mkit
doctor` reports the writable set as a first-class fact, beside the prerequisite table.

## Consequences

- **M3 is partly invalidated and must be re-specified before it is written.** `mkit install` /
  `uninstall` cannot write user scope from a skill-invoked run. Either the tombstone moves to repo
  scope, or those two commands are documented as human-run (`! mkit uninstall`) and `mkit status`
  reports that it could not read what it needed. Settle it in M3, in the open, next to the two
  packaging gaps already blocking it.
- **`install.sh --status` keeps its job and loses its slogan.** The diagnostic surface survives as
  `mkit status` / `mkit doctor`; only the "no setup step" claim is withdrawn, and only for repo
  scope.
- **Repo config is a new staleness surface** — the cost of the decision. It is bounded by point 3:
  a pinned value cheap to verify is verified, and the gate ledger's `drifted` / `stale` vocabulary
  already exists to describe the answer.
- **Sandbox-awareness becomes an invariant** rather than a per-command concern, which means the
  worktree-per-slice execution `implement` would otherwise default to cannot be the default: the
  tool it would drive is itself sandbox-fragile.

## Alternatives rejected

- **Keep discovery only, no config.** Portable and stale-proof, but the spec store cannot be
  discovered reliably and every step would guess independently — the one thing they must agree on.
- **User-profile config (`~/.claude/mkit/config`).** Matched where mkit's state lived when this was
  written, and is unwritable from a skill under the sandbox — that path is in the protected region
  and mkit's user scope has since moved to `~/.mkit/` ([ADR 0002](0002-state-locations-under-a-sandbox.md)).
  It also makes a per-project answer global, which is wrong for the spec store regardless of the
  sandbox, and that is the reason this stays rejected.
- **Config in `.claude/settings.json`.** Already sandbox-denied (`denyWithinAllow`), and it is the
  harness's file, not mkit's.
