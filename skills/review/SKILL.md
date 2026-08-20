---
name: review
description: >-
  Multi-source review of local uncommitted changes or recent commits — CodeRabbit, Codex and Claude in
  parallel, findings verified, worthwhile fixes applied. Trigger on "review my changes", "review the diff",
  "review last N commits", "run codex and coderabbit", or before a commit/PR. Local work only — for a GitHub
  PR use the PR review tools.
model: opus
---

# Review changes (three reviewers → verify → fix)

Part of the **mkit** bundle. Runs a multi-source review over local work, merges and verifies the findings,
applies safe fixes, asks before risky ones, hands back a findings + fixes summary. Typically run right before
`commit`, `finish` or `pr`.

**Read nothing up front.** Most of these references are what the *reviewers* are held to, not what this
session runs on — they go to subagents as resolved paths, and a copy here buys nothing. Later steps name them
by bare filename:

| reference, under `../_shared/references/` | who reads it | when |
| --- | --- | --- |
| `review-severity.md` | every reviewer | handed over in step 3 |
| `lenses-correctness.md` / `lenses-craft.md` | Codex / the Claude reviewer | handed over in step 3 |
| `triage-reconcile.md` | the reconciler subagent — or **this session**, on a short list | handed over in step 4, or read there |
| `triage-verify.md` | each verifier subagent — or **this session**, on a handful of findings | handed over in step 5, or read there |
| `fix-checks.md` | **this session** | read at step 6, before the first fix |
| `quality-gate.md` | **this session** | read at step 2 to detect the repo's check |
| `output-discipline.md`, `agent-delegation.md`, `git-safety.md` | — | background rationale; the rules this run needs are restated below |

**Resolve a path before it goes into a brief.** A subagent has neither this skill nor `${CLAUDE_PLUGIN_ROOT}`
in context, so `../_shared/references/lenses-craft.md` means nothing to it — expand against this file's own
location and hand over the absolute path.

The one piece of reviewer vocabulary this session uses throughout is the finding tag. Every finding carries
`[surface, severity]`: surface is `code` · `comments` · `docs` · `tests` · `config`/`build`; severity is
`critical` (data loss, security hole, crash on a reachable path) · `major` (wrong runtime behavior, broken
contract) · `minor` (a real defect, contained impact — prose defects are always minor). Counts are reported by
tag, never bare.

Pipeline: **gate → find → reconcile → verify → fix → re-check → report.** Each stage keeps the next honest, so
do not collapse them — a reviewer that also fixes anchors on its own conclusions, and a fixer that also decides
what is real never rejects anything.

## How the work is split

**Every stage that reads a lot and decides a little runs in a subagent and hands back a summary, never its
work.** A three-reviewer review is tens of thousands of tokens of transcript; this session needs only enough to
put a decision to the user. Every brief states its **return budget**, **output path** and **prohibitions**, and
hands over established facts — range, shortstat, file list, goal — so the subagent does not re-derive them.
(`../_shared/references/agent-delegation.md` argues the case; this is the roster.)

| stage | who runs it | model | enters this session |
| --- | --- | --- | --- |
| 1 scope | this session — 2 batched Bash calls, one shared with the run directory | — | range, shortstat, file count, goal |
| 2 gate | this session — Bash | — | pass/fail and the failing step only |
| 2a gate triage (on failure) | 1 subagent, reads the log | Sonnet | what failed, cause, suggested fix — ≤15 lines |
| 3 find | 3 subagents, parallel | Opus | 3 × ≤10 lines: path written + counts by tag + lenses not covered |
| 4 reconcile | 1 subagent, text only — inline if ≤10 findings | Sonnet | path written + counts + what it dropped and why |
| 5 verify | 1 subagent per directory group, parallel | Opus | one line per finding: id + verdict (+ corrected fields) |
| 6 fix | this session, bodies read from disk on demand | — | the findings being acted on, in full |
| 6a sweep | this session when the shape greps; 1 subagent per shape when it does not | Sonnet | the occurrence list |
| 7 re-check | this session — Bash | — | pass/fail |
| 8 report | this session, written from the run files | — | the summary itself |

