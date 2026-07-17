# git-flow skill bundle

A cohesive set of git workflow skills that take work from edits → committed → integrated. Designed to be **self-contained
and portable**: lift the folders below into any repo, or promote them to a distributable Claude Code plugin.

## Skills

| Skill            | Does                                                                                   | Trigger examples                                        |
| ---------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| `commit`         | Inspect tree, stage intentionally, split into logical Conventional Commits.            | "commit", "split into commits"                          |
| `finish-feature` | Commit → merge branch back into base → delete branch / remove worktree. **Local**, no PR. | "finish this feature", "merge back and clean up"        |
| `create-pr`      | Commit → push → open GitHub PR → assign reviewers. **Remote review** path.              | "create a PR", "open a pull request", "submit for review" |
| `review-changes` | Review local diff/commits with CodeRabbit + Codex, auto-fix safe issues, summarize.    | "review my changes", "run codex and coderabbit"         |

`commit` is the shared front-end: `finish-feature` and `create-pr` both start by committing. Choose the finisher by
destination — `finish-feature` = merge it yourself locally; `create-pr` = push it for review.

## Shared references (`git-flow/references/`)

The four skills stay DRY by linking into these instead of duplicating rules:

- `git-safety.md` — the non-negotiable git safety protocol (no force-push, no config edits, no AI attribution, …).
- `conventional-commits.md` — commit message format, type table, scope detection.
- `quality-gate.md` — how to **detect** (not hardcode) the project's fast check + full lint/test/build gate.
- `worktree.md` — detect worktree origin (worktrunk `wt` / Claude Code `.claude/worktrees/` / plain git) and clean up correctly.
- `branching.md` — the default branch model (`main` + `feat/*`/`bugfix/*`/`hotfix/*`) to assume when a repo doesn't document its own.

## Worktree support

The finishing skills detect which kind of worktree they're in and use the matching cleanup path:

- **worktrunk** (`wt` + `.config/wt.toml`) → delegate to `wt merge` / `wt remove` (respects the user's squash/rebase + hooks).
- **Claude Code agent worktree** (`.claude/worktrees/`) → merge, then hand back via the ExitWorktree tool.
- **plain `git worktree`** → `git worktree remove` + `git branch -d`.

See `references/worktree.md` for the detection commands and merge-back strategy.

## Portability — nothing project-specific is hardcoded

- Quality-gate commands are **discovered** from the repo toolchain, not hardcoded (works for Bun/Node, .NET, Rust, Go, Python…).
- Commit **scopes** are read from repo conventions (`git-workflow.md`, commitlint, CODEOWNERS) or inferred from paths.
- **Reviewers** come from CODEOWNERS / repo config / the user — no baked-in handles.

## Reuse in another repo (copy method)

Copy these into the target repo and expose them the way that repo loads skills:

```
.agents/skills/commit/
.agents/skills/finish-feature/
.agents/skills/create-pr/
.agents/skills/review-changes/
.agents/skills/git-flow/            # this folder (README + references) — required by the four skills
```

In this repo, active skills are symlinked from `.claude/skills/<name>` → `../../.agents/skills/<name>`. `git-flow/` is a
resource folder (no `SKILL.md`) and is intentionally **not** symlinked — it's referenced via relative paths
(`../git-flow/references/…`), which resolve as long as the folders stay siblings.

> `review-changes` calls the `coderabbit` and `codex` plugins when present and falls back to the built-in `/code-review`
> otherwise, so it degrades gracefully in a repo without them.

## Promote to a Claude Code plugin (distribute method)

To share across machines/teams, package the bundle as a plugin:

```
git-flow/                       # plugin repo root
  .claude-plugin/
    plugin.json                 # { "name": "git-flow", "version": "0.1.0", "description": "...", ... }
    marketplace.json            # marketplace entry (or add to an existing marketplace)
  skills/
    commit/SKILL.md
    finish-feature/SKILL.md
    create-pr/SKILL.md
    review-changes/SKILL.md
    git-flow/references/*.md     # shared refs; keep the `../git-flow/references/…` relative links intact
```

Then `/plugin marketplace add <repo>` + `/plugin install git-flow@<marketplace>`. Plugin skills are namespaced
(`git-flow:commit`), which also removes any name clash with repo-local skills. When installing the plugin into a repo
that already has these as loose skills, retire the loose copies to avoid double-triggering.
