# Output discipline

Command output is context you did not choose to load. A failing test suite, a full branch diff or a
verbose build is thousands of lines that arrive whether or not the decision at hand needs them — and
unlike a subagent, bounding them costs nothing and loses nothing.

Used by every mkit skill. The run directory is `../_shared/references/agent-delegation.md`; this file is
what to put in it.

The measure, from a *small* markdown-only branch: `git diff <base>` was 57 KB, `git diff <base> --stat`
572 bytes, `git log --oneline <base>..HEAD` 339 bytes. Two orders of magnitude, on the smallest real
change there is.

## The pattern

Redirect, then read what the decision needs:

```bash
<command> > "$RUN_DIR/<step>.log" 2>&1; echo "exit=$?"
tail -30 "$RUN_DIR/<step>.log"                              # what failed
grep -nE 'FAIL|FAILED|Error|error:|✗' "$RUN_DIR/<step>.log" | head -40
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
