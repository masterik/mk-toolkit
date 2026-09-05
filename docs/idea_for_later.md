# Ideas for later

Researched but **not committed to**. Each entry records what was measured, so a later decision
starts from evidence instead of re-deriving it. Nothing here is on the [`backlog.md`](backlog.md)
milestone line.

---

## Transcript digest — replace the journal's writer with a reader

**Status:** researched 2026-09-04 against Claude Code 2.1.260. Not scheduled.

**Problem it solves.** The `Stop` / `SubagentStop` nudge is rendered verbatim in the transcript,
prefixed `Stop hook feedback:`, on every turn of every journaling repo. `suppressOutput` hides raw
stdout and nothing else — there is no display setting, and no hook event with useful timing injects
context invisibly (`PreCompact` does, but fires at the wrong moment). So the per-turn noise cannot be
muted; it can only be removed by not needing a nudge at all.

**The idea.** Claude Code already records, losslessly, everything the journal collects. Flip
`journal.sh` from a writer the agent must be nudged into calling, into a **reader** that mines the
session transcript at commit time. `facts.sh commit --journal` serves the same block it does today.

Transcripts live at `~/.claude/projects/<slug>/<session-id>.jsonl`, slug = the cwd with `/` and `.`
replaced by `-`. Subagents get `<session-id>/subagents/agent-*.jsonl`.

### The unit boundary is narration, not prompt

The obvious design — segment by `promptId`, one unit per user request — **does not work**, and this
is the finding that shapes everything else. A real 7.4 MB implementation session (3,644 entries, 120
files changed) contains exactly **one** typed user prompt: *"start implementing
docs/plans/…-handheld-scanners-as-virtual-gates.md"*. Long autonomous runs are precisely the sessions
where recorded intent is worth most, and they are the ones prompt-segmentation collapses to a single
useless unit.

What works instead: **each assistant narration block, plus the `Edit`/`Write`/`NotebookEdit` paths
that follow it until the next block.** A prototype fold produced 79 units / 31 KB from that session
in 73 ms. Sample output:

```
13:24:25  why: Time to start writing the .NET service code. First, the `GateKind` enum and
               `IGateRepository.RemoveAsync`:
   paths: IGateKindResolver.cs, IGateRepository.cs, GateKind.cs, GateRepository.cs, FakeGateRepository.cs
```

### What was measured

| | |
| --- | --- |
| Parse cost | 7.4 MB / 3,644 entries → 66 ms; full digest fold → 73 ms. Non-issue. |
| Token cost | All assistant narration for a 120-file session: 13.4 KB ≈ **3,350 tokens**. An order of magnitude under the planning diff read it replaces. |
| Write lag | **Sub-second.** The docs' "async, may lag" is not a practical risk. |
| Compaction | **Does not truncate** — 801 entries intact before a `compact_boundary` at line 802/2013. Intent survives compaction; in-context memory does not. |
| Field stability | `gitBranch`, `cwd`, `timestamp`, `isSidechain`, `promptId` present in **every** version 2.1.251 → 2.1.260. |
| `file-history-delta` | Tracks *exactly* the Edit/Write set (120 paths == 120 distinct `file_path`s). **Adds nothing** — do not build on it. |
| Bash-edit blind spot | **0.7%** — 3 mutating Bash calls out of 410. Real but marginal; one sampled unit reads *"Fix the stale doc comment reference from the sed edit"*. |

### Why lower-trust input is acceptable here

`commit/SKILL.md` step 2's `journal_uncovered=0 → no read at all` applies **only to the planning
read**. Step 5's per-file `git diff --cached` review is never skippable — *"however fresh the journal
looks"* — and the skill already states **"a plan is a proposal, not a decision — a subagent's and the
journal's alike."**

So the architecture already tolerates an unreliable intent source. The digest does not have to be
right; it has to be a good proposal. That is what makes misattribution survivable.

### Known degradations

- **Misattribution.** The fold attaches edits to the preceding narration, which is sometimes about
  something else (`"Now run the tests and lint/typecheck"` → a `.spec.tsx` path). ~3 of the first 20
  sampled units were wrong.
- **Granularity.** 79 units for what should be ~10 commits. Grouping is judgement — `commit`'s job,
  consistent with *report candidates, let the skill choose*.
