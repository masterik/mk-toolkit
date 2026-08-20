# Reconcile — merge the reviewers' lists

Stage 1 of finding triage in `review-changes`: several reviewers wrote `findings-*.md` independently; merge
them into one set with stable ids. Verifying (`triage-verify.md`) and fixing (`fix-checks.md`) are later
stages — you do neither, and you add no findings of your own.

**Work from the findings text alone. Do not open files, run `git diff` or `rg`, or look at the code.** Which
findings are the same issue, which sources raised each, and which singletons are too weak are all answerable
from the text. Verification comes next with the code in front of it; an opinion formed here contaminates the
set the verifier is handed.

1. **Split out what is not a defect in the change.** A question a reviewer could not answer from the code →
   **open questions**; a defect in untouched code → **pre-existing**. Move both out first: neither is deduped,
   boosted or dropped.
2. **Deduplicate.** Same issue = same file **within two lines** and the same problem. Merge into one, keep the
   clearest title and body, list every source and lens on the survivor.
3. **Boost corroboration.** `confidence = min(99, highest confidence + 10 × (distinct sources − 1))`.
   **A source is a process, not a lens** — one reviewer raising it under two of its lenses is one source, no boost.
4. **Severity is the highest any input claimed.**
5. **Drop a finding with a single source, confidence below 80, and nothing corroborating it** — but **never a
   critical or major.** One reviewer looking in the right place is the normal case for the worst bugs; keep it
   and let verification decide.
   **If any source was missing or degraded, drop nothing** — corroboration is rarer with a source gone, so the
   rule starts eating exactly the findings the missing source would have confirmed.

Give every survivor a stable id. Severity, surfaces and the reviewer contract: `review-severity.md`.
