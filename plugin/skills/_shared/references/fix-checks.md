# Fix — three checks before a fix is done, and what gates

Stage 3 of finding triage in `review`, run in the main session: verified findings get acted on.
Reconciling (`triage-reconcile.md`) and verifying (`triage-verify.md`) happened before you.

Findings arrive as a site; defects live as a shape. Run these in order, on every fix.

**a. Sweep for the shape, not the site.** Name the defect as a *construct* — "a switch over a three-value enum
that only tests one end", "a sentence asserting a shape regardless of the counts", "a phrase that must match a
list beside it" — then **search the repo for that construct and fix every occurrence in the same change**.
Grep the pattern, not the literal string: a phrase wrapped across a line break survives a search for the
phrase. This check catches what the other two cannot; knowing the general rule is no substitute for running
the search, and the identical defect four lines below in the same hunk is the normal miss.

**b. Enumerate what you touched.** Name the input space of what you changed; confirm every member is handled
**and that a test tells them apart**:

- an enum or set of string constants — every value, and every transition if direction matters
- a struct being folded, copied or serialized — every field, not just the ones the change was about
- a platform, filesystem or executor the code branches on — each branch
- an error class the code distinguishes — absent, unreadable, malformed, present-but-empty
- a ratio — numerator and denominator drawn from the same population

The recurring failure is not carelessness on many fronts: it is writing the fix for the case that prompted it,
plus a test pinning that same case, so the test cannot catch what was left out.

**c. Re-read the finding, then read your fix against it.** Fix **the mechanism the finding names**, not the
example illustrating it. "The switch cannot express a change that skips `minor`" is not answered by adding one
more case to the switch.

Verification is `pr`/`finish`'s job, not this one's — `review` does not re-run the repo's check after a
fix (`quality-gate.md`). Leave the fix in the working tree; the gate runs before the next merge/PR.

## What gates, if a skill needs a gate

`critical` and `major` in the confirmed findings. Nothing else:

- `minor` never gates — it is what a review is expected to leave behind.
- `open_questions` want a decision from the user, not an edit. Never edit code to resolve one.
- `pre_existing` and `immaterial` are real, and not this change's business.
- **A degraded run gates nothing either way** — it is not evidence. Report it and stop.
