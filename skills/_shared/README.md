# _shared — reference bundle

This folder is the **shared library** for the mkit plugin's four workflow skills. It is
**not a triggerable skill** — it has no `SKILL.md`. Instead, `commit`, `review-changes`,
`finish-feature`, and `create-pr` link into `references/` via relative paths
(`../_shared/references/…`) so the safety rules and conventions live in exactly one place.

> Keep those `../_shared/references/…` links intact — sibling-relative paths are what make
> the bundle portable if it's ever lifted into another repo.

## The skills that consume it

| Skill            | Does                                                                         | Consumes                                                                 |
| ---------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `commit`         | Inspect tree, stage intentionally, split into logical Conventional Commits.  | `conventional-commits`, `quality-gate` (fast tier), `git-safety`, `output-discipline`, `agent-delegation` |
| `review-changes` | Review local diff/commits with CodeRabbit + Codex + Claude, verify, auto-fix safe, summarize. | `review-severity`, `review-lenses`, `finding-triage`, `agent-delegation`, `output-discipline`, `quality-gate` (fast tier), `git-safety` |
| `finish-feature` | Commit → merge branch back into base → delete branch / remove worktree.      | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, all of the above |
| `create-pr`      | Commit → push → open GitHub PR → assign reviewers.                           | `worktree`, `quality-gate` (full gate), `branching`, `output-discipline`, `agent-delegation`, all of the above |

`commit` is the shared front-end: `finish-feature` and `create-pr` both start by committing.
Pick the finisher by **destination** — `finish-feature` merges it yourself locally,
`create-pr` pushes it for remote review.

## References (`references/`)

- `git-safety.md` — the non-negotiable git safety protocol (no force-push, no config edits,
  no AI attribution, don't skip hooks, confirm before irreversible steps).
- `conventional-commits.md` — commit message format, type table, scope detection.
- `quality-gate.md` — how to **detect** (not hardcode) the repo's fast check and full
  lint/test/build gate, and how to triage a failing step (delegate the diagnosis, report a
  cause and a suggested fix, never the log).
- `worktree.md` — detect the worktree origin (worktrunk `wt` / Claude Code
  `.claude/worktrees/` / plain `git worktree`) and clean up correctly.
- `branching.md` — the default branch model to assume when a repo doesn't document its own.
- `review-severity.md` — the severity bar (`critical`/`major`/`minor`, prose is minor), the
  `[surface, severity]` tag, the read-only reviewer contract, what not to report, and why a
  partial review is never reported as clean.
- `review-lenses.md` — the eight review lenses and which reviewer carries which.
- `finding-triage.md` — what happens after the reviewers return: reconcile (dedupe +
  corroboration arithmetic), verify (five verdicts + the materiality test), and the three
  checks on every fix.
- `agent-delegation.md` — how a skill runs heavy work without paying for it in context: the
  per-run directory inside the git dir, subagent return budgets, resolved reference paths,
  one-writer-at-a-time, and model-per-stage.
- `output-discipline.md` — bounding command output: gate logs to a file and read the tail,
  `--stat` before any diff, never a full branch diff to write prose — and what must never be
  capped (a staged diff you are approving, a body the user acts on).

## Design invariants

- **Nothing project-specific is hardcoded** — quality-gate commands, commit scopes, and
  reviewers are all *discovered* from the target repo, so the bundle works in any project
  (Bun/Node, .NET, Rust, Go, Python…).
- **Composition over replacement** — the skills orchestrate `git`, `gh`, and `wt`; they
  never re-encode git logic.
- **Degrades gracefully** — `review-changes` calls the `coderabbit` and `codex` plugins when
  present, redistributes their lenses to a Claude subagent otherwise, and never reports a
  partial review as a clean one.
- **Independent sources, then one bar** — reviewers never see each other's findings, and all
  of them rate against the single severity bar in `review-severity.md`, which is what makes
  their lists mergeable and corroboration meaningful.
- **The main session holds decisions, not evidence** — stages that read a lot and decide a
  little run in subagents and hand back a summary; diffs, transcripts and findings bodies live
  in a per-run directory under the git dir (`agent-delegation.md`), and command output is
  bounded before it ever arrives (`output-discipline.md`).

For the plugin's overall design and roadmap, see [`concept.md`](../../concept.md).
