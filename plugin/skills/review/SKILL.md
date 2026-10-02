---
name: review
description: >-
  Review local uncommitted changes or recent commits with CodeRabbit, Codex and Claude in parallel; verify
  findings, apply worthwhile fixes. Full (default) or quick (CodeRabbit + Codex). Trigger on "review my
  changes", "review the diff", "quick review", "review last N commits", "run codex and coderabbit", or before a
  commit/PR. Local work only — not for GitHub PRs.
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
| `workflow-contract.md` | **this session** | read at step 1 |
| `triage-reconcile.md` | **this session** | read at step 3 |
| `triage-verify.md` | each verifier subagent — or **this session**, on a handful | handed over in step 4, or read there |
| `fix-checks.md` | **this session** | read at step 5, before the first fix |
| `output-discipline.md`, `agent-delegation.md`, `git-safety.md`, `summary-format.md` | — | background rationale; the rules this run needs are restated below (step 6 is the fullest instance of `summary-format.md`) |

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
| 1 scope | this session — **one** `mkit facts` call | — | run dir, refs path, branch, stat, file list |
| 2 find | 2 (quick) or 3 (full) subagents, parallel | Opus | one ≤10-line reply per subagent: path written + counts by tag + lenses not covered |
| 3 reconcile | this session — `mkit findings reconcile --json` | — | counts, merges, drops, undecided pairs |
| 4 verify | 1 subagent per group from `mkit findings group --json`, parallel | Opus | one line per finding: id + verdict (+ corrected fields) |
| 5 fix | this session, bodies read from disk on demand | — | the findings being acted on, in full |
| 5a sweep | this session when the shape greps; 1 subagent per shape when it does not | Sonnet | the occurrence list |
| 6 report | this session — `mkit findings report --json`, then prose | — | the summary itself |

**Never read a diff into this session.** Subagents read the diff; this session reads what `mkit facts` returned.
Never paste a reference into a brief either — hand over its path under `refs=` and have the subagent read it.

**One writer per file, throughout.** Every fanned-out stage writes one file per subagent —
`findings-<source>.jsonl` per reviewer, `verdicts-<group>.jsonl` per group — and `mkit findings` aggregates
after they return.

## 0. Check `mkit` is there

```bash
command -v mkit >/dev/null && mkit findings schema --json >/dev/null
```

Both halves matter. The first says the binary is installed — which step 1's `mkit facts` needs too,
and every other skill now needs at its own first call; the second says *this* binary knows `findings`
— a subcommand that does not exist is what "too old" looks like, and asking for it here surfaces it
before a reviewer has run rather than at step 3 with a full run behind it. Nothing is compared against
a declared range: any `mkit` that answers is current enough, and neither side declares a minimum.

If either half fails, **stop** — do not start the reviewers. Say:

> `review` needs the `mkit` binary for the findings arithmetic at steps 3, 4 and 6, and it is either
> not installed or too old to know `mkit findings`. Install it with `brew install masterik/tap/mkit`
> (or upgrade with `brew upgrade mkit`), then run the review again.

This is the one place `review` is deliberately not entry-capable. Without the arithmetic there is no
reconcile, no groups, and no ids for verdicts to reference; doing it by hand would put back into
judgement exactly what this stage exists to remove.

## 1. Establish the review scope

Decide what to review (ask only if genuinely ambiguous — a feature branch with uncommitted work is the
standard case): **uncommitted changes** (default when the tree is dirty), or **recent commits**
(`HEAD~3..HEAD`, `<base>..HEAD`). Both a dirty tree and "review my commits" in play → confirm which, or review
both and note the split.

**Decide the mode**: **full** (all three reviewers, all eight lenses — the default) or **quick** (CodeRabbit +
Codex on `bugs`/`impl` only; no Claude craft subagent; no `adversarial`). `$ARGUMENTS` decides it outright when
present (`quick` or `full`). Otherwise, an explicit signal — "quick review", "fast pass", "just check for
bugs" — selects quick; then **the repo's own default**, if it pinned one:

