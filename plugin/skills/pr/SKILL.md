---
name: pr
description: >-
  Commit, push, and open a GitHub PR with description and reviewers. Trigger on "create/open/make a PR",
  "submit for review", "push a branch for merge". Remote path — local merge is finish.
model: sonnet
---

# Create a Pull Request (commit → push → PR → reviewers)

Part of the **mkit** bundle. The review path: commit, push, open the PR, request reviewers. For a local merge
with no review use `finish`.

References: `../_shared/references/conventional-commits.md`, `../_shared/references/quality-gate.md`,
`../_shared/references/worktree.md`, `../_shared/references/git-safety.md`,
`../_shared/references/branching.md`, `../_shared/references/remote.md`,
`../_shared/references/output-discipline.md`,
`../_shared/references/agent-delegation.md`, `../_shared/references/workflow-contract.md`,
`../_shared/references/stacked-prs.md` (only for a stacked PR).

## Goal

A PR that is easy to review and safe to merge:

- from a feature/bugfix branch, never the base
- all intended work committed and pushed
- pre-flight checks pass before the PR is created
- title and description say **what** changed and **why**
- reviewers requested when the repo/team expects them
- no Claude/AI co-authorship attribution anywhere

## Confirm if missing

- **Base branch** — default `main`; ask if unclear. If it is another unmerged feature branch (this
  branch is a layer on top of it), this is a **stacked PR** — see "Stacked PRs" below.
- **Draft?** — draft if the work isn't ready for review.
- **Area prefix** — scoped to one package → prefix the title `[area]` (`[api]`, `[ui]`).
- **Reviewers** — step 6: named in `docs/agents/merge.md`, from `CODEOWNERS`, or ask.
- **Remote** — only when step 0 says `remote_choice=ask` (several remotes, none named in
  `docs/agents/remote.md`): ask which one, default `remote=`. `../_shared/references/remote.md` has the whole
  rule.

## Workflow

**Step 0 — one read-only call.** Once the base is settled, it opens the run directory and returns every
read-only fact this skill needs (`../_shared/references/output-discipline.md`). The same output serves step
2's pre-flight, step 4's context and the final report, and none of it changes when step 3 pushes:

```bash
bash ../_shared/scripts/facts.sh pr --run --base <base> --gh
```

