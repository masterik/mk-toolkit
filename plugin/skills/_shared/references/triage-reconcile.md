# Reconcile — merge the reviewers' lists

Stage 1 of finding triage in `review`: several reviewers wrote `findings-<source>.md` independently;
merge them into one list with stable ids. Verifying (`triage-verify.md`) and fixing
(`fix-checks.md`) are later stages — you do neither, and you add no findings of your own.

This is **the** dedupe rule; nothing else restates it. It runs in the main session, in prose: no
command, no subagent, and **no reading the code**. Which findings describe the same issue is
answerable from the entries themselves, and an opinion about the code formed here contaminates the
list the verifier is handed. Doubt goes forward as a flag, never as a silent merge.

## The finding shape

Each entry carries `title`, `file`, `line` (a single line or a range `42-48`), `surface` (the
category: `code` · `comments` · `docs` · `tests` · `config`/`build`), `severity`, `confidence`,
`lens`, `class` (`finding`, the default · `open_question` · `pre_existing`), `body` (trigger +
consequence) and `fix`. `review`'s step 2 tells reviewers how to write it. A reviewer that wrote
something looser — free prose, a different layout — is read for the same fields, never re-spawned.

## The rule

1. **Set aside what is not a defect in the change.** `open_question` and `pre_existing` entries get
   `X`-ids (`X1`, `X2`, …), go straight to their own report sections, and are never merged or flagged.
2. **Merge** two findings when all three hold: **same file**, **overlapping line ranges** (a single
   line is a range of one), **same surface**. Wording does not gate it — two reviewers describing one
   missing `await` rarely share their phrasing. The merged finding:
   - keeps **every source's text** — title, body and fix from each, labelled by source, nothing
     paraphrased away (a merge that was really two problems stays visible to the verifier);
   - takes the **highest severity** and the **highest confidence** any source claimed;
   - **lists the reviewers** that raised it — agreement between independent reviewers is signal, and
     the report says so. One reviewer raising it under two lenses is still one source.
   - spans the union of the line ranges.
3. **Flag, don't merge, a near-duplicate**: same file, and either overlapping ranges with a different
   surface, or ranges not overlapping but within about **5 lines** of each other (any surface). Keep both,
   and mark the later one `possible duplicate of F<n>`.
   The verifier settles it with the code in front of it.
4. **Number the survivors** `F1`, `F2`, … ordered by severity (critical → major → minor), then file,
   then line. The ids are what verdicts, fixes and the report refer to from here on.

Missing location (no file, or no line): never merged or flagged — it goes forward alone and the
verifier locates it. Same file, far apart, similar wording is one defect shape at two sites or two
findings; either way it is not a merge — keep both, and let `fix-checks.md`'s sweep find the shape.

## Write it down

Write the merged list to `<run-dir>/reconciled.md` — one entry per id, in the shape above plus
`sources` and any `possible duplicate of` flag; then the `X` entries. It is the record a later stage
reads bodies back from, and an audit trail: no code parses it.

Keep only the tally in this session: entries in, merges (which sources each id absorbed), flagged
near-duplicates, entries set aside — what step 6 reports. Severity, surfaces and the reviewer
contract: `review-severity.md`.
