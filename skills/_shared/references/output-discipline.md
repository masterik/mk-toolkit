# Output discipline

Command output is context you did not choose to load. Bounding it costs nothing and loses nothing.

Used by every mkit skill. Owns the run directory; `agent-delegation.md` covers using it as transport
between stages.

Scale, on a *small* markdown-only branch: `git diff <base>` 57 KB · `--stat` 572 B · `git log --oneline`
339 B. Two orders of magnitude, on the smallest real change there is.

## Open the run directory first

A skill's first act. Every log and run file lives in it.

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh <skill>
# -> /Users/you/repo/.git/mkit/<skill>-20260819T112526Z-28csgb
```

- Prints one absolute path, nothing else. `<skill>` = the skill's own name (`commit`, `review`,
  `create-pr`, `finish-feature`).
- A script, not a snippet: its invariants (atomic creation, absolute path, inside the git dir) fail
  silently when hand-rolled. Script header documents each; nonzero exit says why (not a git repo, bad name).
- Empty `${CLAUDE_PLUGIN_ROOT}` fails as `/scripts/run-open.sh: not found`. Intended failure — find the
  plugin checkout and call the script by its real path. Never fall back to `mkdir`.
- **Resolve `${CLAUDE_PLUGIN_ROOT}` before handing that command to a subagent** — it has no plugin root
  (`agent-delegation.md`).

Then, for the rest of the run:

- Write only inside that directory, never elsewhere under the git dir.
- Name it in the final summary. It is the record — which is what lets the summary stay short.
- Prune old runs (keep the last handful) at the **end**, never the start: a concurrent run may be reading them.

### Carry the path, not a variable

**There is no `$RUN_DIR`.** Each Bash call is a fresh shell — cwd persists, environment does not — so a
variable set while opening the directory is empty when the gate redirects into it, landing the log in `/`.
Read the path once, reuse the literal; this bundle writes it as `<run-dir>`, a placeholder like `<base>`.
Where one command needs it twice, bind it **inside that call**:

```bash
RUN=/Users/you/repo/.git/mkit/commit-20260819T112526Z-28csgb   # what run-open.sh printed
npm run lint > "$RUN/lint.log" 2>&1; echo "exit=$?"
```

- **Never re-run `run-open.sh` to get it back** — that opens a second directory and scatters the run.
- **Never park it in a fixed pointer file** (`mkit/current-run`) — two concurrent runs overwrite it, and the
  first then writes into the second's directory.
- **Hand subagents the resolved literal**, same reason they get resolved reference paths.

## The pattern

```bash
RUN=<run-dir>                                          # the literal run-open.sh printed
<command> > "$RUN/<step>.log" 2>&1; echo "exit=$?"
tail -30 "$RUN/<step>.log"                             # what failed
grep -nE 'FAIL|FAILED|Error|error:|✗' "$RUN/<step>.log" | head -40
```

Report the verdict, the failing step, the log path. **Never paste a whole log** — the path is how the user
reads it.

## Quality gates

- One log per step (`lint.log`, `test.log`, `build.log`), so a failure names itself.
- **Pass**: one line — `lint ok · test ok (128 passed) · build ok`. Nothing else enters context.
- **Failure**: the failing step, the exit code, the tail or the grepped failures — not the log.
- Never re-run a gate to see output you discarded. That is what the log is for.

## Diffs

- **`--stat` first**, always — "how big, and where" is what most decisions need.
- **`--name-only`** for a file list a later stage consumes.
- **Full diff per file** (`git diff -- <path>`), never the whole tree, and only for files you must judge.
- Exclude generated noise: `':(exclude)*.lock' ':(exclude)*.snap'`.
- **Never load a full branch diff to write prose.** Commit messages and `git log --oneline` are the source
  for a PR description or summary; the diff is a fallback for the one thing they do not explain.

## What must never be capped

Bounding must not become skipping the thing you are judging.

- **A staged diff you are about to approve.** No secrets / no debug logging / no unrelated churn needs every
  hunk. Bound **by file, not by truncation**: `--stat` to plan, then full diff one file at a time, dropping
  each once judged, **skipping none**. Add a targeted sweep for what a reading misses
  (`git diff --cached -S'<pattern>'`, or grep the staged patch for token shapes) — do not trust a scroll.
- **A finding, message or description the user acts on.** Full body, where they act on it.
  `triage-verify.md` and the review summary rules name which those are.

## Say what you read

Say "tail of test.log", not something implying you read the suite. A diff judged file by file is a complete
review and reads as one. **Never describe a truncated read as a full one** — the point of a bounded read is
that the bound is visible.
