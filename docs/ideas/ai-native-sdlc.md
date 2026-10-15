---
title: "AI-native SDLC — how mkit could cover the whole loop"
date_created: 2026-10-12
last_updated: 2026-10-12
description: >
  Brainstorm mapping Anthropic's "AI-native SDLC playbook" (committed artifacts per stage,
  controls enforced by configuration and hooks, human judgement at explicit gates) onto mkit:
  new skills, new mechanical binary commands, and mods. Ranked, each with what it is, why it
  fits, and what it would cost. Not scheduled.
---

# AI-native SDLC — how mkit could cover the whole loop

**Status:** brainstorm, 2026-10-12. Not scheduled, not on [`backlog.md`](../backlog.md). Nothing built.

**Source:** [The AI-native SDLC playbook](https://claude.com/resources/articles/the-ai-native-sdlc-playbook)
(Anthropic, read 2026-10-12 as a summary, not verbatim). Mod mechanics are in
[`claude-code-mods.md`](claude-code-mods.md); this doc only names which event or API an idea uses.

## The playbook in one screen (enough to recap)

**Thesis.** Agents made writing code fast, so the slow parts are now the human steps around it:
planning, review, testing and deploying. Controls designed for human-written code don't keep up:
line-by-line review, weekly committees, handoff documents. The playbook replaces them with three
things:

1. **A committed artifact at each stage**, which is the next stage's input:
   `intent.md` → `spec.md` → `plan.md` → diff + tests → PR with review findings → incident record.
   The commit chain doubles as the audit trail.
2. **Governance enforced while the work happens.** Skills give advice, hooks and managed settings
   enforce, and nothing waits to be discovered at review. A skill alone is never enough for a rule
   that must always hold; pair it with a hook or a PR-stage check.
3. **People decide at explicit gates.** The product owner accepts the intent and spec, the engineer
   accepts the plan, the code owner approves the PR, and the release manager authorises production.
   Each decision is recorded and attributable. People review what the agent flagged instead of
   redoing each stage.

**Stages and their key plays:**

- **Plan.** An idea becomes an `intent.md`: problem, outcome, affected users and systems,
  constraints, open questions. It is drafted with Claude in a conversation.
- **Design.** The intent becomes a `spec.md`, constrained by policy skills. Policy conflicts are
  flagged, not resolved. The pass is first done by hand, then becomes a slash command, then a CI
  job that opens a spec PR.
- **Build.**
  - Plan mode by default. The plan is questioned until a stranger could implement it, then
    committed as `plan.md`, and updated in the same commit whenever the work drifts from it.
  - `CLAUDE.md` stays short; a correction is added whenever Claude makes the same mistake twice.
  - Build-time hooks are fast and touch single files.
  - Two or three parallel worktree sessions, on work the plan shows to be independent, and only as
    many as review can keep up with.
  - Subagents with scoped tools for recurring jobs.
- **Test.**
  - A one-command feedback loop with measurable done criteria.
  - Bug fixes start with a failing test that is committed first, and the fix may not edit it (a
    hook enforces this).
  - Browser or screenshot loops for UI work.
  - A verifier with a fresh context.
  - Continuous evals: 20–50 real tasks, run on every change to `CLAUDE.md`, skills or hooks, and
    every incident becomes an eval.
- **Deploy.**
  - AI review driven by a `REVIEW.md`: passes, Important vs Nit, a cap on nits, exclusions.
    Findings inform but never approve.
  - Required approvals become hooks that allow, ask or block.
  - Managed settings.
  - Agent jobs in CI that are read-only first; write steps always arrive as PRs.
  - "The agent may act up to the production gate and cannot pass it."
- **Maintain.**
  - A deterministic detector, with no model involved, triggers tiered responses (log → diagnose
    read-only → open a PR).
  - Diagnoses become `intent.md`, so the loop closes.
  - Scheduled security scans, where every dismissal records a reason.

**Metrics.** Each play gets a leading indicator (speed) and a lagging one (quality). Examples:
first-pass merge rate, rework cycles, spec edits after the plan, how often the merged diff still
matches the plan, first-pass CI success, eval pass rate.

## Where mkit already stands

mkit is already shaped like this playbook:

- **Its seven-step workflow is the Build-to-Deploy half:** `brainstorm` → `spec` → `implement` →
  `commit` → `review` → `pr`/`finish`.
- **The binary owns "controls enforced by configuration":** the gate, its fingerprinted ledger,
  findings validation, the sandbox audit.
- **The worklog is close to an audit trail.**

| Playbook stage | mkit today | Gap |
|---|---|---|
| Plan (intent) | — (`brainstorm` unbuilt, M8) | Nothing captures intent |
| Design (spec) | `[spec] store` in config (`github-issues`/`gitlab`/`files`/`none`); `spec` unbuilt (M8) | No spec artifact, no policy-conflict pass |
| Build (plan) | `mkit plan frontier/blocked/validate` planned (M8) | No drift check, no recorded plan approval |
| Build (context) | `AGENTS.md` written by hand | No loop from repeated mistakes back into it |
| Test | `mkit gate run` + ledger; `review` verify step | No test-first protection, no evals for mkit's own skills |
| Deploy | `review` (findings), `pr`, `finish`, `sandbox-audit` | No `REVIEW.md` policy, no finding ratings, no protected paths |
| Maintain | `recap`, worklog | No path from incident or scan to intent, no metrics |

**One adaptation runs through the whole doc: an artifact is not always a file.** The playbook
assumes `intent/`, `spec.md` and `plan.md` live in the repo. mkit's `[spec] store` can also be
GitHub Issues or GitLab. Every idea below therefore treats an "artifact" as *whatever the
configured store holds*: a file under `ref`, or an issue. The binary's job is to resolve and
link it, never to decide where it lives.

## Ground rules (from mkit's invariants)

- **Judgement stays in Markdown.** A skill decides whether an intent is good, whether drift is
  acceptable, or whether a finding is noise. The binary only counts, compares, fingerprints and
  resolves.
- **Commands report; they never commit.** No idea here auto-commits an intent, spec or plan. A
  skill drafts the artifact, a person accepts it, and `commit` commits it. The person's acceptance
  is the playbook's gate.
- **Composable, not sequential.** Each new skill can be the entry point and derives a thin version
  of whatever upstream artifact is missing, naming its assumptions. No skill tells the user to run
  another one first. A missing `intent.md` is a fact, never a stop.
- **Mods are optional.** Every mod idea here has a skill-only fallback. Skills never depend on the
  mod (rule 3 of the mods doc).
- **Write locations don't change.** New state goes to `<toplevel>/.mkit/` (worklog, ratings) or a
  committed artifact the user accepted. Nothing goes in `~/.claude`.

## Ideas by stage

Tags: **[skill]** a Markdown skill holds the judgement · **[bin]** an `mkit` command does a
mechanical check · **[mod]** needs a mod, so needs the ADR 0003 amendment from
[`claude-code-mods.md`](claude-code-mods.md).

### Plan: capturing intent

**[skill] `brainstorm` ends by writing an intent.** M8 already plans `brainstorm` with no binary
support. The playbook gives it a concrete output: a proto-spec with five sections (problem,
proposed outcome, affected users and systems, constraints, open questions). Today a brainstorm
ends as chat scrollback, which `spec` has to reconstruct. With an intent, `spec` has a stable
input and `recap` has something to point at. The skill drafts the intent; the user edits and
accepts it; `commit` commits it. In an issue store, it becomes an issue body instead.

**[bin] `mkit intent new <slug>`.** Renders the five-section template into the configured store
(a file under `[spec] ref`, or stdout for an issue body that the skill passes to `gh issue
create`). It decides no content, which makes it a legitimate mechanical command: one template and
one naming rule, so intents are found consistently. It could just as well be a template in
`_shared/references/` with no binary at all. Prefer that until something has to *find* intents
(see `mkit chain` below).

**[mod] `/intent` without interrupting the task.** The playbook's leading metric for this stage is
the time from first conversation to committed intent. Ideas usually arrive mid-task. An immediate
command plus `$.model.fork` in a side pane captures the idea as a draft intent without derailing
the current session, using the same mechanism as idea 2 of the mods doc (side recap). It writes a
draft into `.mkit/` only; the user promotes it later.

### Design: spec

**[skill] `spec` flags policy conflicts.** M8's `spec` already turns the discussion into a spec
plus a task graph without re-interviewing the user. The playbook adds one pass: read the repo's
policy sources (`AGENTS.md`/`CLAUDE.md`, `REVIEW.md`, `docs/adr/`, `CONTEXT.md`) and **flag**
anything the spec contradicts, without resolving it. Resolving belongs to the owner. This is pure
judgement, so it stays in Markdown, and it gets cheaper once the repo has a `REVIEW.md` (see
Deploy).

**[bin] `mkit chain [<slug>]`.** Reports the state of one feature's artifact chain:

- which of intent, spec and plan exist in the configured store;
- the SHA of each one's last change;
- links found via trailers (see Deploy);
- the key fact: `spec_edited_after_plan=yes|no`.

That last fact is the playbook's lagging Design metric (rework after build starts), and it's
plain `git log` arithmetic. `implement` and `review` read it as a starting fact: a spec changed
after the plan means the plan may be stale, and the skill decides what to do about it. In an
issue store, the timestamps come from `gh issue view --json` instead of git. Without a store
configured, it reports `chain=unavailable`, which is a fact, not a stop.

### Build: plan, drift, parallel work, context

**[skill] `implement` plans first.** The playbook's Build default matches M8's `implement`, with
two additions:

- **(a)** Before the first edit, the skill questions its own plan (what could break, which step is
  riskiest, what it rejected) and asks the user to accept it. The acceptance gets a worklog entry,
  which is the attributable decision.
- **(b)** The accepted plan is committed with the work. M8's task graph *is* the plan; `plan.md`
  is just its human-readable rendering.

**[bin] `mkit plan drift` (strongest fit).** Compares the files the plan names (per slice, from
the task graph) with the files the diff actually touches. It outputs
`plan_drift=none|unplanned:N|untouched:N` and lists them in `notes:`. `commit` reads it as a fact:

- No drift: nothing happens.
- Drift: the skill decides whether it's a legitimate discovery (update the plan in the same
  commit, as the playbook prescribes) or scope creep (ask).

