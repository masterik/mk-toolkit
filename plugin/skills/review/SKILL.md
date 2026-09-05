---
name: review
description: >-
  Multi-source review of local uncommitted changes or recent commits — CodeRabbit, Codex and Claude in
  parallel, findings verified, worthwhile fixes applied. Full mode (default) runs all three reviewers; quick
  mode runs CodeRabbit + Codex on bugs/impl only. Trigger on "review my changes", "review the diff", "quick
  review", "review last N commits", "run codex and coderabbit", or before a commit/PR. Local work only — for a
  GitHub PR use the PR review tools.
argument-hint: "[quick|full]"
model: opus
---

# Review changes (reviewers → verify → fix)

Part of the **mkit** bundle. Runs a multi-source review over local work, merges and verifies the findings,
applies safe fixes, asks before risky ones, hands back a findings + fixes summary. Typically run right before
`commit`, `finish` or `pr`.

**Mode** (`$ARGUMENTS`): `quick` or `full` — e.g. `/mkit:review quick`. Decided in step 1 if omitted; full is
the default.

**Read nothing up front.** Most of these references are what the *reviewers* are held to, not what this
session runs on — they go to subagents as paths under `refs=` (below), and a copy here buys nothing. Later
steps name them by bare filename:

| reference, under `refs=` | who reads it | when |
| --- | --- | --- |
| `review-severity.md` | every reviewer | handed over in step 2 |
| `lenses-correctness.md` / `lenses-craft.md` | Codex / the Claude reviewer | handed over in step 2 |
| `triage-reconcile.md` | **this session** | read at step 3 |
| `triage-verify.md` | each verifier subagent — or **this session**, on a handful | handed over in step 4, or read there |
| `fix-checks.md` | **this session** | read at step 5, before the first fix |
| `output-discipline.md`, `agent-delegation.md`, `git-safety.md` | — | background rationale; the rules this run needs are restated below |

The one piece of reviewer vocabulary this session uses throughout is the finding tag. Every finding carries
`[surface, severity]`: surface is `code` · `comments` · `docs` · `tests` · `config`/`build`; severity is
`critical` (data loss, security hole, crash on a reachable path) · `major` (wrong runtime behavior, broken
contract) · `minor` (a real defect, contained impact — prose defects are always minor). Counts are reported by
tag, never bare.

Pipeline: **find → reconcile → verify → fix → report.** Each stage keeps the next honest, so do not collapse
them — a reviewer that also fixes anchors on its own conclusions, and a fixer that also decides what is real
never rejects anything. `review` does not gate — that starts at `pr` and `finish`
(`../_shared/references/quality-gate.md`).

## How the work is split

**Every stage that reads a lot and decides a little runs in a subagent and hands back a summary, never its
work.** A full review (all three reviewers) is tens of thousands of tokens of transcript; quick (two reviewers,
narrower brief) is proportionally less. Either way this session needs only enough to put a decision to the
user. Every brief states its **return budget**, **output path** and **prohibitions**, and hands over
established facts — range, shortstat, file list, goal, mode — so the subagent does not re-derive them.
(`agent-delegation.md` argues the case; this is the roster.)

| stage | who runs it | model | enters this session |
| --- | --- | --- | --- |
| 1 scope | this session — **one** `facts.sh` call | — | run dir, refs path, branch, stat, file list |
| 2 find | 2 (quick) or 3 (full) subagents, parallel | Opus | one ≤10-line reply per subagent: path written + counts by tag + lenses not covered |
| 3 reconcile | this session — `findings.mjs reconcile` | — | counts, merges, drops, undecided pairs |
| 4 verify | 1 subagent per group from `findings.mjs group`, parallel | Opus | one line per finding: id + verdict (+ corrected fields) |
| 5 fix | this session, bodies read from disk on demand | — | the findings being acted on, in full |
| 5a sweep | this session when the shape greps; 1 subagent per shape when it does not | Sonnet | the occurrence list |
| 6 report | this session — `findings.mjs report`, then prose | — | the summary itself |

**Never read a diff into this session.** Subagents read the diff; this session reads what `facts.sh` returned.
Never paste a reference into a brief either — hand over its path under `refs=` and have the subagent read it.

**One writer per file, throughout.** Every fanned-out stage writes one file per subagent —
`findings-<source>.jsonl` per reviewer, `verdicts-<group>.jsonl` per group — and the scripts aggregate after
they return.

## 1. Establish the review scope

Decide what to review (ask only if genuinely ambiguous — a feature branch with uncommitted work is the
standard case): **uncommitted changes** (default when the tree is dirty), or **recent commits**
(`HEAD~3..HEAD`, `<base>..HEAD`). Both a dirty tree and "review my commits" in play → confirm which, or review
both and note the split.

