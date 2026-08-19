# Agent delegation & context discipline

How an mkit skill runs heavy work without paying for it in context. Used by `review-changes` (three
reviewers, a reconciler, verifiers); the technique applies to any stage that reads a lot and decides a
little.

The rule behind all of it: **the main session holds decisions, not evidence.** Evidence — diffs, tool
transcripts, file contents, findings bodies — lives on disk and is read back only for the finding
actually being acted on.

## The run directory

Give every multi-stage run one directory, and make it the transport between stages:

```bash
RUN_DIR="$(git rev-parse --git-dir)/mkit/<skill>-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RUN_DIR"
```

- Inside the git dir, so it is **never committed and never shows up in `git status`** — no `.gitignore`
  entry needed, and a worktree gets its own (`git rev-parse --git-dir` resolves per worktree).
- Write only inside that `mkit/` subdirectory. Never write anywhere else under the git dir.
- Tell the user the path in the summary: it is the record, which is what makes it safe for the summary
  to stay short.
- Prune old runs (keep the last handful) at the end of a run, never at the start of one — a run in
  progress elsewhere may be reading them.

## What each stage hands back

**A subagent's return value is a decision summary, not its work.** Every brief states its return budget
explicitly, because a subagent with no budget returns everything it read.

| a subagent returns | a subagent never returns |
| --- | --- |
| the path it wrote | the diff, or any part of it |
| counts, tags, ids, verdicts — a handful of lines | file contents it read to reach its conclusion |
| what it could **not** do (a tool that failed, ground it did not cover) | a narration of the steps it took |

Concretely: `wrote findings-codex.md — 4 findings: 1 [code, major], 3 [code, minor]; lenses bugs, impl,
adversarial all covered`. Ten lines is generous; a hundred means the brief had no budget in it.

## Writing a brief

- **Resolve every path before handing it over.** A subagent does not have the calling skill loaded, so
  `${CLAUDE_PLUGIN_ROOT}` and `../_shared/references/…` mean nothing in its context. Substitute the
  resolved absolute path into the brief and tell it to **read the file itself** — pasting a reference's
  text into three briefs costs three copies of it.
- **State the return budget, the output path, and the prohibitions** in the brief, not just the task.
- **Hand over facts already established** (the range, the shortstat, the file list, the goal) so the
  subagent does not spend calls re-deriving them, and say they are given rather than to be measured.
- **One line to the user before spawning**, naming the subject in their terms — "Reviewing the branch
  against main with three reviewers…". Then spawn and stop talking until results are in.

## Parallel vs sequential

- **Read-only stages fan out.** Independent reviewers, per-group verifiers, a repo-wide sweep — spawn
  them in one message so they run concurrently.
- **Exactly one writer at a time.** Never two subagents editing the same working tree; either serialize
  the edits or keep them in the main session. Fixing is a main-session job for that reason (and because
  the user is in the loop for it).
- **Independence is a feature, not an accident.** Reviewers must not see each other's output, and a
  verifier must not see findings outside its own group. Passing one stage's findings into a stage meant
  to judge them independently is anchoring, and it is what a fan-out is designed to avoid.

## Model per stage

Pick per stage rather than running everything at the session's model:

| stage shape | model |
| --- | --- |
| judgement that decides what is true or worth doing — reviewing, verifying, materiality | the strongest available (Opus) |
| mechanical text work with the answer already in the input — merging lists, deduping, formatting | a cheaper tier (Sonnet) |
| a bounded search or a file sweep | a cheaper tier (Sonnet), with a precise brief |

## When a Workflow is appropriate

Deterministic orchestration (fan-out, schema-validated returns, loops with a cap) is a better fit than
hand-spawned subagents for a large run — but **the Workflow tool requires the user's explicit opt-in**,
so a skill must never reach for it on its own. Offer it when the run is large, use it when the user asks
for it in words, and otherwise use parallel subagents, which need no opt-in and give the same context
saving.
