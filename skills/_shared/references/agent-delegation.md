# Agent delegation & context discipline

How a skill runs heavy work without paying for it in context. Used by `review` (three reviewers,
a reconciler, verifiers); applies to any stage that reads a lot and decides a little.

The rule behind all of it: **the main session holds decisions, not evidence.** Evidence — diffs, tool
transcripts, file contents, finding bodies — lives on disk, read back only for the finding being acted on.

## The run directory as transport

Give every multi-stage run one directory. **Open it with `scripts/run-open.sh`** (`output-discipline.md`
owns where it lives and why); this section is only about using it between stages.

- Hand every subagent the **resolved absolute path**. Never `$RUN_DIR` (it inherits no shell) and never the
  `${CLAUDE_PLUGIN_ROOT}/scripts/run-open.sh` command (no plugin root, and it must not open its own
  directory).
- **One writer per file.** A fanned-out stage gets one file per branch (`findings-<source>.md`,
  `verdicts-<group>.md`); the caller aggregates after they return. Concurrent writers to one path interleave
  or clobber, and a lost verdict reads exactly like a finding nobody raised.
- A stage reads the files of the stages before it and nothing else. That is what keeps its brief small.

## What each stage hands back

**A subagent's return value is a decision summary, not its work.** Every brief states its return budget —
a subagent with no budget returns everything it read.

| returns | never returns |
| --- | --- |
| the path it wrote | the diff, or any part of it |
| counts, tags, ids, verdicts — a handful of lines | file contents it read to get there |
| what it could **not** do (a failed tool, ground not covered) | a narration of its steps |

Concretely: `wrote findings-codex.md — 4 findings: 1 [code, major], 3 [code, minor]; lenses bugs, impl,
adversarial all covered`. Ten lines is generous; a hundred means the brief had no budget in it.

## Writing a brief

- **Resolve every path first.** A subagent does not have the calling skill loaded, so
  `${CLAUDE_PLUGIN_ROOT}` and `../_shared/references/…` mean nothing to it. Substitute the absolute path and
  tell it to **read the file itself** — pasting a reference into three briefs costs three copies.
- **State the return budget, the output path, and the prohibitions**, not just the task.
- **Hand over facts already established** (range, shortstat, file list, goal) as given, not to be measured.
- **One line to the user before spawning**, naming the subject in their terms — "Reviewing the branch against
  main with three reviewers…". Then spawn and stop talking until results are in.

## Parallel vs sequential

- **Read-only stages fan out** — independent reviewers, per-group verifiers, a repo sweep. Spawn in one
  message so they run concurrently.
- **Exactly one writer at a time.** Never two subagents editing one working tree; serialize, or keep the
  edits in the main session (which is also where the user is in the loop). Same rule in the run directory:
  fan-out means one output file per subagent, never a shared one they append to.
- **Independence is a feature.** Reviewers never see each other's output; a verifier never sees findings
  outside its group. Feeding one stage's findings into a stage meant to judge them independently is
  anchoring — the thing a fan-out exists to avoid.

## Model per stage

| stage shape | model |
| --- | --- |
| judgement about what is true or worth doing — reviewing, verifying, materiality | the strongest available (Opus) |
| mechanical text work, answer already in the input — merging, deduping, formatting | a cheaper tier (Sonnet) |
| a bounded search or file sweep | a cheaper tier (Sonnet), precise brief |

## When a Workflow is appropriate

Deterministic orchestration (fan-out, schema-validated returns, capped loops) fits a large run better than
hand-spawned subagents — but **the Workflow tool needs the user's explicit opt-in**, so a skill never reaches
for it on its own. Offer it when the run is large; use it when the user asks in words. Otherwise parallel
subagents, which need no opt-in and save the same context.
