---
name: review-changes
description: >-
  Review local uncommitted changes or recent commits with three independent reviewers (CodeRabbit + Codex + Claude),
  verify the findings, fix what's worth fixing, report what's left. Trigger on "review my changes", "review the diff",
  "review last N commits", "run codex and coderabbit", or before a commit/PR. Local work only — for a GitHub PR use the
  PR review tools.
model: opus
---

# Review changes (three reviewers → verify → fix)

Part of the **mkit** workflow bundle. Runs a multi-source review over local work, merges and verifies the findings,
applies safe fixes automatically, asks before risky ones, and hands back a findings + fixes summary. Typically run right
before `commit`, `finish-feature`, or `create-pr`.

Shared references — **read the first four before running anything; they are what the reviewers are held to**:

- `../_shared/references/review-severity.md` — the severity bar, the `[surface, severity]` tag, the reviewer contract,
  what not to report, and the partial-review rule.
- `../_shared/references/review-lenses.md` — the eight lenses and which reviewer carries which.
- `../_shared/references/finding-triage.md` — reconcile, verify, and the three checks on every fix.
- `../_shared/references/agent-delegation.md` — the run directory, return budgets, and how to write a brief.
- `../_shared/references/output-discipline.md` — how to keep gate logs and diffs out of this session.
- `../_shared/references/quality-gate.md`, `../_shared/references/git-safety.md`.

The pipeline is **gate → find → reconcile → verify → fix → re-check → report**. Each stage exists to keep the next one
honest, so do not collapse them: a reviewer that also fixes anchors on its own conclusions, and a fixer that also decides
what is real never rejects anything.

## How the work is split

**Every stage that reads a lot and decides a little runs in a subagent, and hands back a summary — never its work.**
A three-reviewer review over a real diff is tens of thousands of tokens of transcript; none of it belongs in this
session, which needs only enough to put a decision to the user. `../_shared/references/agent-delegation.md` is the
technique; this is the roster.

| stage | who runs it | model | enters this session |
| --- | --- | --- | --- |
| 1 scope | this session — 3 git commands | — | range, shortstat, file count, goal |
| 2 gate | this session — Bash | — | pass/fail and the failing step only |
| 3 find | 3 subagents, parallel | Opus | 3 × ≤10 lines: path written + counts by tag + lenses not covered |
| 4 reconcile | 1 subagent, text only | Sonnet | path written + counts + what it dropped and why |
| 5 verify | 1 subagent per directory group, parallel | Opus | one line per finding: id + verdict (+ corrected fields) |
| 6 fix | this session, bodies read from disk on demand | — | the findings being acted on, in full |
| 6a sweep | 1 subagent per fix shape, read-only | Sonnet | the occurrence list |
| 7 re-check | this session — Bash | — | pass/fail |
| 8 report | this session, written from the run files | — | the summary itself |

Open the run directory in step 1 and pass its path to every subagent:

```bash
RUN_DIR="$(git rev-parse --git-dir)/mkit/review-$(date -u +%Y%m%dT%H%M%SZ)" && mkdir -p "$RUN_DIR"
```

**Never read a diff into this session.** Subagents read the diff; this session reads `--shortstat` and `--name-only`.
Never paste a reference file into a brief either — resolve its absolute path and tell the subagent to read it.

## 1. Establish the review scope

Decide what to review (ask only if genuinely ambiguous — a feature branch with uncommitted work is the standard case):

- **Uncommitted changes** (default when the tree is dirty): staged + unstaged. `git diff HEAD` (or `git diff` /
  `git diff --cached` to separate them).
- **Recent commits**: "last 3 commits" → `git diff HEAD~3..HEAD`; everything since base → `git diff <base>..HEAD`.
- If both a dirty tree and "review my commits" are in play, confirm which (or review both, noting the split).

Two commands, and no more git than this:

```bash
git branch --show-current && git status --short
git diff <range> --shortstat && git diff <range> --name-only
```

Uncommitted work on the default branch is `git diff` **plus** `git diff --staged` — a bare `git diff --shortstat`
reports nothing at all when the work is fully staged.

