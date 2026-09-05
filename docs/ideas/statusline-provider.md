---
title: "Statusline provider — starship speed + ccstatusline features"
date_created: 2026-09-05
last_updated: 2026-09-05
description: >
  Personal-tooling idea, unrelated to mk-toolkit's Go-port mission: a native
  Claude Code statusline provider combining starship's module system with
  ccstatusline's rate-limit usage windows. Not scheduled.
---

# Statusline provider — starship speed + ccstatusline features

**Status:** idea only, 2026-09-05. Not scheduled. Personal tooling — unrelated to mk-toolkit's
Go-port mission, not on `backlog.md`.

**Idea.** A native Claude Code statusline provider combining starship's Rust module speed with
ccstatusline's rate-limit usage windows (5h/weekly).

**Why.** starship 1.26's Claude modules don't expose `rate_limits.five_hour`/`seven_day` from the
statusLine JSON. Stopgap (2026-09-05): a bash+jq wrapper (`~/.config/starship-claude-usage.sh`)
shells out to `starship statusline claude-code` and appends 5h/week gauges parsed from the same
stdin JSON. Works, costs ~22ms/refresh (17ms → 40ms) — negligible against the 10s interval, but a
native module would cut that to sub-ms since starship already parses the JSON in-process.

**How to apply.** Patch starship directly (upstream `claude_usage` module PR), or write a small
standalone Rust binary for the Claude Code case, borrowing ccstatusline's feature set.
