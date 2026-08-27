---
name: pr
description: >-
  Commit, push, and open a GitHub PR with description and reviewers. Trigger on "create a PR", "open a PR",
  "make a PR", "submit for review", "push a branch for merge". Remote review path — local merge is
  finish.
model: sonnet
---

# Create a Pull Request (commit → push → PR → reviewers)

Part of the **mkit** bundle. The review path: commit, push, open the PR, request reviewers. For a local merge
with no review use `finish`.

References: `../_shared/references/conventional-commits.md`, `../_shared/references/quality-gate.md`,
`../_shared/references/worktree.md`, `../_shared/references/git-safety.md`,
`../_shared/references/branching.md`, `../_shared/references/output-discipline.md`,
`../_shared/references/agent-delegation.md`.

## Goal

A PR that is easy to review and safe to merge:

- from a feature/bugfix branch, never the base
- all intended work committed and pushed
- pre-flight checks pass before the PR is created
- title and description say **what** changed and **why**
- reviewers requested when the repo/team expects them
- no Claude/AI co-authorship attribution anywhere

## Confirm if missing

- **Base branch** — default `main`; ask if unclear.
- **Draft?** — draft if the work isn't ready for review.
- **Area prefix** — scoped to one package → prefix the title `[area]` (`[api]`, `[ui]`).
- **Reviewers** — step 6: from repo config, or ask.

## Workflow

**Step 0 — one call.** Once the base is settled, it opens the run directory and returns every read-only fact
this skill needs (`../_shared/references/output-discipline.md`). The same outputs serve step 2's pre-flight,
step 4's context and the final report, and none change when step 3 pushes:

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh pr --base <base> --gh
```

That covers the branch, the status, `commits:` for `<base>..HEAD`, the stat, `codeowners=` for step 6, and
`pr=` — whether this branch already has one, which is the check that otherwise gets skipped. Keep the `run=`
literal; this file writes it as `<run-dir>`. There is no `$RUN_DIR` (a shell variable does not survive to the
next Bash call), and re-running the script opens a second directory instead of returning the first.

### 1. Commit remaining work

Dirty tree → run `commit` (logical commits, Conventional Commit messages) so nothing intended is left behind.
Same inside a worktree — just confirm `git branch --show-current` is the feature branch.

### 2. Pre-flight checks

1. **Not on the base branch.** `branch=` equal to `default_branch=` (or `main`/`master`): stop and warn. PRs
   come from feature/bugfix branches (`../_shared/references/branching.md`).
2. **Commits exist ahead of base** — `commits_ahead_of_base=0` means nothing to PR. Step 0's count
   predates step 1, so re-count after committing (`git rev-list --count <base>..HEAD`) before you
   trust a zero: a branch whose work was all uncommitted reads 0 in the step 0 snapshot.
3. **Existing PR** — `pr=` is a URL only when one exists. Otherwise it is a sentinel naming why
   there is none: `none` (no PR yet — the case that justifies creating one), `gh-missing`,
   `jq-missing`, `gh-unauthenticated`, `no-remote`. Test for a URL, not for non-emptiness — every
   sentinel is a non-empty string, so a "non-empty" reading always fires and shows `none` as if it
   were a PR. Only `none` means proceed; the rest are blocked states to surface, not to push past.
4. **Full quality gate** (`../_shared/references/quality-gate.md`), in order, stopping at the first failure:

   ```bash
   ${CLAUDE_PLUGIN_ROOT}/scripts/gate-detect.sh
   ${CLAUDE_PLUGIN_ROOT}/scripts/gate-run.sh <run-dir> --chain 'lint=<cmd>' 'test=<cmd>' 'build=<cmd>'
   ```

   One line per passing step; on failure the step, the exit code, the grepped failures and the tail — never
   the log. The user may proceed anyway for a draft. Delegate the diagnosis only when that verdict is not
   enough ("when a step fails") — choosing between fixing and opening a draft needs a cause, not a transcript.

   `full_cache=` (from `gate-detect.sh`, pipe-parallel with `full=`) may be consumed **per step**: a draft PR
   is the recoverable case and CI runs remotely anyway. Each step served that way is `cached (Nm ago)` on its
   own line — never a pass. Never cache a whole chain in one go.

### 3. Push the branch

```bash
git push -u origin <feature-branch>     # -u on first push; plain git push afterward
```

Pushing from a worktree works normally. Never force-push unless asked. If the remote rejects because the
branch diverged, stop and surface it.

### 4. Gather context for the PR

**Nothing to run here** — `commits:` and `base_stat` came back from step 0, and pushing changed neither.
Re-running them spends a turn to reprint context.

**Commit messages are the primary source for the title and description**; the diff is a fallback for the one
thing they do not explain. **Do not load the branch diff into this session** — on a real branch it is tens of
thousands of tokens for a dozen lines of prose, and even on the smallest change it is ~100× the log it would
supplement (`../_shared/references/output-discipline.md`).

### 5. Draft title + description — in a subagent

Hand drafting to a subagent that reads what it needs and returns only the draft. Spawn it **in the same
message as step 6's reviewer lookup** — both read-only and independent.

Its brief carries: base and branch names, the `git log --oneline <base>..HEAD` output, the `--stat`, the
format spec below, and its return budget. It runs the full diff itself
(`git diff <base> -- . ':(exclude)*.lock' ':(exclude)*.snap'`) and **returns at most 25 lines**: title,
description, and any note about what it could not explain from the commits. No diff, no file contents, no
narration.

**Draft inline instead on a small branch** — ~≤5 files or ~≤100 changed lines, where log plus `--stat` tell
the whole story and the round trip costs more than it saves.

Then review the draft here and fix any drift from the format: **this session owns the wording**, since it is
what step 7 shows the user and step 8 puts on the PR.

**Title**: imperative, verb-first (`Add`, `Fix`, `Update`, `Remove`, `Refactor`), ~≤70 chars, optional
`[area]` prefix. Derive from the highest-impact commit. Not a commit-format string (`feat(web): …` is a commit
subject, not a PR title).

**Description**:

```
<one-line summary of what this PR does and why>

