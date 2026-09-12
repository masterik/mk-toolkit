# mkit — Concept

## Summary
mkit is a **Go binary (`mkit`) plus a Claude Code plugin** — a cohesive **kit of agent
coding-workflow skills**. It
gives Claude a safe, repeatable way to take work from **edits → committed → reviewed →
integrated**: stage and commit cleanly, review the diff, then either merge back locally or
open a PR — without re-deriving fragile `git` + `gh` + `wt` command sequences on every task.

It's a *workflow* toolkit, not just a git one: `review` drives CodeRabbit/Codex/Claude,
`pr` drives GitHub, and `finish` handles worktree cleanup — the parts of the
dev loop the agent runs, git-centric but not git-limited.

Nothing installs into the repo's own toolchain. The plugin is essentially
**knowledge + procedure**: each skill tells Claude *when* it applies and *how* to drive the
underlying tools, with the safety rules that keep destructive steps from firing by accident.
The agent is the interface; the skills are the muscle memory. The `mkit` binary is not a second
interface onto that — it is the mechanical layer the skills call, and it is progressively
replacing the shell scripts below. Milestones: [`backlog.md`](backlog.md).

Alongside the Markdown sits a thin layer of **helper scripts** (`scripts/`, six of them) for
the steps that are identical every run and fail silently when hand-rolled: opening the run
directory, gathering the starting facts, detecting and running the quality gate, the
arithmetic over a review's findings, and classifying every local branch/worktree a `cleanup`
run has to decide about.
They ship with the plugin — no `PATH`, no build, no install — and they exist for reliability
more than for tokens: prose re-executed every run kept getting one invariant of three wrong. **Judgement stays in Markdown; a mechanical invariant
belongs in a script.** Prerequisites: [`prerequisites.md`](prerequisites.md).

**Every script in the payload is now called by a skill.** A `SessionStart` hook once named a
missing prerequisite before any skill ran; it and `install.sh` were removed in 0.15.0, because
that report belongs to the binary (`mkit doctor`, M7) rather than to a second implementation in
shell. The cost is real and accepted: a missing tool now surfaces as a thinner fact block or a
`pr=gh-missing` annotation, far from its cause, until a human runs `doctor`. A binary
cannot report its own absence and cannot speak at session start, so this is not a like-for-like
replacement — it is a deliberate trade of unprompted coverage for one implementation.

**Claude-only for now.** Other agents (Codex, opencode, …) are a later concern — the skills
are plain Markdown, so support for another agent is a thin packaging step, not a rewrite.

## Design Principles
- **Judgement in Markdown, mechanics in the binary:** the workflow lives in Markdown the agent
  reads, not in code it executes. That has not changed and is not going to — the skills are the
  product. What changed is the layer beneath them. This document previously argued against a
  compiled binary on the grounds that the glue is ~4 ms of a ~10 s agent turn, so a faster
  language would optimize nothing and cost a release pipeline. **That reasoning still holds, and
  the port is not about speed.** It buys three things shell cannot: prerequisites disappear
  (`node` left with M4, `shasum` with M5's gate commands; `jq` leaves
  [`prerequisites.md`](prerequisites.md) as its last consumers land), whole families of
  degradation branch go with them (`jq-missing`, `no-hash`,
  `gate_cache=no-jq` — a binary is never half-capable), and a real TUI becomes possible for the
  steps where a human wants to tick a list before anything runs. Homebrew ships the **binary
  only**; the plugin payload ships from the GitHub marketplace, and the two version independently
  ([ADR 0003](adr/0003-two-distribution-channels.md)). Ordered milestones and the invariants the
  port must hold: [`backlog.md`](backlog.md).
- **A script for a mechanical invariant, never for a decision:** `scripts/` may open a
  directory, run a logged command, classify a worktree or do confidence arithmetic. It may not
  choose commit boundaries, assign severity, judge materiality, or decide that a fix is safe.
  Where the line is genuinely unclear the script reports candidates and the skill picks —
  `mkit gate detect` proposing `full=` beside `docs_candidates:` is the shape to copy.
