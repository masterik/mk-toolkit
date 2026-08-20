# Output discipline

Command output is context you did not choose to load. Bounding it costs nothing and loses nothing.

Used by every mkit skill. Owns the run directory and the two scripts that keep output bounded;
`agent-delegation.md` covers using the run directory as transport between stages.

Scale, on a *small* markdown-only branch: `git diff <base>` 57 KB · `--stat` 572 B · `git log --oneline`
339 B. Two orders of magnitude, on the smallest real change there is.

## One call to start

A skill's first act is `${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh <skill>`. It opens this run's directory
**and** returns every read-only fact the skill starts from, as `key=value` lines:

- **`run=`** — this run's directory: atomic (`mktemp -d`), absolute, inside the git dir, its own per
  worktree. Every log and run file lives in it.
- **`refs=`** — the resolved path of this bundle. Hand subagents *that*; `../_shared/references/…` means
  nothing without the calling skill loaded.
- **`branch` `upstream` `default_branch` `linked` `worktree_origin` `cleanup_path` `clean` `staged`
  `unstaged` `untracked` `conflicted`**, plus `status:` and file lists.
- **Both diff stats, separately.** A bare `git diff --shortstat` reports nothing when the work is fully
  staged, which reads exactly like a clean tree; `unstaged_stat` and `staged_stat` make that misread
  unavailable.
- Flags: `--base <branch>` (adds `base..HEAD` commits, stat, `ff_from_base`) · `--range <range>` ·
  `--gh` (does a PR already exist) · `--no-run` (probe without opening a directory).
- Nonzero exit says why. Empty `${CLAUDE_PLUGIN_ROOT}` fails as `/scripts/facts.sh: not found` —
  intended: find the plugin checkout and call the script by its real path, never `mkdir` a substitute.

Then, for the rest of the run: write only inside `run=`; name it in the final summary (it is the record,
which is what lets the summary stay short); prune with `run-open.sh --prune` at the **end**, never the
start — a concurrent run may be reading the older directories.

### Carry the path, not a variable

**There is no `$RUN_DIR`.** Each Bash call is a fresh shell — cwd persists, environment does not. Read
`run=` once and pass that literal: as `gate-run.sh`'s first argument, as `findings.mjs`'s, and into every
brief. Bind it inside a single call when one command needs it twice. Never re-run `facts.sh` to get it
back (a second directory, run scattered), never park it in a fixed pointer file (concurrent runs
overwrite it).

## Quality gates

`scripts/gate-run.sh <run-dir> --chain '<step>=<cmd>' …` runs them. Full output to
`<run-dir>/gate-<step>.log`, exit code captured before anything can clobber it, chain stopped at the
first failure, verdict on stdout:

```
lint ok 3s
test FAIL exit=1 12s log=<run-dir>/gate-test.log
failures (max 40): 214:FAIL src/api/user.test.ts > rejects a failed write
tail -30: …
gate=FAILED step=test exit=1
```

- **Pass**: one line per step, then `gate=ok steps=…`. Nothing else enters context.
- **Failure**: the step, its exit code, the grepped failures and the tail — never the log. The script's
  own exit status is the failing step's, so a skill can branch on it.
- `--tail N` / `--grep N` widen the excerpt; `--keep-going` runs past a failure when you deliberately
  want the whole picture.
- Never re-run a gate to see output you discarded. That is what the log is for. Which commands to run:
  `gate-detect.sh` (`quality-gate.md`).

## Diffs

- **`--stat` first**, always — `facts.sh` already returned it. "How big, and where" is what most
  decisions need.
- **Full diff per file** (`git diff -- <path>`), never the whole tree, and only for files you must judge.
  The script's file lists already exclude `*.lock` and `*.snap`.
- **Never load a full branch diff to write prose.** Commit messages and `git log --oneline` are the
  source for a PR description or summary; the diff is a fallback for the one thing they do not explain.

## What must never be capped

Bounding must not become skipping the thing you are judging.

- **A staged diff you are about to approve.** No secrets / no debug logging / no unrelated churn needs
  every hunk. Bound **by file, not by truncation**: the `--stat` to plan, then full diff one file at a
  time, dropping each once judged, **skipping none**. Add a targeted sweep for what a reading misses
  (`git diff --cached -S'<pattern>'`, or grep the staged patch for token shapes) — do not trust a scroll.
- **A finding, message or description the user acts on.** Full body, where they act on it.
  `triage-verify.md` and the review summary rules name which those are.

## Say what you read

Say "tail of gate-test.log", not something implying you read the suite. A diff judged file by file is a
complete review and reads as one. **Never describe a truncated read as a full one.** Where a script
capped something it says so (`... 12 more files not shown`, `log_lines=1841`) — pass that fact on.
