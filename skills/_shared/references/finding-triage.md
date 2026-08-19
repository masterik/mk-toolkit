# Finding triage — reconcile, verify, fix

What happens to findings **after** the reviewers return them: merge the lists, decide which findings are
real and worth acting on, then fix them without leaving the same defect three lines away.

Used by `review-changes`. Severity, surfaces and the reviewer contract are in `review-severity.md`.

## 1. Reconcile — merge the lists

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

## 2. Verify — one verdict per finding

Open the file at the line named and read enough around it to judge. Then return exactly **one** verdict:

| verdict | means |
| --- | --- |
| **confirmed** | the problem is real as described |
| **refined** | real, but the description, location, severity or confidence is wrong — return the corrected values; every field omitted keeps its original |
| **rejected** | not real: the code already handles it, the reviewer misread it, or the claimed trigger cannot occur |
| **immaterial** | accurate, and still not worth acting on |
| **pre_existing** | real, but present in code the change did not touch |

Rules that make verification worth the pass:

- **Judge the finding, not the reviewer.** A confident description is not evidence; a hedged one is not a
  reason to reject. Where the code contradicts the finding, say so and reject it.
- **Do not hunt for new problems.** Verification is not a second review.
- **Verify in groups, and give each group only its own findings** — a verifier that sees the whole set
  anchors on it. Group by directory or by file.
- A finding whose location is missing or wrong: find what it points at and **refine** it with the real
  location, or reject it when the code supports none.
- A finding that contradicts the project's own conventions is **wrong**, not right.

### The materiality test

Apply it **only after confirming the problem is real** — `immaterial` is not a softer `rejected`, and a
wrong finding is rejected rather than dismissed as minor.

1. **Can it happen?** Name the input or state that triggers it. A path no caller reaches, a branch guarded
   upstream, or a condition the type system excludes is immaterial.
2. **Does it matter when it happens?** Name the consequence — wrong output, data loss, a crash, a security
   hole, a maintainer misled. An outcome nobody would observe is immaterial.
3. **Is the fix worth it?** Severity measures the value of fixing; **the fix's blast radius measures what
   fixing costs.** Name the fix, then say how far it reaches: does it stay at the finding's own site, or
   does it edit shared code, alter a signature, restructure control flow, change what callers see? A
   restructuring larger than the problem it removes is immaterial at any severity, and a `minor` whose fix
   reaches well beyond its own site is how a nit becomes a regression.

   **This is a comparison, not a checklist, and reach alone never decides it.** Most real fixes add a
   branch — an error now checked, a nil now guarded, a boundary now correct. Those change control flow and
   are exactly what a minor finding usually is: **confirm them.** Question 2 already established that
   someone suffers the consequence, so a fix proportionate to it is worth making however small the defect.
   Dismiss only when the cost genuinely outweighs what question 2 named, and **say what the cost was**.

A finding that survives all three is `confirmed` or `refined`. Style preferences, hypothetical futures and
restatements of the code as written are immaterial by definition.

## 3. Fix — three checks before the fix is done

Findings arrive as a site; defects live as a shape. Run these in order, on every fix.

**a. Sweep for the shape, not the site.** Name the defect as a *construct* — "a switch over a three-value
enum that only tests one end", "a sentence asserting a shape regardless of the counts", "a phrase that
must match a list beside it" — then **grep the repo for that construct and fix every occurrence in the
same change**. Grep for the pattern, not the literal string: a phrase wrapped across a line break
survives a search for the phrase. This check catches what the other two cannot; knowing the general rule
is not a substitute for running the search, and the identical defect four lines below in the same hunk is
the normal miss.

**b. Enumerate what you touched.** Name the input space of the thing you changed and confirm every member
is handled **and that a test tells them apart**:

- an enum or set of string constants — every value, and every transition between them if direction matters
- a struct being folded, copied or serialized — every field, not just the ones the change was about
- a platform, filesystem or executor the code branches on — each branch
- an error class the code distinguishes — absent, unreadable, malformed, present-but-empty
- a ratio — that numerator and denominator are drawn from the same population

The recurring failure is not carelessness on many fronts: it is writing the fix for the case that prompted
it, plus a test pinning that same case, so the test cannot catch what was left out.

**c. Re-read the finding, then read your fix against it.** Fix **the mechanism the finding names**, not
the example it used to illustrate it. A finding that says "the switch cannot express a change that skips
`minor`" is not answered by adding one more case to the switch.

Then run the repo's check (`quality-gate.md`). **A fix that breaks the build is not committed.**

## What gates, if a skill needs a gate

`critical` and `major` in the confirmed findings. Nothing else:

- `minor` never gates — it is what a review is expected to leave behind.
- `open_questions` want a decision from the user, not an edit. Never edit code to resolve one.
- `pre_existing` and `immaterial` are real and are not this change's business.
- **A degraded run gates nothing either way** — it is not evidence. Report it and stop.
