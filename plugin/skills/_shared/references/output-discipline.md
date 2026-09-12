# Output discipline

Command output is context you did not choose to load. Bounding it costs nothing and loses nothing.

Used by every mkit skill. Owns the run directory and the two scripts that keep output bounded;
`agent-delegation.md` covers using the run directory as transport between stages.

Scale, on a *small* markdown-only branch: `git diff <base>` 57 KB · `--stat` 572 B · `git log --oneline`
339 B. Two orders of magnitude, on the smallest real change there is.

## One call to start

A skill's first act is `${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh <skill>`. It opens this run's directory
**and** returns every read-only fact the skill starts from, as `key=value` lines:

- **`run=`** — this run's directory: atomic (`mktemp -d`), absolute, `<toplevel>/.mkit/<skill>-…`, its
  own per worktree. Every log and run file lives in it.
- **`tmp=`** — where a file that dies with the command goes. See "Where a write may land".
- **`run_ignored=` `user_dir=` `user_dir_writable=` `git_bin=`** — the four facts about what this
  machine will let you write and how to call git. See below, and `git-safety.md`.
- **`refs=`** — the resolved path of this bundle. Hand subagents *that*; `../_shared/references/…` means
  nothing without the calling skill loaded.
- **`branch` `upstream` `default_branch` `linked` `worktree_origin` `cleanup_path` `clean` `staged`
  `unstaged` `untracked` `conflicted`**, plus `status:` and file lists.
- **Both diff stats, separately.** A bare `git diff --shortstat` reports nothing when the work is fully
  staged, which reads exactly like a clean tree; `unstaged_stat` and `staged_stat` make that misread
  unavailable.
- **`notes:`** — the last block, present only when something needs a sentence: a cause and the remedy
  for it. Values with spaces never go on a `key=value` line, because several of those lines pack more
  than one pair.
- Flags: `--base <branch>` (adds `base..HEAD` commits, stat, `ff_from_base`) · `--range <range>` ·
  `--gh` (does a PR already exist) · `--no-run` (probe without opening a directory).
- Nonzero exit says why. Empty `${CLAUDE_PLUGIN_ROOT}` fails as `/scripts/facts.sh: not found` —
  intended: find the plugin checkout and call the script by its real path, never `mkdir` a substitute.

Then, for the rest of the run: write only inside `run=`; name it in the final summary (it is the record,
which is what lets the summary stay short); prune with `run-open.sh --prune` at the **end**, never the
start — a concurrent run may be reading the older directories.

## Where a write may land

Three boundaries can refuse a write, independently, and none of them announces itself in advance: the
OS sandbox (the kernel, over the whole process tree), the permission gate's auto-mode classifier
(before the tool runs), and the worktree-isolation guard (also before the tool runs). `facts.sh`
reports what each of them permits **as a starting fact**, so a refusal is something you read at the
start rather than hit in the middle.

**The rule is by lifetime, not by caller.**

| what | where | why |
| --- | --- | --- |
| dies with the command — a diff you are about to edit, a scratch list | **`tmp=`** (`$TMPDIR`) | the one location no grant is needed for in a normal session |
| a later step or a later session reads it — logs, findings, plans, briefs | **`run=`** | inside the working directory, so all three layers permit it |
| the user's own tracked content | **nowhere** | mkit's only in-tree write is the ignored `.mkit/`; it never touches your files |

**Use the `tmp=` value; never write the literal `/tmp`.** They are not the same instruction: `tmp=` is
whatever `$TMPDIR` resolves to and is where an ephemeral file belongs, while a hardcoded `/tmp` is outside
every grant this machine makes. A write to `tmp=` can still be refused — say so and stop rather than
falling back somewhere else. Never `~/.claude/…` either (a protected region — a write there fails even
when an allowlist entry appears to cover it), never a bare relative path, never a helper script into the
target repository's working tree. A `mktemp` with no template resolves the Darwin per-user temp
directory and **ignores `$TMPDIR`** — always give it a path: `mktemp "${TMPDIR:-/tmp}/mkit-x.XXXXXX"`,
the same fallback `mkit_tmpfile` uses, so a session with `$TMPDIR` unset still lands somewhere real.

Two facts to act on before you stage anything:

- **`run_ignored=no`** — `.mkit/` is not ignored in this repo, so `git add -A` would sweep run
  artefacts into a commit and `git worktree remove` would refuse. **Do not run a staging step.** The
  `notes:` block names the remedy; it has to be applied from the main checkout.
- **`user_dir_writable=no`** — mkit cannot record what it has already told the user. Report it with
  the remedy from `notes:` and move on. Do not retry the write, and never suggest allowlisting a path
  under `~/.claude`: that region cannot be granted.

### Carry the path, not a variable

**There is no `$RUN_DIR`.** Each Bash call is a fresh shell — cwd persists, environment does not. Read
`run=` once and pass that literal: as `gate-run.sh`'s first argument, as `mkit findings`'s run directory, and into every
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
