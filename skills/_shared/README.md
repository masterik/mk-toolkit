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
| `commit`         | Inspect tree, stage intentionally, split into logical Conventional Commits.  | `conventional-commits`, `quality-gate` (fast tier), `git-safety`        |
| `review-changes` | Review local diff/commits with CodeRabbit + Codex, auto-fix safe, summarize. | `quality-gate` (fast tier), `git-safety`                                |
| `finish-feature` | Commit → merge branch back into base → delete branch / remove worktree.      | `worktree`, `quality-gate` (full gate), `branching`, all of the above   |
| `create-pr`      | Commit → push → open GitHub PR → assign reviewers.                           | `worktree`, `quality-gate` (full gate), `branching`, all of the above   |

`commit` is the shared front-end: `finish-feature` and `create-pr` both start by committing.
Pick the finisher by **destination** — `finish-feature` merges it yourself locally,
`create-pr` pushes it for remote review.

## References (`references/`)

- `git-safety.md` — the non-negotiable git safety protocol (no force-push, no config edits,
  no AI attribution, don't skip hooks, confirm before irreversible steps).
- `conventional-commits.md` — commit message format, type table, scope detection.
- `quality-gate.md` — how to **detect** (not hardcode) the repo's fast check and full
  lint/test/build gate.
- `worktree.md` — detect the worktree origin (worktrunk `wt` / Claude Code
  `.claude/worktrees/` / plain `git worktree`) and clean up correctly.
- `branching.md` — the default branch model to assume when a repo doesn't document its own.

## Design invariants

- **Nothing project-specific is hardcoded** — quality-gate commands, commit scopes, and
  reviewers are all *discovered* from the target repo, so the bundle works in any project
  (Bun/Node, .NET, Rust, Go, Python…).
- **Composition over replacement** — the skills orchestrate `git`, `gh`, and `wt`; they
  never re-encode git logic.
- **Degrades gracefully** — `review-changes` calls the `coderabbit` and `codex` plugins when
  present and falls back to the built-in `/code-review` otherwise.

For the plugin's overall design and roadmap, see [`concept.md`](../../concept.md).
