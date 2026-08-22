---
name: commit
description: >-
  Stage and split local changes into logical Conventional Commits. Trigger on "commit", "make a commit",
  "split into commits", or what the commit message should be. Commits only — merge is finish, PR
  is pr.
model: sonnet
---

# Commit work

Part of the **mkit** bundle (`commit` · `finish` · `pr` · `review`); shared references
live in `../_shared/references/`. This skill only commits — merge-and-cleanup is `finish`, a PR is
`pr`.

Keep output bounded throughout. For this skill that is three rules, restated below where they apply:
**`--stat` before any diff**, **gate output goes through `gate-run.sh` — report pass/fail, not the log**,
**never truncate a staged diff you are about to approve**. `../_shared/references/output-discipline.md` has
the reasoning; read it only for a case not covered here. `commit` is the front-end both finishers call, so
what it loads, they load.

## Goal

Commits that are easy to review and safe to ship:

- only intended changes included
- logically scoped (split when needed)
- messages saying what changed and why

## Ask if missing

- One commit or several? With a usable `journal:` block, **propose** instead of asking — *"3 units
  recorded; I'd make 2 commits — ok?"*. Without one, ask; unsure → several small ones when unrelated.
- Conventional Commits are required.
- Any repo rules: max subject length, required scopes.

## Workflow

**Step 0 — one call for everything read-only**, which also opens this run's directory
(`../_shared/references/output-discipline.md`):

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh commit --journal
```

Keep the `run=` and `refs=` literals it prints; this file writes the first as `<run-dir>`. There is no
`$RUN_DIR` — a shell variable does not survive to the next Bash call — and re-running the script opens a
second directory instead of returning the first.

1. **Inspect before staging** — the call above returned all of it.
   - Confirm `branch=` is the intended one; matters inside a worktree (`linked=yes`).
   - Read **both** stats: `unstaged_stat` and `staged_stat`. They are separate because a bare
     `git diff --stat` reports nothing when the work is already fully staged, which reads exactly like a
     clean tree.
2. **Read the `journal:` block** — recorded intent, which decides how much diff you read
   (`../_shared/references/journal.md`). *The read*, wherever it still applies: the full diff per file, only
   for files you must judge — never the whole tree at once.
   - `journal_uncovered=0` **and** every entry `fresh` → the entries *are* step 3's input; **no read at all**.
   - Any `uncovered:` path, or any `drifted` / `unknown-head` entry → read, **scoped to those paths only**.
   - `journal=off` / `empty` / `jq-missing` / `unreadable` → nothing recorded; read as today. Large mixed
     tree: stop after the `--stat` and let step 3 delegate.
   - `committed` → drop; `orphaned` → drop and name in the final report. `overlap:` (a path two `fresh`
     entries both claim) → patch staging in step 4: the script named the path, **you decide the hunks**.
3. **Decide commit boundaries.**
   - Split by: feature vs refactor, backend vs frontend, formatting vs logic, tests vs prod code, dependency
     bumps vs behavior changes.
   - Changes mixed within one file → plan patch staging.
   - Journal entries arrive in dependency order; keep it, but merge freely — tests with the feature they
     cover.
   - **Large mixed uncovered tree (>~10 files or >~400 changed lines): delegate the read**
     (`../_shared/references/agent-delegation.md`). The subagent does not have this skill loaded, so its brief
     carries the branch, the stats, the file list and the split heuristics above — all of which step 0
     returned. It reads the diff and
     **returns a commit plan and nothing else** — per proposed commit: type and scope, one-line subject, the
     paths it covers, one line of rationale, plus any file needing patch staging because it is mixed. Under
     ~20 lines; no diff, no file contents.
   - Below that, read it here: the `--stat` plus a few targeted per-file diffs costs less than the round trip.
   - **A plan is a proposal, not a decision** — a subagent's and the journal's alike. Sanity-check it against
     the `--stat` (every changed path in exactly one group, nothing invented), then do the staging, wording
     and splits here — the rationale is what lets you answer "why is X with Y?" without re-reading the diff.
4. **Stage only what belongs in the next commit.** Prefer `git add -p` for mixed changes; unstage with
   `git restore --staged -p` or `git restore --staged <path>`.
5. **Review what will actually be committed.** `git diff --cached --stat` to plan, then
   `git diff --cached -- <path>` **file by file, skipping none** — the one read that must not be truncated,
   since the checks below only work on the actual hunks (`../_shared/references/output-discipline.md`). Check
   for: secrets or tokens, accidental debug logging, unrelated formatting churn. **Not skippable, however
   fresh the journal looks** — a stale entry that wrote a plausible-but-wrong message into permanent history
   is a worse failure than the tokens the read costs.
6. **Describe the staged change in 1–2 sentences** before writing the message: what changed, why. If you
   cannot describe it cleanly the commit is too big or mixed — back to step 3.
7. **Write the message.** Conventional Commits required (`../_shared/references/conventional-commits.md`).
   Multi-line: `git commit -v` or `git commit -F <file>`. The subject is authored **here, against the staged
   hunks** — an entry's `type`/`scope`/`subject` is only a proposal, and the final subject must match what was
   actually staged; body `why` lines come from the entries.
8. **Run the smallest relevant verification** — the repo's fastest meaningful check
   (`../_shared/references/quality-gate.md`, fast tier):

   ```bash
   ${CLAUDE_PLUGIN_ROOT}/scripts/gate-detect.sh                       # once per run: what to run
   ${CLAUDE_PLUGIN_ROOT}/scripts/gate-run.sh <run-dir> fast -- <fast command>
   ```

   Report pass/fail and, on failure, the step and its exit code — never the log. The verdict usually *is* the
   diagnosis; delegate only when it is not ("when a step fails").
9. **Repeat** until the working tree is clean.

## Final report (always)

On the way out, folded into another call: `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh --prune`, plus
`${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh drop --committed` if entries were spent.
After the last commit run `git log --oneline -n <N>` (N = commits made this run) to confirm hashes, then
report every commit — never skip this, even for one:

```
<short-sha>  <type>(<scope>): <summary>
  <what/why, 1 sentence>
```

One block per commit, in order. Say so if staged changes were deliberately left out. With a journal, one more
line: how many entries were consumed vs left, naming any dropped as `orphaned`.

## Conventional Commit format

```text
<type>(<scope>): <summary>

<What changed.>
<Why it changed.>
```

Summary imperative and specific. Type table and scope detection:
`../_shared/references/conventional-commits.md`.

## Git safety

Follow `../_shared/references/git-safety.md`: never update git config, never skip hooks unless asked, never
force-push, never add AI co-authorship, and if a hook fails, fix it and make a NEW commit.
