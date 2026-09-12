# Reconcile — merge the reviewers' lists

Stage 1 of finding triage in `review`: several reviewers wrote `findings-*.jsonl` independently;
merge them into one set with stable ids. Verifying (`triage-verify.md`) and fixing
(`fix-checks.md`) are later stages — you do neither, and you add no findings of your own.

## Run it

```bash
mkit findings reconcile <run-dir> --sources-expected 3 --json
```

It writes `<run-dir>/reconciled.jsonl` and returns the counts, every merge, every drop and every
pair it refused to decide. Both comparisons are *head-relative* — every candidate is measured
against each cluster's first member, never against the ones already folded into it. A third report
of the same defect, too far out to merge, stays its own finding and comes back in `review_pairs`
when its text resembles that head; when it resembles a later member instead, nothing flags it and
only reading the set will find it.

That is the whole stage — **no subagent, and no reading the code**:
which findings are the same issue and which singletons are too weak are answerable from the
text, and an opinion formed here contaminates the set the verifier is handed.

What the binary does, so you can check its work rather than repeat it:

1. **Splits out what is not a defect in the change** — `class: open_question` and
   `class: pre_existing` move aside first, get `x`-ids, and are never deduped, boosted or dropped.
2. **Merges on the documented key**: same file, within ±2 lines. Text similarity does not gate the
   merge (two reviewers describing one missing `await` at lines 42 and 43 share a quarter of their
   tokens, so any gate safe enough to trust would let every real duplicate through); every member's
   title and body survives on the winner
   (`also`), so a merge that was really two problems is still visible to the verifier.
3. **Boosts corroboration**: `confidence = min(99, highest + 10 × (distinct sources − 1))`. A
   source is a *process*, not a lens — one reviewer raising it under two lenses is one source.
4. **Severity is the highest any input claimed.**
5. **Drops a finding with a single source, confidence below 80, and minor severity** — never a
   critical or major, and **nothing at all when `sources_present < sources_expected`**: with a
   source gone the rule would eat exactly the findings that source would have corroborated.
6. **Stable ids** on every survivor, ordered by severity, confidence, then location.

## What is still yours

- `low_sim` on a merge — the locations matched but the wordings barely overlap. The two were merged
  on location, not text. Read both titles (the merged-away one rides along in `also`) and check it
  is one problem; if it is two, split it in `reconciled.jsonl`, say so in the summary, and treat
  them as two.
- `review_pairs` — same file, similar wording, different lines. One defect shape at two sites, which
  `fix-checks.md` sweeps for, or two findings? Your call. The binary never resolves these, on
  purpose: it owns the mechanical invariants and this one is judgement.
- `suggest` from `mkit findings group` — the documented threshold applied, still a suggestion.
- Anything a reviewer mislabelled: a "finding" that is plainly a question, a `pre_existing` that
  the diff actually touched.

Pass `--sources-expected` honestly: it is what switches the drop rule off, and the count of
reviewers you *launched*, not the count that answered. A `findings-<source>.jsonl` that exists
but is empty counts as that source reporting **zero**; a file that does not exist is a source
that did not report. Never write an empty one to make the arithmetic line up — that re-arms the
drop rule on exactly the findings the missing source would have corroborated.

Severity, surfaces and the reviewer contract: `review-severity.md`. The record shape:
`mkit findings schema` (`--json` for the machine-readable form).

`mkit` is a separate install from this plugin, so `review` checks it is there at step 0 before any
reviewer runs. If `mkit findings` is missing here, stop and say so rather than doing the arithmetic
by hand — `brew install masterik/tap/mkit`, or `brew upgrade mkit`.
