---
name: pr
description: >-
  Commit, push, and open a GitHub PR with description and reviewers. Trigger on "create/open/make a PR",
  "submit for review", "push a branch for merge". Remote path — local merge is finish.
model: sonnet
---

# Create a Pull Request (commit → push → PR → reviewers)

The review path. Goal: a PR from a feature branch (never the base), all intended work committed and pushed,
the gate passed, a title and description saying **what** and **why**, reviewers requested where expected, no
AI attribution anywhere. A local merge without review is `finish`.

Confirm if unclear: **base** (default `main`; another open feature branch → stacked, see the end), **draft**
(work not ready for review), an **`[area]` title prefix** when scoped to one package.

## Workflow

**0. One read-only call**, once the base is settled. The path is relative to this skill's directory: resolve
it and run it absolute. A key or exit not named here: `../_shared/references/output-discipline.md`.

```bash
bash ../_shared/scripts/facts.sh pr --run --base <base> --gh
```
PowerShell (Windows): run it as `../_shared/references/powershell.md` says.

Keep `run_dir=` as `<run-dir>` (no shell variable survives; re-running `--run` opens a second directory),
`toplevel=` as `<toplevel>`, and `command -v git` as `<git>` for calls whose output you parse
(`../_shared/references/git-safety.md`). It returns the branch, status, `commits:` and `base_stat` for
`<base>..HEAD`, `codeowners=`, `pr=` and the remote. **Remote:** with `remote_choice=no` and no
`docs/agents/remote.md`, use `remote=`; otherwise follow `../_shared/references/remote.md` (re-run without
`--run`, with `--remote <name>` from that file, or from asking — only on `remote_choice=ask`).

### 1. Commit remaining work

Dirty tree → do `commit`'s work inline (`../commit/SKILL.md`) so nothing intended is left behind.

### 2. Pre-flight

1. **Not on the base.** `branch=` equal to `default_branch=` (or `main`/`master`) → stop: PRs come from a
   feature branch.
2. **Commits ahead of base.** If step 1 committed, re-count (`git rev-list --count <base>..HEAD`) — step 0's
   `commits_ahead_of_base` predates it. Zero → nothing to PR; stop.
3. **Existing PR.** `pr=` is a URL only when one exists: show it and stop. `none` → proceed. `gh-missing`,
   `gh-unauthenticated`, `no-remote`, `gh-error` are blocked states: surface them (with the `notes:` entry). Test for a URL, not non-emptiness.
4. **Full quality gate** — `../_shared/references/quality-gate.md`: find the commands, run each to a log in
   `<run-dir>`, stop at the first failure, one line per step. A failure is the user's call: fix, or proceed
   as a draft. Skip only under its one reuse rule, and say so.

### 3. Push

```bash
git push -u <remote> <feature-branch>
```

`<remote>` is the settled `remote=`, never a hardcoded `origin`. `-u` on first push. Diverged → stop and
surface it; never force-push.

### 4. Context

**Nothing to run**: `commits:` and `base_stat` came from step 0 (re-read the log only if step 1 committed).
**Commit messages are the source** for title and description. Never load the branch diff here.

### 5. Draft title + description

**Small branch** (~≤5 files or ~≤100 changed lines) → draft inline from the log and stat. Otherwise a
subagent, spawned **in the same message as step 6's lookup**: its brief carries base and branch, the
`"<git>" -C "<toplevel>" log --oneline <base>..HEAD` output, the stat, the format below and its budget. It
reads `git diff <base> -- . ":(exclude)*.lock" ":(exclude)*.snap"` itself and **returns at most 25 lines**:
title, description, anything the commits did not explain. This session owns the final wording.

**Title**: imperative, verb-first, ~≤70 chars, optional `[area]` prefix, from the highest-impact commit. Not
a commit subject (`feat(web): …`).

**Description**:

```
<one-line summary of what this PR does and why>

## Changes

- <bullet per logical change>

## Notes (optional)

<trade-offs, follow-ups, known limitations>

Closes #<issue> (if applicable)
```

Honest and brief. Never co-authorship lines or any mention of AI assistance.

### 6. Reviewers

Never hardcode handles. In order:

0. **`docs/agents/merge.md` names reviewers** → exactly those; skip `CODEOWNERS` (no cross-check). One that
   names none, or none usable → say so in one line and go on.
1. `CODEOWNERS` (`codeowners=`): owners of the changed paths.
2. Repo/team conventions in docs or from the user; a configured default
   (`gh repo view <remote_repo>`, the repo argument only when `remote_repo=` is not `none`).
3. Reviewers the user named.
4. None determinable and the change needs review → ask (offer to skip for a draft).

### 7. Preview

Show and confirm (unless the user said "just create it"):

```
Title:     <title>
Base:      <base> ← <feature-branch>
Remote:    <remote> (<from docs/agents/remote.md | origin | chosen by you | fallback>)  [several remotes only]
Draft:     <yes/no>
Reviewers: <handles or "none"> (<from docs/agents/merge.md | from CODEOWNERS | repo default | named by you | asked>)

<description>
```

### 8. Create

Write the body to `<run-dir>/pr-body.md`, then one command:

```bash
gh pr create --repo <remote_repo> --head <feature-branch> --base <base> --title "<title>" --body-file "<run-dir>/pr-body.md" [--draft] [--reviewer <handle>]...
```

Drop `--repo`/`--head` only when `remote_repo=none`. Reviewers or labels not set at creation:
`gh pr edit <url> --add-reviewer <handle> --add-label <label>`.

## Stacked PRs

Only when the base is another open feature branch or the base PR is stacked
(`../_shared/references/stacked-prs.md`, "Detect"); never ask about stacks otherwise.

- Gate and pre-flight run on this layer only. The base branch must already be pushed; never force-push a
  lower layer.
- Preview adds `Stack: #<n> of <size>, on top of #<below>`, and says merging it later merges everything below.
- Create needs `gh stack` (absent → stop with its install line): `gh pr create --base <lower-branch>`, then
  `gh stack link` (or `gh stack submit`). A failed link leaves a plain PR on a non-default base: say so.
- Report the stack number and position. Merging is `finish`.

## Final report (always)

Bold verdict (`**Verdict:** PR opened, ready for review, 2 reviewers requested.`), form per
`../_shared/references/summary-format.md`, then:

```
PR:        <url>
Base:      <base> ← <feature-branch>
Draft:     <yes/no>
Reviewers: <handles or "none"> (<where they came from>)

Title: <title>

<full PR description>

Commits (<base>..HEAD):
<short-sha>  <subject>
...
```

Never just "N commits pushed". End with what the user still has to do, or "nothing needed from you": a remote
chosen but not recorded (a `docs/agents/remote.md` would settle it), CI running, a reviewer not determined, a
gate skipped under the reuse rule, a `docs/agents/*.md` that did not say what it seemed to mean.

Follow `../_shared/references/git-safety.md`: never push to the base, never force-push unasked, never skip
hooks, never add AI attribution.
