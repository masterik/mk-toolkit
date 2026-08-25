# mkit — Concept

## Summary
mkit is a **Claude Code plugin** — a cohesive **kit of agent coding-workflow skills**. It
gives Claude a safe, repeatable way to take work from **edits → committed → reviewed →
integrated**: stage and commit cleanly, review the diff, then either merge back locally or
open a PR — without re-deriving fragile `git` + `gh` + `wt` command sequences on every task.

It's a *workflow* toolkit, not just a git one: `review` drives CodeRabbit/Codex/Claude,
`pr` drives GitHub, and `finish` handles worktree cleanup — the parts of the
dev loop the agent runs, git-centric but not git-limited.

There is no CLI and nothing to install into the repo's toolchain. The plugin is essentially
**knowledge + procedure**: each skill tells Claude *when* it applies and *how* to drive the
underlying tools, with the safety rules that keep destructive steps from firing by accident.
The agent is the interface; the skills are the muscle memory.

Alongside the Markdown sits a thin layer of **helper scripts** (`scripts/`, seven of them) for
the steps that are identical every run and fail silently when hand-rolled: opening the run
directory, gathering the starting facts, detecting and running the quality gate, the
arithmetic over a review's findings, the commit journal's coverage and freshness arithmetic,
and classifying every local branch/worktree a `cleanup` run has to decide about.
They ship with the plugin — no `PATH`, no build, no install — and they exist for reliability
more than for tokens: prose re-executed every run kept getting one invariant of three wrong. **Judgement stays in Markdown; a mechanical invariant
belongs in a script.** Prerequisites: [`PREREQUISITES.md`](PREREQUISITES.md).

Two scripts are not called by a skill at all — the hooks, registered by `hooks/hooks.json` at
the plugin root. A `Stop` / `SubagentStop` hook (`scripts/hooks/journal-nudge.sh`) tells the
agent which dirty paths no journal entry covers, so *why* a unit of work exists gets recorded
while the session still knows it — instead of being reverse-engineered from the diff at commit
time. A `SessionStart` hook (`scripts/hooks/session-bootstrap.sh`) writes the one user-scoped
marker that turns journaling on everywhere, so the feature needs no install step; it says so
once, and then produces nothing on every later session.

Journaling is therefore **on by default**, and reversible at two scopes: `journal.sh disable`
in a repo, `install.sh --uninstall` for the user. Both leave a tombstone, for the same reason —
absent files carry no provenance, so an opt-out that is only an absence gets re-asserted by the
next thing that re-establishes the default.

**Claude-only for now.** Other agents (Codex, opencode, …) are a later concern — the skills
are plain Markdown, so support for another agent is a thin packaging step, not a rewrite.

## Design Principles
- **Skills, not a binary:** the workflow lives in Markdown the agent reads, not in code it
  executes. Nothing to build, version, or keep on `PATH` — which is also why the scripts are
  shell plus one dependency-free Node file, and never a compiled binary: a plugin install is a
  clone, and the glue is ~4 ms of a ~10 s agent turn, so a faster language would optimize
  nothing and cost a release pipeline.
- **A script for a mechanical invariant, never for a decision:** `scripts/` may open a
  directory, run a logged command, classify a worktree or do confidence arithmetic. It may not
  choose commit boundaries, assign severity, judge materiality, or decide that a fix is safe.
  Where the line is genuinely unclear the script reports candidates and the skill picks —
  `gate-detect.sh` proposing `fast=` beside `docs_candidates:` is the shape to copy.
- **A recorded fact is an input, never a permission:** mkit accumulates state between runs —
  journal entries, gate results, hook arithmetic — and every one of them is evidence handed to
  the agent, never a decision taken on its behalf. This *extends* the rule above rather than
  restating it: a script only ever ran because a skill called it, so "report candidates, the
  skill picks" was enough. State outlives the skill that wrote it, and a hook fires with no
  skill in the loop at all, so the line has to be drawn again. Three instances of the one rule:
  - *the hook names the gap; the agent supplies the judgement* — a lifecycle hook may compute
    which dirty paths have no journal entry and hand the answer back to the model. It may not
    author the record.
  - *a past session's judgement is an input, never a decision* — a journal entry records why a
    unit of work exists; it never says what the commits should be. `commit` still reads the
    staged hunks and authors the message. A stale entry that quietly wrote a plausible-but-wrong
    message into permanent history is a worse failure than the tokens it saved.
  - *a past run's proof is an input, never a permission* — the gate ledger records that a
    command exited 0 over exactly this content. Whether that is still good enough to skip on is
    a safety-against-latency trade-off, so the skill decides it and must report the step as
    `cached`. A run printing `gate=ok` having executed nothing is the failure this guards.
  - *configuration is the one exception, and it is bounded* — the `SessionStart` hook does not
    report a fact; it **writes configuration**, unasked, that changes what mkit does in every
    repo. Nothing else in the codebase does that, so it is named here as a deliberate exception
    rather than left to look like the rule. Three bounds make it one: it writes a single empty
    marker and one wrapper and nothing else, ever; it announces itself once, in the user's own
    view, naming both files and both opt-outs, because a hook cannot ask and after-the-fact
    disclosure is then the whole of consent; and a tombstone at either scope stops it dead.
    The test is whether a user who never reads the docs still ends up informed and in control —
    not whether the default is convenient.