- **Paths.** `git status` stays the authority; the digest supplies only *why*. An unattributed dirty
  path falls back to the diff read, which is also how the Bash blind spot is absorbed.
- **Slug collisions.** `/a/b/c`, `/a/b-c`, `/a/b.c` all map to `-a-b-c`. Resolve by globbing
  candidates then confirming against the verbatim `.cwd` field inside each transcript.
- **Undocumented, unversioned format.** Stable across the ten releases observable, with no
  compatibility guarantee. **The one risk measurement cannot retire.**
- **Scope.** `journal.jsonl` is repo-local and disposable. Transcripts sit outside the repo and hold
  the whole session including unrelated chatter — a new prerequisite and permission-allowlist entry,
  and strictly more reading than the job needs.

### Effort

| Work | Size |
| --- | --- |
| `journal.sh digest` — slug resolution, window by last-commit time, branch filter, narration/path fold, intersect with the dirty set | ~200–250 lines bash+jq, or ~300 Go |
| `facts.sh --journal` repoint | ~20 lines |
| `commit/SKILL.md` — rewrite step 2's grammar; downgrade *no read at all* to *narrows the read* | judgement-heavy |
| Test fixtures — synthetic transcript JSONL | **new fixture class**; the suite has nothing like it. Do not underestimate. |
| Deletes | `journal-nudge.sh` (255), `journal-nudge.bats` (442), ~500 of `journal.sh`, ~600 of `journal.bats`, both `Stop` registrations, the enable/disable/tombstone tree, `session-bootstrap.sh`'s journal half |

Net ≈ **−1,400 payload lines, +300**, and it collapses M6 — the largest remaining port — into
something much smaller.

### If it is ever taken up

Staged, and **the writer is not deleted first**:

1. Drop `journal-nudge.sh` and its two hook registrations. Ends the per-turn noise immediately, two-file
   diff, reversible. Costs automatic gap-naming; coverage becomes best-effort.
2. Build `journal.sh digest` as a **pure reader alongside** the existing journal. Run both across
   several real features and compare the digest's proposals against the entries actually written.
3. Delete the writer only once step 2 shows the proposals are good enough — and ship a `version` floor
   plus a graceful *unrecognized transcript shape → `journal=unavailable`* branch, since the format is
   the risk that stays.

### Rejected alternatives

- **Move the nudge to another hook event.** No event with useful timing injects invisibly.
- **Silent `PostToolUse` capture.** Gives exact provenance and keeps records repo-local, but supplies
  no *why* on its own, and puts a hook on the hot path of every edit. Viable as a fallback if the
  digest's intent quality disappoints.
- **Threshold / frequency reduction.** Smallest diff; reduces the noise without removing it. Not an
  endpoint.

---


## Statusline provider — starship speed + ccstatusline features

**Status:** idea only, from personal-tooling work on 2026-09-05. Not scheduled. **Unrelated to
mk-toolkit's Go-port mission** — kept here only as a holding pen for later, not because it belongs on
this project's `backlog.md`.

**The idea.** Build a dedicated statusline provider for Claude Code that combines starship's native
Rust module system/speed with ccstatusline's Claude-Code-specific features (rate-limit usage windows:
5h and weekly).

**Why:** starship 1.26's built-in Claude modules (`claude_model`, `claude_context`, `claude_cost`)
don't expose `rate_limits.five_hour` / `rate_limits.seven_day` from the statusLine JSON — that's the
kind of thing ccstatusline-style tools already track. As a stopgap (2026-09-05) a bash+jq wrapper
(`~/.config/starship-claude-usage.sh`, wired into `~/.claude/settings.json`'s `statusLine.command`)
shells out to `starship statusline claude-code` and appends 5h/week gauges parsed from the same stdin
JSON. It works but costs ~22ms/refresh in process-spawn overhead (measured: ~17ms baseline vs ~40ms
wrapped) — negligible against the 10s refresh interval, but a native module would cut that to
sub-millisecond since starship already parses that JSON in-process for its other Claude modules.

**How to apply:** if picked up later, options range from patching starship directly (upstream PR for
a `claude_usage` module) to a small standalone Rust binary that only handles the Claude Code
statusline case, borrowing ccstatusline's feature set.
