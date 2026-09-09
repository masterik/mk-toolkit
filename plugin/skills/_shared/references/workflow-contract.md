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
when it finishes. A later step reads it to be **cheaper and better informed** — never to decide
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
| `review` | a diff + a goal | goal from branch, commits, or worklog | findings, verdicts, fixes applied |
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