The comparison is mechanical (path sets, with a glob when a plan names a directory). Deciding what
the drift *means* is not. It also yields the playbook's "merged diff matches `plan.md`" metric for
free.

**[bin] Parallel work from the frontier.** M8's `mkit plan frontier` already reports unblocked
slices. Adding *file disjointness* between frontier slices (no two touch the same path) gives the
playbook's "use the plan to find independent work". The binary reports the disjoint sets; the
skill (or the user) decides how many to run. The sandbox caveat from M8 still applies: `wt` is
fragile and parallel worktrees are opt-in via `[implement] worktrees = true`.

**[mod] Review-capacity band.** The playbook's limit on parallelism is review capacity, not
compute. A small `AbovePrompt` band would show open worktrees and branches with unreviewed commits
(from `mkit branch status --json` and the worklog), turning amber when unreviewed branches
outnumber reviewed ones. It only reports; it never blocks a new session.

**[skill + bin] `learn`: closing the "same mistake twice" loop (strong fit).** The playbook's
`CLAUDE.md` rule is: add a correction whenever Claude repeats a mistake. mkit already has the raw
data:

- `review`'s findings, with similarity grouping;
- the worklog (what was done and fixed);
- `sandbox-audit`'s events (repeated sandbox misses).

The pieces:

- **[bin] `mkit findings history`** groups findings across past runs using the existing similarity
  code. Output: clusters, counts, first and last seen, files. Mechanical.