**Decide the mode**: **full** (all three reviewers, all eight lenses — the default) or **quick** (CodeRabbit +
Codex on `bugs`/`impl` only; no Claude craft subagent; no `adversarial`). `$ARGUMENTS` decides it outright when
present (`quick` or `full`). Otherwise, an explicit signal — "quick review", "fast pass", "just check for
bugs" — selects quick; ask only if genuinely ambiguous; **default to full** otherwise, since "before a
commit/PR" is the typical high-stakes trigger this skill is built for. Quick is a **deliberate** narrower
scope, not a degraded run — step 2 and step 6 must never describe it the way a missing/failed reviewer is
described.

Then one call, which also opens the run directory:

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh review --range <range>     # omit --range for a dirty tree
```

Keep the `run=` and `refs=` literals; every later step and every brief needs them, and re-running the script
opens a second directory (`output-discipline.md`).

**Also capture the goal** — what the change is trying to achieve, one or two lines, from the user, branch name,
commit messages or ticket. The `impl` lens is judged against it; with no goal, say so and expect lower
confidence rather than inventing one.

Write `<run-dir>/scope.md`: the range, the command producing the diff, the stat, the file list, the goal, and
**the mode**. Later stages read that file instead of being told again — step 2's roster and step 3's
`--sources-expected` both key off the mode recorded here.

## 2. Run the reviewers in parallel

Spawn every reviewer the mode calls for in **one message**, so they run concurrently and none sees another's
findings.

- **full** (default): all three, as below.
- **quick**: CodeRabbit + Codex only — no Claude subagent. Both briefs carry `lenses-correctness.md` and
  explicitly narrow to the `bugs` and `impl` sections, instructing **skip `adversarial`**. CodeRabbit's
  narrowing is best-effort only — it is not steerable and may still return broader findings; that is expected,
  not a broken brief.

Each reviewer is read-only over a tree whose state was never checked by this skill — `review-severity.md`
already tells it not to run the tests, build or linter, and not to report anything one would catch.

| reviewer | how to invoke | lenses |
| --- | --- | --- |
| **CodeRabbit** | the CodeRabbit review skill (`coderabbit:code-review`) or the `coderabbit:code-reviewer` agent | full: not steerable — takes its own broad pass; map its findings onto lenses afterwards. quick: brief also asks for `bugs`/`impl` only, best-effort — it may still return broader findings |
| **Codex** | the Codex review path (`codex:rescue` skill / `codex:codex-rescue` agent), prompted for a review pass — always with `--wait` appended, see the async caveat below | full: `bugs`, `impl`, `adversarial`. quick: `bugs`, `impl` only — the brief says so explicitly |
| **Claude** (full only) | a subagent over the same diff — or the built-in `code-review` skill at a high effort level | `architecture`, `quality`, `tests`, `docs`, `comments` |

Each brief carries: `<run-dir>/scope.md`, the paths of `review-severity.md` and of **its own lens file** under
`refs=` — `lenses-correctness.md` for Codex (and, in quick mode, for CodeRabbit too, so its best-effort
narrowing has something to narrow against), `lenses-craft.md` for Claude — with an instruction to read them.
A reviewer gets the lens file it carries and not the other; hand over both only when it is covering for a
missing reviewer.

**Quick's narrower roster is not the same thing as a missing reviewer.** Quick never spawns the Claude
subagent and never asks Codex to cover `adversarial` — that is the mode working as designed, and step 6 must
say so in mode-neutral language. "Availability & fallback" below is for a reviewer that was *launched* and
then errored, went silent, or came back unusable — a different situation, reported a different way.

Each reviewer **writes `<run-dir>/findings-<source>.jsonl`, one JSON object per line**:

```json
{"surface":"code","severity":"major","confidence":85,"file":"src/api/user.ts","line":42,
 "lens":["bugs"],"title":"names the mechanism","body":"trigger + consequence","fix":"concrete fix"}
