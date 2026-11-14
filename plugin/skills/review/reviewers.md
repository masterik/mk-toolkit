# review step 2: the reviewers

Spawn every reviewer the mode calls for **in one message**, so none sees another's findings.

| reviewer | how to invoke | full | quick |
| --- | --- | --- | --- |
| **CodeRabbit** | the `coderabbit:code-review` skill or the `coderabbit:code-reviewer` agent | its own broad pass (not steerable); map findings onto lenses afterwards | brief asks for `bugs`/`impl` only — best-effort; broader findings are expected |
| **Codex** | the `codex:rescue` skill / `codex:codex-rescue` agent, prompted for a review pass, **`--wait --write` appended**, dispatched from inside the reviewed tree (below) | `bugs`, `impl`, `adversarial` | `bugs`, `impl` only; the brief says skip `adversarial` |
| **Claude** | a subagent over the same diff, or the built-in `code-review` skill at a high effort (never `--fix`) | `architecture`, `quality`, `tests`, `docs`, `comments` | not run |

## Each brief carries

- `<run-dir>/scope.md`, and the paths `<refs>/review-severity.md` plus **its own lens file**:
  `lenses-correctness.md` for Codex (and CodeRabbit in quick), `lenses-craft.md` for Claude. Both only when
  covering for a missing reviewer. An instruction to read them.
- **Read-only**: no edits, staging or commits; don't run tests, build or linter.
- **Write `<run-dir>/findings-<source>.md`**, one entry per finding, **at most 15**, body under 80 words
  (at the cap, say so and keep the worst):

  ```markdown
  ### Missing await on the user lookup
  - file: src/api/user.ts
  - line: 42-44
  - surface: code · severity: major · confidence: 85 · lens: bugs
  - body: trigger + consequence
  - fix: concrete fix
  ```

  Title, `file`, `surface`, `severity` required; a line or range wherever there is one (no location → never
  merged). `class: open_question` or `class: pre_existing` on the tag line for a non-finding. A reviewer that
  strays from the shape is read for the same fields, never re-spawned.
- **Nothing at or above the bar → write the file empty and say so.** The file is the completion signal:
  present-and-empty = looked, found nothing; absent = never reported.
- **Return at most ten lines**: the path written, counts by `[surface, severity]`, lenses it could not cover.

## Codex: two invocation details

- **`--wait --write`, appended to the task text.** Without `--wait` the agent backgrounds the job and ends its
  turn while Codex still runs. Without `--write` Codex runs read-only and cannot write its findings file. It
  is no licence to edit: the brief still says read-only; the findings file is the one write. The brief also
  says: if it dies, report the error rather than reconstruct findings; never write a placeholder before it
  returns.
- **Codex's writable roots are the launching directory's git toplevel plus `$TMPDIR`**, not a path named in
  the brief. Dispatch with the working directory already inside the reviewed tree.

## Fallback and the wait bound

Quick's narrower roster is **by design**, not a missing reviewer. A reviewer that was launched and then
errored, went silent or came back unusable: redistribute its lenses to an available reviewer and **note both
facts in the summary**. Claude only: run **two** subagents with different lens splits. Never claim a tool ran
when it did not; never report a partial review as clean.

A silent source: ask once what happened (*job errored* / *ran but did not write* / *returned prose* /
*genuinely zero*). Still no file after ~10 minutes → declare it **missing**, redistribute its lenses, go on,
and say in the summary which lenses went uncovered. Never poll repeatedly.
