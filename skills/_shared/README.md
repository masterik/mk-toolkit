# _shared — reference bundle

The **shared library** for mkit's four workflow skills. **Not a triggerable skill** — no `SKILL.md`.
`commit`, `review-changes`, `finish-feature` and `create-pr` link into `references/` via relative paths
(`../_shared/references/…`), so safety rules and conventions live in exactly one place.

> Keep those `../_shared/references/…` links intact — sibling-relative paths are what make the bundle
> portable if it is lifted into another repo.

## The skills that consume it

| Skill            | Does                                                                         | Consumes                                                                 |
| ---------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `commit`         | Inspect tree, stage intentionally, split into logical Conventional Commits.  | `conventional-commits`, `quality-gate` (fast tier), `git-safety`, `output-discipline`, `agent-delegation` |
| `review-changes` | Review local diff/commits with CodeRabbit + Codex + Claude, verify, auto-fix safe, summarize. | `review-severity`, `lenses-correctness`/`lenses-craft`, `triage-reconcile`/`triage-verify`/`fix-checks`, `agent-delegation`, `output-discipline`, `quality-gate` (fast tier), `git-safety` |
| `finish-feature` | Commit → merge branch back into base → delete branch / remove worktree.      | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, all of the above |
| `create-pr`      | Commit → push → open GitHub PR → assign reviewers.                           | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, `agent-delegation`, all of the above |

`commit` is the shared front-end — both finishers start by committing. Pick the finisher by **destination**:
`finish-feature` merges locally yourself, `create-pr` pushes for remote review.

## References (`references/`)

- `git-safety.md` — the non-negotiable git safety protocol: no force-push, no config edits, no AI attribution,
  don't skip hooks, confirm before irreversible steps.
- `conventional-commits.md` — message format, type table, scope detection.
- `quality-gate.md` — how to **detect** (not hardcode) the repo's fast check and full lint/test/build gate, and
  how to triage a failing step: delegate the diagnosis, report cause + suggested fix, never the log.
- `worktree.md` — detect the worktree origin (worktrunk `wt` / Claude Code `.claude/worktrees/` / plain
  `git worktree`) and clean up correctly.
- `branching.md` — the branch model to assume when a repo documents none.
- `review-severity.md` — the severity bar (`critical`/`major`/`minor`, prose is minor), the
  `[surface, severity]` tag, the read-only reviewer contract, what not to report, and why a partial review is
  never reported as clean.
- `lenses-correctness.md` / `lenses-craft.md` — the eight review lenses, split by the reviewer that carries
  each set: `bugs`/`impl`/`adversarial` to Codex, `architecture`/`quality`/`tests`/`docs`/`comments` to the
  Claude subagent. Split so a reviewer loads only its own lenses.
- `triage-reconcile.md`, `triage-verify.md`, `fix-checks.md` — what happens after the reviewers return, one
  file per stage and per consumer: reconcile (dedupe + corroboration arithmetic, in a subagent), verify (five
  verdicts + the materiality test, one subagent per group), and the three checks on every fix plus what gates
  (the main session).
- `agent-delegation.md` — running heavy work without paying for it in context: the run directory as transport,
  subagent return budgets, resolved reference paths, one-writer-per-file, model-per-stage.
- `output-discipline.md` — the run directory itself (opened with `scripts/run-open.sh` before anything logs,
  written as `<run-dir>` — there is no `$RUN_DIR`) and bounding command output: gate logs to a file and read
  the tail, `--stat` before any diff, never a full branch diff to write prose — plus what must never be capped
  (a staged diff you are approving, a body the user acts on).

## Design invariants

- **Nothing project-specific is hardcoded** — quality-gate commands, commit scopes and reviewers are
  *discovered* from the target repo, so the bundle works in any project (Bun/Node, .NET, Rust, Go, Python…).
- **Composition over replacement** — the skills orchestrate `git`, `gh` and `wt`; they never re-encode git logic.
- **Degrades gracefully** — `review-changes` calls the `coderabbit` and `codex` plugins when present,
  redistributes their lenses to a Claude subagent otherwise, and never reports a partial review as clean.
- **Independent sources, then one bar** — reviewers never see each other's findings, and all rate against the
  single severity bar in `review-severity.md`. That is what makes their lists mergeable and corroboration
  meaningful.
- **The main session holds decisions, not evidence** — stages that read a lot and decide a little run in
  subagents and hand back a summary; diffs, transcripts and finding bodies live in a per-run directory under
  the git dir, opened before anything logs, and command output is bounded before it arrives
  (`output-discipline.md`).

For the plugin's overall design and roadmap see [`concept.md`](../../concept.md).
