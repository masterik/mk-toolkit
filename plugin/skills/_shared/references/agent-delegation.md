# Agent delegation

How a skill runs heavy work without paying for it in context: **the main session holds decisions, not
evidence.** Diffs, transcripts, file contents and finding bodies live on disk in the run directory, read back
only for the item being acted on.

## The run directory as transport

- Hand every subagent the **`run_dir=` literal** (`output-discipline.md`) — never `$RUN_DIR`, never the
  `facts.sh --run` command (a subagent must not open its own run directory).
- **One writer per file.** A fanned-out stage gets one file per subagent (`findings-<source>.md`,
  `verdicts-<batch>.md`); the caller aggregates after they return. A lost write reads exactly like nothing
  found.
- A stage reads the files of the stages before it and nothing else.

## Writing a brief

- **Absolute paths only.** A subagent does not have the calling skill loaded: resolve
  `../_shared/references/` against the calling `SKILL.md`'s directory once (`<refs>`), and tell it to
  **read** `<refs>/<file>.md` itself. Never paste a reference into a brief.
- **State the return budget, the output path and the prohibitions**, not just the task. A subagent returns
  a decision summary — the path it wrote, counts/ids/verdicts, what it could **not** do — never the diff, file
  contents or a narration. Ten lines is generous.
- **The output file is the completion signal, not the reply.** Say what an empty result looks like (write
  the file empty and say so). A subagent whose turn ended with no file reported nothing, whatever it says;
  one wrapping an async job must not end its turn before the file is written. Check the path before
  believing the summary.
- **Hand over established facts** (range, stat, file list, goal) as given.
- **One line to the user before spawning**, naming the subject in their terms; then wait.
- **Never poll, never schedule a wakeup.** A finished subagent notifies you. If asked, say it is still
  running.

## A delegated plan is complete, or it is repaired

For any plan over a file set (the commit plan):

- The brief states the **coverage contract**: every path in exactly one group; every mixed file carries a
  hunk assignment. A path it cannot place, it names — never drops.
- The caller checks by **set comparison** against `facts.sh`'s `*_file_list` blocks: same set, no path twice,
  nothing invented.
- **On a gap, send the missing paths back to the same subagent** and ask only where they go.
- **Never delegate a hunk-level judgement**: more than ~3 files whose changes span commits are read in the
  main session.

## Parallel vs sequential

- **Read-only stages fan out** — spawn in one message.
- **Exactly one writer at a time** in a working tree: edits stay in the main session.
- **Independence:** reviewers never see each other's output; a verifier sees only its own batch.

## Model per stage

| stage shape | model |
| --- | --- |
| judgement — reviewing, verifying, materiality | the strongest available (Opus) |
| mechanical text work, or a bounded search with a precise brief | a cheaper tier (Sonnet) |
| bookkeeping over a stage's output — dedupe by a written rule, counts, batching | no subagent: the main session |

The Workflow tool needs the user's explicit opt-in: offer it for a large run, never reach for it unasked.
