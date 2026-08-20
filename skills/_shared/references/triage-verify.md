# Verify — one verdict per finding

Stage 2 of finding triage in `review`: judge the reconciled findings against the code. Reconciling
happened before you (`triage-reconcile.md`), fixing happens after (`fix-checks.md`) — you do neither.

Open the file at the line named, read enough around it to judge, return exactly **one** verdict:

| verdict | means |
| --- | --- |
| **confirmed** | real as described |
| **refined** | real, but description, location, severity or confidence is wrong — return corrected values; omitted fields keep their original |
| **rejected** | not real: the code already handles it, the reviewer misread it, or the claimed trigger cannot occur |
| **immaterial** | accurate, and still not worth acting on |
| **pre_existing** | real, but in code the change did not touch |

Rules that make the pass worth running:

- **Judge the finding, not the reviewer.** Confidence is not evidence; hedging is not grounds for rejection.
  Where the code contradicts the finding, say so and reject.
- **Do not hunt for new problems.** Verification is not a second review.
- **You are given only your own group's findings** — seeing the whole set anchors you. Do not go looking for
  the rest.
- Missing or wrong location: find what it points at and **refine** with the real location, or reject when the
  code supports none.
- A finding that contradicts the project's own conventions is **wrong**, not right.

Severity, surfaces and the reviewer contract: `review-severity.md`.

## The materiality test

Apply **only after confirming the problem is real** — `immaterial` is not a softer `rejected`, and a wrong
finding is rejected, not dismissed as minor.

1. **Can it happen?** Name the input or state that triggers it. A path no caller reaches, a branch guarded
   upstream, or a condition the type system excludes is immaterial.
2. **Does it matter when it happens?** Name the consequence — wrong output, data loss, a crash, a security
   hole, a maintainer misled. An outcome nobody would observe is immaterial.
3. **Is the fix worth it?** Severity measures the value of fixing; **blast radius measures the cost.** Name
   the fix, then its reach: does it stay at the finding's site, or edit shared code, alter a signature,
   restructure control flow, change what callers see? A restructuring larger than the problem it removes is
   immaterial at any severity, and a `minor` whose fix reaches well beyond its site is how a nit becomes a
   regression.

   **A comparison, not a checklist — reach alone never decides it.** Most real fixes add a branch: an error
   now checked, a nil now guarded, a boundary now correct. That is what a minor finding usually is —
   **confirm them.** Question 2 already established someone suffers the consequence, so a proportionate fix
   is worth making however small the defect. Dismiss only when cost genuinely outweighs what question 2
   named, and **say what the cost was**.

Surviving all three → `confirmed` or `refined`. Style preferences, hypothetical futures and restatements of
the code as written are immaterial by definition.
