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

One script is not called by a skill at all — the `SessionStart` hook
(`scripts/hooks/session-bootstrap.sh`), registered by `hooks/hooks.json` at the plugin root. It
names, once per tool, any prerequisite the scripts need and this machine lacks: a gap that
otherwise surfaces as a thinner fact block or a `gate_cache=no-jq` annotation, far from its
cause. It installs nothing, and produces zero bytes on every session after it has said its
piece. `install.sh --uninstall` silences it for good, leaving a tombstone — absent files carry
no provenance, so a dismissal that is only an absence gets re-asserted next session.

**Claude-only for now.** Other agents (Codex, opencode, …) are a later concern — the skills
are plain Markdown, so support for another agent is a thin packaging step, not a rewrite.

## Design Principles
- **Judgement in Markdown, mechanics in the binary:** the workflow lives in Markdown the agent
  reads, not in code it executes. That has not changed and is not going to — the skills are the
  product. What changed is the layer beneath them. This document previously argued against a
  compiled binary on the grounds that the glue is ~4 ms of a ~10 s agent turn, so a faster
  language would optimize nothing and cost a release pipeline. **That reasoning still holds, and
  the port is not about speed.** It buys three things shell cannot: prerequisites disappear
  (`node`, `jq` and `shasum` leave [`prerequisites.md`](prerequisites.md) as their consumers
  land), whole families of degradation branch go with them (`jq-missing`, `no-hash`,
  `gate_cache=no-jq` — a binary is never half-capable), and a real TUI becomes possible for the
  steps where a human wants to tick a list before anything runs. Homebrew then ships binary and
  plugin together, so one `brew upgrade` updates both. Ordered milestones and the invariants the
  port must hold: [`backlog.md`](backlog.md).
- **A script for a mechanical invariant, never for a decision:** `scripts/` may open a
  directory, run a logged command, classify a worktree or do confidence arithmetic. It may not
  choose commit boundaries, assign severity, judge materiality, or decide that a fix is safe.
  Where the line is genuinely unclear the script reports candidates and the skill picks —
  `gate-detect.sh` proposing `fast=` beside `docs_candidates:` is the shape to copy.
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
Five skills. Four move work through its lifecycle: `commit` is the shared front-end;
`finish` and `pr` both begin by committing, and you pick the finisher by
**destination** — merge it yourself locally, or push it for review. `cleanup` sits outside
that line: repo-wide gardening — it doesn't touch code, it sweeps every local branch and
worktree the other four leave behind.

| Skill | Does | Trigger examples |
|-------|------|------------------|
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
- `agent-delegation.md` — context discipline: the per-run directory inside the git dir as the
  transport between stages, subagent return budgets, resolved reference paths, parallel-vs-
  sequential rules, and which model each stage shape wants.
- `output-discipline.md` — bounding command output: gate logs written to a file and read by
  their tail, `--stat` before any diff, never a full branch diff to write prose — plus what
  must never be capped.

## Architecture
```
 Claude Code
   │   loads plugin skills (via .claude-plugin/plugin.json)
   │   loads plugin hooks (via hooks/hooks.json — auto-discovered)
   ▼
 commit · review · finish · pr · cleanup   ← SKILL.md (when & how)
   │   all link into
   ▼
 _shared/references/*.md   (safety · conventions · quality gate · worktree · branching
                            severity bar · lenses · finding triage
                            agent delegation · output discipline)
   +
 scripts/                  the mechanical steps, one call each
   run-open.sh             open a run directory · --prune old ones
   facts.sh                run dir + refs path + branch/status/worktree/stats, in one call
   gate-detect.sh          what this repo's fast + full checks are · what the ledger proved
   gate-run.sh             run a gate step: log it, bound it, stop at the first failure
   findings.mjs            reconcile · group · report over a review's findings (JSONL)
   branch-scan.sh          classify every local branch/worktree for `cleanup` · one gh call
   +
 scripts/hooks/            the one thing no skill calls
   session-bootstrap.sh    SessionStart: name a missing prerequisite once, then stay silent
   +
 mkit (Go)                 the same mechanical steps, being ported off shell one at a time
   --json everywhere       the skill-facing contract · no TUI off a TTY · flags reach everything
   M2 storage prune (done) · M3 install/status/uninstall · M4 findings · M5 the jq consumers
   │   drive
   ▼
 git   +   gh (GitHub CLI)   +   wt (Worktrunk)   +   rg   +   jq
```

