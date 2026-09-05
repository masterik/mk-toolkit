---
title: "Concept: recoverability — a git-recoverability permission axis for file-mutating tools"
date_created: 2026-09-05
last_updated: 2026-09-05
description: >
  Successor idea captured at agent-guard's archival. Distills the one
  capability that survived the 2026-09 viability re-assessment: git tracking
  status as a permission signal, applied to the Write/Edit/NotebookEdit tools
  the Claude Code sandbox does not cover, shipped as a plugin. Self-contained —
  assumes the reader has not read the agent-guard repo.
---

# recoverability

**Working name.** A Claude Code plugin that asks before an agent overwrites work git can't give
back.

**Status:** idea only. Nothing implemented. Successor to the archived agent-guard.

## Premise

Claude Code 2.1.258's auto mode + OS sandbox + read boundary absorbed ~70% of agent-guard's spec
(workspace boundary, destructive-command analyzers, permission wizard). Two gaps remain, and they
intersect:

1. The sandbox wraps Bash only — `Write`/`Edit`/`NotebookEdit` reach the filesystem through the
   agent process, unsandboxed.
2. Nothing in the permission path knows about git — the classifier reasons about *what* a command
   does, not whether its target still exists elsewhere.

## The idea

One axis, applied to `Write`/`Edit`/`NotebookEdit` only (Bash stays out of scope — the sandbox and
native `rm`/`mv`/`dd`/`sed` analyzers already cover it better than a hook can):

| Target state | Decision | Why |
|---|---|---|
| Tracked, committed, clean | allow | `git checkout --` restores it |
| Tracked, dirty | allow | last commit restores a prior version |
| Untracked / ignored | ask | nothing restores it |
| Outside the repo | not our problem | sandbox + `blockReadsOutsideWorkingDirectories` own this |

Bidirectional value: fewer prompts on committed-file writes (currently a classifier guess), more
prompts where they matter (untracked scratch files — `/rewind` only partly covers these, under its
size caps).

## Not

Not a sandbox, workspace boundary, permission-preset wizard, Bash guard, or settings.json installer.
Not a security control — it defends against accidental loss, not an adversary.

## Shape

A plugin: `.claude-plugin/plugin.json` + `hooks/hooks.json` (PreToolUse, matcher
`Write|Edit|NotebookEdit`). Auto-registers via marketplace — no settings.json surgery, no wizard
(the wizard was ~40% of agent-guard's spec and is now redundant/impossible: classifier rules are
ignored from project settings).

Constraints: <30ms hook budget (no full-repo `git status` — a `git ls-files --error-unmatch`
equivalent or cached index read); fail closed (unknown state → ask); behavior under
`bypassPermissions` is unverified — docs and the SDK guidance disagree, test empirically.

## The gate

Don't build before running this experiment: one week of real work under `defaultMode: auto` +
sandbox enabled, counting (a) prompts a tracked/untracked check would have eliminated, and (b)
unrecoverable untracked overwrites the platform let through. Under ~1/day combined: not worth a
plugin. An observe mode (log the decision, enforce nothing) is the cheapest way to run the count
and the natural first milestone.

## Open questions

- Does a PreToolUse hook fire/gate under `bypassPermissions`? Untested.
- Do `/rewind` checkpoints already make untracked loss rare enough? Size caps are the crux.
- Is "ask on untracked" tolerable in a fresh repo where everything is untracked? May need a
  first-run heuristic.
- Extend the axis to MCP file-writing tools? Same gap, wider surface.

## Salvage from agent-guard

Worth mining: `src/git/` (tracking-status queries), `src/decision/` (allow/ask/deny matrix),
`src/adapters/` (PreToolUse JSON I/O), `benches/` (latency harness),
`docs/research-2026-09-viability.md` (the platform-state analysis this derives from).

Not worth it: `src/shell/`, `src/commands/`, `src/paths/`, the configurator spec — superseded by
native platform behavior.
