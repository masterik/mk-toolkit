# Output discipline

Command output is context you did not choose to load. Bounding it costs nothing and loses nothing.

Used by every mkit skill. Owns `facts.sh`, the run directory and the habits that keep output bounded;
`agent-delegation.md` covers using the run directory as transport between stages.

Scale, on a *small* markdown-only branch: `git diff <base>` 57 KB · `--stat` 572 B · `git log --oneline`
339 B. Two orders of magnitude, on the smallest real change there is.

## One call to start

A repo-scoped skill's first act is `facts.sh` (`explain` runs nothing). It ships with the plugin in
`_shared/scripts/`; a skill writes the call relative to its own `SKILL.md`, as `bash
../_shared/scripts/facts.sh <skill> …`. **Resolve that path against the directory of the `SKILL.md` you
loaded** (Claude Code names it when it loads the skill) and run the absolute path: the shell's working
directory is the user's repo, and no agent sets an environment variable for the skill's location. Always
`bash <path>`, never the bare path — that also runs under Git Bash when the agent's shell is PowerShell.

It returns every read-only fact the skill starts from, as `key=value` lines:

- **`run_dir=`** — with `--run` only: this run's directory, fresh and absolute under `$TMPDIR`. Every
  log and run file lives in it; nothing goes into the work tree.
- **`user_dir=` `user_dir_writable=`** — the user-scoped directory (`~/.mkit`) and whether it takes a
  write. See below.
- **`branch` `upstream` `remote` `default_branch` `linked` `worktree_origin` `cleanup_path` `clean`
  `staged` `unstaged` `untracked` `conflicted`**, plus `status:` and file lists.
- **Both diff stats, separately.** A bare `git diff --shortstat` reports nothing when the work is fully
  staged, which reads exactly like a clean tree; `unstaged_stat` and `staged_stat` make that misread
  unavailable.
- **`notes:`** — the last block, present only when something needs a sentence: a cause and the remedy
  for it. Values with spaces never go on a `key=value` line, because several of those lines pack more
  than one pair.
- Flags: `--run` (open a run directory) · `--base <branch>` (adds `base..HEAD` commits, stat,
  `ff_from_base`) · `--range <range>` (an unresolvable one prints `range_state=unresolvable` and exits 1,
  like `--base`) · `--gh` (does a PR already exist: `pr=`, `pr_state=`, `pr_draft=`) · `--remote <name>`
  (`remote.md`).
- Nonzero exit says why: 1 outside a work tree, on a failed git query, or on an unresolvable
  `--base`/`--range`; 2 on bad usage. Never `mkdir` a substitute for the run directory.
- The output is the skills' own contract: it changes with the skills that parse it, in the
  same commit.

Then, for the rest of the run: write only inside `<run-dir>` (the `run_dir=` value); name it in the
final summary (it is the record, which is what lets the summary stay short). It lives under `$TMPDIR`,
so nothing needs pruning.

## Where a write may land

Three boundaries can refuse a write, independently, and none of them announces itself in advance: the
OS sandbox (the kernel, over the whole process tree), the permission gate's auto-mode classifier
(before the tool runs), and the worktree-isolation guard (also before the tool runs). `facts.sh`
reports what it can **as a starting fact**, so a refusal is something you read at the start rather than
hit in the middle.

**The rule is by lifetime, not by caller.**

| what | where | why |
| --- | --- | --- |
| dies with the command — a diff you are about to edit, a scratch list | **`$TMPDIR`** | the one location no grant is needed for in a normal session |
| a later step of this run reads it — logs, findings, plans, briefs | **`<run-dir>`** | `facts.sh --run` made it under `$TMPDIR` |
| the user's own tracked content | **nowhere** | no skill writes into the work tree; it never touches your files |

**Use `$TMPDIR`; never write the literal `/tmp`.** They are not the same instruction: `$TMPDIR` is
where an ephemeral file belongs, while a hardcoded `/tmp` is outside every grant this machine makes. A
write to `$TMPDIR` can still be refused — say so and stop rather than
falling back somewhere else. Never `~/.claude/…` either (a protected region — a write there fails even
when an allowlist entry appears to cover it), never a bare relative path, never a helper script into the
target repository's working tree. A `mktemp` with no template resolves the Darwin per-user temp
directory and **ignores `$TMPDIR`** — always give it a path: `mktemp "${TMPDIR:-/tmp}/mkit-x.XXXXXX"`,
the same fallback `facts.sh` uses, so a session with `$TMPDIR` unset still lands somewhere real.

One fact to act on:

- **`user_dir_writable=no`** — mkit cannot record what it has already told the user. Report it with
  the remedy from `notes:` and move on. Do not retry the write, and never suggest allowlisting a path
  under `~/.claude`: that region cannot be granted.

### Carry the path, not a variable

**There is no `$RUN_DIR`.** Each Bash call is a fresh shell — cwd persists, environment does not. Read
`run_dir=` once and pass that literal: as the gate logs' directory, as the home of `review`'s findings files, and
into every brief. Bind it inside a single call when one command needs it twice. Never re-run `facts.sh --run` to get it
back (a second directory, run scattered), never park it in a fixed pointer file (concurrent runs
overwrite it).

## Quality gates

Each step's full output goes to `<run-dir>/gate-<step>.log`; only a verdict enters context
(`quality-gate.md` has the run, the excerpt and the one reuse rule):

```
lint ok
test FAIL exit=1 log=<run-dir>/gate-test.log
gate=FAILED step=test exit=1
```

- **Pass**: one line per step, then `gate=ok`. Nothing else enters context.
- **Failure**: the step, its exit code, the failure lines and the tail — never the whole log.
- Never re-run a gate to see output you discarded. That is what the log is for. Which commands to run:
  `mkit gate detect` (`quality-gate.md`).

## Diffs

- **`--stat` first**, always — `facts.sh` already returned it. "How big, and where" is what most
  decisions need.
- **Full diff per file** (`git diff -- <path>`), never the whole tree, and only for files you must judge.
  `facts.sh`'s file lists already exclude `*.lock` and `*.snap`.
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
complete review and reads as one. **Never describe a truncated read as a full one.** Where a command
capped something it says so (`... 12 more files not shown`, `log_lines=1841`) — pass that fact on.
