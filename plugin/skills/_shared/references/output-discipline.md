# Output discipline

Owns `facts.sh`, the run directory, where a write may land, and bounded output. `agent-delegation.md` covers
the run directory as transport between stages.

## One call to start

A repo-scoped skill's first act is `bash ../_shared/scripts/facts.sh <skill> …`. **Resolve that path
against the directory of the `SKILL.md` you loaded** and run the absolute path — the shell's working
directory is the user's repo, and no agent sets a skill-location variable. Always `bash <path>`, never the
bare path (it also runs under Git Bash).

It prints `key=value` lines, then `<name>:` blocks, then a trailing `notes:` block:

- **`run_dir=`** — with `--run` only: a fresh absolute directory under `$TMPDIR` for this run's files.
- **`toplevel` `primary` `branch` `upstream` `pushed` `ahead`/`behind` `default_branch` `linked`
  `worktree_origin` `cleanup_path` `clean` `staged` `unstaged` `untracked` `conflicted`**, `status:` and
  file lists (`*.lock`/`*.snap` excluded).
- **`unstaged_stat` and `staged_stat`, separately** — a fully staged tree has no unstaged stat.
- **`remote` `remote_source` `remote_repo` `remotes` `remote_choice`** (`remote.md`); `user_dir=`
  `user_dir_writable=`.
- `codeowners=` (a path or `none`). `--base <b>` adds `commits:` (`<b>..HEAD`), `base_stat`,
  `commits_ahead_of_base`, `ff_from_base`; `--range <r>` adds `range_stat` and its file list; `--gh` adds
  `pr=` `pr_state=` `pr_draft=`; `--remote <name>` picks the remote.
- **`notes:`** — a cause and its remedy, when something needs a sentence. Report it as given.
- Exit 1: not a work tree, a failed git query, or an unresolvable `--base`/`--range`
  (`*_state=unresolvable`). Exit 2: bad usage. Never `mkdir` a substitute run directory.

`user_dir_writable=no` → report it with the remedy from `notes:` and move on; never suggest allowlisting a
path under `~/.claude`.

## Carry the path, not a variable

**There is no `$RUN_DIR`.** Each Bash call is a fresh shell. Read `run_dir=` once and pass that literal
(skills write it `<run-dir>`) to every log, file and brief. Never re-run `facts.sh --run` to get it back
(a second directory), never park it in a pointer file. Name it once in the final summary.

## Where a write may land

| what | where |
| --- | --- |
| dies with the command — a patch, a scratch list | `$TMPDIR` |
| a later step reads it — logs, findings, plans, briefs | `<run-dir>` |
| the user's tracked content | nowhere: no skill writes its own files into the work tree |

Use `$TMPDIR`, never the literal `/tmp`, never `~/.claude/…` (protected), never a bare relative path. A
`mktemp` always gets a template under `<run-dir>` or `$TMPDIR` — without one it ignores `$TMPDIR`. A refused
write: say so and stop rather than falling back elsewhere.

## Diffs

- **`--stat` first** — `facts.sh` already returned it.
- **Full diff per file** (`git diff -- <path>`), only for files you must judge, never the whole tree.
- **Never load a full branch diff to write prose.** Commit messages and the log are the source for a PR
  description or summary.

## What must never be capped

- **A staged diff you are about to approve.** Bound **by file, not by truncation**: every file, one at a
  time, skipping none; add a targeted sweep (`git diff --cached -S'<pattern>'`) for what a reading misses.
- **A finding, message or description the user acts on.** Full body, where they act on it.

## Say what you read

"Tail of gate-test.log", not a claim to have read the suite. **Never describe a truncated read as a full
one**; pass on a command's own cap notice (`... 12 more files not shown`).