- **[skill] `learn`** reads the clusters plus worklog fixes, decides which ones are *rules* (not
  one-offs), and proposes an `AGENTS.md` diff. It never edits the file directly, the same posture
  as `sandbox-audit` proposing a settings diff.

This is the idea most likely to improve every other skill, because it improves the context they
all run in.

### Test: verification

**Test-first bug fixes, three layers.** The playbook's play: commit a failing test, then fix
without touching it.

- **[skill]** `implement` in fix mode (or a small `fix` skill) writes the failing test first, runs
  the gate to see it fail, and commits it.
- **[bin] `mkit gate tests-locked --since <sha>`** reports `tests_changed=yes|no` plus the files,
  by fingerprinting the test paths at the lock commit (reusing `gate.Fingerprint`). Which paths
  count as tests is discovered per ecosystem (`*_test.go`, `test/`, `__tests__/`, …) or pinned in
  config. `review` reads the fact and raises an Important finding when a fix weakened its own
  test.
- **[mod]** A `tool.check` hook on `Edit`/`Write` to a locked test path, active only while
  `$.state` says a fix is in progress, asks via `$.ui.ask` before the edit lands. This is the
  playbook's "a hook blocks test-file edits". The binary check above is the fallback for anyone
  running without mods.

**[skill] A verifier with a fresh context.** `review` already verifies findings. The playbook's
verifier is different: a subagent with a fresh context that *runs* the thing (the gate, the app,
a browser check) against the spec's done criteria and reports pass or fail without fixing
anything. It avoids the bias of the context that wrote the code. It could be a step in `finish`,
or a `verify` skill on its own. A mod (`agent.spawn`) could pin it to a cheaper model and to
read-only tools.

