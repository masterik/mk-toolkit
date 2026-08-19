---
name: commit
description: >-
  Git commit workflow: inspect tree, stage intended changes, split into logical Conventional Commits. Trigger on
  "commit", "make a commit", "split into commits", or asking what the commit message should be. Commits only — merge with
  finish-feature, PR with create-pr.
model: sonnet
---

# Commit work

Part of the **mkit** workflow bundle (`commit` · `finish-feature` · `create-pr` · `review-changes`). Shared references live in
`../_shared/references/`. This skill only commits — for merge-and-cleanup use `finish-feature`, for a PR use `create-pr`.

Keep command output bounded throughout — `../_shared/references/output-discipline.md`. It is the one reference to read
before step 1, because `commit` is the front-end both finishers call, so what it loads they load too.

## Goal

Make commits that are easy to review and safe to ship:

- only intended changes are included
- commits are logically scoped (split when needed)
- commit messages describe what changed and why

## Inputs to ask for (if missing)

- Single commit or multiple commits? (If unsure: default to multiple small commits when there are unrelated changes.)
- Commit style: Conventional Commits are required.
- Any rules: max subject length, required scopes.

## Workflow (checklist)

1. Inspect the working tree before staging
   - `git status`
   - Confirm you are on the branch you intend (`git branch --show-current`) — matters when running inside a worktree.
   - **`git diff --stat` first**, then the full diff per file for the files you actually have to judge — never the whole
     tree at once (`../_shared/references/output-discipline.md`).
   - On a large mixed tree (roughly >10 files or >400 changed lines), delegate the read: hand a subagent the file list
     and have it return a **commit plan** — groups of paths with one line of rationale each — then do the staging here.
     Below that threshold the round trip costs more than it saves.
2. Decide commit boundaries (split if needed)
   - Split by: feature vs refactor, backend vs frontend, formatting vs logic, tests vs prod code, dependency bumps vs
     behavior changes.
   - If changes are mixed in one file, plan to use patch staging.
3. Stage only what belongs in the next commit
   - Prefer patch staging for mixed changes: `git add -p`
   - To unstage a hunk/file: `git restore --staged -p` or `git restore --staged <path>`
4. Review what will actually be committed
   - `git diff --cached --stat` to plan, then `git diff --cached -- <path>` **file by file, skipping none**. This is the
     one read that must not be truncated — the checks below only work on the actual hunks
     (`../_shared/references/output-discipline.md`, "what must never be capped").
   - Sanity checks:
     - no secrets or tokens
     - no accidental debug logging
     - no unrelated formatting churn
5. Describe the staged change in 1-2 sentences (before writing the message)
   - "What changed?" + "Why?"
   - If you cannot describe it cleanly, the commit is probably too big or mixed; go back to step 2.
6. Write the commit message
   - Use Conventional Commits (required) — format and type/scope guidance: `../_shared/references/conventional-commits.md`.
   - Prefer an editor or a message file for multi-line messages: `git commit -v` or `git commit -F <file>`.
7. Run the smallest relevant verification
   - Run the repo's fastest meaningful check (see `../_shared/references/quality-gate.md` — the "fast check" tier)
     before moving on.
   - Redirect its output to a log and report pass/fail plus the failing step only — not the log
     (`../_shared/references/output-discipline.md`). On a long failure log, triage it per
     `../_shared/references/quality-gate.md` ("when a step fails") rather than reading it here.
8. Repeat for the next commit until the working tree is clean

## Final report (always, at the end)

After the last commit, run `git log --oneline -n <N>` (N = commits made this run) to confirm exact hashes, then report
every commit created this run — never skip this, even for a single commit:

```
<short-sha>  <type>(<scope>): <summary>
  <what/why, 1 sentence>
```

One block per commit, in order. If any staged changes were intentionally left out (e.g. deferred to a later commit),
say so.

## Conventional Commit Format

```text
<type>(<scope>): <summary>

<What changed.>
<Why it changed.>
```

Keep the summary imperative and specific. Full type table and scope-detection guidance:
`../_shared/references/conventional-commits.md`.

## Git safety

Follow the git safety protocol: `../_shared/references/git-safety.md`. In short — never update git config, never skip
hooks unless asked, never force-push, never add AI co-authorship, and if a hook fails, fix it and make a NEW commit.