- **A recorded fact is an input, never a permission:** mkit accumulates state between runs —
  gate results, hook arithmetic — and every one of them is evidence handed to the agent, never a
  decision taken on its behalf. This *extends* the rule above rather than restating it: a script
  only ever ran because a skill called it, so "report candidates, the skill picks" was enough.
  State outlives the skill that wrote it, and a hook fires with no skill in the loop at all, so
  the line has to be drawn again. Two instances of the one rule:
  - *the hook names the gap; the agent supplies the judgement* — a lifecycle hook may compute
    that a prerequisite is missing and hand the answer back to the model. It may not act on it.
  - *a past run's proof is an input, never a permission* — the gate ledger records that a
    command exited 0 over exactly this content. Whether that is still good enough to skip on is
    a safety-against-latency trade-off, so the skill decides it and must report the step as
    `cached`. A run printing `gate=ok` having executed nothing is the failure this guards.
- **Composition over replacement:** orchestrate `git`, GitHub CLI (`gh`), and Worktrunk
  (`wt`); never reimplement what they already do well.
- **Boundary-aware from day one:** three independent layers can refuse a write — the OS sandbox
  (the kernel, over the whole process tree), the permission gate's auto-mode classifier, and the
  worktree-isolation guard — and telling them apart is what makes a fix reviewable, since they are
  configured in different places and one of them cannot be configured at all. Anything under
  `~/.claude` is a *protected* path where an allowlist entry is inert, so mkit's user-scoped state
  is `~/.mkit/` instead, where one `permissions.additionalDirectories` entry actually opens it
  ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md), measured 2026-09-09). Composed tools are
  exposed too: a `mktemp` with no template ignores `$TMPDIR` on macOS and is denied outright. This
  is the same shape as a missing prerequisite and gets the same treatment — **named, never hit**,
  and named with a remedy that works. A command that would write a denied path says which path and
  what to change; `mkit doctor` reports the writable set beside the prerequisite table. It also sets
  the default for where state goes: `<toplevel>/.mkit/`, inside the working directory, because that
  is the one place all three layers permit with no configuration at all.
- **Configuration is per-repo, and is an input:** what a repo can't tell you by inspection — which
  store holds its specs, which of three test commands is the cheap one, who reviews what — is
  pinned once by `mkit init` and committed to `.mkit/config.toml`, so a colleague and a fresh clone
  inherit it. User scope keeps only what must outlive every repo — which is nothing today: the
  hook's tombstone and its once-per-tool messages went with the hook in 0.15.0. Every step still
  runs with no config at all, discovering what it can and reporting what
  it assumed: config removes repeated discovery, and never becomes a precondition
  ([ADR 0001](adr/0001-per-repo-config-and-init.md)).
- **Safe by default:** irreversible actions (force-push, branch delete, history rewrite,
  hook-skipping) are gated by an explicit safety protocol the skills share.
- **DRY via shared references:** the skills link into one `_shared/references/` bundle
  instead of each restating the same safety and convention rules.
- **Portable across repos, deliberately not across platforms:** nothing project-specific
  is hardcoded — quality-gate commands, commit scopes, and reviewers are all *discovered* from
  the target repo. Platform portability is the opposite call for the shell layer: **macOS is the
  supported OS**, and no script detects or branches on one. What that buys is a single narrow
  target rather than a matrix — bash 3.2, BSD userland, no `flock` — so the discipline shows up
  as constructs avoided (`mktemp`+`mv` instead of `sed -i`, a stored `epoch` instead of parsing
  dates, `mkdir` as the lock primitive) rather than as conditionals to keep in sync. The binary
  does **not** widen that: `.goreleaser.yaml` builds `darwin` only. Go would cross-compile for
  free, and the temptation is to take it — but an untested OS in the release matrix is a support
  claim nobody verifies, and the Homebrew **cask** the tap publishes cannot install on Linux
  anyway. amd64 + arm64 is the whole matrix. Other platforms stay out until someone needs one.

## The Skills
Eight skills. Seven are **steps** in one workflow — think it through, write it down, build it,
record it, check it, ship it — and `cleanup` sits outside the line: repo-wide gardening that
doesn't touch code, sweeping every branch and worktree the steps leave behind.

