# _shared: reference bundle

This is the shared library for mkit's skills. It is not a triggerable skill and has no `SKILL.md`.
Skills link into `references/` by relative path (`../_shared/references/…`), so each rule lives in
one place.

> Keep those relative links intact. They are what make the bundle portable to another repo.

## Consumers

| Skill | Consumes |
| --- | --- |
| `commit` | `conventional-commits`, `git-safety`, `output-discipline`, `agent-delegation`, `quality-gate`, `summary-format`, `workflow-contract` |
| `review` | `review-severity`, `lenses-correctness`, `lenses-craft`, `triage-reconcile`, `triage-verify`, `fix-checks`, `agent-delegation`, `output-discipline`, `git-safety`, `quality-gate`, `summary-format`, `workflow-contract` |
| `pr` | everything `commit` uses, plus `worktree`, `quality-gate`, `branching`, `stacked-prs`, `workflow-contract` |
| `finish` | `conventional-commits`, `git-safety`, `output-discipline`, `summary-format`, `worktree`, `quality-gate`, `branching`, `stacked-prs`, `workflow-contract` |
| `cleanup` | `worktree`, `branching`, `git-safety`, `output-discipline`, `summary-format` |
| `sandbox-audit` | `output-discipline`, `summary-format` |
| `explain` | `plain-english` |
| `recap` | `plain-english`, `workflow-contract` |

`commit` is the shared front end, since both finishers start by committing. Choose a finisher by
destination: `finish` merges locally, and `pr` pushes for remote review. `cleanup` works repo-wide,
sweeping every branch and worktree. The workflow has seven steps, but three of them (`brainstorm`,
`spec`, `implement`) are not built yet ([backlog](../../../docs/backlog.md), M8).

## References

- `workflow-contract.md`: how the steps compose. Every step is entry-capable (runs alone, in any order,
  with any subset skipped). Covers the rules that keep this true, the per-branch worklog, and what each
  step owes the next.
- `git-safety.md`: no force-push, no config edits, no AI attribution, no skipped hooks; confirm before
  any irreversible step.
- `conventional-commits.md`: message format, type table, scope detection.
- `quality-gate.md`: find the gate with `mkit gate detect` (never hardcoded), run it with
  `mkit gate run`, and know when a failure needs a delegated diagnosis.
- `worktree.md`: the `cleanup_path` values `mkit facts` reports (`exit-worktree`, `wt`,
  `git-worktree`, `none`) and the teardown for each.
- `branching.md`: the branch model to assume when a repo documents none.
- `stacked-prs.md`: detecting a GitHub-native stack and routing around the calls that break on one.
  Without a stack, `pr` and `finish` are unchanged.
- `review-severity.md`: the severity bar (`critical`, `major`, `minor`; prose findings are minor), the
  `[surface, severity]` tag, the read-only reviewer contract, what not to report, and why a partial
  review is never clean.
- `lenses-correctness.md` / `lenses-craft.md`: the eight review lenses, split by reviewer. `bugs`,
  `impl` and `adversarial` go to Codex. `architecture`, `quality`, `tests`, `docs` and `comments` go to
  the Claude subagent.
- `triage-reconcile.md`, `triage-verify.md`, `fix-checks.md`: what happens after the reviewers return.
  - Reconcile: `mkit findings reconcile` does the arithmetic, and the main session judges what is
    left open.
  - Verify: five verdicts plus a materiality test, with one subagent per `mkit findings group` group.
  - Fix-checks: the three checks every fix must pass.
- `agent-delegation.md`: the run dir as transport, subagent return budgets, resolved reference paths,
  one writer per file, model per stage.
- `output-discipline.md`: `mkit facts` as a run's first call, `mkit gate run`, and bounding output
  (`--stat` before any diff). Also what must never be capped: a staged diff being approved, or a body
  the user acts on.
- `summary-format.md`: the form of a skill's closing message and how it asks for a decision. Each skill
  keeps its own content list.
- `plain-english.md`: register, terms, and when to draw a diagram, for `explain` and `recap`.

## Design invariants

- **Nothing project-specific is hardcoded.** Gate commands, scopes and reviewers are discovered from the
  target repo.
- **Compose, don't replace.** Orchestrate `git`, `gh` and `wt`; never re-encode git logic.
- **Degrade gracefully.** If the `coderabbit` or `codex` plugin is missing, its lenses go to a Claude
  subagent. A partial review is never reported as clean.
- **Independent sources, one bar.** Reviewers never see each other's findings, and all of them rate
  against `review-severity.md`. That is what makes merging their findings and counting corroboration
  meaningful.
- **The main session holds decisions, not evidence.** Heavy reading runs in subagents. Diffs,
  transcripts and finding bodies live in a per-run dir under `<toplevel>/.mkit/`. Output is bounded
  before it reaches the main session.
- **A command for a mechanical invariant, never for a decision.** `mkit` owns the steps that are the same
  every run and fail silently when done by hand. Judgement stays in Markdown
  ([concept](../../../docs/concept.md)).
