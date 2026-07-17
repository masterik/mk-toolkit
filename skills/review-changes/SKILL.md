---
name: review-changes
description: >-
  Review your own uncommitted changes or recent commits using the CodeRabbit and Codex review tools, then fix what's
  worth fixing and report findings. Use before finishing a feature or opening a PR, or when the user says "review my
  changes", "review the diff", "review the last N commits", "code review before I commit/PR", "run codex and
  coderabbit", or "/review-changes". This reviews local work — for reviewing an existing GitHub PR, use the PR review
  tools directly.
model: sonnet
---

# Review changes (CodeRabbit + Codex, then fix)

Part of the **git-flow** bundle. Runs a two-tool review pass over local work, applies safe fixes automatically, asks
before risky ones, and hands back a findings + fixes summary. Typically run right before `commit`, `finish-feature`, or
`create-pr`.

Shared references: `../git-flow/references/quality-gate.md`, `../git-flow/references/git-safety.md`.

## 1. Establish the review scope

Decide what to review (ask only if genuinely ambiguous):

- **Uncommitted changes** (default when the tree is dirty): staged + unstaged.
  `git diff HEAD` (or `git diff` / `git diff --cached` to separate them).
- **Recent commits**: e.g. "last 3 commits" → `git diff HEAD~3..HEAD`; or everything since base → `git diff <base>..HEAD`.
- If both dirty tree and "review my commits" are in play, confirm which (or review both, noting the split).

Capture the file list (`git diff --name-only <range>`) so both reviewers and the fixer target the same surface.

## 2. Run both reviewers (in parallel)

Run the two tools independently so their findings are unbiased, then reconcile. Prefer spawning them as parallel
subagents so their verbose output stays out of the main context — collect only structured findings back.

- **CodeRabbit** — invoke the CodeRabbit review skill (`coderabbit:code-review`, or the `coderabbit:code-reviewer`
  agent). Point it at the same diff/range.
- **Codex** — invoke the Codex review path (the `codex:rescue` skill / `codex:codex-rescue` agent), prompting it for a
  code-review pass over the same diff/range: correctness bugs, risky edge cases, and quality issues.

**Availability & fallback**: if a tool/plugin isn't installed or errors out, note it in the summary and continue with
whatever is available. If neither is available, fall back to the built-in `/code-review` (or `code-review` skill) so the
user still gets a review. Never claim a tool ran if it didn't.

Ask each reviewer to return findings as a list, each with: file + line, severity (blocker / warning / nit), a one-line
description, and a concrete suggested fix.

## 3. Reconcile findings

- Merge the two lists; **dedupe** issues both tools flagged (keep the clearer wording, note "flagged by both").
- Drop clear false positives, briefly noting why.
- Rank by severity: blockers → warnings → nits.

## 4. Triage and fix — auto-fix safe, ask on risky

For each surviving finding, classify and act:

- **Safe → fix automatically**: unambiguous, low-risk, behavior-preserving. E.g. typos, missing null/error handling that
  is clearly correct, obvious off-by-one, dead code, lint/style, missing `await`, docstring/comment fixes, tightening a
  type. Apply the edit.
- **Risky → ask first**: anything that changes behavior, public API/contracts, data/migrations, concurrency, security
  posture, or where the "right" fix is a judgement call or spans a broad refactor. Present the finding + proposed fix and
  get the user's decision before editing.
- **Not worth fixing → skip**: nits the user is unlikely to want, or findings you judge incorrect. List them as
  "considered, not changed" with a one-line reason.

Group related safe fixes and apply them in coherent edits. Keep fixes minimal and in the style of the surrounding code.

## 5. Re-verify

After applying fixes, run the repo's fast check (`../git-flow/references/quality-gate.md` — fast tier: lint/typecheck or
the touched package's tests) to confirm nothing broke. Report the result. If a fix introduced a failure, revert or
correct it before summarizing.

## 6. Summarize (the deliverable)

Report, concisely:

- **Scope reviewed** — the range/diff and which tools actually ran (and any that didn't, with why).
- **Findings** — grouped by severity, deduped, noting which tool(s) raised each.
- **Fixed automatically** — what was changed and why it was safe.
- **Needs your decision** — risky findings awaiting approval, with the proposed fix.
- **Considered, not changed** — skipped nits / rejected findings, one line each.
- **Verification** — the check that ran after fixing and its result.

Do not commit as part of this skill unless asked — leave the fixes staged/unstaged for the user to commit (or chain into
`commit`).

## Git safety

Follow `../git-flow/references/git-safety.md`. Apply only fixes to the working tree; never rewrite pushed history, never
force-push, never commit or push as a side effect of the review unless explicitly asked.
