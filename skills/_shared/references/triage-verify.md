# Verify — one verdict per finding

Stage 2 of finding triage in `review-changes`: the reconciled findings now get judged against the
code. Reconciling happened before you (`triage-reconcile.md`) and fixing happens after
(`fix-checks.md`) — you do neither.

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
- **You are given only your own group's findings** — a verifier that sees the whole set anchors on it.
  Do not go looking for the rest.
- A finding whose location is missing or wrong: find what it points at and **refine** it with the real
  location, or reject it when the code supports none.
- A finding that contradicts the project's own conventions is **wrong**, not right.

Severity, surfaces and the reviewer contract are in `review-severity.md`.

## The materiality test

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
