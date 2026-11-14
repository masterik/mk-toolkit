# The workflow contract

How mkit's steps compose. Every skill in the bundle is held to it.

```
brainstorm → spec → implement → commit → review → pr ──┐
                                                        ├─→ (merged)
                                              finish ──┘

setup           optional, once per repo: settings as prose in docs/agents/ (repo-settings.md)
cleanup         repo-wide gardening, not a step in the line
sandbox-audit   user-wide: past sessions' sandbox and gate events → a proposed settings diff (needs the binary)
explain         reading aid: the last answer or one session item, in plain English (runs nothing)
recap           reading aid: the branch's status in plain English (read-only, records nothing)
```

The arrows are the **common** path, not a required one. Each step is **entry-capable**: it runs first, in
any order, with any subset of the others skipped.

## The five rules

1. **Discover, then derive. Never demand.** Establish what you need by looking — the repo, its history and
   PR, the conversation. What you cannot find, derive a **thin version** of inline, and carry on.
2. **No step sends the user to another step.** Do the thin version and say what you did. A step may inline
   the minimum of the step immediately upstream (`pr` and `finish` commit); it never runs the whole of another
   skill, and never a step **downstream** of itself.
3. **Say what you assumed**, in one clause: "no spec found — goal taken from the branch name, confidence
   lowered". A run that assumed something and reported a clean result has lied by omission.
4. **Leave the trail where git keeps it; never gate on it.** Conclusions live in commit messages (a body says
   why), the PR body and its review state. A later step reads `git log <base>..HEAD`, the PR and the session
   to be better informed, never to decide whether it may run. A missing source is one fewer input, never a
   stop. Reasons behind *uncommitted* work live only in the session: after a compaction, describe that work
   from the diff and say the why is lost.
5. **Confidence degrades; the step still completes.** A missing upstream artifact lowers what a step can
   claim, nothing else.

A run directory (`facts.sh --run`, under `$TMPDIR`) belongs to one call: scratch, not a record.
No skill needs the `mkit` binary but `sandbox-audit`.

## What each step owes

| step | needs | derives when absent | leaves behind |
| --- | --- | --- | --- |
| `brainstorm` | a question | — | the decisions reached, and the ones deferred |
| `spec` | decisions | reads the conversation and the repo; no re-interview | the spec artifact + task graph |
| `implement` | a task graph | one slice covering the ask | which slices landed, gate results |
| `commit` | a dirty tree | — | the commits, messages saying why |
| `review` | a diff + a goal | goal from the user or a spec in the session, then branch, commits, ticket | findings, verdicts, fixes applied |
| `pr` | commits + a branch | commits first, inline | the PR: its body, gate verdict |
| `finish` | a merge-able branch | commits first, inline | the merge, the cleanup |

A step with nothing to do says so and stops (`commit` on a clean tree): a completed run with an empty result,
not a failure.

## Where the artifacts live

The spec and task-graph store is **read, never assumed**: `docs/agents/issue-tracker.md` names it
(`repo-settings.md`). A step whose artifact store is denied by a sandbox says which path was denied and falls
back to the run directory.
