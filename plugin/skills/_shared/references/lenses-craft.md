# Review lenses — craft set

Five reading stances: `architecture`, `quality`, `tests`, `docs`, `comments`. A lens is **what a reviewer goes
looking for** — it decides where to look and how hard to push, never what severity to give (that is
`review-severity.md`, the same bar for every lens).

This set goes to the **Claude subagent reviewer** in `review` — promptable, and it needs repo context
the other reviewers do not gather. The correctness set (`bugs`, `impl`, `adversarial`) is in
`lenses-correctness.md`, carried by a different reviewer; you do not carry it. Lenses are redistributed when a
reviewer is missing, so you may be handed both files — then cover both sets and say so.

Tag every finding with the lens that raised it: that is what makes coverage gaps visible in the summary.

---

## architecture — the project's own conventions

You are **not** proposing an architecture or suggesting a library. You are finding where the change departs
from what this codebase and its written rules already settled.

Read the project's convention docs first (`CLAUDE.md`/`AGENTS.md`, design notes), plus two or three
neighbouring files in each package the change touches.

- organization: code in the wrong package, a new grab-bag helpers package, a test file split from its source
- dependencies constructed inline where neighbours inject them; new package-level mutable state in code that
  had none
- interface shape: wide where the codebase keeps them narrow, declared beside its implementation rather than
  its caller, a concrete type where the caller should be free to substitute
- a surface existing only for tests — an exported symbol with no caller outside tests. Run the search, put
  what it returned in the finding
- error wrapping, logging, naming, control flow contradicting what the same package does everywhere else
- an old symbol kept for compatibility after every caller moved to its replacement
- a decision this project already settled, re-opened without saying so

**Every finding cites its authority** — the rule it breaks, or a file and line showing the pattern it
contradicts. No citation means a preference; leave it out. Where the project's stated rules and your taste
disagree, **the project wins**.

## quality — code that works and could be written better

Judge bottom-up, on what is in front of you, not against the goal.

- non-idiomatic constructs the language treats as such; a boolean argument nobody can read at the call site
- structure that fights the reader: a function past one screen, nesting deeper than three levels where an
  early return would flatten it, a branch after a return that cannot be taken
- abstraction with nothing behind it: a wrapper that only forwards, a factory serving one implementation, a
  settings object for two options
- generality for a case that does not exist — an extension point nothing extends, a fallback that cannot
  trigger, a second implementation kept beside the one actually used
- errors discarded with no reason given, an empty failure branch, a default returned after a failure without
  a word about it; an error wrapped so the original can no longer be inspected
- duplication the change introduces — **cite the existing copy** so the finding is actionable

Duplication is often deliberate and right: two occurrences are not yet a pattern, tests repeat setup to stay
readable, and repetition keeping two modules independent buys something real. Flag it only when you can name
the shared form and show it costs nothing.

## tests — whether they would catch anything

Review the tests, not the production code behind them. Read a neighbouring test file first, so what you call a
convention is this project's convention.

**A missing test is a finding only when you can name the defect it would catch.** "This has no test" is not a
finding; "nothing would catch that an empty input returns the previous result" is. Exception: a project whose
own rules require tests for new behavior — reportable on the rule alone.

- a fix with no test that fails without it
- a test that would still pass if the code it covers were deleted or inverted — assertions on values the test
  supplied, on a stub it configured, or behind a condition that quietly skips them
- ignored failures, disabled cases, a skip whose stated reason no longer holds
- state shared between tests, an assumption about run order, a sleep standing in for synchronization
- concurrency added by the change with nothing exercising it under a race detector
- assertions on which internal calls happened where the observable result is what matters
- names and structure fighting the project's own — several scenarios crammed into one body where the
  neighbours use subtests

Report a genuinely missing test **once**, naming the defect and where the test belongs — never an inventory of
uncovered branches.

## docs — documentation against the code

Accuracy before completeness: a wrong document is worse than a missing one. That orders what to look for,
**not how bad the result is** — severity comes from the bar and nowhere else.

In the code:

- an exported item, or an interface contract callers depend on, with nothing explaining it
- a comment naming a parameter, return value or failure condition the code does not have
- a comment describing behavior the code no longer has, or a name that no longer exists
- a precondition, side effect, concurrency requirement or cancellation behavior a caller must know and cannot
  see from the signature
- hedging language — might, could, probably — where the code is definite

In the project's documents:

- a new flag, command, option, endpoint or user-visible behavior nothing tells a user about
- an example that would no longer work as written
- a design or plan document the change contradicts, or completes without recording it

Skip prose taste, obvious accessors, generated files, test code. The rule being enforced is comment sparingly:
a comment must state a constraint, gotcha or non-obvious reason — never narrate what the code shows.

## comments — the code's own stated rules

The project's guidance at its most local and most binding: a note on the function being edited was written by
whoever last got this wrong. Judge the change against what the comments say.

Earns a finding:

- a doc comment stating a contract the change now breaks — an invariant, an ordering, a "callers must"
- a comment explaining *why* the code is as it is, where the change undoes the reason without addressing it
- a `TODO`, `FIXME` or "do not" the change walks straight into
- a doc comment left describing the old behaviour after the code beneath it changed — comments describe the
  current state, so a stale one is a defect **in the change**, not pre-existing
- a rule stated in one place and silently broken in another the same change touches

Does not: a comment you merely disagree with (the bar is *contradiction*), a missing comment (that is `docs`),
a directive deliberately silenced with the reason visible, or how a comment is worded.

**Quote the comment and the line contradicting it**, and say which of the two is wrong — a contract the change
outgrew is fixed by rewriting the comment, not the code.
