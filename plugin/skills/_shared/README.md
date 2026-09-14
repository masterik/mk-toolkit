# _shared — reference bundle

The **shared library** for mkit's skills. **Not a triggerable skill** — no `SKILL.md`.
`commit`, `review`, `finish`, `pr` and `cleanup` link into `references/` via relative paths
(`../_shared/references/…`), so safety rules and conventions live in exactly one place.

The workflow is seven steps plus `cleanup`; `brainstorm`, `spec` and `implement` are designed and
not yet built ([`backlog.md`](../../../docs/backlog.md), M6–M8), so the table below lists the four
step skills that exist today. `references/workflow-contract.md` is what all seven are held to; the
existing four are retrofitted to link and record against it in M6.

> Keep those `../_shared/references/…` links intact — sibling-relative paths are what make the bundle
> portable if it is lifted into another repo.

## The skills that consume it

| Skill      | Does                                                                                           | Consumes                                                                                                   |
| ---------- | ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `commit`   | Inspect tree, stage intentionally, split into logical Conventional Commits.                    | `conventional-commits`, `git-safety`, `output-discipline`, `agent-delegation` |
| `review`   | Review local diff/commits — full mode with CodeRabbit + Codex + Claude, quick mode with CodeRabbit + Codex only — verify, auto-fix safe, summarize. | `review-severity`, `lenses-correctness`/`lenses-craft`, `triage-reconcile`/`triage-verify`/`fix-checks`, `agent-delegation`, `output-discipline`, `git-safety` |
| `finish`   | Commit → merge branch back into base → delete branch / remove worktree.                        | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, all of the above |
| `pr`       | Commit → push → open GitHub PR → assign reviewers.                                              | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, `agent-delegation`, all of the above |
| `cleanup`  | Classify every local branch/worktree, delete/keep by that classification, keep only the default and a local `develop`-like branch, switch and pull. Local only. | `worktree`, `branching`, `git-safety`, `output-discipline` |

`commit` is the shared front-end — both finishers start by committing. Pick the finisher by **destination**:
`finish` merges locally yourself, `pr` pushes for remote review.

`cleanup` is the one skill outside that line: repo-wide gardening rather than feature work — it
sweeps every local branch and worktree, not just the one the other four just touched.

## References (`references/`)

- `workflow-contract.md` — how the seven steps compose: every step **entry-capable** (runs alone, in any
  order, with any subset skipped), the five rules that make that hold, the per-branch worklog they record
  into, and what each step owes the next. The reason `commit` alone is a complete run rather than step four
  of seven.
- `git-safety.md` — the non-negotiable git safety protocol: no force-push, no config edits, no AI attribution,
  don't skip hooks, confirm before irreversible steps.
- `conventional-commits.md` — message format, type table, scope detection.
- `quality-gate.md` — detecting the repo's lint/test/build gate with `mkit gate detect` (never
  hardcoded), running it through `mkit gate run`, and when a failure still needs a delegated
  diagnosis.
- `worktree.md` — the `cleanup_path` `mkit facts` reports (`exit-worktree` / `wt` / `git-worktree` / `none`) and
  the teardown each one takes.
- `branching.md` — the branch model to assume when a repo documents none.
- `review-severity.md` — the severity bar (`critical`/`major`/`minor`, prose is minor), the
  `[surface, severity]` tag, the read-only reviewer contract, what not to report, and why a partial review is
  never reported as clean.
- `lenses-correctness.md` / `lenses-craft.md` — the eight review lenses, split by the reviewer that carries
  each set: `bugs`/`impl`/`adversarial` to Codex, `architecture`/`quality`/`tests`/`docs`/`comments` to the
  Claude subagent. Split so a reviewer loads only its own lenses.
- `triage-reconcile.md`, `triage-verify.md`, `fix-checks.md` — what happens after the reviewers return, one
  file per stage and per consumer: reconcile (`mkit findings reconcile` does the arithmetic; the main session
  judges what it leaves open), verify (five verdicts + the materiality test, one subagent per group from
  `mkit findings group`), and the three checks on every fix plus what gates (the main session).
- `agent-delegation.md` — running heavy work without paying for it in context: the run directory as transport,
  subagent return budgets, resolved reference paths, one-writer-per-file, model-per-stage.
- `output-discipline.md` — the one call that starts a run (`mkit facts`, which opens the run directory
  and returns every starting fact), the gate runner (`mkit gate run`), and bounding command output:
  `--stat` before any diff, never a full branch diff to write prose — plus what must never be capped (a staged
  diff you are approving, a body the user acts on).

## Design invariants

- **Nothing project-specific is hardcoded** — quality-gate commands, commit scopes and reviewers are
  *discovered* from the target repo, so the bundle works in any project (Bun/Node, .NET, Rust, Go, Python…).
- **Composition over replacement** — the skills orchestrate `git`, `gh` and `wt`; they never re-encode git logic.
- **Degrades gracefully** — `review` calls the `coderabbit` and `codex` plugins when present,
  redistributes their lenses to a Claude subagent otherwise, and never reports a partial review as clean.
- **Independent sources, then one bar** — reviewers never see each other's findings, and all rate against the
  single severity bar in `review-severity.md`. That is what makes their lists mergeable and corroboration
  meaningful.
- **The main session holds decisions, not evidence** — stages that read a lot and decide a little run in
  subagents and hand back a summary; diffs, transcripts and finding bodies live in a per-run directory under
  `<toplevel>/.mkit/` — ignored, one per worktree — opened before anything logs, and command output is
  bounded before it arrives (`output-discipline.md`).
- **A command for a mechanical invariant, never for a decision** — `mkit` owns the steps that are the same
  every run and fail silently when hand-rolled (opening the run directory, logging a gate step, classifying a
  worktree, confidence arithmetic). Judgement stays in Markdown; see [`concept.md`](../../../docs/concept.md).

For the plugin's overall design and roadmap see [`concept.md`](../../../docs/concept.md).
