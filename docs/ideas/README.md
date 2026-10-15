# Ideas

Researched but **not committed to**. Each entry records what was measured, so a later decision
starts from evidence instead of re-deriving it. One idea per file; nothing here is on the
[`backlog.md`](../backlog.md) milestone line.

- [`journal-transcript-digest.md`](journal-transcript-digest.md) — replace the commit journal's
  writer with a reader that mines the Claude Code session transcript.
- [`statusline-provider.md`](statusline-provider.md) — a dedicated Claude Code statusline
  provider combining starship's speed with ccstatusline's usage-window features.
- [`recoverability-permission-gate.md`](recoverability-permission-gate.md) — a Claude Code
  plugin that gates `Write`/`Edit`/`NotebookEdit` on git recoverability, the gap left after
  auto mode and the sandbox absorbed most of agent-guard's scope.
- [`claude-code-mods.md`](claude-code-mods.md) — what Claude Code mods (in-process plugin hooks
  with UI) could do for mkit's skills: facts on skill load, background gate, side recap, panes
  for `init`/`cache prune`, exact `sandbox-audit` events.
- [`ai-native-sdlc.md`](ai-native-sdlc.md) — Anthropic's AI-native SDLC playbook mapped onto mkit:
  intent/spec/plan chain, `plan drift`, `learn` from repeated findings, test-first lock, trailers
  and `trace`, evals for mkit's own skills, metrics.
- [`bash-approval-mod.md`](bash-approval-mod.md) — a mod that shows the command with syntax
  highlighting and marks risky parts before approving a Bash call; the built-in permission prompt
  can't be redrawn, so it either replaces the prompt via `tool.check` + `$.ui.ask` or adds notes
  around it.