The steps are **composable, not sequential**. Any one of them runs as the only thing in a session,
in any order, with any subset of the others skipped; a step that can't find what an earlier step
would have produced derives the thin version itself and says it did. That property is a contract,
not an emergent convenience, and it lives in one place:
[`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md). Three of the
seven — `brainstorm`, `spec`, `implement` — are the front half, and are not built yet
([`backlog.md`](backlog.md), M6–M8).

Within the back half, `commit` is the shared front-end; `finish` and `pr` both begin by committing,
and you pick the finisher by **destination** — merge it yourself locally, or push it for review.

| Skill | Does | Trigger examples |
|-------|------|------------------|
| **`brainstorm`** | Interview the idea until the open questions are answered or deferred on purpose. Facts are the agent's job, decisions are yours. Produces decisions, not artifacts. | "help me think through", "let's brainstorm", "poke holes in this" |
| **`spec`** | Synthesise what's already been discussed into a spec plus a task graph of vertical slices with blocking edges. Never re-interviews. | "write it up", "turn this into a spec", "break this down" |
| **`implement`** | Work the task graph's frontier: build a slice, gate it, take the next. Gate-cached between slices. | "implement this", "build the spec", "work the tickets" |
| **`commit`** | Inspect the tree, stage intentionally, split into logical Conventional Commits. | "commit", "split into commits" |
| **`review`** | Review the local diff/commits — full (CodeRabbit + Codex + Claude, all lenses) or quick (CodeRabbit + Codex, bugs/impl only) — verify the findings, fix what's worth fixing, summarize. | "review my changes", "quick review", "run codex and coderabbit" |
| **`finish`** | Commit → merge the branch back into its base → delete branch / remove worktree. **Local**, no PR. | "finish this feature", "merge back and clean up" |
| **`pr`** | Commit → push → open a GitHub PR → assign reviewers. **Remote review** path. | "create a PR", "open a pull request", "submit for review" |
| **`cleanup`** | Classify every local branch (merged, PR'd, unpushed, gone), delete/keep by that classification, remove the worktrees that go with them, keep only the default branch and a local `develop`-like one, then switch and pull. **Local only** — never touches a remote branch. | "clean up branches", "prune stale branches", "tidy up worktrees" |

### Shared references — `skills/_shared/`
`_shared/` is **not** a triggerable skill (it has no `SKILL.md`); it is the shared library
the five skills link into via `../_shared/references/…`:

- `git-safety.md` — the non-negotiable git safety protocol (no force-push, no config edits,
  no AI attribution, don't skip hooks, …).
- `conventional-commits.md` — commit message format, type table, scope detection.
- `quality-gate.md` — how to **detect** (not hardcode) the repo's fast check + full
  lint/test/build gate, how to triage a failing step, and the gate ledger: what a past run
  proved over which content (`fresh`/`failed`/`drifted`/`stale`/`unknown-head`/`none`), the
  per-skill posture, and the rule that a cached step is always labelled `cached`.
- `worktree.md` — detect the worktree origin (Worktrunk `wt` / Claude Code
  `.claude/worktrees/` / plain `git worktree`) and clean up correctly.
- `branching.md` — the default branch model to assume when a repo doesn't document its own.
- `review-severity.md` — the severity bar (`critical`/`major`/`minor`), the `[surface, severity]`
  tag, the read-only reviewer contract, what not to report, and the partial-review rule.
- `lenses-correctness.md` / `lenses-craft.md` — the eight review lenses, split along the reviewer
  that carries each set: `bugs`/`impl`/`adversarial` (Codex) and
  `architecture`/`quality`/`tests`/`docs`/`comments` (the Claude subagent).
- `triage-reconcile.md` / `triage-verify.md` / `fix-checks.md` — reconcile (dedupe +
  corroboration), verify (five verdicts + the materiality test), and the three checks on every
  fix — one file per stage, so each stage loads only its own.
- `agent-delegation.md` — context discipline: the per-run directory under `<toplevel>/.mkit/` as
  the transport between stages, subagent return budgets, resolved reference paths, parallel-vs-
  sequential rules, and which model each stage shape wants.
- `output-discipline.md` — bounding command output: gate logs written to a file and read by
  their tail, `--stat` before any diff, never a full branch diff to write prose — plus what
  must never be capped.

## Architecture
```
 Claude Code
   │   loads plugin skills (via .claude-plugin/plugin.json) — no hooks, deliberately
   ▼
 brainstorm · spec · implement · commit · review · pr · finish   ← SKILL.md (when & how)
 cleanup                                    (the seven steps, plus repo-wide gardening)
   │   all link into
   ▼
 _shared/references/*.md   (safety · conventions · quality gate · worktree · branching
                            severity bar · lenses · finding triage
                            agent delegation · output discipline)
   +
 scripts/                  the mechanical steps, one call each
   run-open.sh             open a run directory · --prune old ones
   facts.sh                run dir + refs path + branch/status/worktree/stats, in one call
   +
 mkit (Go)                 the same mechanical steps, being ported off shell one at a time
   --json everywhere       the skill-facing contract · no TUI off a TTY · flags reach everything
   M2 storage prune · M7 profile/init/doctor · M4 findings · M5 the jq consumers (in progress)
   gate detect             what this repo's checks are · what the ledger already proved
   gate run                run a gate step: log it, bound it, stop at the first failure
   branch scan             classify every local branch/worktree for `cleanup` · one gh call
   work                    the per-branch worklog: what ran, over what content, concluding what
   plan                    task-graph arithmetic: frontier · blocked · cycles · edge validation
   repo profile            what this repo told us, and what a human pinned — reported apart
   init                    write the repo config (the one command that writes it)
   doctor                  the agent's environment: prereqs · permissions · hooks · sandbox
   │   drive
   ▼
 git   +   gh (GitHub CLI)   +   wt (Worktrunk)   +   rg   +   jq
```

The scripts never act: no staging, no merging, no `wt merge`, no edits. They report facts and
run commands the skill named. One of them also *remembers*: `mkit gate run` records that a command
exited 0 over a fingerprint of the content it read — `<toplevel>/.mkit/gate.jsonl`, beside the run
directories, never committed, and a linked worktree gets its own. It adds no script: the ledger
is a side effect of a gate that was running anyway, read back by the detector that already prints
the commands. The hook is the only piece that runs without a skill asking, and it is held to the
same line — it names a missing tool and never installs one, always exits 0, and says each thing
at most once. `rtk` is deliberately not among any of
them — it reshapes output for an agent to read, which is exactly what a parser must not
tolerate; it stays on the agent's own direct commands.

The skills are the single source of truth for the *workflow*; the underlying tools remain
the source of truth for the *operations*. The plugin never re-encodes git logic.

## Workflow Model
Work is modeled as a **feature** — a branch that accumulates thinking, then edits, then commits,
then gets integrated:

```
brainstorm → spec → implement → commit → review → pr ──┐
                                                        ├─→ (merged)
                                              finish ──┘

cleanup  (repo-wide, not per-feature: sweep every local branch/worktree finish and pr left behind)
```

**The arrows are the common path, never a required one.** This is the workflow's load-bearing
property, and the reason it is a set of steps rather than a pipeline: real work enters in the
middle. A bug fix starts at `implement` with no spec. A branch someone else pushed starts at
`review`. Half the commits in this repo were `commit` alone. A workflow that only pays off when
walked end to end is a workflow that gets abandoned at the first exception, so each step is
**entry-capable** — it establishes what it needs by discovery, derives the thin version of anything
missing, and names what it assumed.

What makes that cheap rather than merely possible is the **worklog**:
`<toplevel>/.mkit/work/<branch>.jsonl`, one append-only record per finished step, carrying the gist,
the artifact pointer, and the content fingerprint it ran over. A step reads it to skip work the
branch has already done — `review` taking the goal `spec` wrote instead of re-deriving it from
commit messages — and never to decide whether it is allowed to run. It is the gate ledger's rule
one level up: *a recorded fact is an input, never a permission*. The full contract, including what
each step owes the next, is
[`workflow-contract.md`](../plugin/skills/_shared/references/workflow-contract.md).

Two things follow that are easy to get wrong. A step never sends the user to another step, because
"run `spec` first" is a refusal in a suggestion's clothing; and a step never runs anything
**downstream** of itself, because a `review` that commits has taken a decision that was the user's
to make.

Worktree awareness is built in: the finishing skills detect whether they're in a Worktrunk
worktree, a Claude Code agent worktree, or a plain checkout, and use the matching cleanup
path for the *one* branch they just merged. `cleanup` uses the same lookup table, applied to
every worktree in the repo rather than just the current one.

## Configuration
mkit facilitates the workflow at three levels, and they are deliberately different in kind. The
rule across all three: **mkit computes and reports; the human or the agent decides.**

**The machine** — is the toolchain here at all. `facts.sh` reports the prerequisite-adjacent
facts a skill needs at its first call, and that is now the whole of it: the `SessionStart` hook
that named a missing tool once, and `install.sh --status`, are both gone. Neither installed
anything, and neither was a place to *ask* — a human-run diagnostic is a separate surface, and
`mkit doctor` is it (M7). Nothing runs unprompted: there is deliberately no automatic version of
this at all, loud or quiet ([ADR 0003](adr/0003-two-distribution-channels.md) withdrew M3 and made
installation manual).

**The repo** — what this project can't tell you by inspection. `mkit repo profile --json` reports
the gate commands, the spec store, commit scopes, reviewers and merge style, marking each as
**discovered** or **pinned** so a skill can tell a fact from a preference. Discovery runs first and
stays authoritative for anything it can establish; `mkit init` writes only the remainder, committed,
so the answer is the same for every clone and every collaborator. Discovery also reads formats mkit
did not write — `docs/agents/issue-tracker.md` where a repo has one — because a store the user has
already declared beats a heuristic over `git remote`. Pinning is cached with the ledger's existing
vocabulary (`fresh` / `drifted` / `stale`), which is what keeps a pin from outliving its truth.

**The agent** — the level nobody builds, and where a broken run actually comes from. A workflow
step doesn't usually fail because `git` is missing; it fails because a permission prompt blocked a
command mid-run, a hook wasn't registered, an expected plugin was disabled, or the sandbox denied a
write the step assumed. `mkit doctor` reports that surface: the prerequisite gaps, the permission
allowlist against what the skills actually invoke ([`prerequisites.md`](prerequisites.md) already
documents the set), hook registration, and **the writable path set under the current sandbox**. It
fixes none of it — a tool that quietly widens its own permissions is the thing a permission prompt
exists to prevent — and hands the gap back as a named finding.

## Distribution
mkit ships as a Homebrew-installed binary plus a standard Claude Code plugin payload:

```
cmd/mkit/               # entrypoint only — build the root command, exit non-zero on error
internal/
  cli/                  # the cobra tree; root.go owns --json / --no-tui / --yes
  core/                 # data-returning logic — never prints, never assumes a terminal
    storage/            # M2: provider/category table, Scan, guarded Apply, HumanBytes
  tui/                  # Bubble Tea rendering over core, one subpackage per command
    storageprune/       # M2: size-sorted tick-list for `storage prune --apply`
  buildinfo/            # version/commit/date, injected by -X ldflags at release
.goreleaser.yaml        # darwin × amd64/arm64, plus the homebrew_casks tap entry
.claude-plugin/
  marketplace.json      # marketplace entry — source "./plugin". Must sit at the repo root:
                         #   `/plugin marketplace add owner/repo` only ever looks for
                         #   .claude-plugin/marketplace.json there, no subdirectory support
plugin/                 # the payload, shipped from the GitHub marketplace (never the cask)
  .claude-plugin/
    plugin.json          # plugin manifest (name, skills discovered from skills/)
  skills/
    commit/SKILL.md
    pr/SKILL.md
    review/SKILL.md
    finish/SKILL.md
    cleanup/SKILL.md
    _shared/             # shared references (README + references/*.md) — no SKILL.md
  scripts/
    lib/common.sh        # sourced helpers: plugin root, refs path, mkit dir, rg-or-grep, wt
                         #   binary, tree fingerprint, gate ledger path
    run-open.sh  facts.sh
                         # no hooks/ and no install.sh — see "no hook, and no setup step"
docs/
  concept.md             # this file
  backlog.md             # ordered work list
  prerequisites.md       # required + recommended tooling, setup, permission allowlist
tests/                   # dev-only: the script layer's own suite (tests/run.sh); Go tests
                         #   live beside their package
```

Install the plugin with:

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

and the binary with `brew install masterik/tap/mkit`. Releases are tag-driven: pushing `vX.Y.Z`
has GoReleaser build every platform archive and commit the Homebrew **cask** to
`masterik/homebrew-tap` (`homebrew_casks` — `brews` is deprecated in GoReleaser v2). The cask
carries the binary and nothing else, and **the two steps stay two**: the marketplace ships the
skills, Homebrew ships the executable, and neither is on the other's release schedule
([ADR 0003](adr/0003-two-distribution-channels.md)). The payload calls the binary for nothing
yet, so the plugin works with no binary installed at all; from M4 it will say so up front when
the binary is absent or too old, rather than failing partway.

Plugin skills are namespaced (`mkit:commit`), which avoids clashing with any repo-local
skills of the same name.

## Roadmap
- **Now — Claude Code plugin.** The back-half skills + the shared reference bundle, packaged and
  installable. This is the product; everything below serves it.
- **Next — the workflow's front half.** `brainstorm`, `spec` and `implement`, so the seven steps
  exist and the line runs from an idea to a merge. The skills are the judgement; the binary
  contributes the mechanical parts they stand on — the worklog, task-graph frontier arithmetic, and
  the repo profile that stops every step discovering the same facts apart. Ordered as M6–M8 in
  [`backlog.md`](backlog.md), and sequenced behind the configuration surface rather than racing
  it: a workflow spanning seven steps wants one place to read the repo's answers from.
- **Next — configuration as a surface.** `mkit init`, `mkit repo profile` and `mkit doctor`,
  covering the repo and agent levels described above. `doctor` is the one with no predecessor:
  every other command reports something a script already computed, while the agent's own
  environment — permissions, hooks, sandbox — has never been reported at all.
- **Now — the Go port.** `mkit`, a single binary with a subcommand tree, taking over the
  mechanical layer script by script so that prerequisites and degradation branches go away and a
  TUI becomes possible. M1 (scaffold, release chain, Homebrew cask) shipped in `v0.12.0`; M2
  (`mkit storage prune`, `internal/core/storage/` + `internal/tui/storageprune/`) is done; M3 was
  **withdrawn** when the distribution model changed ([ADR 0003](adr/0003-two-distribution-channels.md)),
  so **M7 (`repo profile`/`init`/`doctor`) is next** — `mkit init` per project is the priority and no
  port blocks it. Each script's `.bats` file is the spec for its port, and the
  script is deleted in the same commit that replaces it — two implementations of one invariant is
  the failure the script layer exists to prevent. Ordered list: [`backlog.md`](backlog.md).
- **Considered and dropped — recorded intent.** A commit journal once had a `Stop` /
  `SubagentStop` hook nudge the agent to record *why* each unit of work existed, for `commit` to
  spend instead of re-deriving intent from the diff. It was removed: a `Stop` hook's
  `additionalContext` is rendered verbatim in the transcript on **every turn**, with no way to
  suppress it, so the standing cost was paid by every session in every repo while the benefit
  arrived only at commit time. `commit` re-derives intent from the diff, which it had to be able
  to do anyway — `commit`'s per-file staged read was never skippable, however fresh a record
  looked. Anything replacing it has to record intent without spending transcript on every turn —
  one candidate, researched and unscheduled, is in
  [`ideas/journal-transcript-digest.md`](ideas/journal-transcript-digest.md): mine the transcript
  Claude Code already writes instead of nudging an agent to write a second one.
- **Later — other agents.** Codex, opencode, and others are plain-Markdown consumers of the
  same skill content; supporting one is a packaging step, added only if needed, with no
  change to the skills themselves.
