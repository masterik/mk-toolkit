# Repo settings

Where each per-repo setting comes from. The one place the discovery rules are written; skills link here
instead of re-deriving them.

A repo records a choice as short prose in **`docs/agents/<area>.md`** at its root — written by the `setup`
skill or by hand, committed like any other doc. The files are **lazy**: one exists only where the repo chose
something other than what discovery or the default gives. So for each setting:

1. **`docs/agents/<area>.md` exists** → read it and use what it says. It is the repo's answer: don't
   cross-check it against discovery or re-derive it.
2. **No file** → discover by the rule below, or take the default. **Never stop** on a missing file and never
   tell the user to run `setup`; a missing file is one fewer input (`workflow-contract.md`, rule 4).
3. **A file that does not say** (no value for the setting you need, or two that contradict) → fall back to
   rule 2 for that setting and say so in one line: the file is how the repo asked, and an ask that silently
   went nowhere is the failure worth surfacing.

The files are prose for the model, never parsed by a script — `facts.sh` and `branch-status.sh` never read
them. Read only the file the step needs, when it needs it. Say where each value came from in the report
(`workflow-contract.md`, rule 3): *from `docs/agents/<area>.md`*, *discovered*, *default* or *asked*.

The `### mkit workflow` sub-block under `## Agent skills` in `AGENTS.md`/`CLAUDE.md` lists the files that
exist. It is an index for people; the files are the answer.

## The areas

| Area | File | Holds | No file → |
| --- | --- | --- | --- |
| Commit | `docs/agents/commit.md` | scopes · subject length | scopes discovered · ~72 chars |
| Review | `docs/agents/review.md` | default mode | `full` |
| Merge | `docs/agents/merge.md` | merge style · reviewers | `merge` · `CODEOWNERS`, else ask |
| Remote | `docs/agents/remote.md` | the remote to push to | `origin` → lone remote → ask |
| Gate | `docs/agents/gate.md` | the gate commands, in order | detected |
| Cleanup | `docs/agents/cleanup.md` | branches never deleted · develop branch | default + `develop`/`development`/`dev` |
| Spec store | `docs/agents/issue-tracker.md` | where issues and specs live | none assumed |

### Commit — `commit`

- **Scopes.** File → its list is the repo's list: pick from it, or say why none fits; a change spanning
  several areas omits the scope. No file → discover with `conventional-commits.md` ("Scope"): scopes a
  repo document names (`CONTRIBUTING.md`, `.commitlintrc`), else the scopes recent history uses
  (`git log --format=%s -200`, the `type(scope):` parentheses), else the changed paths' top-level
  directory. Discovered scopes are a starting set, not a rule.
- **Subject length.** File → that many characters at most. No file → the conventional ~72. Never derived
  from history: past subjects show what was written, not what is required.

### Review — `review`

- **Mode** (`full` | `quick`). File → the mode `review` opens with when nobody named one. It is a default,
  never a ceiling: `$ARGUMENTS` and an explicit "quick review" still win. No file → `full`.

### Merge — `finish`, `pr`

- **Merge style** (`merge` | `squash` | `rebase`). File → the style `finish` uses on every path. No file →
  `merge`, as a **default**, not a repo statement: it gives way without comment where the remote refuses it
  and never overrides the user's own worktrunk config (`finish` step 4, `worktree.md`).
- **Reviewers.** File naming reviewers → request exactly those; skip `CODEOWNERS`. No file, or one silent on
  reviewers → `CODEOWNERS` owners of the changed paths, then repo/team conventions, then ask (`pr` step 6).

### Remote — `pr`, `finish`, `cleanup`

`remote.md` (this directory) has the whole rule; rule 0 is the file. No file → `origin` → the lone remote
→ ask when several remain.

### Gate — `pr`, `finish`

`quality-gate.md`, "Find the gate": the file is row 1 and, when present, the only row consulted. No file →
the rest of that table (task-runner `check`/`ci`, per-step recipes over the ecosystem default, else ask).

### Cleanup — `cleanup`

- **Kept branches.** File → each name it keeps goes to `branch-status.sh` as one `--keep <name>`; names, not
  patterns. They are added to what cleanup already protects, never substituted for it. No file → no
  `--keep`.
- **Develop branch.** File naming one → `--develop <name>`. No file → the first local `develop` /
  `development` / `dev`.

### Spec store — `spec`, `implement` (M8)

Not mkit's own file: `docs/agents/issue-tracker.md` belongs to mattpocock's setup, and mkit reads it rather
than asking the same question twice. No file → no store is assumed; a step that needs one asks.