**[bin] Done criteria become gate steps.** The playbook wants measurable done criteria. When a
spec states them as commands (a benchmark threshold, a specific test), `spec` can propose them as
extra gate steps (pinned in `[gate.commands]` by the user). "Done" then means the gate passes, and
the ledger's cache keeps re-runs cheap.

**[repo] Evals for mkit's own skills.** mkit's skills *are* its configuration, which is exactly
what the playbook says to put under continuous evals. Build 20–50 cases from real worklog history
(a dirty tree that `commit` must split, a review with a known finding, a `cleanup` with a
squash-merged branch). Run them with `claude plugin eval` on every PR touching `plugin/skills/**`
and gate merges on the pass rate. Every skill regression found in real use becomes a case. It
costs CI minutes and model spend, but it's the only way to catch a skill wording change that
quietly breaks a workflow. This repo item comes before any user-facing eval feature.

### Deploy: review policy, approval gates, audit trail

**[skill + `init`] `REVIEW.md` support.** The playbook puts review policy in a repo file: which
passes to run, what is Important vs Nit, a cap on nits, exclusions (generated code, anything CI
already enforces). `review` would read `REVIEW.md` when it exists and fall back to its built-in
defaults when it doesn't. `mkit init` would gain an optional page that renders a commented
template, the same way it renders `config.toml`. The repo then owns the policy, and `review`
stops hardcoding it.

