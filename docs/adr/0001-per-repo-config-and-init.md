# ADR 0001 — Per-repo configuration, an `init` command, and repo-scoped state

**Status:** accepted · **Date:** 2026-09-09 ·
**Partly superseded by** [ADR 0002](0002-state-locations-under-a-sandbox.md): the state-location
table below, and decision 5's premise. Decisions 1–4 stand.
**Also by** [ADR 0003](0003-two-distribution-channels.md): every reference below to settling
something "in M3" is void — M3 was withdrawn.
**Amended 2026-09-10** — decision 1's config **path** is now settled: `<toplevel>/.mkit/config.toml`.
See [Amendment: the config path](#amendment-the-config-path) at the end.

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

> **Amended 2026-09-10 — user scope is empty, and the principle is unchanged.** Both files
> existed only to silence the `SessionStart` hook, which was removed in 0.15.0 along with
> `install.sh`; `~/.mkit/` holds nothing today. It keeps its definition because it is where the
> binary's user-scoped state will land, and because `facts.sh` still probes it so an unwritable
> one is a starting fact rather than a later surprise. The **path** of the repo config is settled
> separately — see [Amendment: the config path](#amendment-the-config-path).

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

## Amendment: the config path

**Date:** 2026-09-10 · **Amends:** decision 1 ("Repo config lives in the working tree, committed")
· **Closes:** [#3](https://github.com/masterik/mk-toolkit/issues/3)'s blocking question.

### What was open

Decision 1 said repo config lives in the working tree, committed, so a colleague and a fresh clone
inherit it — and never named a file. [ADR 0002](0002-state-locations-under-a-sandbox.md) then made
`<toplevel>/.mkit/` mkit's ignored scratch root, and this repo's `.gitignore` carried the rule. So
the only directory this ADR had established was, by construction, the one place the config could
not live: anything written there is ignored, no collaborator inherits it, and the single property
the config exists for is lost.

The deciding question was framed as *whose config is it* — mkit's, or the harness's.

### Decision

**Repo config is `<toplevel>/.mkit/config.toml`, committed.** One mkit directory in a working
tree, not two. The `.gitignore` and `info/exclude` rule becomes a pair rather than a line:

```gitignore
.mkit/*
!.mkit/config.toml
```

`.mkit/*` and not `.mkit/`, because **git cannot re-include a file whose parent directory is
excluded** and a directory-only rule excludes the parent. The pair is the unit; writing only the
first hides repo config from `git add`.

TOML rather than JSON because this file is committed and read by a human in a diff — it takes
comments, and `mkit init` writes them. It is parsed, never round-tripped: `init` renders a
commented template, so nothing depends on a marshaller preserving them.

### Why not the alternatives

- **A root-level `mkit.toml`.** The most discoverable option, and self-contained: it plants no
  directory in a repo that has not opted into one. Rejected for adding a second mkit location to
  every working tree when one already exists, and for saying the config is mkit's own when the
  thing it configures — gate commands, spec store, scopes, reviewers, merge style — is a property
  of the repo.
- **A path under `.claude/`.** Says the config is harness configuration mkit happens to consume,
  which ties a repo-wide answer to one agent. It also sits next to a region the sandbox denies
  (`.claude/settings.json`, `skills/`, `hooks/`), and pointing a remedy at a partly-denied
  neighbourhood is the failure ADR 0002 exists to prevent.
- **`.agents/mkit.toml`.** Considered seriously and the closest call. `.agents/` is an emerging
  vendor-neutral convention, it already exists in this repo, and it answers "whose config" better
  than either horn above — the config is the *repo's*, and mkit is one consumer. Rejected on
  ownership in practice: `.agents/` here is generated and reconciled by a skills installer against
  `skills-lock.json`, and in another repo it may be managed by a skills manager or a plugin
  packager. Writing mkit's config into a tree another tool believes it owns invites a conflict
  mkit does not control. Revisit if `.agents/` acquires a specified config slot.
- **Un-ignoring nothing and force-adding the file.** `git add -f` once makes the file tracked, and
  tracked files ignore ignore-rules forever. Rejected as a trap: the file is invisible to
  `git status` until someone remembers the `-f`, and a colleague who deletes and recreates it
  cannot re-add it without knowing why. The negation states the intent where git can read it.

### Consequences

- **`mkit_run_ignored` changes its subject, not its meaning.** It probes `.mkit/gate.jsonl` — a
  real scratch path — rather than `.mkit/`. The old subject still works, for a reason obscure
  enough to be worth not relying on: `check-ignore .mkit/` *with the trailing slash* is matched by
  `.mkit/*`, because git reads the slash as naming something inside the directory; the bare
  `.mkit` is not matched. That subtlety was already load-bearing here once — a directory-only
  pattern needs the slash to match before the directory exists — and turning on it twice, for two
  unrelated reasons, is a trap. A concrete scratch path depends on none of it and every rule shape
  matches it.
- **A repo configured before this amendment is a named state, not a silent failure.** Under a
  legacy directory-only `.mkit/` rule, `init` would write a config that never travels.
  `mkit_config_committable` detects it and `mkit_config_ignored_remedy` names the file to edit —
  which matters, because **`.gitignore` outranks the common dir's `info/exclude`** (measured), so
  a negation written into the exclude cannot lift a rule that lives in a committed `.gitignore`.
  `facts.sh` reports it as `config_state=shadowed` and `mkit doctor` reports it as a check.
- **The config stays out of the gate fingerprint.** `mkit_tree_fingerprint` excludes `.mkit`
  wholesale, so a tracked `config.toml` does not enter it. That is correct rather than an
  oversight: the fingerprint answers "is the content a gate command reads unchanged", and the
  gate ledger already keys each record on the normalized command string — so a config edit that
  changes a gate command yields a different key and cannot produce a false hit.
- **Decision 3 is unaffected and still governs.** Config is an input, never a permission.
  `config_state=absent` is a normal state, never a note and never a blocker.
