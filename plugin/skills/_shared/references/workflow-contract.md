# The workflow contract

How mkit's steps compose. Every skill in the bundle is held to this; it is the reason any one of
them can be the only one you run.

```
brainstorm → spec → implement → commit → review → pr ──┐
                                                        ├─→ (merged)
                                              finish ──┘

cleanup   repo-wide gardening, not a step in the line
```

The arrows are the **common** path, not a required one. Each step is **entry-capable**: it runs as
the first thing in a session, in any order relative to the others, with any subset of the others
skipped. A user who types `commit` has asked for a commit, not for the six steps before it.

## The five rules

**1. Discover, then derive. Never demand.** A step establishes what it needs by looking: at the
repo, at the worklog, at the conversation. What it cannot find, it derives the **thin version** of
itself, inline, and carries on. `review` with no spec derives a one-line goal from the branch and
commit messages — which is what it already does — and every step works that way.

**2. No step sends the user to another step.** "Run `spec` first" is a refusal wearing a
suggestion's clothes. Do the thin version and say what you did. A step may inline the minimum of
the step immediately upstream of it (`pr` and `finish` already commit); it never runs the whole of
another skill, and never runs a step **downstream** of itself.

**3. Say what you assumed.** A step that derived what it could have read states that in its output,
in one clause: "no spec found — goal taken from the branch name, confidence lowered". This is what
makes a skipped step safe rather than silent. A run that assumed something and reported a clean
result has lied by omission.

**4. Record for the next step; never gate on the last one.** Every step appends one worklog record
when it finishes — with one exception, and it is the shape of the rule rather than a hole in it:
`finish` usually destroys its own log, since the worklog lives in the worktree it removes and is
keyed on the branch it deletes. So `finish` records where it stopped short and skips where the
cleanup ran, and that skip is not a degradation to report.

The exception has its own exception: on `cleanup_path=none` there is no worktree to remove, so the
log outlives the branch — and a branch name is reusable, which would hand a later `review` on a
recreated `feat/x` the *old* `feat/x`'s records. `finish` retires the log itself on that path. A later step reads it to be **cheaper and better informed** — never to decide
whether it is allowed to run. This is `a recorded fact is an input, never a permission`, applied to
the workflow rather than the gate ledger.

**5. Confidence degrades; the step still completes.** A missing upstream artifact lowers what a
step can claim, and lowers nothing else. It is never grounds for stopping, and never grounds for
reporting less than was actually found.

## The worklog

`<toplevel>/.mkit/work/<branch>.jsonl` — append-only, one record per finished step, never committed,
a linked worktree gets its own. Read and written through the binary; no skill parses it by hand.

```
mkit work show --json          # what has run on this branch, and what each concluded
mkit work append --json ...    # one record, at the end of a step
```

A record carries the step, when it ran, the content fingerprint it ran over, a pointer to whatever
artifact it produced (an issue URL, a path, a run directory), a one-line gist, and the assumptions
it made. The gist is what a later step reads instead of re-deriving intent; the fingerprint is what
tells it whether the gist still describes the tree in front of it.

### Reading it

**Ungated, and never gating.** A step reads the worklog unconditionally. It used to be gated on a
`mkit_bin=` fact, because the binary was optional and the log was a bonus; since M5 the binary is a
hard requirement and every skill's first call is `mkit facts`, which stops the run when it is absent
or too old. By the time any step reads the log, there is no missing-binary case left to check for.

```bash
mkit work show --json --limit 20
```

What the log itself reports is still **one fewer input, never a stop** — rule 4 above, applied to
the record: an empty log, an unreadable one, or a binary that knows `facts` but not `work` all leave
a step less informed rather than blocked. That is what separates this call from `review`'s step-0 `mkit findings` probe: without the
findings arithmetic there is no review, and without the worklog there is a slightly less informed
one. What each step does with what it reads is the step's own business, and stays in its `SKILL.md`.

**Per branch, not per invocation.** A run directory (`<skill>-<timestamp>/`) belongs to one call and
holds its working files. The worklog spans every call on a branch, which is the unit of work the
finishing steps act on.

## What each step owes

| step | needs | derives when absent | records |
| --- | --- | --- | --- |
| `brainstorm` | a question | — | the decisions reached, and the ones deferred |
| `spec` | decisions | reads the conversation and the repo; no re-interview | the spec artifact + task graph |
| `implement` | a task graph | one slice covering the ask | which slices landed, gate results |
| `commit` | a dirty tree | — | the commits made, and their scope |
| `review` | a diff + a goal | goal from a matching `spec`/`implement` gist, then the user, a stale one, branch, commits, ticket | findings, verdicts, fixes applied |
| `pr` | commits + a branch | commits first, inline | the PR URL, gate verdict |
| `finish` | a merged-able branch | commits first, inline | the merge, the cleanup |

A step with nothing to do says so plainly and stops — `commit` on a clean tree, `implement` on an
empty frontier. That is a completed run reporting an empty result, not a failure.

## Where the artifacts live

The spec and task-graph store is **discovered, never assumed**: `mkit repo profile --json` reports
it, along with everything else discovered or pinned about this repo. Steps read that answer rather
than each reaching their own, because a spec written to one store and read from another is the one
failure this contract cannot absorb.

Under a sandbox, some of what mkit would write is unreachable — `mkit doctor` reports the writable
set. A step whose artifact store is denied says which path was denied and falls back to the run
directory, which is inside the repo and therefore writable.
