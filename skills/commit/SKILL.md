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

Keep command output bounded throughout, which for this skill is three rules, stated again where they apply below:
**`--stat` before any diff**, **gate output goes to a log — report pass/fail, not the log**, and **never truncate a
staged diff you are about to approve**. `../_shared/references/output-discipline.md` has the reasoning and the rest of
the pattern; read it only if a case here is not covered. `commit` is the front-end both finishers call, so what it
loads they load too.

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

**Before step 1 — open the run directory.** Every log this skill writes lives in it
(`../_shared/references/output-discipline.md`). Nothing in step 1 depends on it, so issue both in **one call** instead
of spending a turn on each:

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh commit
git branch --show-current && git status --short && git diff --stat && git diff --cached --stat
```

It prints one absolute path. **Reuse that literal**; this file writes it as `<run-dir>`. There is no
`$RUN_DIR` — a shell variable does not survive to the next Bash call — and re-running the script opens a
second directory rather than returning the first.

1. Inspect the working tree before staging — the call above already returned all of it
   - Confirm you are on the branch you intend — matters when running inside a worktree.
   - Read both `--stat`s: unstaged **and** staged. A bare `git diff --stat` reports nothing when the work is already
     fully staged, which reads exactly like a clean tree.
   - Then the full diff per file for the files you actually have to judge — never the whole tree at once
     (`../_shared/references/output-discipline.md`).
   - On a large mixed tree, stop after the `--stat` and let step 2 delegate the read instead.
2. Decide commit boundaries (split if needed)
   - Split by: feature vs refactor, backend vs frontend, formatting vs logic, tests vs prod code, dependency bumps vs
     behavior changes.
   - If changes are mixed in one file, plan to use patch staging.
   - **On a large mixed tree — roughly >10 files or >400 changed lines — delegate the read**
     (`../_shared/references/agent-delegation.md`). Hand a subagent the branch, the `--stat`, the file list and the
     split heuristics above (it does not have this skill loaded, so put them in the brief), and have it read the diff
     and **return a commit plan and nothing else** — for each proposed commit: the type and scope, a one-line subject,
     the paths it covers, one line of rationale, and a note on any file that has to be patch-staged because it is
     mixed. Under ~20 lines total; no diff, no file contents.
   - Below that threshold, read it here — the `--stat` plus a few targeted per-file diffs costs less than the round
     trip.
   - **The plan is a proposal, not a decision.** Sanity-check it against the `--stat` (every changed path is in exactly
     one group, nothing invented), then do the staging, the message wording and the splits in this session — the
     rationale is what lets you answer "why is X with Y?" without re-reading the diff.
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