```

`surface`, `severity`, `file`, `title` required; `class` is `finding` (default), `open_question` or
`pre_existing`. Full shape: `node ${CLAUDE_PLUGIN_ROOT}/scripts/findings.mjs schema`. Cap it: **at most 15 findings, body
under 80 words**; a reviewer at the cap says so and keeps the worst. JSONL because step 3 is a script — a
reviewer that writes prose costs a re-spawn, so the brief says "one JSON object per line, nothing else".

Each reviewer **returns at most ten lines**: the path it wrote, counts by `[surface, severity]`, and any lens
it could not cover. No diff, no file contents, no narration.

**The file is the completion signal, not the reply.** A reviewer is done when
`findings-<source>.jsonl` exists — so the brief must say: found nothing at or above the bar → **write an empty
file and say so**. The two states are not the same downstream, and only the reviewer can tell them apart:
present-and-empty means that source reported zero and keeps the drop rule armed; absent means it never
reported and disarms it. An agent that finishes its turn with no file has reported nothing, whatever its reply
says.

**Read-only, every reviewer run.** No reviewer edits, stages or commits anything — including the built-in
`code-review` skill, which must not be given `--fix`. Fixing is step 5.

**Codex wraps an async job — force it to run in the foreground.** `codex:codex-rescue`'s own contract
(`codex-cli-runtime`) forbids it from polling, monitoring, or waiting on a job it backgrounded — its brief
telling it to "keep waiting" cannot override that, because the agent's own rules take precedence over
instructions passed into its prompt. Left to its own routing heuristic, it treats a full review as
"complicated" and backgrounds the job, forwards it, and ends its turn immediately — reporting itself idle
while Codex is still working. **Always append `--wait` to the task text forwarded to `codex:codex-rescue`**:
per `codex-cli-runtime`, `--wait`/`--background` are recognized as execution-control tokens, and `--wait`
forces the underlying call to block until Codex actually finishes, so the agent's turn cannot end early. Its
brief must still add: if it dies, say so with the error rather than reconstructing findings; and **never write
a placeholder before it returns**, because an empty file claims a zero-finding review that did not happen.

**Availability & fallback.** A missing or erroring tool: redistribute its lenses to an available reviewer and
**note both facts in the summary**. Claude only: run **two** subagents with different lens splits, so
corroboration still means something. Never claim a tool ran if it did not, and never report a partial review as
"clean" (`review-severity.md`, last section).

**Bound the wait.** A reviewer that has neither written its file nor reported an error is not evidence of
anything, and step 3 cannot start without the full set. Ask a silent source once what happened —
distinguishing *job errored* / *ran but did not write* / *returned prose* / *genuinely zero* — and if the file
is still missing after roughly ten minutes, declare that source **missing**, redistribute its lenses, and go on
with a lower `--sources-expected`. Do not poll it repeatedly, and do not hold the run open indefinitely; a slow
source blocking every later stage costs more than the corroboration it would have added. Say in the summary
that it was dropped and which lenses went uncovered.

## 3. Reconcile the lists

```bash
node ${CLAUDE_PLUGIN_ROOT}/scripts/findings.mjs reconcile <run-dir> --sources-expected <N>
```

`N` is the number of reviewers you **launched**, not the number that answered — it is what switches the
weak-singleton drop rule off when a source is missing. That follows the mode recorded in `scope.md`: **2** for
quick (CodeRabbit + Codex), **3** for full. **Never create an empty `findings-<source>.jsonl` to
make the count line up**: the script reads a present file as that source reporting zero, which re-arms the drop
rule and silently deletes exactly the single-source findings the missing reviewer would have corroborated. A
source that never wrote a file is missing; lower `N` and say so. Then read `triage-reconcile.md` and check the two
things the script deliberately leaves open: `LOW-SIM` merges and `review_pairs`. No subagent: the merge is
arithmetic, and forming an opinion here contaminates the set the verifier is handed.

If a reviewer wrote prose instead of JSONL, `validate` names the lines; convert it here rather than re-spawning.

## 4. Verify the survivors

```bash
node ${CLAUDE_PLUGIN_ROOT}/scripts/findings.mjs group <run-dir>
```

It groups by directory, folds groups too small to be worth a round trip, writes `verify-<group>.jsonl`, and
suggests inline vs fan-out. On fan-out, spawn **one subagent per group in a single message**; each gets only
its own group file — a verifier seeing the whole set anchors on it — plus `scope.md` and the path to
`triage-verify.md`.

Each verifier **writes `<run-dir>/verdicts-<group>.jsonl`** (never a shared file: parallel writers to one path
interleave, and a lost verdict is indistinguishable from a finding nobody raised) and returns one line per
finding: `id → confirmed | refined | rejected | immaterial | pre_existing`, with corrected fields on a
`refined` and one clause of reasoning on a `rejected` or `immaterial`. Verification applies the materiality
test only after a finding is confirmed real, and **does not look for new problems**.

On `suggest=inline`, verify here and write `verdicts-all.jsonl` — there is only one writer.

`rejected` findings leave the report entirely. `immaterial` and `pre_existing` get their own summary sections,
**out of the counts**.

## 5. Triage and fix — auto-fix safe, ask on risky

This session fixes: the user is in the loop, and two subagents editing one tree collide. Read bodies back from
`reconciled.jsonl` **only for findings being acted on** — the first point where full detail earns its place.

- **Safe → fix automatically**: unambiguous, low-risk, behavior-preserving. Typos, a clearly correct missing
  nil/error check, an obvious off-by-one, dead code, lint/style, a missing `await`, a stale comment,
  tightening a type.
- **Risky → ask first**: anything changing behavior, public API/contracts, data/migrations, concurrency or
  security posture — or where the right fix is a judgement call or spans a broad refactor. Present the finding
  and the proposed fix, get a decision before editing.
- **Not worth fixing → skip**, with a one-line reason. `immaterial` verdicts are already here by definition.

**Every fix gets the three checks in `fix-checks.md`** — read it here, before the first fix. Check one, sweep
for the *shape* not the site, is a read-only repo search, and **how you run it depends on whether the shape
greps**. A literal or near-literal pattern is one `rg` call: run it here, no subagent. Delegate only when the
construct needs judgement to recognise ("a switch over a three-value enum that only tests one end") or the
search spans the whole repo — hand the subagent the construct in words, get back the occurrence list,
file:line only. **The check itself is never optional**, whichever way it runs. Then enumerate the input space
you touched (every enum value, struct field, error class — and a test telling them apart), and re-read the
finding to confirm the fix answers the **mechanism** it named, not the example. Group related safe fixes into
coherent edits, in the style of the surrounding code.

## Optional: a second round after fixing

Step 5 may leave enough changed that it is worth another review pass over the fixes themselves — never a
requirement, a judgment call. If you run one, keep the bar narrow: `bugs` + `impl` only, **nothing below
major** — `impl` is the lens that catches a fix not addressing its finding. Do **not** widen it to
documentation you just wrote: a `docs` pass over fresh prose finds a defect in the sentence the last round
produced, round after round, while the code stands still. A second round is a new run directory, and
reviewers get the range and the goal — **never the previous round's findings**, which anchor them on
conclusions they should re-derive.

## 6. Summarize (the deliverable)

```bash
node ${CLAUDE_PLUGIN_ROOT}/scripts/findings.mjs report <run-dir>
```

It merges the verdicts onto the findings (applying `refined` corrections), writes `final.jsonl`, and returns
the counts, the reportable set, the gating count, and — the one that matters — `UNVERIFIED` ids and orphan
verdicts. **Never write the summary while `UNVERIFIED` is non-empty**: a lost verdict reads exactly like a
finding nobody raised.

Then the prose, in this order. It is a **decision brief, not a record** — the record is the run directory, so
name that path once and let it hold the detail.

1. **Mode** — which mode ran, and which reviewers/lenses were **not run by design**, e.g. "Mode: quick —
   CodeRabbit + Codex (bugs, impl); architecture/quality/tests/docs/comments and adversarial not run." Word it
   so it reads distinctly from item 2 below: a deliberate narrower scope, never a partial or degraded full run.
2. **Completeness first** — sources expected vs reported. A failed source goes above every finding.
3. **Scope** — range reviewed, shortstat, which reviewers actually ran (and lenses no source carried).
4. **Counts** — one line, by tag: "2 `[code, major]`, 1 `[docs, minor]`". Never a bare "3 findings".
5. **Findings** — grouped by severity, each headed `[surface, severity] Title — file:line, conf N`, then
   trigger, consequence, fix. Mark any finding more than one source raised, and any that was `refined`.
6. **Fixed automatically** — what changed, why it was safe.
7. **Needs your decision** — risky findings awaiting approval, with the proposed fix.
8. **Considered, not changed** — skipped findings and `immaterial` verdicts, one line each.
9. **Open questions** and **pre-existing** — their own short sections, one line each, out of the counts.
10. **Run directory** — the `<run-dir>` path, and whether a second round ran (and its result) or was
    skipped. Say plainly that a fixed tree is unverified here — `review` does not build or test it;
    that check runs at `pr`/`finish`.

A body appears in full where the reader acts on it, and nowhere twice: findings the user must decide on carry
their full body, the one-line sections stay one line. End on the decision the user has to make — findings
without an ask is the middle of the job, not the end.

Do not commit unless asked — leave fixes in the working tree for the user to commit (or chain into `commit`).
Fold `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh --prune` into step 6's call rather than spending a turn on it.

## Git safety

Follow `git-safety.md`. Apply only fixes to the working tree; never rewrite pushed history, never force-push,
never commit or push as a side effect of the review unless explicitly asked. The run directory is the one thing
this skill writes outside the working tree, and it goes only in `<git-dir>/mkit/`.
