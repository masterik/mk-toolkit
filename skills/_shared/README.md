# _shared — reference bundle

The **shared library** for mkit's six skills. **Not a triggerable skill** — no `SKILL.md`.
`commit`, `review`, `finish`, `pr`, `note` and `cleanup` link into `references/` via relative paths
(`../_shared/references/…`), so safety rules and conventions live in exactly one place.

> Keep those `../_shared/references/…` links intact — sibling-relative paths are what make the bundle
> portable if it is lifted into another repo.

## The skills that consume it

| Skill      | Does                                                                                           | Consumes                                                                                                   |
| ---------- | ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `commit`   | Inspect tree, stage intentionally, split into logical Conventional Commits.                    | `conventional-commits`, `quality-gate` (fast tier), `git-safety`, `output-discipline`, `agent-delegation` |
| `review`   | Review local diff/commits — full mode with CodeRabbit + Codex + Claude, quick mode with CodeRabbit + Codex only — verify, auto-fix safe, summarize. | `review-severity`, `lenses-correctness`/`lenses-craft`, `triage-reconcile`/`triage-verify`/`fix-checks`, `agent-delegation`, `output-discipline`, `quality-gate` (fast tier), `git-safety` |
| `finish`   | Commit → merge branch back into base → delete branch / remove worktree.                        | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, all of the above |
| `pr`       | Commit → push → open GitHub PR → assign reviewers.                                              | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, `agent-delegation`, all of the above |
| `note`     | Record why the unit of work just finished exists, into the commit journal. One call, no diff read. | `journal`, `conventional-commits` |
| `cleanup`  | Classify every local branch/worktree, delete/keep by that classification, keep only the default and a local `develop`-like branch, switch and pull. Local only. | `worktree`, `branching`, `git-safety`, `output-discipline` |

`commit` is the shared front-end — both finishers start by committing. Pick the finisher by **destination**:
`finish` merges locally yourself, `pr` pushes for remote review.

`note` and `cleanup` are the two skills outside that line: `note` writes intent *during*
implementation, which `commit` later spends (normally the `Stop` / `SubagentStop` hook does that
writing — `note` is the manual path); `cleanup` is repo-wide gardening rather than feature work —
it sweeps every local branch and worktree, not just the one the other five just touched.

## References (`references/`)

- `git-safety.md` — the non-negotiable git safety protocol: no force-push, no config edits, no AI attribution,
  don't skip hooks, confirm before irreversible steps.
- `conventional-commits.md` — message format, type table, scope detection.
- `quality-gate.md` — detecting the repo's fast check and full lint/test/build gate with
  `scripts/gate-detect.sh` (never hardcoded), running them through `scripts/gate-run.sh`, and when a failure
  still needs a delegated diagnosis.
- `worktree.md` — the `cleanup_path` `facts.sh` reports (`exit-worktree` / `wt` / `git-worktree` / `none`) and
  the teardown each one takes.
- `branching.md` — the branch model to assume when a repo documents none.
- `review-severity.md` — the severity bar (`critical`/`major`/`minor`, prose is minor), the
  `[surface, severity]` tag, the read-only reviewer contract, what not to report, and why a partial review is
  never reported as clean.
- `lenses-correctness.md` / `lenses-craft.md` — the eight review lenses, split by the reviewer that carries
  each set: `bugs`/`impl`/`adversarial` to Codex, `architecture`/`quality`/`tests`/`docs`/`comments` to the
  Claude subagent. Split so a reviewer loads only its own lenses.
- `triage-reconcile.md`, `triage-verify.md`, `fix-checks.md` — what happens after the reviewers return, one
  file per stage and per consumer: reconcile (`findings.mjs reconcile` does the arithmetic; the main session
  judges what it leaves open), verify (five verdicts + the materiality test, one subagent per group from
  `findings.mjs group`), and the three checks on every fix plus what gates (the main session).
- `journal.md` — the commit journal: intent is a changelog, not a commit plan; the hook names the gap and
  the agent supplies the judgement; the `unit` record, the five freshness classes, and who decides what.
- `agent-delegation.md` — running heavy work without paying for it in context: the run directory as transport,
  subagent return budgets, resolved reference paths, one-writer-per-file, model-per-stage.
- `output-discipline.md` — the one call that starts a run (`scripts/facts.sh`, which opens the run directory
  and returns every starting fact), the gate runner (`scripts/gate-run.sh`), and bounding command output:
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
  the git dir, opened before anything logs, and command output is bounded before it arrives
  (`output-discipline.md`).
- **A script for a mechanical invariant, never for a decision** — `scripts/` owns the steps that are the same
  every run and fail silently when hand-rolled (opening the run directory, logging a gate step, classifying a
  worktree, confidence arithmetic). Judgement stays in Markdown; see [`concept.md`](../../concept.md).

For the plugin's overall design and roadmap see [`concept.md`](../../concept.md).
