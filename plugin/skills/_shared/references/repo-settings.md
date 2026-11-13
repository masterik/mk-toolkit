# Repo settings

How a skill reads a per-repo setting. A repo records a choice as short prose in **`docs/agents/<area>.md`**
at its root — written by `setup` or by hand, committed like any doc. The files are **lazy**: one exists only
where the repo chose something other than discovery or the default. For each setting:

1. **The file exists** → use what it says. It is the repo's answer: don't cross-check or re-derive it.
2. **No file** → discover or take the default (table below). **Never stop**, never mention the missing file,
   never tell the user to run `setup`.
3. **The file does not say** (no value for this setting, two that contradict, a value the skill doesn't
   know) → fall back to rule 2 for that setting and **say so in one line**.

Read only the file the step needs, when it needs it. In the report, say where each value came from: *from
`docs/agents/<area>.md`*, *discovered*, *default* or *asked*. No script reads these files. The
`### mkit workflow` sub-block under `## Agent skills` in `AGENTS.md`/`CLAUDE.md` indexes them for people.

| Area | File | Holds | No file → | Detail in |
| --- | --- | --- | --- | --- |
| Commit | `commit.md` | scopes · subject length | scopes discovered · ~72 chars | `commit` step 0 |
| Review | `review.md` | default mode | `full` | `review` step 1 |
| Merge | `merge.md` | merge style · reviewers | `merge` · `CODEOWNERS`, else ask | `finish` step 4 · `pr` step 6 |
| Remote | `remote.md` | the remote to push to | `origin` → lone remote → ask | `remote.md` |
| Gate | `gate.md` | the gate commands, in order | detected | `quality-gate.md` |
| Cleanup | `cleanup.md` | kept branches · develop branch | default + `develop`/`development`/`dev` | `cleanup` |
| Spec store | `issue-tracker.md` | where issues and specs live | none assumed; a step that needs one asks | — |

`issue-tracker.md` is mattpocock's setup file; mkit reads it, never writes it.
