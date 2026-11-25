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

A multi-source review of local work: independent reviewers find, this session reconciles, verifiers judge,
this session fixes safe findings and asks on risky ones, then reports. Pipeline: **find → reconcile → verify →
fix → report** — never collapse stages (a reviewer that fixes anchors on itself; a fixer that decides what is
real rejects nothing). `review` runs no gate.

**Read nothing up front.** `<refs>` is the absolute path of `../_shared/references/`, resolved against this
skill's directory; reviewers and verifiers get reference paths under it and read them themselves
(`../_shared/references/agent-delegation.md` has the brief rules). **Never read a diff into this session.**

Every finding carries `[surface, severity]`: surface `code` · `comments` · `docs` · `tests` ·
`config`/`build`; severity `critical` (data loss, security hole, crash on a reachable path) · `major` (wrong
runtime behaviour, broken contract) · `minor` (real defect, contained impact; prose defects are always minor).
Counts are always by tag.

| stage | who | enters this session |
| --- | --- | --- |
| 1 scope | this session, one `facts.sh` call | run dir, stat, file list |
| 2 find | 2 (quick) or 3 (full) subagents, parallel, strongest model | ≤10 lines each: path written, counts by tag, lenses not covered |
| 3 reconcile | this session, by `triage-reconcile.md` | the capped findings; out: `reconciled.md` |
| 4 verify | 1 subagent per batch (strongest model), or this session on a handful | one line per finding: id + verdict |
| 5 fix | this session | the findings acted on, in full |
| 6 report | this session | the summary |

## 1. Scope

**What**: uncommitted changes (default when dirty) or commits (`HEAD~3..HEAD`, `<base>..HEAD`). Both in play →
confirm which, or review both and note the split. Ask only if genuinely ambiguous.

**Mode**: **full** (three reviewers, eight lenses — the default) or **quick** (CodeRabbit + Codex on
`bugs`/`impl`; no Claude craft reviewer, no `adversarial`). In order: the text after the skill name (`quick`/`full`); an
explicit signal ("quick review", "fast pass", "just check for bugs"); `docs/agents/review.md` naming a mode
(`../_shared/references/repo-settings.md` — a default, never a ceiling; say when it decided; a mode it names
that `review` doesn't know is reported in one line, then the run carries on as without it); else full. Quick is
a deliberate narrower scope, never described as a degraded run.

Then one call; the path is relative to this skill's directory — resolve it and run it absolute
(`../_shared/references/output-discipline.md`):

```bash
bash ../_shared/scripts/facts.sh review --run --range <range>
```

Omit `--range` for a dirty tree. Keep `run_dir=` as `<run-dir>` and `<refs>` for every later step and brief
(re-running `--run` opens a second directory).

**Goal** (one or two lines), in order: the user or a spec/plan agreed this session; the branch name; the
commit messages; the ticket. Say which; with none, say so and expect lower confidence
(`../_shared/references/workflow-contract.md`, rule 3).

Write `<run-dir>/scope.md`: range, the command producing the diff, stat, file list, goal, **mode**.

## 2. Run the reviewers in parallel

Read [reviewers.md](reviewers.md) now — the roster per mode, each brief, the findings file shape, Codex's
invocation, fallback and the wait bound — then spawn every reviewer **in one message**. All are **read-only**.

## 3. Reconcile

Read `<refs>/triage-reconcile.md`, then every `findings-<source>.md` that exists. Apply **its** rule — the only
dedupe rule — and write `<run-dir>/reconciled.md`. No subagent, no reading the code. In doubt whether two
entries are one problem, keep both and flag.

**Count sources honestly**: expected = reviewers **launched** (2 quick, 3 full, per `scope.md`). A source
with no file is missing; never create an empty file to make the count line up.

## 4. Verify

Batch the `F` findings **by file** (a file's findings stay together; fold small files until each batch holds
≥3; ≤~8 batches). About five or fewer in total → verify here after reading `<refs>/triage-verify.md`, into
`<run-dir>/verdicts-all.md`. Otherwise write each batch to `<run-dir>/verify-<batch>.md` (ids, locations,
every source's wording, reviewers, flags) and spawn **one subagent per batch in one message**, each given only
its batch file, `scope.md` and `<refs>/triage-verify.md`.

Each writes `<run-dir>/verdicts-<batch>.md` and returns one line per finding:
`id → confirmed | refined | rejected | immaterial | pre_existing | duplicate of F<n>` (corrected fields on
`refined`, one clause on `rejected`/`immaterial`). Verifiers **look for no new problems**.

`rejected` leaves the report; `duplicate of F<n>` folds into `F<n>` (reviewers join); `immaterial` and
`pre_existing` get their own sections, **out of the counts**.

## 5. Fix — auto-fix safe, ask on risky

Read bodies from `reconciled.md` **only for findings being acted on**.

- **Safe → fix**: unambiguous, low-risk, behaviour-preserving (typo, a clearly correct missing check, an
  obvious off-by-one, dead code, lint, a missing `await`, a stale comment, a tighter type).
- **Risky → ask first**: changes behaviour, public API/contracts, data/migrations, concurrency or security
  posture, or the fix is a judgement call or a broad refactor.
- **Not worth fixing → skip**, one-line reason.

Read `<refs>/fix-checks.md` before the first fix; **every fix passes its three checks**. The shape sweep is
one `rg` call here when the shape greps; delegate to a cheaper-model subagent (file:line list back) only when it
needs judgement to recognise or spans the whole repo. Group related safe fixes, in the surrounding style.

**Optional second round** over the fixes: a new run directory, `bugs` + `impl` only, nothing below `major`,
never a `docs` pass over prose you just wrote, and reviewers never see the previous round's findings.

## 6. Report

Apply the verdicts onto `reconciled.md` (refine, drop `rejected`, fold duplicates, split `F3a`/`F3b`). **Every
`F` id needs exactly one verdict**: an id with none goes back to verification; only one that cannot be
re-verified is reported unverified, named above the counts.

Then write the closing message by [closing.md](closing.md).

Do not commit unless asked. Follow `../_shared/references/git-safety.md`: fixes go to the working tree only;
never rewrite pushed history, force-push, commit or push as a side effect. This skill's own files go in
`<run-dir>` or `$TMPDIR`, never the working tree.