- **Composition over replacement:** orchestrate `git`, GitHub CLI (`gh`), and Worktrunk
  (`wt`); never reimplement what they already do well.
- **Safe by default:** irreversible actions (force-push, branch delete, history rewrite,
  hook-skipping) are gated by an explicit safety protocol the skills share.
- **DRY via shared references:** the skills link into one `_shared/references/` bundle
  instead of each restating the same safety and convention rules.
- **Portable across repos, targeted at one OS:** nothing project-specific is hardcoded —
  quality-gate commands, commit scopes, and reviewers are all *discovered* from the target repo.
  Platform portability is the opposite call: **macOS is the supported OS**, and no script
  detects or branches on one. What that buys is a single narrow target rather than a matrix —
  bash 3.2, BSD userland, no `flock` — so the discipline shows up as constructs avoided
  (`mktemp`+`mv` instead of `sed -i`, a stored `epoch` instead of parsing dates, `mkdir` as the
  lock primitive) rather than as conditionals to keep in sync.

## The Skills
Six skills. Four move work through its lifecycle: `commit` is the shared front-end;
`finish` and `pr` both begin by committing, and you pick the finisher by
**destination** — merge it yourself locally, or push it for review. The other two sit
outside that line: `note` records intent *during* implementation for `commit` to spend
later, and normally the hook fires it, not the user; `cleanup` is repo-wide gardening —
it doesn't touch code, it sweeps every local branch and worktree the other five leave behind.