The path is relative to this skill's directory (`../_shared/references/output-discipline.md` says how to
resolve it). That covers the branch, the status, `commits:` for `<base>..HEAD`, the stat, `codeowners=` for
step 6, and `pr=` — whether this branch already has one, which is the check that otherwise gets skipped — and
the remote: `remote=`, `remote_source=`, `remote_repo=`, `remotes=`, `remote_choice=`. When `remote.md` says
to (a remote named in `docs/agents/remote.md`, or `remote_choice=ask` and the user's answer), re-run this
call without `--run` and with `--remote <name>` added — `pr=` and `default_branch=` above describe the
fallback remote until you do. Keep the `run_dir=` literal; this file writes it as `<run-dir>`, its
`toplevel=` as `<toplevel>`, and the absolute git binary as `<git>` (`../_shared/references/git-safety.md`).
There is no `$RUN_DIR` (a shell variable does not survive to the next Bash call), and re-running with
`--run` opens a second directory instead of returning the first.

Repo settings this skill reads, each only if the file exists (`../_shared/references/repo-settings.md`):
`docs/agents/remote.md` (now, per `remote.md`), `docs/agents/gate.md` (step 2) and the reviewers in
`docs/agents/merge.md` (step 6). A missing file is never mentioned and never stops the run.

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
   `gh-unauthenticated`, `no-remote`. Test for a URL, not for non-emptiness — every
   sentinel is a non-empty string, so a "non-empty" reading always fires and shows `none` as if it
   were a PR. Only `none` means proceed; the rest are blocked states to surface, not to push past.
4. **Full quality gate** (`../_shared/references/quality-gate.md`), in order, stopping at the first failure:

   Find the commands with the reference's table ("Find the gate"); run each yourself, its output to a log in the run directory.
   One line per passing step; on failure the step, the exit code, the failure lines and the tail — never
   the log. The user may proceed anyway for a draft. Delegate the diagnosis only when that verdict is not
   enough ("when a step fails") — choosing between fixing and opening a draft needs a cause, not a transcript.
   Skip the gate only under the reference's one reuse rule, and say so.

### 3. Push the branch

```bash
git push -u <remote> <feature-branch>   # -u on first push; plain git push afterward
```

`<remote>` is step 0's `remote=` — or the user's answer when `remote_choice=ask` — never a hardcoded
`origin` (`../_shared/references/remote.md`).

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

Its brief carries: base and branch names, the `"<git>" -C "<toplevel>" log --oneline <base>..HEAD`
output, the `--stat`, the format spec below, and its return budget. It runs the full diff itself
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

0. **Reviewers named in `docs/agents/merge.md`** — the repo already answered this. Use that list and
   **skip 1 entirely**; do not read `CODEOWNERS` to cross-check it — that would be asking a question settled
   in a committed file. Say where the list came from in the preview and the report (rule 3). A `merge.md`
   with no reviewers in it is no answer: go on to 1.
1. `CODEOWNERS` — step 0 reported `codeowners=<path>` or `none`; owners of the changed paths are the natural
   reviewers.
2. Repo/team conventions the user or docs mention; a configured default reviewer/team (`gh repo view` — with `<remote_repo>` unless `remote_repo=none`; org
   settings).
3. Reviewers the user named.
4. None determinable and the change clearly needs review → ask (offer to skip for a draft).

### 7. Preview before creating

Show the full PR and confirm (unless the user already said "just create it"):

```
Title:     <title>
Base:      <base> ← <feature-branch>
Remote:    <remote> (<named in docs/agents/remote.md | origin | chosen by you | fallback>)  [only with several remotes]
Draft:     <yes/no>
Reviewers: <handles or "none"> (<from docs/agents/merge.md | from CODEOWNERS | repo/org default |
           named by you | asked>)

<description>
```

The parenthesis is rule 3 in its shortest form: where the handles came from, so a wrong list is fixed in
the right place. Drop it only where there are no reviewers at all.

### 8. Create the PR

Body to a temp file, to avoid shell-escaping issues (`<run-dir>` if there is one, else `$TMPDIR`):

```bash
gh pr create \
  --title "<title>" \
  --body-file <path/to/pr-body.md> \
  --base <base> \
  [--draft] \
  [--reviewer <handle> --reviewer <handle>]
```

Name the repo: `--repo <remote_repo> --head <feature-branch>` whenever `remote_repo=` is not `none` —
without it `gh` can pick another repo than the one just pushed to, without refusing
(`../_shared/references/remote.md`, "Using it"). The same goes for `gh repo view` in step 6 (a repo argument only when `remote_repo=` is not `none`).

Then remove the temp file.

### 9. Post-creation

- Add reviewers/labels afterwards if not set at creation:
  `gh pr edit <url> --add-reviewer <handle> --add-label <label>`.
- Mention CI will run automatically if applicable.

## Stacked PRs

GitHub-native stacks (`../_shared/references/stacked-prs.md`). Applies only when the base is another open
feature branch, or the base PR's `.stack` is non-null (`stacked-prs.md`, "Detect").
Otherwise skip this section; never ask about stacks on an ordinary PR.

- **Step 2** — the gate and pre-flight are unchanged; run them on this layer only.
- **Step 3** — the base branch must already be pushed (GitHub rejects a PR against a missing base). Never
  force-push a lower layer to "fix" the stack.
- **Step 7** — add a `Stack:` line to the preview: position, size, and the PR below
  (`#<n> of <size>, on top of #<below>`). Say that merging this PR later also merges everything below it.
- **Step 8** — needs the `gh stack` extension (`gh extension list`); absent → stop with its install line
  (`../_shared/references/stacked-prs.md`). Open this PR with `gh pr create --base <lower-branch>` as
  usual, then `gh stack link` (or `gh stack submit` when the stack is tracked locally) to put it in the
  stack. Same-repo only. A failed link leaves a plain PR on a non-default base: say so — it reviews fine,
  it just will not merge as a unit.
- **Report** — name the stack number and position; the commit list is still `<base>..HEAD` of this layer.
  Merge is `finish` (`gh stack merge`, not `gh pr merge`).

## Final report (always)

Close with a full report, even if step 1 made no new commits. Open with a one-line bold verdict
(`**Verdict:** PR opened, ready for review, 2 reviewers requested.`), then the block below; form per
`../_shared/references/summary-format.md`:

```
PR:        <url>
Base:      <base> ← <feature-branch>
Draft:     <yes/no>
Reviewers: <handles or "none"> (<where they came from>)

Title: <title>

<full PR description>

Commits (<base>..HEAD):
<short-sha>  <subject>
<short-sha>  <subject>
...
```

The commit list is step 0's `commits:` block — already in context, and cheap precisely because the diff never
was. Never just "N commits pushed."

After the block, say what the reader still has to do or decide, or "nothing needed from you": a remote chosen
but not recorded (a `docs/agents/remote.md` naming it would end the question),
CI still running, a reviewer that could not be determined, a gate skipped under the reuse rule, a
`docs/agents/*.md` file that did not say what it seemed to mean. Never leave those implied by the block's fields alone.

## Common failure scenarios

| Situation                           | What to do                                                        |
| ----------------------------------- | ----------------------------------------------------------------- |
| No commits ahead of base            | Stop — nothing to PR. Check the user is on the right branch.      |
| Branch not pushed / push rejected   | Push with `-u`; if diverged, surface it — do not force-push.      |
| `remote=none`                       | Stop — nothing to push to; say to add a remote.                   |
| Several remotes, none named         | Ask which (`remote_choice=ask`), then re-run facts with `--remote`. |
| PR already exists for this branch   | step 0's `pr=` holds a URL — show it (`none`/`gh-missing` are not). |
| Lint/test/build fails               | Report which step; user decides fix vs proceed-as-draft.          |
| On the base branch                  | Stop — tell the user to create a feature branch first.            |
| No reviewers determinable           | Ask, or skip for a draft.                                         |
| `merge.md` names no usable reviewer | Say so in one line, then fall back to `CODEOWNERS`.               |

## Git safety

Follow `../_shared/references/git-safety.md`: never push to the base branch directly, never force-push without
an explicit request, never skip hooks unless asked, never add AI attribution to the PR.
