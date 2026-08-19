# Review lenses

Eight named reading stances for a code review. A lens is **what a reviewer goes looking for** — it
decides where to look and how hard to push, never what severity to give (that is
`review-severity.md`, and it is the same bar for every lens).

Used by `review-changes`: each reviewer is handed a lens set, so the sources cover different ground
instead of three tools all reporting the same obvious things. Tag every finding with the lens that
raised it — the tag is what makes coverage gaps visible in the summary.

## Assigning lenses to reviewers

Not every reviewer is steerable. Assign by what each one accepts:

| reviewer | lenses | why |
| --- | --- | --- |
| **CodeRabbit** | not steerable — takes its own broad pass | map its findings onto lenses **after** it returns, from what each one argues |
| **Codex** | `bugs`, `impl`, `adversarial` | promptable, and strongest on correctness and on attacking the change |
| **Claude subagent** | `architecture`, `quality`, `tests`, `docs`, `comments` | promptable, and needs repo context the other two do not gather |

When a reviewer is missing, redistribute its lenses rather than dropping them — and when only Claude is
available, run **two** subagents with different lens splits, so corroboration still means something.
Never silently narrow coverage: say which lenses no source carried.

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

## quality — code that works and could be written better

Judge bottom-up, on what is in front of you, rather than against the goal.

- non-idiomatic constructs the language treats as such; a boolean argument nobody can read at the call
  site
- structure that fights the reader: a function past one screen, nesting deeper than three levels where an
  early return would flatten it, a branch after a return that cannot be taken
- abstraction with nothing behind it: a wrapper that only forwards, a factory serving one implementation,
  a settings object for two options
- generality for a case that does not exist — an extension point nothing extends, a fallback that cannot
  trigger, a second implementation kept beside the one actually used
- errors discarded with no reason given, an empty failure branch, a default returned after a failure
  without a word about it; an error wrapped so the original can no longer be inspected
- duplication the change introduces — **cite the existing copy** so the finding is actionable

Duplication is often deliberate and right: two occurrences are not yet a pattern, tests repeat setup to
stay readable, and repetition that keeps two modules independent is buying something real. Flag it only
when you can name the shared form and show it costs nothing.

## architecture — the project's own conventions

You are **not** proposing an architecture and not suggesting a library. You are finding where the change
departs from what this codebase and its written rules already settled.

Read the project's convention docs first (`CLAUDE.md`/`AGENTS.md`, design notes), plus two or three
neighbouring files in each package the change touches.

- organization: code in the wrong package, a new grab-bag helpers package, a test file split from the
  source it covers
- dependencies constructed inline where the neighbours inject them; new package-level mutable state in
  code that had none
- interface shape: wide where the codebase keeps them narrow, declared beside its implementation rather
  than its caller, a concrete type where the caller should be free to substitute
- a surface that exists only for tests — an exported symbol with no caller outside tests. Run the search
  and put what it returned in the finding
- error wrapping, logging, naming and control flow that contradict what the same package does everywhere
  else
- an old symbol kept for compatibility after every caller moved to its replacement
- a decision this project already settled, being re-opened without saying so

**Every finding cites its authority** — the rule it breaks, or a file and line showing the pattern it
contradicts. No citation means it is a preference; leave it out. Where the project's stated rules and
your taste disagree, **the project wins**.

## tests — whether they would catch anything

Review the tests, not the production code behind them. Read a neighbouring test file first, so what you
call a convention is this project's convention.

**A missing test is a finding only when you can name the defect it would catch.** "This has no test" is
not a finding; "nothing would catch that an empty input returns the previous result" is. The exception is
a project whose own rules require tests for new behavior — reportable on the rule alone.

- a fix with no test that fails without it
- a test that would still pass if the code it covers were deleted or inverted — assertions on values the
  test supplied, on a stub it configured, or behind a condition that quietly skips them
- ignored failures, disabled cases, a skip whose stated reason no longer holds
- state shared between tests, an assumption about run order, a sleep standing in for synchronization
- concurrency added by the change with nothing exercising it under a race detector
- assertions on which internal calls happened where the observable result is what matters
- names and structure that fight the project's own — several scenarios crammed into one body where the
  neighbours use subtests

Report a genuinely missing test **once**, naming the defect and where the test belongs — never an
inventory of uncovered branches.

## docs — documentation against the code

Accuracy before completeness: a wrong document is worse than a missing one. That orders what to look
for, **not how bad the result is** — severity comes from the bar and nowhere else.

In the code:

- an exported item, or an interface contract callers depend on, with nothing explaining it
- a comment naming a parameter, return value or failure condition the code does not have
- a comment describing behavior the code no longer has, or a name that no longer exists
- a precondition, side effect, concurrency requirement or cancellation behavior a caller must know and
  cannot see from the signature
- hedging language — might, could, probably — where the code is definite

In the project's documents:

- a new flag, command, option, endpoint or user-visible behavior nothing tells a user about
- an example that would no longer work as written
- a design or plan document the change contradicts, or completes without recording it

Skip prose taste, obvious accessors, generated files and test code. Comment sparingly is the rule being
enforced: a comment must state a constraint, gotcha or non-obvious reason — never narrate what the code
already shows.

## comments — the code's own stated rules

The project's guidance at its most local and most binding: a note sitting on the function being edited
was written by whoever last got this wrong. Judge the change against what the comments say.

What earns a finding:

- a doc comment stating a contract the change now breaks — an invariant, an ordering, a "callers must"
- a comment explaining *why* the code is as it is, where the change undoes the reason without addressing
  it
- a `TODO`, `FIXME` or "do not" the change walks straight into
- a doc comment left describing the old behaviour after the code beneath it changed — comments describe
  the current state, so a stale one is a defect **in the change**, not a pre-existing one
- a rule stated in one place and silently broken in another the same change touches

What does not: a comment you merely disagree with (the bar is *contradiction*), a missing comment (that
is `docs`), a directive deliberately silenced with the reason visible, or how a comment is worded.

**Quote the comment and the line that contradicts it**, and say which of the two you think is wrong — a
contract the change outgrew is fixed by rewriting the comment, not the code.

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