**Also capture the goal** — what this change is trying to achieve, in one or two lines, from the user, the branch name,
the commit messages or the ticket. The `impl` lens is judged against it; when there is no goal, say so and expect lower
confidence rather than inventing one.

Write `$RUN_DIR/scope.md`: the range, the command that produces the diff, the shortstat, the file list, and the goal.
Every later stage reads that file instead of being told again.

## 2. Run the quality gate *first*

Run the repo's fast check (`../_shared/references/quality-gate.md`, fast tier) **before** the review, not after.

- If it fails, fix that first (or stop and report it) — reviewing a red tree wastes three reviewers on lint output.
- Once it passes, **tell every reviewer it passed**, and that they must not run the tests, the build or the linter. That
  is what earns the right to reject "anything a linter catches" as a finding.
- Keep the output out of this session: redirect each step to `$RUN_DIR/gate-<step>.log`, then report pass/fail and, on
  failure, the failing step and the tail (`../_shared/references/output-discipline.md`).

## 3. Run three reviewers in parallel

Spawn all three in **one message** so they run concurrently and no reviewer sees another's findings.

| reviewer | how to invoke | lenses |
| --- | --- | --- |
| **CodeRabbit** | the CodeRabbit review skill (`coderabbit:code-review`) or the `coderabbit:code-reviewer` agent | not steerable — takes its own broad pass; map its findings onto lenses afterwards |
| **Codex** | the Codex review path (`codex:rescue` skill / `codex:codex-rescue` agent), prompted for a review pass | `bugs`, `impl`, `adversarial` |
| **Claude** | a subagent over the same diff — or the built-in `code-review` skill at a high effort level | `architecture`, `quality`, `tests`, `docs`, `comments` |

Each brief carries: `$RUN_DIR/scope.md`, its lens set, the **resolved absolute paths** of `review-severity.md` and
`review-lenses.md` with an instruction to read them, and the fact that the gate already passed.

Each reviewer **writes** `$RUN_DIR/findings-<source>.md` — one entry per finding: `[surface, severity]`, file + line,
confidence (0–100), the lens(es) that raised it, a title naming the mechanism, a body giving trigger + consequence, and a
concrete fix. Cap it: **at most 15 findings, body under 80 words**; a reviewer at the cap says so and keeps the worst.

Each reviewer **returns at most ten lines**: the path it wrote, counts by `[surface, severity]`, and any lens it could
not cover. No diff, no file contents, no narration.

**Read-only, all three.** No reviewer edits, stages or commits anything — including the built-in `code-review` skill,
which must not be given `--fix`. Fixing is step 6.

**Availability & fallback.** If a tool or plugin is missing or errors out, redistribute its lenses to a reviewer that is
available and **note both facts in the summary**. If only Claude is available, run **two** subagents with different lens
splits so corroboration still means something. Never claim a tool ran if it did not, and never report the result of a
partial review as "clean" — `../_shared/references/review-severity.md`, last section.

## 4. Reconcile the lists

One subagent, on a cheaper model: this stage is mechanical, and the answer is entirely in its input.

Its brief: read the `findings-*.md` files and `../_shared/references/finding-triage.md` §1 (resolved path), write
`$RUN_DIR/reconciled.md`, and **read nothing else** — no source files, no `git diff`, no `rg`. Which findings are the
same issue and which singletons are too weak is answerable from the text; verification comes next with the code in front
of it, and an opinion formed here contaminates the set the verifier is handed.

In short: split out open questions and pre-existing issues, dedupe on same file within ±2 lines and same problem, boost
confidence by distinct **sources** (a source is a reviewer, not a lens), take the highest claimed severity, and drop weak
uncorroborated singletons — **never a critical or major, and nothing at all if a source was missing**. Give every
surviving finding a stable id.

It returns: the path, the counts, and one line per dropped finding.

## 5. Verify the survivors

Group the reconciled findings **by directory**, then spawn **one subagent per group in a single message**. Each gets only
its own group — a verifier that sees the whole set anchors on it — plus `scope.md` and the resolved path to
`finding-triage.md` §2.

Each returns one line per finding: `id → confirmed | refined | rejected | immaterial | pre_existing`, with corrected
fields on a `refined` and one clause of reasoning on a `rejected` or `immaterial`. Verdicts also go to
`$RUN_DIR/verdicts.md`. Verification applies the materiality test only after a finding is confirmed real, and **does not
look for new problems**.

