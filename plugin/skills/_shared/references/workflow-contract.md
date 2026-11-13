# The workflow contract

How mkit's steps compose. Every skill in the bundle is held to this; it is the reason any one of
them can be the only one you run.

```
brainstorm → spec → implement → commit → review → pr ──┐
                                                        ├─→ (merged)
                                              finish ──┘

cleanup         repo-wide gardening, not a step in the line
sandbox-audit   user-wide: past sessions' sandbox and gate events → a proposed settings diff
explain         reading aid: the last answer or one session item, in plain English (no binary)
recap           reading aid: the branch's status in plain English (read-only, records nothing)
```

The arrows are the **common** path, not a required one. Each step is **entry-capable**: it runs as
the first thing in a session, in any order relative to the others, with any subset of the others
skipped. A user who types `commit` has asked for a commit, not for the six steps before it.

## The five rules

**1. Discover, then derive. Never demand.** A step establishes what it needs by looking: at the
repo, at its history and PR, at the conversation. What it cannot find, it derives the **thin version** of
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

**4. Leave the trail where git keeps it; never gate on the last one.** A step's conclusions live in what
it already produces — commit messages (a body says why), the PR body and its review state — not in a side
log. A later step reads those: `git log <base>..HEAD` and the PR, plus the session. It reads them to be
**cheaper and better informed**, never to decide whether it is allowed to run: a recorded fact is an
input, never a permission. A missing source is one fewer input, never a stop. Reasons behind *uncommitted*
work live only in the session, so they don't survive a compaction; a step working after one describes that
work from the diff and says the why is lost. `explain` and `recap` are reading aids, not steps: they
conclude nothing a later step acts on, so they leave nothing behind.

**5. Confidence degrades; the step still completes.** A missing upstream artifact lowers what a
step can claim, and lowers nothing else. It is never grounds for stopping, and never grounds for
reporting less than was actually found.

## Reading what came before

**Ungated, and never gating.** Every repo-scoped skill's first call is the bundled `facts.sh`
(`output-discipline.md`); its `commits:` block is `git log --oneline <base>..HEAD`, and `--gh` adds `pr=`.
`sandbox-audit` is user-wide and opens no run directory; its first call, `mkit audit sandbox`, is its own
dependency check. `explain` works from the session (reading a file only to confirm what it names) and runs
nothing. What each step does with what it reads is the step's own business, and stays in its
`SKILL.md`.

**Per branch, not per invocation.** A run directory (`<skill>-<timestamp>/`) belongs to one call and
holds its working files; it is scratch, not a record. The branch's history and its PR span every call,
which is the unit of work the finishing steps act on.

## What each step owes

| step | needs | derives when absent | leaves behind |
| --- | --- | --- | --- |
| `brainstorm` | a question | — | the decisions reached, and the ones deferred |
| `spec` | decisions | reads the conversation and the repo; no re-interview | the spec artifact + task graph |
| `implement` | a task graph | one slice covering the ask | which slices landed, gate results |
| `commit` | a dirty tree | — | the commits, messages saying why |
| `review` | a diff + a goal | goal from the user or a spec in the session, then branch, commits, ticket | findings, verdicts, fixes applied (in the session and run directory) |
| `pr` | commits + a branch | commits first, inline | the PR: its body, gate verdict |
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