```bash
mkit repo profile --json     # review_mode: "full" | "quick", tagged pinned
```

A pinned `review_mode` is the repo saying which roster it wants when nobody said — three reviewers is tens
of thousands of tokens, and whether a team spends that by default is a decision, not a `command -v` result.
It is a default, never a ceiling: `$ARGUMENTS` and an explicit signal both still win, and the user can ask
for the other mode at any point. Say it was pinned when it decided the run (rule 3), and record it in
`scope.md` like any other mode. A failed call or an absent value changes nothing and is not mentioned.

**Silence, with one exception — a pin that went nowhere.** A non-empty `config_problems`, or a `review_mode`
whose `source` is `unavailable` and whose `cause` names `.mkit/config.toml`, is not "no answer": it is the
repo pinning a mode mkit could not honour — `mode = "quik"`, or a `[reviewers]` table where `[review]` was
meant. Report that sentence in one line, verbatim, then carry on with the mode you would otherwise have
chosen. Mind which field carries it: the sentence is `detail` on a `config_problems` entry and `cause` on the value itself — there is no `detail` on a value, and an agent that looks for one reports nothing. Falling silently back to full is the failure this reporting exists to prevent: the config looks
applied, and the only symptom is a roster the team thought they had changed.
Ask only if genuinely ambiguous; **default to full** otherwise, since "before a
commit/PR" is the typical high-stakes trigger this skill is built for. Quick is a **deliberate** narrower
scope, not a degraded run — step 2 and step 6 must never describe it the way a missing/failed reviewer is
described.

Then one call, which also opens the run directory:

```bash
mkit facts review --range <range>     # omit --range for a dirty tree
```

Keep the `run=` and `refs=` literals; every later step and every brief needs them, and re-running `mkit facts`
opens a second directory (`output-discipline.md`).

Then read what already ran on this branch:

```bash
mkit worklog show --json --limit 20
```

**Unconditional, here and everywhere else** (`workflow-contract.md`, "Reading it"). Step 0 already
stopped the run if the binary was missing, and since M5 every other skill's first call does the same —
so no skill has a missing-binary case left to check for.

What step 0 did *not* establish is that this binary knows `worklog` — it proved `findings` — and a binary from
before the worklog landed answers one and not the other. So the second half of the shared rule still holds:
**a nonzero exit is not a stop**, carry on without the log and say so in step 6. Read what it printed before
naming the cause: an unknown subcommand is that skew, while a log it found and could not read is a different
fact and worth reporting as one. A branch nothing has run on is different again — zero records and exit 0,
because being first is the normal case, not a problem to report.

The envelope's own `fingerprint` is the tree as it is right now; each record carries the tree it ran over.
Comparing the two is what the goal order below means by "matching".

**Also capture the goal** — what the change is trying to achieve, one or two lines. In this order:

1. a worklog gist whose `fingerprint` matches the tree being reviewed — a `spec` or `implement` record is
   the goal stated by whoever set it, not one inferred from the outside
2. the user
3. the branch name
4. the commit messages
5. the ticket

