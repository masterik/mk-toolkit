---
title: "Transcript digest — replace the journal's writer with a reader"
date_created: 2026-09-04
last_updated: 2026-09-04
description: >
  Researched against Claude Code 2.1.260. Flips journal.sh from a writer the
  agent must be nudged into calling, into a reader that mines the session
  transcript at commit time. Not scheduled.
---

# Transcript digest — replace the journal's writer with a reader

**Status:** researched 2026-09-04, Claude Code 2.1.260. Not scheduled.

**Problem.** The `Stop`/`SubagentStop` nudge renders verbatim in the transcript every turn.
`suppressOutput` hides only stdout; no hook event injects context invisibly at the right time. Can't
be muted — only removed by not needing a nudge.

**Idea.** Claude Code already losslessly records what the journal collects. Flip `journal.sh` from a
writer the agent is nudged to call into a **reader** that mines the session transcript at commit
time. `facts.sh commit --journal` serves the same block.

Transcripts: `~/.claude/projects/<slug>/<session-id>.jsonl`, slug = cwd with `/`/`.` → `-`. Subagents:
`<session-id>/subagents/agent-*.jsonl`.

## Unit boundary: narration, not prompt

Segmenting by `promptId` fails: a real 7.4 MB / 3,644-entry / 120-file session had exactly **one**
typed prompt. Long autonomous runs — where recorded intent matters most — collapse to one useless
unit.

Works instead: each assistant narration block plus the `Edit`/`Write`/`NotebookEdit` paths that
follow it until the next block. Prototype: 79 units / 31 KB, 73 ms.

```
13:24:25  why: Time to start writing the .NET service code. First, the `GateKind` enum and
               `IGateRepository.RemoveAsync`:
   paths: IGateKindResolver.cs, IGateRepository.cs, GateKind.cs, GateRepository.cs, FakeGateRepository.cs
```

## Measured

| | |
| --- | --- |
| Parse cost | 7.4 MB / 3,644 entries → 66 ms; full fold → 73 ms |
| Token cost | 120-file session's narration: 13.4 KB ≈ 3,350 tokens — an order of magnitude under the diff read it replaces |
| Write lag | Sub-second; not a practical risk |
| Compaction | Survives — 801 entries intact before `compact_boundary` |
| Field stability | `gitBranch`/`cwd`/`timestamp`/`isSidechain`/`promptId` stable across 2.1.251 → 2.1.260 |
| `file-history-delta` | Exactly mirrors the Edit/Write set — adds nothing, don't build on it |
| Bash-edit blind spot | 0.7% (3/410 mutating Bash calls) — real but marginal |

## Why lower-trust input is OK

`commit/SKILL.md` step 2's `journal_uncovered=0` skip applies only to the planning read; step 5's
per-file diff review is never skippable regardless. The architecture already tolerates unreliable
intent — the digest only needs to be a good proposal, not a correct one.

## Known degradations

- **Misattribution** — edits attach to the preceding narration, sometimes wrongly (~3/20 sampled).
- **Granularity** — 79 units for ~10 commits; grouping is `commit`'s judgement call.
- **Paths** — `git status` stays authoritative; unattributed paths fall back to the diff read.
- **Slug collisions** — `/a/b/c`, `/a/b-c`, `/a/b.c` all hash to `-a-b-c`; resolve by globbing +
  confirming `.cwd`.
- **Format risk** — undocumented, unversioned, stable over 10 releases but no guarantee. Can't be
  retired.
- **Scope** — transcripts sit outside the repo and hold unrelated chatter — a new prerequisite +
  allowlist entry.

## Effort

| Work | Size |
| --- | --- |
| `journal.sh digest` (slug resolution, time window, branch filter, fold, intersect dirty set) | ~200–250 lines bash+jq, or ~300 Go |
| `facts.sh --journal` repoint | ~20 lines |
| `commit/SKILL.md` step 2 rewrite | judgement-heavy |
| Synthetic transcript JSONL fixtures | new fixture class — don't underestimate |
| Deletes | `journal-nudge.sh`+bats (~700), most of `journal.sh`/`journal.bats` (~1,100), both `Stop` registrations, enable/disable/tombstone tree, journal half of `session-bootstrap.sh` |

Net ≈ **−1,400 payload lines, +300** — collapses M6, the largest remaining port.

## If taken up

1. Drop `journal-nudge.sh` + hook registrations first — kills the noise immediately, reversible.
2. Build `digest` as a reader alongside the existing journal; compare its proposals against real
   entries across several features.
3. Delete the writer only once proposals prove good — ship a version floor + `journal=unavailable`
   fallback.

## Rejected

- Move the nudge to another hook event — none injects invisibly at the right time.
- Silent `PostToolUse` capture — exact provenance, but no *why*, and a hook on every edit's hot path.
- Reduce nudge threshold/frequency — smaller diff, doesn't remove the noise.