**[mod + bin] Rate findings.** The playbook tunes review monthly by rating findings. After
`review`'s report, a mod form (idea 5 of the mods doc) lets the user mark each finding useful,
noise or wrong. **[bin] `mkit findings rate`** appends the ratings to `.mkit/`. Over time this
gives precision by category, which `learn` can use to propose `REVIEW.md` changes ("Nit-level
naming findings are rated noise 80% of the time"). Without mods, the skill asks once with
AskUserQuestion, or skips it.

**[mod] Protected paths.** A `[guard] protected = ["gen/**", "vendor/**"]` list in `config.toml`,
enforced by a `tool.check` hook on `Edit`/`Write` that allows, asks or blocks, with the reason
shown. This is the playbook's "hooks as approval gates", scoped to the repo. Config is an input
here, never a permission (backlog invariant 10): with no list, nothing is guarded. It overlaps
with [`recoverability-permission-gate.md`](recoverability-permission-gate.md); build them as one
mod if both happen.

**[skill + bin] Recorded decisions and trailers.** Two cheap additions give the playbook's audit
chain:

- **Decisions in the worklog.** When a person accepts an intent, spec or plan, or approves a PR's
  findings, the skill appends a worklog record: what was accepted, its SHA or issue number, and
  that the user accepted it. The worklog already records skill runs; this adds the *decision*.
- **Trailers.** `commit` adds `Intent:`, `Spec:` and `Plan:` trailers (paths or issue refs) when
  `mkit chain` knows them. **[bin] `mkit trace <sha>`** follows the trailers back from any commit
  to its intent. This is also the playbook's "link between record IDs and commit SHAs" for repos
  whose specs live in a tracker.

### Maintain: closing the loop

**[skill] `triage`: from incident or scan to intent.** The playbook's loop closes by writing
diagnoses as intents. A `triage` skill takes a failed CI run (`gh run view --log-failed`), a scan
finding or a bug report, and diagnoses it read-only. It then either proposes a small fix, which
goes through `commit` → `pr`, or drafts an `intent.md`, which goes back to Plan. Every dismissal
records a reason in the worklog. It's judgement throughout, so it stays a skill.

**[skill] `scan`: scheduled security review.** `review`'s pipeline (find → reconcile → verify)
pointed at the whole repo, or at what changed since the last scan, with a security-only pass.
Findings get triaged the same way as above. Scheduling stays outside mkit: a cron or scheduled
task invokes the skill. mkit never runs as a daemon.

**[bin] `mkit metrics`.** The playbook measures every play. mkit can compute the Build, Test and
Deploy ones from data it already owns, with no model:

- first-pass gate success: the gate ledger's first run per fingerprint;
- rework cycles: review → fix → review loops in the worklog;
- time from plan acceptance to merge: worklog decision record → merge commit;
- plan adherence: from `plan drift`;
- spec rework: from `chain`.

The output is `--json` plus a human table. A mod pane could chart it. The deterministic-detector
rule applies: the numbers are computed, and only interpreting them is left to a skill (`recap`
could mention a trend).

## Most promising (suggested order)

1. **`mkit plan drift` + `implement` plans first.** Already on M8's path, purely mechanical, and
   it turns the playbook's central Build rule into a starting fact.
2. **`learn` + `mkit findings history`.** Reuses existing similarity code and improves the context
   every other skill runs in.
3. **Trailers + `mkit trace` + decision records.** Small, and gives the audit chain and the
   tracker-to-SHA link at no ongoing cost.
4. **Test-first trio** (skill + `tests-locked` + mod). The strongest guardrail. The binary half
   works without mods.
5. **Evals for mkit's own skills.** Protects all of the above from wording regressions.
6. The rest as they earn it: `REVIEW.md`, finding ratings, `chain`, `metrics`, `triage`/`scan`,
   the mod bands.

## Avoid

- **Model-based detection or metrics.** The playbook itself says detection involves no model.
  `metrics` computes; a skill only interprets.
- **Auto-committing intent, spec or plan,** or opening spec PRs from a CI job inside mkit. It
  breaks "commands never commit" and removes the person's acceptance, which *is* the gate. A team
  can wire `claude -p` in CI themselves.
- **mkit-generated managed settings.** Enterprise policy is out of scope. `sandbox-audit` already
  proposes user and project settings diffs and never applies them; it stays that way.
- **A "governance mode" that blocks by default.** Every guard is opt-in config. A repo without
  config must work exactly as today (backlog invariant 10).
- **Duplicating Claude Code's own review products.** Managed Code Review and `claude-code-action`
  cover CI-side PR review. mkit's `review` stays the local, pre-PR pass, and `REVIEW.md` is the
  shared policy both can read.

## Costs and open questions

- **More skills.** Each new skill (`learn`, `triage`, `scan`, `verify`, `fix`) needs evals (idea
  5) to stay trustworthy. Fold where possible: `fix` into `implement`, `verify` into `finish`.
- **Artifact naming.** One rule for where intents, specs and plans live under `files` stores
  (`docs/specs/<slug>/{intent,spec,plan}.md`?) must be decided once, in config. Otherwise
  `chain`, `trace` and `drift` each guess.
- **Test-path discovery** for `tests-locked` is ecosystem-specific, the same class of problem as
  `gate.Detect`, and should reuse its runner and ecosystem tables.
- **Mod items** inherit every cost listed in the mods doc: the ADR 0003 amendment, a minimum Claude
  Code version, and no sandbox.

## Related

- [`claude-code-mods.md`](claude-code-mods.md): mechanics and ground rules for every **[mod]** idea
  here. Its idea 5 (review form) is the base for finding ratings.
- [`recoverability-permission-gate.md`](recoverability-permission-gate.md): overlaps with the
  protected-paths guard.
- [`journal-transcript-digest.md`](journal-transcript-digest.md): an alternative source for
  `learn` (mistakes visible in transcripts, not only in findings).
- [`backlog.md` M8](../backlog.md): `brainstorm`, `spec`, `implement`, `mkit plan`, the base most
  of these ideas build on.
