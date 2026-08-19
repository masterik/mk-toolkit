# Review lenses — correctness set

Three reading stances: `bugs`, `impl`, `adversarial`. A lens is **what a reviewer goes looking
for** — it decides where to look and how hard to push, never what severity to give (that is
`review-severity.md`, and it is the same bar for every lens).

This set goes to the **Codex reviewer** in `review-changes` — promptable, and strongest on
correctness and on attacking the change. The craft set (`architecture`, `quality`, `tests`,
`docs`, `comments`) is in `lenses-craft.md` and goes to a different reviewer; you do not carry
it. When a reviewer is missing its lenses are redistributed, so you may be handed both files —
in that case cover both sets and say so.

Tag every finding with the lens that raised it — the tag is what makes coverage gaps visible in
the summary.

---

## bugs — correctness defects

Code that does the wrong thing when it runs. Read the changed code **and its callers** — a defect is
often only visible from the caller's side.

- logic inverted, off by one, or wrong at the boundary: empty input, a single element, the last
  iteration, the zero value
- nil or missing values dereferenced on a path that can actually produce them
- concurrency: state shared without synchronization, a lock not released on every return, a channel that
  can block forever, a goroutine outliving what it writes to
- resources not released on the error path — files, connections, timers, contexts
- errors dropped, swallowed, or returned without the context needed to act on them
- an error path that leaves state half-updated; state persisting between calls where the caller assumes
  it does not

**Name the input or sequence that triggers it before reporting it.** No trigger means you found a smell,
not a bug — leave it out.

## impl — goal fit

Judge the change against **the goal it was given**, not against your idea of a better change. Say in one
line what the code actually does, then compare that with what the goal claims. A divergence between the
two is a finding on its own, even when the code is internally clean.

- a requirement covered only partly — one call site updated out of three, an option accepted and never
  read, an error path the goal implies and the code skips
- a fix that removes the symptom while the cause stays in place
- new code nothing reaches: a component never constructed, a branch no caller enters, an entry point the
  goal describes but nothing is wired to
- a signature or behavior change whose callers were not all brought along
- an approach out of proportion to the problem — name the smaller shape concretely instead of calling it
  too complex
- work the goal never asked for, bundled in: renames, restructuring, drive-by cleanups

When the goal is absent or vague, **say so and lower your confidence** rather than inventing one to
review against. Whether the change belongs in the project at all is usually an open question for the
author, not a defect.

## adversarial — read it to break it

Read the change as someone trying to break it, not as someone trying to approve it. Assume the author's
reasoning is plausible and still wrong somewhere. Work the seams:

- the unstated assumption the change depends on, and what happens when it does not hold
- inputs the author did not consider: empty, huge, malformed, duplicated, out of order, hostile
- the failure path nobody exercised, and the state it leaves behind
- what happens on a retry, on a restart, or with two of these running at once
- trust boundaries: data from a caller, a file, a network or a subprocess, used without validation
- what the tests assert versus what the code actually promises — a passing test is not a proof

Do not repeat what a careful first read already surfaces. Your value is the finding the other reviewers
will not have.

**Attack the change hard; rate it against the same bar as everyone else.** An adversarial reading
inflates: the effort spent constructing a trigger makes the trigger feel likely, and a bad enough
consequence starts to read as major however contrived the path to it. So **state the trigger in one
sentence before writing a severity** — if it needs a precondition this codebase does not produce, it is
not `major`, and the body says what would have to be true first. A real defect reached only by an unusual
path is still worth reporting. `critical` is not "the worst thing I found"; it is what is dangerous on an
ordinary path.

**Title the mechanism you demonstrated, not the outcome you can construct from it.** "The two paths open
the marker with different flags" is what you established; "a live marker is skipped and its task is
deleted" adds a consequence you inferred. Mechanism in the title, consequence in the body, and mark
plainly which part is observed and which part follows from it.
