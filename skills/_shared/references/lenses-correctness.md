# Review lenses — correctness set

Three reading stances: `bugs`, `impl`, `adversarial`. A lens is **what a reviewer goes looking for** — it
decides where to look and how hard to push, never what severity to give (that is `review-severity.md`, the
same bar for every lens).

This set goes to the **Codex reviewer** in `review-changes` — promptable, strongest on correctness and on
attacking the change. The craft set (`architecture`, `quality`, `tests`, `docs`, `comments`) is in
`lenses-craft.md`, carried by a different reviewer; you do not carry it. Lenses are redistributed when a
reviewer is missing, so you may be handed both files — then cover both sets and say so.

Tag every finding with the lens that raised it: that is what makes coverage gaps visible in the summary.

---

## bugs — correctness defects

Code that does the wrong thing when it runs. Read the changed code **and its callers** — a defect is often
only visible from the caller's side.

- logic inverted, off by one, or wrong at the boundary: empty input, a single element, the last iteration,
  the zero value
- nil or missing values dereferenced on a path that can actually produce them
- concurrency: state shared without synchronization, a lock not released on every return, a channel that can
  block forever, a goroutine outliving what it writes to
- resources not released on the error path — files, connections, timers, contexts
- errors dropped, swallowed, or returned without the context needed to act on them
- an error path leaving state half-updated; state persisting between calls where the caller assumes it does not

**Name the input or sequence that triggers it before reporting.** No trigger means a smell, not a bug — leave
it out.

## impl — goal fit

Judge the change against **the goal it was given**, not your idea of a better change. State in one line what
the code does, then compare with what the goal claims. A divergence is a finding on its own, even when the
code is internally clean.

- a requirement covered only partly — one call site updated out of three, an option accepted and never read,
  an error path the goal implies and the code skips
- a fix that removes the symptom while the cause stays
- new code nothing reaches: a component never constructed, a branch no caller enters, an entry point the goal
  describes but nothing is wired to
- a signature or behavior change whose callers were not all brought along
- an approach out of proportion to the problem — name the smaller shape concretely, don't call it too complex
- work the goal never asked for, bundled in: renames, restructuring, drive-by cleanups

Goal absent or vague: **say so and lower your confidence**, never invent one to review against. Whether the
change belongs in the project at all is usually an open question for the author, not a defect.

## adversarial — read it to break it

Read as someone trying to break the change, not approve it. Assume the author's reasoning is plausible and
still wrong somewhere. Work the seams:

- the unstated assumption the change depends on, and what happens when it does not hold
- inputs the author did not consider: empty, huge, malformed, duplicated, out of order, hostile
- the failure path nobody exercised, and the state it leaves behind
- retry, restart, or two of these running at once
- trust boundaries: data from a caller, file, network or subprocess, used without validation
- what the tests assert versus what the code promises — a passing test is not a proof

Do not repeat what a careful first read surfaces. Your value is the finding the others will not have.

**Attack hard; rate against the same bar as everyone else.** Adversarial reading inflates: effort spent
constructing a trigger makes it feel likely, and a bad enough consequence starts to read as major however
contrived the path. So **state the trigger in one sentence before writing a severity** — if it needs a
precondition this codebase does not produce it is not `major`, and the body says what would have to be true
first. A real defect reached only by an unusual path is still worth reporting. `critical` is not "the worst
thing I found"; it is what is dangerous on an ordinary path.

**Title the mechanism you demonstrated, not the outcome you can construct from it.** "The two paths open the
marker with different flags" is what you established; "a live marker is skipped and its task is deleted" adds
an inferred consequence. Mechanism in the title, consequence in the body, and mark plainly which part is
observed and which follows from it.
