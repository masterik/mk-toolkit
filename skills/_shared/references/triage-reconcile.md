# Reconcile — merge the reviewers' lists

Stage 1 of finding triage in `review-changes`: several reviewers have written `findings-*.md`
independently; this stage merges them into one set with stable ids. Verification is a separate
stage (`triage-verify.md`) and fixing a third (`fix-checks.md`) — you do neither.

**Work from the findings text alone. Do not open files, do not run `git diff` or `rg`, do not go looking
at the code.** Which findings are the same issue, which sources raised each, and which singletons are too
weak to keep are all answerable from the text. Verification comes next, with the code in front of it —
forming an opinion here contaminates the set the verifier is handed. Reconciling adds no findings of its
own.

1. **Split out what is not a defect in the change.** A question a reviewer could not answer from the code
   goes to **open questions**; a defect in code the change did not touch goes to **pre-existing**. Move
   both out first — neither is deduped, boosted or dropped.
2. **Deduplicate.** Two findings are the same when they name the same file **within two lines** of each
   other and describe the same problem. Merge into one, keeping the clearest title and body, and list
   every source and lens on the survivor.
3. **Boost corroboration.** `confidence = min(99, highest confidence + 10 × (distinct sources − 1))`.
   **A source is a process, not a lens** — one reviewer reporting the same problem under two of its
   lenses is still one source and earns no boost.
4. **Severity is the highest any input claimed.**
5. **Drop a finding that has a single source, confidence below 80, and nothing corroborating it** — but
   **never drop a critical or a major this way.** A single source is not evidence against a serious
   defect: one reviewer looking in the right place is the normal case for the worst bugs. Keep it and let
   verification decide.
   **If any source was missing or degraded, drop nothing.** Corroboration is rarer with a source gone, so
   the drop rule starts eating exactly the findings the missing source would have confirmed.

Give every surviving finding a stable id. Severity, surfaces and the reviewer contract are in
`review-severity.md`.