The scripts never act: no staging, no merging, no `wt merge`, no edits. They report facts and
run commands the skill named. One of them also *remembers*: `gate-run.sh` records that a command
exited 0 over a fingerprint of the content it read — `<git-dir>/mkit/gate.jsonl`, beside the run
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
Work is modeled as a **feature** — edits that become commits and then get integrated:

```
edit → commit → review → finish
                          ├── finish  (local merge, delete branch / worktree)
                          └── pr      (push, open PR, review remotely)

cleanup  (repo-wide, not per-feature: sweep every local branch/worktree finish and pr left behind)
```

Worktree awareness is built in: the finishing skills detect whether they're in a Worktrunk
worktree, a Claude Code agent worktree, or a plain checkout, and use the matching cleanup
path for the *one* branch they just merged. `cleanup` uses the same lookup table, applied to
every worktree in the repo rather than just the current one.

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
plugin/                 # the payload M3 will register; not in the cask yet (backlog.md)
  .claude-plugin/
    plugin.json          # plugin manifest (name, skills discovered from skills/)
  hooks/
    hooks.json           # SessionStart registration, and nothing else — plugin root, not
                         #   .claude-plugin/; auto-discovered, so the manifest carries no
                         #   `hooks` key
  skills/
    commit/SKILL.md
    pr/SKILL.md
    review/SKILL.md
    finish/SKILL.md
    cleanup/SKILL.md
    _shared/             # shared references (README + references/*.md) — no SKILL.md
  scripts/
    lib/common.sh        # sourced helpers: plugin root, refs path, mkit dir, rg-or-grep, wt
                         #   binary, the prereq table, one-time state, jq-free JSON escape
    hooks/session-bootstrap.sh  # the SessionStart hook — names a missing prerequisite, once
    run-open.sh  facts.sh  gate-detect.sh  gate-run.sh  findings.mjs  branch-scan.sh
  install.sh             # --status / --uninstall. Installs nothing: there is no setup step.
                         #   --uninstall is the only way to silence the hook for good.
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
`masterik/homebrew-tap` (`homebrew_casks` — `brews` is deprecated in GoReleaser v2). Once M3
lands, `mkit install` registers the Homebrew-installed payload as a `directory` marketplace and
the two steps collapse into one.

Plugin skills are namespaced (`mkit:commit`), which avoids clashing with any repo-local
skills of the same name.

## Roadmap
- **Now — Claude Code plugin.** The five skills + the shared reference bundle, packaged and
  installable. This is the product; everything below serves it.
- **Now — the Go port.** `mkit`, a single binary with a subcommand tree, taking over the
  mechanical layer script by script so that prerequisites and degradation branches go away and a
  TUI becomes possible. M1 (scaffold, release chain, Homebrew cask) shipped in `v0.12.0`; M2
  (`mkit storage prune`, `internal/core/storage/` + `internal/tui/storageprune/`) is done; M3
  (`mkit install`/`status`/`uninstall`) is next. Each script's `.bats` file is the spec for its port, and the
  script is deleted in the same commit that replaces it — two implementations of one invariant is
  the failure the script layer exists to prevent. Ordered list: [`backlog.md`](backlog.md).
- **Considered and dropped — recorded intent.** A commit journal once had a `Stop` /
  `SubagentStop` hook nudge the agent to record *why* each unit of work existed, for `commit` to
  spend instead of re-deriving intent from the diff. It was removed: a `Stop` hook's
  `additionalContext` is rendered verbatim in the transcript on **every turn**, with no way to
  suppress it, so the standing cost was paid by every session in every repo while the benefit
  arrived only at commit time. `commit` re-derives intent from the diff, which it had to be able
  to do anyway — `commit`'s per-file staged read was never skippable, however fresh a record
  looked. Anything replacing it has to record intent without spending transcript on every turn.
- **Later — other agents.** Codex, opencode, and others are plain-Markdown consumers of the
  same skill content; supporting one is a packaging step, added only if needed, with no
  change to the skills themselves.