Open the run directory **before step 2's gate writes its first log**
(`../_shared/references/output-discipline.md`). It does not depend on the scope, so open it **in the same call
as step 1's branch and status**:

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh review
git branch --show-current && git status --short
```

One absolute path, created atomically so two concurrent reviews in one checkout cannot collide. **Keep that
literal**: there is no `$RUN_DIR` in a later call, this file writes it as `<run-dir>`, and every subagent gets
the resolved path — never the variable, never the `${CLAUDE_PLUGIN_ROOT}` command.

**One writer per file, throughout.** Every fanned-out stage writes one file per subagent —
`findings-<source>.md` per reviewer, `verdicts-<group>.md` per group — and this session aggregates after they
return.

**Never read a diff into this session.** Subagents read the diff; this session reads `--shortstat` and
`--name-only`. Never paste a reference into a brief either — resolve its path and have the subagent read it.

## 1. Establish the review scope

Decide what to review (ask only if genuinely ambiguous — a feature branch with uncommitted work is the
standard case):

- **Uncommitted changes** (default when the tree is dirty): staged + unstaged — `git diff HEAD`, or
  `git diff` / `git diff --cached` to separate them.
- **Recent commits**: "last 3 commits" → `git diff HEAD~3..HEAD`; everything since base →
  `git diff <base>..HEAD`.
- Both a dirty tree and "review my commits" in play → confirm which, or review both and note the split.

Branch and status came back with the run directory. Once the range is settled, one more call — and no more git
than this:

```bash
git diff <range> --shortstat && git diff <range> --name-only
```

Uncommitted work on the default branch is `git diff` **plus** `git diff --staged`: a bare
`git diff --shortstat` reports nothing when the work is fully staged.

**Also capture the goal** — what the change is trying to achieve, one or two lines, from the user, branch name,
commit messages or ticket. The `impl` lens is judged against it; with no goal, say so and expect lower
confidence rather than inventing one.

Write `<run-dir>/scope.md`: range, the command producing the diff, shortstat, file list, goal. Later stages
read that file instead of being told again.

## 2. Run the quality gate *first*

The repo's fast check (`../_shared/references/quality-gate.md`, fast tier), **before** the review.

- Failing → fix that first, or stop and report it: reviewing a red tree wastes three reviewers on lint output.
  On a long failure log, triage it in a subagent ("when a step fails").
- Passing → **tell every reviewer it passed**, and that they must not run the tests, build or linter. That is
  what earns the right to reject "anything a linter catches" as a finding.
- Keep output out of this session: redirect each step to `<run-dir>/gate-<step>.log`, report pass/fail and, on
  failure, the failing step and the tail (`../_shared/references/output-discipline.md`).

## 3. Run three reviewers in parallel

Spawn all three in **one message**, so they run concurrently and none sees another's findings.

| reviewer | how to invoke | lenses |
| --- | --- | --- |
| **CodeRabbit** | the CodeRabbit review skill (`coderabbit:code-review`) or the `coderabbit:code-reviewer` agent | not steerable — takes its own broad pass; map its findings onto lenses afterwards |
| **Codex** | the Codex review path (`codex:rescue` skill / `codex:codex-rescue` agent), prompted for a review pass | `bugs`, `impl`, `adversarial` |
| **Claude** | a subagent over the same diff — or the built-in `code-review` skill at a high effort level | `architecture`, `quality`, `tests`, `docs`, `comments` |

Each brief carries: `<run-dir>/scope.md`, the **resolved absolute paths** of `review-severity.md` and of **its
own lens file** — `lenses-correctness.md` for Codex, `lenses-craft.md` for Claude — with an instruction to read
them, plus the fact that the gate passed. A reviewer gets the lens file it carries and not the other; hand over
both only when it is covering for a missing reviewer.

Each reviewer **writes** `<run-dir>/findings-<source>.md`, one entry per finding: `[surface, severity]`,
file + line, confidence (0–100), the lens(es) that raised it, a title naming the mechanism, a body giving
trigger + consequence, and a concrete fix. Cap it: **at most 15 findings, body under 80 words**; a reviewer at
the cap says so and keeps the worst.

Each reviewer **returns at most ten lines**: the path it wrote, counts by `[surface, severity]`, and any lens
it could not cover. No diff, no file contents, no narration.

**Read-only, all three.** No reviewer edits, stages or commits anything — including the built-in `code-review`
skill, which must not be given `--fix`. Fixing is step 6.

**Availability & fallback.** A missing or erroring tool: redistribute its lenses to an available reviewer and
**note both facts in the summary**. Claude only: run **two** subagents with different lens splits, so
corroboration still means something. Never claim a tool ran if it did not, and never report a partial review as
"clean" (`../_shared/references/review-severity.md`, last section).

## 4. Reconcile the lists

One subagent on a cheaper model: mechanical, and the answer is entirely in its input.

Brief: read the `findings-*.md` files and `../_shared/references/triage-reconcile.md` (resolved path), write
`<run-dir>/reconciled.md`, **read nothing else** — no source files, no `git diff`, no `rg`. Same-issue and
too-weak-singleton are answerable from the text; verification comes next with the code in front of it, and an
opinion formed here contaminates the set the verifier is handed.

In short: split out open questions and pre-existing issues, dedupe on same file within ±2 lines and same
problem, boost confidence by distinct **sources** (a source is a reviewer, not a lens), take the highest
claimed severity, drop weak uncorroborated singletons — **never a critical or major, and nothing at all if a
source was missing**. Stable id on every survivor. Returns: the path, the counts, one line per dropped finding.

**Reconcile inline instead when the lists are short** — ~≤10 findings across sources, where the round trip
costs more than the merge. Same rules either way; write `reconciled.md` here and move on. Delegate above that,
and whenever a source returned near its 15-finding cap.

## 5. Verify the survivors

Group the reconciled findings **by directory**, spawn **one subagent per group in a single message**. Each gets
only its own group — a verifier seeing the whole set anchors on it — plus `scope.md` and the resolved path to
`triage-verify.md`.

Each returns one line per finding: `id → confirmed | refined | rejected | immaterial | pre_existing`, with
corrected fields on a `refined` and one clause of reasoning on a `rejected` or `immaterial`. Verdicts also go to
**`<run-dir>/verdicts-<group>.md` — one file per verifier, named for its group**, never a shared `verdicts.md`:
parallel writers to one path interleave or clobber, and a lost verdict is indistinguishable from a finding
nobody raised. Step 8 reads the `verdicts-*.md` set directly — never concatenate them. Verification applies the
materiality test only after a finding is confirmed real, and **does not look for new problems**.

A handful of findings in one or two files: verify inline instead and write straight to
`<run-dir>/verdicts-all.md`, since there is only one writer.

`rejected` findings leave the report entirely. `immaterial` and `pre_existing` get their own summary sections,
**out of the counts**.

## 6. Triage and fix — auto-fix safe, ask on risky

This session fixes: the user is in the loop, and two subagents editing one tree collide. Read bodies back from
`reconciled.md` **only for findings being acted on** — the first point where full detail earns its place.

- **Safe → fix automatically**: unambiguous, low-risk, behavior-preserving. Typos, a clearly correct missing
  nil/error check, an obvious off-by-one, dead code, lint/style, a missing `await`, a stale comment,
  tightening a type.
- **Risky → ask first**: anything changing behavior, public API/contracts, data/migrations, concurrency or
  security posture — or where the right fix is a judgement call or spans a broad refactor. Present the finding
  and the proposed fix, get a decision before editing.
- **Not worth fixing → skip**, with a one-line reason. `immaterial` verdicts are already here by definition.

**Every fix gets the three checks in `../_shared/references/fix-checks.md`** — read it here, before the first
fix. Check one, sweep for the *shape* not the site, is a read-only repo search, and **how you run it depends on
whether the shape greps**. A literal or near-literal pattern is one `rg` call: run it here, no subagent.
Delegate only when the construct needs judgement to recognise ("a switch over a three-value enum that only
tests one end") or the search spans the whole repo — hand the subagent the construct in words, get back the
occurrence list, file:line only. **The check itself is never optional**, whichever way it runs. Then enumerate
the input space you touched (every enum value, struct field, error class — and a test telling them apart), and
re-read the finding to confirm the fix answers the **mechanism** it named, not the example. Group related safe
fixes into coherent edits, in the style of the surrounding code.

## 7. Re-check

Re-run step 2's fast check and report pass/fail, plus the failing step and the tail of its log if it broke. A
fix that introduced a failure gets reverted or corrected before summarizing. Never fabricate a passing result.

Another round after fixing keeps the bar narrow: `bugs` + `impl` only, **nothing below major** — `impl` is the
lens that catches a fix not addressing its finding. Do **not** widen it to documentation you just wrote: a
`docs` pass over fresh prose finds a defect in the sentence the last round produced, round after round, while
the code stands still. A second round is a new run directory, and reviewers get the range and the goal —
**never the previous round's findings**, which anchor them on conclusions they should re-derive.

## 8. Summarize (the deliverable)

Written from the run files, in this order. It is a **decision brief, not a record** — the record is the run
directory, so name that path once and let it hold the detail.

1. **Completeness first** — sources expected vs reported. A failed source goes above every finding.
2. **Scope** — range reviewed, shortstat, which reviewers actually ran (and lenses no source carried).
3. **Counts** — one line, by tag: "2 `[code, major]`, 1 `[docs, minor]`". Never a bare "3 findings".
4. **Findings** — grouped by severity, each headed `[surface, severity] Title — file:line, conf N`, then
   trigger, consequence, fix. Mark any finding more than one source raised, and any that was `refined`.
5. **Fixed automatically** — what changed, why it was safe.
6. **Needs your decision** — risky findings awaiting approval, with the proposed fix.
7. **Considered, not changed** — skipped findings and `immaterial` verdicts, one line each.
8. **Open questions** and **pre-existing** — their own short sections, one line each, out of the counts.
9. **Verification** — the check that ran after fixing, its result, and the `<run-dir>` path.

A body appears in full where the reader acts on it, and nowhere twice: findings the user must decide on carry
their full body, the one-line sections stay one line. End on the decision the user has to make — findings
without an ask is the middle of the job, not the end.

Do not commit unless asked — leave fixes in the working tree for the user to commit (or chain into `commit`).
Prune old `mkit/review-*` run directories at the end, keeping the last few; fold that into step 7's re-check
call rather than spending a turn on it.

## Git safety

Follow `../_shared/references/git-safety.md`. Apply only fixes to the working tree; never rewrite pushed
history, never force-push, never commit or push as a side effect of the review unless explicitly asked. The run
directory is the one thing this skill writes outside the working tree, and it goes only in `<git-dir>/mkit/`.