| Skill | Does | Trigger examples |
|-------|------|------------------|
| **`commit`** | Inspect the tree, stage intentionally, split into logical Conventional Commits. | "commit", "split into commits" |
| **`review`** | Review the local diff/commits — full (CodeRabbit + Codex + Claude, all lenses) or quick (CodeRabbit + Codex, bugs/impl only) — verify the findings, fix what's worth fixing, summarize. | "review my changes", "quick review", "run codex and coderabbit" |
| **`finish`** | Commit → merge the branch back into its base → delete branch / remove worktree. **Local**, no PR. | "finish this feature", "merge back and clean up" |
| **`pr`** | Commit → push → open a GitHub PR → assign reviewers. **Remote review** path. | "create a PR", "open a pull request", "submit for review" |
| **`note`** | Record why the unit of work just finished exists, into the repo's commit journal. Capture only — no diff read, no staging. | "note this", "record what I just did" |
| **`cleanup`** | Classify every local branch (merged, PR'd, unpushed, gone), delete/keep by that classification, remove the worktrees that go with them, keep only the default branch and a local `develop`-like one, then switch and pull. **Local only** — never touches a remote branch. | "clean up branches", "prune stale branches", "tidy up worktrees" |

### Shared references — `skills/_shared/`
`_shared/` is **not** a triggerable skill (it has no `SKILL.md`); it is the shared library
the six skills link into via `../_shared/references/…`:

- `git-safety.md` — the non-negotiable git safety protocol (no force-push, no config edits,
  no AI attribution, don't skip hooks, …).
- `conventional-commits.md` — commit message format, type table, scope detection.
- `journal.md` — the commit journal: its two governing rules, the one record kind, how an entry
  is classified against the current tree (`fresh`/`drifted`/`committed`/`orphaned`/`unknown-head`), and
  the known gaps. `note`, `commit` and the hook's own nudge text all point here.
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
 commit · review · finish · pr · note · cleanup   ← SKILL.md (when & how)
   │   all link into
   ▼
 _shared/references/*.md   (safety · conventions · quality gate · worktree · branching
                            severity bar · lenses · finding triage · commit journal
                            agent delegation · output discipline)
   +
 scripts/                  the mechanical steps, one call each
   run-open.sh             open a run directory · --prune old ones
   facts.sh                run dir + refs path + branch/status/worktree/stats, in one call
   gate-detect.sh          what this repo's fast + full checks are · what the ledger proved
   gate-run.sh             run a gate step: log it, bound it, stop at the first failure
   findings.mjs            reconcile · group · report over a review's findings (JSONL)
   journal.sh              record intent · classify entries against the tree · coverage
   branch-scan.sh          classify every local branch/worktree for `cleanup` · one gh call
   +
 scripts/hooks/            the two things no skill calls
   session-bootstrap.sh    SessionStart: write the user-scoped setup, once, then stay silent
   journal-nudge.sh        Stop/SubagentStop: name the dirty paths no entry covers
   │   drive
   ▼
 git   +   gh (GitHub CLI)   +   wt (Worktrunk)   +   rg   +   jq
```

The scripts never act: no staging, no merging, no `wt merge`, no edits. They report facts and
run commands the skill named. Two of them also *remember*: `journal.sh` records why a unit of
work exists, and `gate-run.sh` records that a command exited 0 over a fingerprint of the content
it read — `<git-dir>/mkit/journal.jsonl` and `gate.jsonl`, beside the run directories, never
committed, and a linked worktree gets its own of each. Neither adds a script: the ledger is a
side effect of a gate that was running anyway, read back by the detector that already prints the
commands. The hook is the only piece that runs without a skill asking, and
it is held to the same line — it names which paths lack an entry and never writes one, always
exits 0, and stays silent unless the repo opted in. `rtk` is deliberately not among any of
them — it reshapes output for an agent to read, which is exactly what a parser must not
tolerate; it stays on the agent's own direct commands.

The skills are the single source of truth for the *workflow*; the underlying tools remain
the source of truth for the *operations*. The plugin never re-encodes git logic.

## Workflow Model
Work is modeled as a **feature** — edits that become commits and then get integrated:

```
edit → commit → review → finish
  │                       ├── finish  (local merge, delete branch / worktree)
  │                       └── pr      (push, open PR, review remotely)
  └── note  (opt-in: record why this unit exists, for commit to spend)

cleanup  (repo-wide, not per-feature: sweep every local branch/worktree finish and pr left behind)
```

Worktree awareness is built in: the finishing skills detect whether they're in a Worktrunk
worktree, a Claude Code agent worktree, or a plain checkout, and use the matching cleanup
path for the *one* branch they just merged. `cleanup` uses the same lookup table, applied to
every worktree in the repo rather than just the current one.

## Distribution
mkit ships as a standard Claude Code plugin:

```
.claude-plugin/
  plugin.json          # plugin manifest (name, skills discovered from skills/)
  marketplace.json     # marketplace entry — source "./"
hooks/
  hooks.json           # SessionStart / Stop / SubagentStop registration — plugin root, not
                       #   .claude-plugin/; auto-discovered, so the manifest carries no
                       #   `hooks` key
skills/
  commit/SKILL.md
  pr/SKILL.md
  review/SKILL.md
  finish/SKILL.md
  note/SKILL.md
  cleanup/SKILL.md
  _shared/             # shared references (README + references/*.md) — no SKILL.md
scripts/
  lib/common.sh        # sourced helpers: plugin root, refs path, mkit dir, rg-or-grep, wt
                       #   binary, the prereq table, the wrapper generator, jq-free JSON escape
  hooks/session-bootstrap.sh  # the SessionStart hook — writes the user-scoped setup itself
  hooks/journal-nudge.sh      # the Stop/SubagentStop hook — silent in a repo that opted out
  run-open.sh  facts.sh  gate-detect.sh  gate-run.sh  findings.mjs  journal.sh  branch-scan.sh
install.sh             # --status / --uninstall / --bin. Not needed for setup: the hook does
                       #   that. --uninstall is the only way to opt out globally.
tests/                  # dev-only: the script layer's own test suite (tests/run.sh)
PREREQUISITES.md       # required + recommended tooling, setup, permission allowlist
```

Install it with:

```
/plugin marketplace add masterik/workflow_tool
/plugin install mkit@masterik
```

Plugin skills are namespaced (`mkit:commit`), which avoids clashing with any repo-local
skills of the same name.

## Roadmap
- **Now — Claude Code plugin.** The six skills + the shared reference bundle, packaged and
  installable. This is the whole product.
- **Now, on by default — intent capture.** The commit journal: the `Stop` hook nudges the agent
  that did the work to record *why* each unit exists, and `commit` spends those records instead
  of re-deriving intent from the diff. The `SessionStart` hook turns it on with no install step;
  `journal.sh disable` (per repo) and `install.sh --uninstall` (global) both stick. `commit` is
  the only consumer so far — `review` could use the `why` lines as reviewer context and `pr`
  could draft a description from them; one consumer first, then decide.
- **Later — a stable opt-out surface.** The bootstrap notice names paths inside the installed
  plugin, which are version-scoped for a marketplace install and go stale on upgrade. A
  `journal.sh default off` subcommand, reachable as `mkit-journal default off` from anywhere,
  would replace both with something that keeps working.
- **Later — other agents.** Codex, opencode, and others are plain-Markdown consumers of the
  same skill content; supporting one is a packaging step, added only if needed, with no
  change to the skills themselves.