A gist whose fingerprint **no longer matches** drops out of first place and sits **below the user** — it
describes a tree that no longer exists, and a goal the user stated in this session is about the one being
reviewed. So take it only when nothing above it in the list answers: prefer the user, then fall back to the
stale gist ahead of the branch name, and name the downgrade in step 6 ("goal from the worklog, recorded
before the last N files changed"). The `impl` lens is
judged against the goal; with no goal at all, say so and expect lower confidence rather than inventing one.
Whichever source it came from, say which — that is the contract's third rule, and it is the same degrade
this step already performs when there is no spec.

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
| **Codex** | the Codex review path (`codex:rescue` skill / `codex:codex-rescue` agent), prompted for a review pass — always with `--wait --write` appended, and dispatched from inside the reviewed tree; see the two caveats below | full: `bugs`, `impl`, `adversarial`. quick: `bugs`, `impl` only — the brief says so explicitly |
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
`pre_existing`. Full shape: `mkit findings schema` (`--json` for the machine-readable form). Cap it: **at most 15 findings, body
under 80 words**; a reviewer at the cap says so and keeps the worst. JSONL because step 3 is a command — a
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
while Codex is still working. **Always append `--wait --write` to the task text forwarded to
`codex:codex-rescue`**: per `codex-cli-runtime`, `--wait`/`--background` are recognized as execution-control
tokens, and `--wait` forces the underlying call to block until Codex actually finishes, so the agent's turn
cannot end early. Its brief must still add: if it dies, say so with the error rather than reconstructing
findings; and **never write a placeholder before it returns**, because an empty file claims a zero-finding
review that did not happen.

**Codex's sandbox is set by the invocation, not by the brief.** `codex:codex-rescue` runs the companion under
Codex's *own* sandbox, and two invocation details — neither expressible in prose inside the brief — decide
whether it can write `findings-codex.jsonl` at all:

- **`--write`, or the whole run is read-only.** Without that token the companion starts Codex in `read-only`
  mode, and the findings write fails with *"session filesystem policy prohibits writes"* — a review that
  completed, found things, and reported none of them, which step 3 reads as a missing source.
  `codex-cli-runtime` defaults to *dropping* `--write` for review, diagnosis and research, so a review brief
  lands squarely in that exclusion; appending it explicitly is what overrides the default. It is not a licence
  to edit — "Read-only, every reviewer run" above still holds and the brief must keep saying so; the
  findings file is the one thing it may write.
- **The writable root is the launching working directory, not the path named in the brief.** Codex's
  `workspace-write` covers the git toplevel of the directory the companion was started in, plus `$TMPDIR`.
  Telling it in prose to "work in worktree X" moves nothing. Dispatch the Codex brief with the working
  directory already inside the same tree as `<run-dir>` — reviewing a sibling worktree from elsewhere
  burns a full review and then dies with `EPERM` on the findings file.

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
mkit findings reconcile <run-dir> --sources-expected <N> --json
```

`N` is the number of reviewers you **launched**, not the number that answered — it is what switches the
weak-singleton drop rule off when a source is missing. That follows the mode recorded in `scope.md`: **2** for
quick (CodeRabbit + Codex), **3** for full. **Never create an empty `findings-<source>.jsonl` to
make the count line up**: `mkit findings` reads a present file as that source reporting zero, which re-arms the drop
rule and silently deletes exactly the single-source findings the missing reviewer would have corroborated. A
source that never wrote a file is missing; lower `N` and say so. Then read `triage-reconcile.md` and settle the two things the
binary deliberately leaves open, because both are judgement and it owns mechanical invariants only:

- **`low_sim`** — a merge whose two members barely share wording. They were merged on location, not text.
  Read both titles (the merged-away one rides along in `also`) and check it really is one problem; split it
  in `reconciled.jsonl` if it is two.
- **`review_pairs`** — same file, similar wording, different lines. One shape at two sites, or two findings?
  Your call, and the binary does not make it.

No subagent: the merge is arithmetic, and forming an opinion here contaminates the set the verifier is handed.

If a reviewer wrote prose instead of JSONL, `mkit findings validate <run-dir>` names the lines; convert it
here rather than re-spawning.

## 4. Verify the survivors

```bash
mkit findings group <run-dir> --json
```

It groups by directory, folds groups too small to be worth a round trip, writes `verify-<group>.jsonl`, and
reports `suggest`. `suggest` is the documented threshold applied, not a decision: `inline` means a handful of
findings in one group — verify them here and write `verdicts-all.jsonl`, since a lone subagent buys
independence you already have; `fanout` means one verifier per group. Either way the call is yours. On fan-out, spawn **one subagent per group in a single message**; each gets only
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
mkit findings report <run-dir> --json
```

It merges the verdicts onto the findings (applying `refined` corrections), writes `final.jsonl`, and returns
the counts, the reportable set, the gating count, and — the ones that matter — `unverified` ids and
`orphans`. **Never write the summary while `unverified` is non-empty**: a lost verdict reads exactly like a
finding nobody raised. Go back and verify those ids — inline is fine, it is a handful by definition — and
re-run `report`. Only an id whose verifier cannot be re-run at all is reported unverified, named individually
and above the counts, never folded into them.

Then the prose, in this order. It is a **decision brief**: the reader has not seen the run and must be able to
tell, from this message alone, what was found, what was done about each finding, and what is still open. The
record is the run directory — name that path once — but a bare count or a bare title is not a summary. Every
finding the user could ask "what was that?" about gets a self-contained entry (what, where, why it matters).

1. **Verdict** — one or two sentences first: how many real findings, how many fixed, how many need a
   decision, and whether anything is blocking. Then the rest.
2. **Mode** — which mode ran, and which reviewers/lenses were **not run by design**, e.g. "Mode: quick —
   CodeRabbit + Codex (bugs, impl); architecture/quality/tests/docs/comments and adversarial not run." Word it
   so it reads distinctly from item 3 below: a deliberate narrower scope, never a partial or degraded full run.
3. **Completeness** — sources expected vs reported. A failed source goes above every finding.
4. **Scope** — range reviewed, shortstat, which reviewers actually ran (and lenses no source carried).
5. **Counts** — one line, by tag: "2 `[code, major]`, 1 `[docs, minor]`". Never a bare "3 findings".
6. **Findings** — a table first, one row per reportable finding: `id | [surface, severity] | file:line |
   title | status` where status is `fixed`, `needs decision` or `skipped`. Open questions get no row: they are not
   findings, and item 10 lists them. Then, grouped by
   severity, each finding headed `F<id> [surface, severity] Title` (the id as it appears in the table) with **trigger, consequence,
   and what was done or proposed**, in full sentences. Mark any finding more than one source raised, and any
   that was `refined`. This is the one place a finding's body appears; later sections refer to it by id.
7. **Fixed automatically** — per fix: the id, the file, what changed, and why it was safe to apply unasked.
8. **Needs your decision** — each risky finding with the question phrased as a choice: the proposed fix, the
   alternative if there is one, and what happens if left alone. The written entry stays in the summary; the
   ask itself goes through the interactive prompt described below.
9. **Considered, not changed** — each skipped finding and `immaterial` verdict: id, title, and the concrete
   reason it was left (not "immaterial" alone).
10. **Open questions** — things the review could not settle: a reviewer's unverified doubt, an assumption about
    intent, behaviour that depends on something outside the diff. Each names what is unknown, why it matters,
    and who or what could answer it. Always print the section; write "none" if empty, so its absence is never
    ambiguous. Keep **pre-existing** problems in their own separate section, out of the counts, one entry each
    with file:line.
11. **Run directory** — the `<run-dir>` path, and whether a second round ran (and its result) or was
    skipped. Say plainly that a fixed tree is unverified here — `review` does not build or test it;
    that check runs at `pr`/`finish`.

Do not repeat a finding's body in a later section — refer to it by id — but do not shrink an entry to a title
either. End on the decision the user has to make (or "nothing needed from you" if nothing is open) — findings
without an ask is the middle of the job, not the end.

**Ask through `AskUserQuestion` when the host has it**, after the summary is printed, never instead of it.
One question per finding needing a decision (batch up to four per call, more in further calls), header = the
finding id, options = the proposed fix (marked recommended, first), the alternative, and `leave as is`, each
with a one-line description of its consequence. Put the diff of a proposed fix in an option's `preview` where
it fits. The question text names the finding and the file, so it reads alone. **No such tool** (another host,
or a non-interactive run) → end with the same choices as a numbered list per finding, recommended first, and
ask the user to reply with the numbers. The summary is complete either way; the prompt only collects answers.

**Format it to read the same in the Claude app and in a terminal** — both render Markdown, and a terminal
does not reflow wide or nested structure:

- One `##` heading per section, in the order above, with a blank line around it. Open with the verdict under
  no heading, as a single bold line: `**Verdict:** 3 findings — 2 fixed, 1 needs your decision.`
- **Tables only for short cells** — the findings table and the counts. Keep each cell to a few words (id,
  tag, `path:line`, a short title, status); prose never goes in a cell, since a terminal wraps a long row into
  an unreadable smear. Keep the table under about 100 columns; drop a column before wrapping one.
- A finding's full entry is a `###` heading (`### F1 [code, major] Title`) followed by a `path:line · conf N`
  line and three labelled bullets — **Trigger**, **Consequence**, **Fix** — each one or two sentences. No
  nesting past one level.
- Code and paths in backticks; a code fence only for a real snippet or diff, never to make text stand out.
- Use plain text status words (`fixed`, `needs decision`, `skipped`), not colour emoji — they render at
  different widths and carry no meaning a screen reader or a pipe can keep. A single leading marker such as
  `✓` or `?` is fine where it saves a word.
- No HTML, no horizontal rules between every section, no bold on whole sentences. Bold marks the verdict,
  the bullet labels and the final ask — nothing else.
- The closing ask is its own short paragraph at the very end, after the run directory line, so it is the last
  thing on screen.

Then record the run:

```bash
mkit worklog append --step review --gist '<one line: what this review concluded>' \
  --artifact '<run-dir>' [--assume '<what this run derived rather than found>']...
```

The run directory is the right `--artifact` here — it is what this step produced — but it is **perishable**:
step 6 folds in `mkit scratch prune`, which keeps the newest five per skill and additionally spares anything
touched in the last 60 minutes, so the path survives an unpredictable number of later reviews and then stops
existing. That is expected; the gist carries the conclusion. Name the reviewed range in the gist so the record
still says what it covered once the directory is gone.

After the summary is produced, and it never changes the summary: a failed append is one line of note, not a
failed run. A run that found nothing still records — "no findings" is the most useful gist this step
produces. The `--assume` list is where a derived goal goes, so the next step inherits the caveat instead of
re-deriving it.

**Whenever the fingerprinted tree is not the content the reviewers read, say so in an `--assume`.** The
fingerprint is always of the working tree at the moment of the append, and a later step that matches it reads
"reviewed" — over content no reviewer necessarily saw. Two things open that gap, and the second is the easier
one to miss:

- **Step 5 applied fixes.** The fingerprint is the tree *after* them; the reviewers ran before. Which
  sentence depends on the second round: with none, `--assume 'fixes applied after the reviewers ran; the
  fixed tree is unverified here'`; with one, name what it actually covered — `--assume 'fixes re-reviewed by
  <sources> over the fixed files only'`.
- **The review was of a committed range while the tree was dirty.** The reviewers read commits; the
  fingerprint is of a working tree carrying edits they never saw, whether or not this run touched anything.
  Name the scope: `--assume 'reviewed <range> only; the tree also carried uncommitted changes at this
  fingerprint'`.

Both are the same failure — a fingerprint standing for more than was examined — and both are what item 11 of
the summary already says out loud. A record that states its scope lets `finish` match the fingerprint without
concluding more than the run proved.

Do not commit unless asked — leave fixes in the working tree for the user to commit (or chain into `commit`).
Fold `mkit scratch prune` into step 6's call rather than spending a turn on it.

## Git safety

Follow `git-safety.md`. Apply only fixes to the working tree; never rewrite pushed history, never force-push,
never commit or push as a side effect of the review unless explicitly asked. Everything this skill writes of
its own goes in `run=` (`<toplevel>/.mkit/…`, which is ignored) or in `tmp=` if it dies with the command —
never anywhere else in the working tree. `output-discipline.md` has the rule; a fix applied to a source file
is the user's change, not the skill's scratch.
