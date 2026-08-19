---
name: create-pr
description: >-
  Pull request flow: commit remaining work, push branch, open a GitHub PR with title/description, assign reviewers.
  Trigger on "create a PR", "open a PR", "make a PR", "submit for review", "push a branch for merge". Remote review path
  — for a local merge use finish-feature.
model: sonnet
---

# Create a Pull Request (commit → push → PR → reviewers)

Part of the **mkit** workflow bundle. This is the review path: it commits, pushes, opens the PR, and requests reviewers. For a
local merge with no review, use `finish-feature` instead.

Shared references: `../_shared/references/conventional-commits.md`, `../_shared/references/quality-gate.md`,
`../_shared/references/worktree.md`, `../_shared/references/git-safety.md`, `../_shared/references/branching.md`,
`../_shared/references/output-discipline.md`, `../_shared/references/agent-delegation.md`.

## Goal

Open a PR that is easy to review and safe to merge:

- based on a feature/bugfix branch (never the base branch)
- all intended work committed and pushed
- pre-flight checks pass before the PR is created
- title and description communicate **what** changed and **why**
- reviewers requested when the repo/team expects them
- no Claude/AI co-authorship attribution anywhere

## Inputs to confirm (if missing)

- **Target (base) branch** — default `main`; ask if unclear.
- **Draft?** — create as draft if the work isn't ready for review.
- **Area prefix** — if scoped to one package, prefix the title `[area]` (e.g. `[api]`, `[ui]`).
- **Reviewers** — see step 6; determine from repo config or ask.

## Workflow

### 1. Commit remaining work

If the tree is dirty, run the `commit` workflow (logical commits, Conventional Commit messages) so nothing intended is
left behind. Works the same inside a worktree — just confirm `git branch --show-current` is the feature branch.

### 2. Pre-flight checks

Before writing any PR description, verify:

1. **Not on the base branch** — if `git branch --show-current` is `main`/`master`/the default branch, stop and warn;
   PRs come from feature/bugfix branches (see `../_shared/references/branching.md` for how to tell them apart).
2. **Commits exist ahead of base** — `git log --oneline <base>..HEAD`; if empty, there's nothing to PR.
3. **Full quality gate** — run the project gate (`../_shared/references/quality-gate.md`): lint → test → build or the
   repo equivalent, in order, stop on failure. Report which step failed. The user may proceed anyway for a draft PR.
   Redirect each step to its own log: one line per passing step, and on failure the step, the exit code and the tail —
   never the log itself (`../_shared/references/output-discipline.md`).

### 3. Push the branch

```bash
git push -u origin <feature-branch>     # -u on first push; plain git push afterward
```

Pushing from a worktree works normally. Never force-push unless the user asks. If the remote rejects because the branch
diverged, stop and surface it rather than force-pushing.

### 4. Gather context for the PR

Two commands here, and no more:

```bash
git log --oneline <base>..HEAD
git diff <base> --stat
```

**Commit messages are the primary source for the title and description** — the diff is a fallback for the one thing they
do not explain. **Do not load the branch diff into this session.** On a real branch it is tens of thousands of tokens
spent to produce a dozen lines of prose; on the smallest possible change it is already 100× the size of the log it would
supplement (`../_shared/references/output-discipline.md`).

### 5. Draft title + description — in a subagent

Hand the drafting to a subagent, which reads what it needs and returns only the draft. Spawn it **in the same message as
step 6's reviewer lookup** — both are read-only and independent.

Its brief carries: the base and branch names, the `git log --oneline <base>..HEAD` output, the `--stat`, the format spec
below, and its return budget. It runs the full diff itself
(`git diff <base> -- . ':(exclude)*.lock' ':(exclude)*.snap'`) and **returns at most 25 lines**: the title, the
description, and any note about what it could not explain from the commits. No diff, no file contents, no narration.

**Draft inline instead when the branch is small** — roughly ≤5 files or ≤100 changed lines, where the log plus the
`--stat` already tell the whole story and a round trip costs more than it saves.

Then review the draft here, and fix it if it drifts from the format — **this session owns the wording**, since it is what
step 7 shows the user and step 8 puts on the PR.

**Title**: imperative, verb-first (`Add`, `Fix`, `Update`, `Remove`, `Refactor`), ~≤ 70 chars, optional `[area]` prefix.
Derive from the highest-impact commit. Not a commit-format string (`feat(web): …` is a commit subject, not a PR title).

**Description**:

```
<one-line summary of what this PR does and why>

## Changes

- <bullet per logical change>

## Notes (optional)

<trade-offs, follow-ups, known limitations>

Closes #<issue> (if applicable)
```

Keep it honest and brief. **Never** include co-authorship lines or any mention of AI/Claude assistance.

### 6. Determine reviewers

Runs alongside step 5's draft — spawn both in one message, or do it inline here when the lookup is a `CODEOWNERS` read
and nothing more. "Assign reviewers if needed" — figure out who, without hardcoding handles:

1. Check for a `CODEOWNERS` file (`.github/`, repo root, or `docs/`) — owners of the changed paths are the natural
   reviewers.
2. Check repo/team conventions the user or docs mention; check whether a default reviewer/team is configured
   (`gh repo view`, org settings).
3. If the user named reviewers, use those.
4. If none can be determined and the change clearly needs review, ask the user (offer to skip for a draft).

### 7. Preview before creating

Show the full PR and confirm (unless the user already said "just create it"):

```
Title:     <title>
Base:      <base> ← <feature-branch>
Draft:     <yes/no>
Reviewers: <handles or "none">

<description>
```

### 8. Create the PR

Write the body to a temp file to avoid shell-escaping issues (use the session scratch dir if available):

```bash
gh pr create \
  --title "<title>" \
  --body-file <path/to/pr-body.md> \
  --base <base> \
  [--draft] \
  [--reviewer <handle> --reviewer <handle>]
```

Then remove the temp file.

### 9. Post-creation

- Add reviewers/labels after the fact if not set at creation:
  `gh pr edit <url> --add-reviewer <handle> --add-label <label>`.
- Mention CI will run automatically if applicable.

## Final report (always, at the end)

Always close with a full report, even if step 1 made no new commits:

```
PR:        <url>
Base:      <base> ← <feature-branch>
Draft:     <yes/no>
Reviewers: <handles or "none">

Title: <title>

<full PR description>

Commits (<base>..HEAD):
<short-sha>  <subject>
<short-sha>  <subject>
...
```

Get the commit list from `git log --oneline <base>..HEAD` (from step 4 — it is already in context, and it is cheap
precisely because the diff never was) — don't just say "N commits pushed."

## Common failure scenarios

| Situation                            | What to do                                                              |
| ------------------------------------ | ----------------------------------------------------------------------- |
| Branch has no commits ahead of base  | Stop — nothing to PR. Check the user is on the right branch.            |
| Branch not pushed / push rejected    | Push with `-u` first; if diverged, surface it — do not force-push.      |
| PR already exists for this branch    | `gh pr view` and show the existing PR URL.                             |
| Lint/test/build fails                | Report which step; user decides fix vs proceed-as-draft.               |
| On the base branch                   | Stop — instruct the user to create a feature branch first.             |
| No reviewers determinable            | Ask, or skip for a draft.                                              |

## Git safety

Follow `../_shared/references/git-safety.md`: never push to the base branch directly, never force-push without an
explicit request, never skip hooks unless asked, never add AI attribution to the PR.