## Changes

- <bullet per logical change>

## Notes (optional)

<trade-offs, follow-ups, known limitations>

Closes #<issue> (if applicable)
```

Honest and brief. **Never** include co-authorship lines or any mention of AI/Claude assistance.

### 6. Determine reviewers

Runs alongside step 5 — spawn both in one message, or do it inline when the lookup is a `CODEOWNERS` read and
nothing more. Figure out who, without hardcoding handles:

1. `CODEOWNERS` — step 0 reported `codeowners=<path>` or `none`; owners of the changed paths are the natural
   reviewers.
2. Repo/team conventions the user or docs mention; a configured default reviewer/team (`gh repo view`, org
   settings).
3. Reviewers the user named.
4. None determinable and the change clearly needs review → ask (offer to skip for a draft).

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

Body to a temp file, to avoid shell-escaping issues (session scratch dir if available):

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

- Add reviewers/labels afterwards if not set at creation:
  `gh pr edit <url> --add-reviewer <handle> --add-label <label>`.
- Mention CI will run automatically if applicable.

## Final report (always)

Close with a full report, even if step 1 made no new commits:

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

The commit list is step 0's `commits:` block — already in context, and cheap precisely because the diff never
was. Never just "N commits pushed." Prune with `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh --prune` on the way out.

## Common failure scenarios

| Situation                           | What to do                                                        |
| ----------------------------------- | ----------------------------------------------------------------- |
| No commits ahead of base            | Stop — nothing to PR. Check the user is on the right branch.      |
| Branch not pushed / push rejected   | Push with `-u`; if diverged, surface it — do not force-push.      |
| PR already exists for this branch   | step 0's `pr=` holds a URL — show it (`none`/`gh-missing` are not). |
| Lint/test/build fails               | Report which step; user decides fix vs proceed-as-draft.          |
| On the base branch                  | Stop — tell the user to create a feature branch first.            |
| No reviewers determinable           | Ask, or skip for a draft.                                         |

## Git safety

Follow `../_shared/references/git-safety.md`: never push to the base branch directly, never force-push without
an explicit request, never skip hooks unless asked, never add AI attribution to the PR.
