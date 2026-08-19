# Output discipline

Command output is context you did not choose to load. A failing test suite, a full branch diff or a
verbose build is thousands of lines that arrive whether or not the decision at hand needs them — and
unlike a subagent, bounding them costs nothing and loses nothing.

Used by every mkit skill. This file owns the run directory and what goes in it;
`agent-delegation.md` covers using it as transport between subagent stages.

The measure, from a *small* markdown-only branch: `git diff <base>` was 57 KB, `git diff <base> --stat`
572 bytes, `git log --oneline <base>..HEAD` 339 bytes. Two orders of magnitude, on the smallest real
change there is.

## Open the run directory first

Every log and run file in this bundle lives in one **run directory**, and the very first thing a skill
does is open one. It is a script, not a snippet, because the invariants are mechanical and getting one
wrong fails silently:

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh <skill>
# -> /Users/you/repo/.git/mkit/<skill>-20260819T112526Z-28csgb
```

It prints one absolute path on stdout and nothing else. `<skill>` is the skill's own name (`commit`,
`review`, `create-pr`, `finish-feature`). The header of the script documents why each invariant is there —
atomic creation, absolute path, inside the git dir. If it exits nonzero it says why: not a git repository,
or a bad name.

If `${CLAUDE_PLUGIN_ROOT}` is empty the command fails loudly as `/scripts/run-open.sh: not found` — that
is the intended failure, not something to work around: find the plugin's checkout and call the script by
its real path rather than falling back to hand-rolled `mkdir`.

**Resolve `${CLAUDE_PLUGIN_ROOT}` before handing that command to a subagent** — a subagent has no plugin
root in its context, the same reason it gets resolved reference paths (`agent-delegation.md`).

Then, for the rest of the run:

- Write only inside that directory. Never anywhere else under the git dir.
- Name it in the final summary: it is the record, which is what makes it safe for the summary to stay
  short.
- Prune old runs (keep the last handful) at the **end** of a run, never at the start — a run in progress
  elsewhere may be reading them.

### Carry the path, not a variable

**There is no `$RUN_DIR`.** A shell variable does not survive to the next command: each Bash call runs in
a fresh shell — working directory persists, environment does not — so anything assigned while opening the
directory is *empty* by the time the gate redirects into it, which lands the log in `/` exactly as if it
had never been set. This bundle therefore writes the path as `<run-dir>` throughout: a placeholder you
substitute, like `<base>` or `<feature-branch>`, and never a variable you expect to still be bound.

The path's durable home is **your own context** — you read it once from `run-open.sh` and reuse the
literal. Where a command needs it more than once, bind it **inside that same call**:

```bash
RUN=/Users/you/repo/.git/mkit/commit-20260819T112526Z-28csgb   # what run-open.sh printed
npm run lint > "$RUN/lint.log" 2>&1; echo "exit=$?"
```

- **Never re-run `run-open.sh` to "get it back".** That opens a *second* directory, scattering one run's
  files across several and leaving the summary pointing at a nearly empty one.
- **Never park it in a fixed pointer file** (`mkit/current-run`) to read back later. Two concurrent runs
  overwrite one pointer, and the first run's later steps then write into the second's directory — the
  collision the script exists to rule out, reintroduced one level up.
- **Hand subagents the resolved literal**, for the same reason they get resolved reference paths.

## The pattern

Redirect, then read what the decision needs:

```bash
RUN=<run-dir>                                          # the literal run-open.sh printed
<command> > "$RUN/<step>.log" 2>&1; echo "exit=$?"
tail -30 "$RUN/<step>.log"                             # what failed
grep -nE 'FAIL|FAILED|Error|error:|✗' "$RUN/<step>.log" | head -40
```

Report the verdict, the failing step, and the log path. **Never paste a whole log** — the path is how
the user reads it if they want it.

## Quality gates

- One log per step (`lint.log`, `test.log`, `build.log`), so a failure names itself.
- **On pass**: one line. `lint ok · test ok (128 passed) · build ok`. Nothing else enters context.
- **On failure**: the failing step, the exit code, and the tail or the grepped failures — not the log.
- Never re-run a gate to see output you discarded; that is what the log file is for.

## Diffs

- **`--stat` first**, always. It answers "how big, and where" — which is what most decisions need.
- **`--name-only`** when you want the file list for a later stage.
- **Full diff per file** (`git diff -- <path>`), never the whole tree at once, and only for files whose
  content you actually have to judge.
- Exclude generated noise when you do load one: `':(exclude)*.lock' ':(exclude)*.snap'`.
- **Never load a full branch diff to write prose.** Commit messages and `git log --oneline` are the
  source for a PR description or a summary; the diff is a fallback for the one thing the messages do not
  explain.

## What must never be capped

Bounding output must not turn into skipping the thing you are judging. Two cases:

- **A staged diff you are about to approve.** The pre-commit checks — no secrets, no debug logging, no
  unrelated churn — require reading every staged hunk. Bound it **by file, not by truncation**: `--stat`
  to plan, then the full diff one file at a time, dropping each once judged, and **skip no file**. Add a
  targeted sweep for what a reading might miss (`git diff --cached -S'<pattern>'`, or grep the staged
  patch for token/key shapes) rather than trusting a scroll.
- **A finding, message or description you are handing the user to act on.** Full body, where they act on
  it. `finding-triage.md` and the review summary rules cover which those are.

## Say what you read

If you read the tail of a log, say "tail of test.log" rather than implying you read the suite. If you
judged a diff file by file, that is a complete review and reads as one. **Never describe a truncated
read as a full one** — the whole point of a bounded read is that the bound is visible.