For a handful of findings in one or two files, verify inline instead — a subagent per group is not worth the round trip.

`rejected` findings leave the report entirely. `immaterial` and `pre_existing` get their own summary sections and are
kept **out of the counts**.

## 6. Triage and fix — auto-fix safe, ask on risky

This session does the fixing: the user is in the loop for it, and two subagents editing one tree collide. Read bodies
back from `reconciled.md` **for the findings being acted on only** — that is the first point where full detail earns its
place in context.

- **Safe → fix automatically**: unambiguous, low-risk, behavior-preserving. Typos, a missing nil/error check that is
  clearly correct, an obvious off-by-one, dead code, lint/style, a missing `await`, a stale comment, tightening a type.
- **Risky → ask first**: anything that changes behavior, public API/contracts, data/migrations, concurrency or security
  posture — or where the right fix is a judgement call or spans a broad refactor. Present the finding and the proposed
  fix, and get a decision before editing.
- **Not worth fixing → skip**, with a one-line reason. `immaterial` verdicts are already here by definition.

**Every fix gets the three checks in `../_shared/references/finding-triage.md` §3.** The first one — sweep for the
*shape* not the site — is a read-only repo search, so **delegate it**: hand a subagent the construct in words ("a switch
over a three-value enum that only tests one end") and have it return the occurrence list, file:line only. Then enumerate
the input space you touched (every enum value, struct field, error class — and a test that tells them apart), and re-read
the finding to confirm the fix answers the **mechanism** it named rather than the example it used. Group related safe
fixes into coherent edits, in the style of the surrounding code.

## 7. Re-check

Re-run the fast check from step 2 and report the result — pass/fail, plus the failing step and the tail of its log if it
broke. If a fix
introduced a failure, revert or correct it before summarizing. Never fabricate a passing result.

If the user wants another review round after fixing, keep the bar narrow: re-review with `bugs` + `impl` and **report
nothing below major** — `impl` is the lens that catches a fix that does not address its finding. Do **not** widen the
round to cover documentation you just wrote; a `docs` pass over fresh prose finds a defect in the sentence the last round
produced, round after round, while the code stands still. A second round is a new `$RUN_DIR`, and reviewers are told the
range and the goal — **never handed the previous round's findings**, which would anchor them on conclusions they should
re-derive.

## 8. Summarize (the deliverable)

Write it from the run files, in this order. It is a **decision brief, not a record** — the record is `$RUN_DIR`, so name
that path once and let it hold the detail.

1. **Completeness first** — sources expected vs reported. If any source failed, that goes above every finding.
2. **Scope** — the range reviewed, the shortstat, and which reviewers actually ran (with lenses no source carried).
3. **Counts** — one line, by tag: "2 `[code, major]`, 1 `[docs, minor]`" — never a bare "3 findings".
4. **Findings** — grouped by severity, each headed `[surface, severity] Title — file:line, conf N`, then trigger,
   consequence and fix. Mark any finding more than one source raised, and any that was `refined`.
5. **Fixed automatically** — what changed and why it was safe.
6. **Needs your decision** — risky findings awaiting approval, with the proposed fix.
7. **Considered, not changed** — skipped findings and `immaterial` verdicts, one line each.
8. **Open questions** and **pre-existing** — their own short sections, one line each, out of the counts.
9. **Verification** — the check that ran after fixing, its result, and the `$RUN_DIR` path.

A body appears in full where the reader acts on it and nowhere twice: findings the user must decide on carry their full
body; everything in the one-line sections stays one line. End on the decision the user has to make — findings without an
ask is the middle of the job, not the end of it.

Do not commit as part of this skill unless asked — leave the fixes in the working tree for the user to commit (or chain
into `commit`). Prune old `mkit/review-*` run directories at the end, keeping the last few.

## Git safety

Follow `../_shared/references/git-safety.md`. Apply only fixes to the working tree; never rewrite pushed history, never
force-push, never commit or push as a side effect of the review unless explicitly asked. The run directory is the one
thing this skill writes outside the working tree, and it goes only in `<git-dir>/mkit/`.
