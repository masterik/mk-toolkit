---
title: "Claude Code mods — what they could do for mkit's skills"
date_created: 2026-10-12
last_updated: 2026-10-12
description: >
  Survey of Claude Code "mods" (in-process JS/TS plugin hooks that can draw UI,
  intercept tool calls and prompts, register commands/tools and call a model)
  mapped onto mkit's existing skills and binary. Ranked ideas, each with the
  mechanism, what it replaces, and its cost. Not scheduled.
---

# Claude Code mods — what they could do for mkit

**Status:** idea only, 2026-10-12. Not scheduled. Not on [`backlog.md`](../backlog.md). Nothing built.

**Source:** official docs read 2026-10-12 —
[overview](https://code.claude.com/docs/en/plugins/mods/overview),
[events](https://code.claude.com/docs/en/plugins/mods/events),
[API](https://code.claude.com/docs/en/plugins/mods/api),
[interface](https://code.claude.com/docs/en/plugins/mods/interface),
[reference](https://code.claude.com/docs/en/plugins/mods/reference) (as of Claude Code v2.1.290).
Not read: `create`, `test`, `troubleshoot`, `admin` beyond the overview's summary. Community
inspiration: [claudemod.com mod listing](https://www.claudemod.com/browse?category=mod) (descriptions
only, no code read). Anthropic samples: `anthropics/claude-code-playground`, `claude-code/mods`
(token-weather, blast-radius, replay-theater).

## What a mod is (enough to recap)

A **mod is a plugin whose code runs inside Claude Code's process**. Layout:
`.claude-plugin/plugin.json` + `hooks/hooks.json` with `"modules": ["./register.js"]` + the module,
which exports `register(on, options)`. Installed like any plugin (marketplace, `/plugin install`).
Terminal needs v2.1.287+; Desktop's bundled Claude Code v2.1.286+. On by default.

A hook is middleware: `on(event, [matcher], async ($, e, next) => …)`. It can **observe** (`return
next(e)`), **rewrite** (`next({...e, field})`), or **answer** (return a result without calling
`next`). `$` is the mods API — the *only* way a mod touches the outside world (`$.fs`, `$.process`,
`$.http`, `$.model`, `$.ui`, …), which is why `claude plugin validate` can list what a mod does before
you install it.

Facts that matter for mkit:

- **Hooks run almost everywhere; drawing doesn't.** Hooks run in the terminal, Desktop Code tab,
  VS Code chat panel, `claude -p`, Agent SDK, Remote Control and cloud sessions. Panes/bands/redraws
  appear only in the terminal and Desktop. So a mod can carry *behaviour* for headless runs, not just
  UI. Exception: Desktop WSL loads no plugins at all.
- **A mod can feed Claude.** `prompt.submit` → `context` adds text only Claude reads;
  `$.prompt.submit({ text })` starts a turn (Claude sees it attributed to the mod, or `asUser: true`);
  `tool.call` → `{ deny }` / `{ result }` is what Claude reads as the tool result; `command.run` →
  `{ text }` prints and Claude reads it. `$.ui.log` is a transcript line Claude does **not** read.
- **`skill.prompt`** fires when a skill's text is expanded for Claude; a hook can rewrite it.
- **Commands can be `immediate: true`** — run while Claude is mid-turn, no turn started.
- **`$.model.complete`** (standalone prompt, any model) and **`$.model.fork`** (current conversation +
  your prompt, session model, mostly prompt-cached) — billed to the user's plan/key.
- **`$.tool.register`** adds a real tool (`mcp__<plugin>__<name>`), handled in a `tool.call` hook.
- **`$.ui.ask`** holds a tool call on a question in the `AskUserQuestion` dialog; time waiting
  doesn't count against the hook's 10 s budget. Rejects in `claude -p` (no one to ask).
- **`tool.check`** fires after permission rules + settings hooks decided; a mod can return
  allow/ask/deny. A mod's allow can override an `ask` rule or a non-managed `PreToolUse` block.
- **`agent.spawn`** can set a subagent's model or deny it. **`session.compact`** fires before
  compaction. **`turn.complete`** gives the final answer, duration, usage.
- **`attribution.text`** controls commit/PR attribution text.
- **`prompt.autocomplete`** adds rows under Claude Code's own autocomplete.
- **Render sites:** `Pane` (sidebar in wide fullscreen terminal, else framed above the prompt),
  `AbovePrompt` (the band), plus redraws of Claude's own `ToolUse`, `ToolResult`, `Spinner`,
  `AskUserQuestion`, `PromptHint`, messages, etc. **Not** the permission prompt. Elements: Box, Text,
  Button, Link, Code, Markdown, Input, Select, Client; Svg Desktop-only; Raster/Image terminal-only.
- **`$.ui.status(text)` is not a statusline.** It's one line under the prompt, prefixed `⚠ <mod>:`.
  A full statusline replacement isn't what it's for (see [`statusline-provider.md`](statusline-provider.md)).
- **State:** module variables (lost on reload), `$.state` (reactive, session-scoped, reset by
  `/clear`), `$.store` (JSON under `~/.claude/plugins/store/`, 4 MiB, shared by all sessions,
  get+set not atomic).
- **Not sandboxed.** A mod's `$.process`/`$.fs`/`$.http` run with the user's full permissions,
  outside the Bash sandbox. Sandboxing covers only the Bash commands Claude runs.
- **Can be off:** `--safe-mode`, `disableAllHooks`, org `allowManagedModsOnly`/`allowManagedHooksOnly`.
  A hook that throws or times out is skipped (fail-open unless a `.catch` answers); three hooks-worker
  crashes unload all non-built-in mods.
- **Limits:** 10 s own execution per hook (50 ms for `prompt.edit`), `$.process.run` 30 s default
  (10 min max), redraws throttled to 10/s (30/s visible pane in terminal), 4 MiB file read/write.
- **Tests:** `claude plugin test` runs `*.test.ts` without a session; `claude plugin validate`
  prints `hooks:` and `calls:`.

## Ground rules if mkit ever ships a mod

These follow from the binary invariants in `AGENTS.md`; breaking one is a design error, same as there.

1. **Same plugin as the skills**, behaviours gated by `userConfig`. The mod lives in `plugin/hooks/`
   beside `plugin/skills/`; see "Packaging: one plugin, not two" below. Requires amending
   [ADR 0003](../adr/0003-two-distribution-channels.md) and `AGENTS.md`'s "Markdown only" / "No hooks"
   rules first.
2. **No logic in JS.** Every check, classification or write goes through `mkit`. The mod wires events
   to the binary and renders — a third renderer beside `cmd/` text and `tui/`, same layering.
3. **Skills never depend on it.** Mods can be off or absent; every skill keeps working unchanged. A
   mod may *pre-empt* a skill step (supply its output early) but the skill's own path stays.
4. **One implementation per invariant.** If the mod needs a rule (Conventional Commit format, risky
   command shape), the binary grows a command for it and the mod calls that.
5. **Writes stay in mkit's three locations.** The mod being unsandboxed is not a licence to write
   elsewhere (see "Avoid" below).
6. **Reads `--json`.** The mod is the natural first consumer of the `--json` forms nothing reads yet;
   that makes those forms live contracts, versioned like the `key=value` facts.

## Packaging: one plugin, not two

Decided in discussion 2026-10-12 (supersedes a first-look "separate `mkit-mod` plugin"). Not an ADR
yet.

**Why one plugin.**
- **Mod and skill text are coupled.** Idea 1 injects a facts block that a `SKILL.md` sentence must
  recognise; idea 2 sends `SKILL.md` text as a fork prompt. Shipped together they can't drift; as two
  plugins a user can update one and not the other.
- **One source for instructions.** The mod reads `SKILL.md` via `$.plugin.root` instead of carrying
  a copy.
- **Lockstep already exists.** Every release tag bumps `plugin.json`; one plugin adds no versioning.
  Two would need a second version plus a manifest `dependencies` entry.
- **One install**, no half-installed states.
- **Mods off doesn't break skills.** Per the docs, `disableAllHooks` and org `allowManagedModsOnly`
  stop a mod but leave the rest of its plugin loaded (skills, commands, agents, MCP). Ground rule 3
  holds either way.

**What it costs.**
- **"The plugin is Markdown, nothing runs" goes.** Installing mkit means running unsandboxed code in
  Claude Code's process. ADR 0003 and `AGENTS.md` need amending. The 0.15.0 hook removal was about
  per-turn transcript noise from a `Stop` hook, not hooks as such, so it doesn't block this.
- **No per-component switch for users.** Disabling the plugin in `/plugin` drops the skills too;
  `disableAllHooks` is global. Answer: per-behaviour `userConfig` fields read in `register(on, options)`
  (set via `pluginConfigs`), each behaviour a no-op when off.
  - **Default on** (passive, cheap): facts on skill load, worklog on compaction/observation, bands.
  - **Default off** (heavier or intrusive): background gate, tool-call confirmations, CI watcher,
    event recording for `sandbox-audit`.
- **Review surface.** CI adds `claude plugin validate --strict` (lists `hooks:` and `calls:`) and
  `claude plugin test`.

**When two plugins would win.** Only if "nothing runs" is kept as a selling point for skills-only
users. For a personal toolkit the drift risk costs more than that guarantee.

## Most promising

### 1. Pre-run `mkit facts` when a skill loads

**Mechanism.** `skill.prompt` hook, matched to mkit's skills. It runs `mkit facts <skill>` via
`$.process.run` (argument list, no shell) and prepends the `key=value` block to the skill text, under
a fixed marker. `SKILL.md` step 1 gains one sentence: "if a facts block is already present, use it;
otherwise run `mkit facts <skill>`."

**Why it's good.**
- Saves the first tool round-trip of every skill.
- **Binary absent / too old becomes deterministic.** Today the agent must recognise
  `command not found` ≡ `unknown command "facts"` and stop with a `brew` remedy. The mod can check
  `mkit version --json` before Claude reads anything and inject the stop sentence (sourced from the
  binary, not re-worded).
- Sidesteps output-reshaping `PreToolUse` hooks (e.g. RTK): `$.process.run` isn't a Bash tool call, so
  no settings hook rewrites it. That's the same problem `git_bin=` exists for.

**Cost / risk.**
- Facts open a run dir — a skill loaded but abandoned leaves one behind (`scratch prune` handles it).
- The facts snapshot is taken at skill load, not at step 1; for long skill preambles that's fine.
- `skill.prompt` text that changes per call invalidates the prompt cache for that part (expected).
- Needs `--no-run` vs run-dir semantics decided per skill (`recap` already uses `--no-run`).

### 2. `/recap` and `/explain` answered on the side

**Mechanism.** Mod registers `immediate: true` commands (e.g. `/mkit-recap-now`) that call
`$.model.fork({ prompt })` with the recap or explain instructions plus `mkit worklog show --json` /
`mkit facts recap --no-run` output, and render the answer in a `Pane` with `Markdown`. Returns `{}` so
nothing is printed to the transcript.

**Why it's good.**
- Works **while Claude is mid-turn** — "where are we?" without interrupting.
- The answer **doesn't enter the main context** — recap/explain are read-only and their output is for
  the human, so spending main-context tokens on them is waste.
- Fork reuses the cached conversation prefix, so it's cheap.

**Cost / risk.**
- The Markdown skill and the mod would hold two copies of the recap instructions unless the mod reads
  `SKILL.md` from the plugin root and sends it as the prompt (preferred: one source).
- Pane only in terminal/Desktop; elsewhere fall back to the normal skill.
- `$.model.fork` can't call tools, so the recap uses only what the mod hands it plus the conversation.

### 3. mkit's interactive screens inside a Claude session

**Mechanism.** A `Pane` that renders `initplan.Plan` (the `mkit init` form) and the `cache prune`
tick-list using `Select` / `Button` / `Input`, then applies through the binary. Needs two binary
additions: `mkit init --plan --json` (emit `Build`'s pages) and `mkit init --apply <answers.json>`
(run `Apply`). Same for cache prune (`cache prune --json` already lists; needs an apply-by-ids form).

**Why it's good.**
- The `init` wizard and cache-prune tick-list are unreachable inside Claude Code today: there's no
  TTY, and "no TUI off a TTY" is an invariant. A pane is a TTY-free renderer of the same data.
- Fits the layering exactly: `core` returns data, `tui/` and the mod both render it.
- `sandbox-audit`'s proposed settings diff could use the same pane to *show* the diff (not apply it).

**Cost / risk.**
- Re-implements wizard navigation (Esc backs up, locked branch, review page) in JS — UI behaviour, not
  logic, but still a second UI to keep in step with `tui/repoinit`.
- New `--json` contracts for plan and answers.

### 4. Background gate

**Mechanism.** `tool.call` hook on `Edit`/`Write`/`NotebookEdit` (and Bash commands that write)
restarts a debounce timer (`$.clock.after`). When edits go quiet, the mod runs `mkit gate run` with
`$.process.spawn` (cancelling a previous run). The band (`AbovePrompt`) shows
`gate ✓ <fingerprint>` / `gate ✗ test` / `gate running`.

**Why it's good.**
- When `commit`, `review` or `finish` asks for the gate, the ledger usually already has a result for
  the current fingerprint → cache hit, no wait.
- Uses the binary's existing ledger and fingerprint; no second ledger.
- Gives a live "is this green?" signal while working.

**Cost / risk.**
- **Which `gate run` form** the mod calls matters: the two forms normalise the ledger key differently
  (that seam is what makes the `review` → `finish` hit work). Pick the form the skills look up.
- Runs tests while Claude is still editing — CPU and possibly flaky partial states; the fingerprint
  keeps results honest, but noise is real. Make it opt-in, per repo (`.mkit/config.toml`).
- Runs **outside the sandbox** (mod process). The gate commands are the repo's own, but that's still a
  wider blast radius than today's sandboxed gate.

### 5. Review decisions as one form

**Mechanism.** After `review` step 6, a pane lists each "Needs your decision" finding with a `Select`
(apply / skip / defer) and a `Code` preview of the proposed fix. A Submit button sends the choices back
with `$.prompt.submit({ text, asUser: true })`. Data comes from `mkit findings report --json` in the
run dir.

**Why it's good.**
- Batch decisions instead of one `AskUserQuestion` round per finding.
- `$.prompt.submit` is the confirmed return path (it was unconfirmed in the first look).

**Cost / risk.**
- The printed summary stays the deliverable; the pane is a mirror. The skill must accept answers from
  either place.
- Couples the mod to `findings report --json` — already a live contract, so changes there now break
  two consumers.

### 6. Exact event data for `sandbox-audit`

**Mechanism.** The mod observes `tool.call` results (`deny`, `isError`, output) and `tool.check`
decisions (`allow`/`ask`/`deny`, with `agentId`), and appends one JSON line per event to
`~/.mkit/events.jsonl` (via an `mkit` command, to keep the writer in the binary). `mkit audit sandbox`
reads that file when present, falling back to transcripts otherwise.

**Why it's good.**
- `sessionaudit` today infers events from transcripts with hard-won heuristics: pairing results to
  calls by `tool_use_id`, anchoring the three refusal shapes at result start, judging EPERM by line
  shape, deciding whether an override was preemptive. Rule, auto-mode and user denials would become
  **exact records**.
- This replaces a heuristic data source rather than duplicating the classifier (reverses the first
  look's "second implementation" objection).

**Cost / risk.**
- Sandbox blocks still need output parsing — they're Bash output, not a decision event.
- Only covers sessions where the mod was loaded; history before install stays transcript-only, so the
  transcript path never goes away.
- Writing outside a work tree from a hook on every tool call: keep it cheap, append-only, rotated like
  the ledger.

## Also useful

- **Recap survives compaction.** `session.compact` hook appends a worklog record (current gist, last
  commit, open questions) just before compaction. `recap` already reads the worklog, so post-compaction
  recaps stop depending on what the summary kept.
- **Worklog written by observation.** A `tool.call` hook sees a successful `git commit`,
  `gh pr create` or merge and calls `mkit worklog append` itself. Today `commit`/`review`/`pr`/`finish`
  each have to remember to write it.
- **Confirmation enforced at the tool call.** `finish` and `cleanup` force-delete branches
  (`git branch -D`) and remove worktrees (`git worktree remove --force`). Holding those with `$.ui.ask`
  (Anthropic's blast-radius sample pattern) guarantees the human sees them even if the agent skipped
  step 3. Fails safe in `claude -p` because `$.ui.ask` rejects there — but that also means headless
  `finish` would be blocked, so match only when interactive.
- **Conventional Commit check before `git commit`.** `tool.call` on Bash `git commit -m …` calls a new
  `mkit check commit-msg` and denies with an actionable reason. `attribution.text` can also set the
  commit/PR attribution trailer consistently.
- **Subagent models pinned by code.** `review`'s stage table says Opus for find/verify, Sonnet for the
  sweep. `agent.spawn` can set `model` from the subagent's description/prompt marker instead of relying
  on the prose being followed.
- **Typed mkit tools.** `$.tool.register` exposes `mkit_facts`, `mkit_findings`, `mkit_gate_run`
  returning JSON. Upside: no Bash, no output reshaping, no sandbox prompts, per-tool permissions.
  Downside: skills still need the Bash path when mods are off → two paths to maintain. Only worth it if
  the mod ever becomes required.
- **CI watcher (terminal).** `$.clock.every` + `gh pr checks --json` after `pr`; band shows checks;
  `$.prompt.submit` when CI goes red. Desktop already has its own PR/CI monitor, so terminal only.
- **Autocomplete.** `prompt.autocomplete` suggests finding ids after `/mkit:explain` (from the latest
  review run dir) and branch names in `cleanup`.
- **Review progress band.** Live swimlanes for CodeRabbit / Codex / Claude reviewers and verify groups
  (driven by `agent.spawn`, `$.agent.list()` statuses, and run-dir files appearing). Makes a silent or
  failed reviewer obvious. Inspired by Agent Flow / Claude Flightdeck on claudemod.com.
- **Branch-state band.** Staged/unstaged counts, gate hit/miss, open PR, worklog head — read from
  `mkit … --json`. Shared by every skill.
- **Secrets check before review.** Not a mod idea as such: `pr` and `review` send diffs to Codex and
  CodeRabbit, so a `secrets_suspect=` fact in `mkit facts` (binary, works on every host) would flag it.
  A mod could additionally hold the reviewer call. Inspired by Secret Redactor on claudemod.com.

## Avoid

- **One-click settings apply.** A mod can write `~/.claude/settings.json` or `info/exclude` from a
  button because it runs outside the sandbox. That turns "human-run" remedies into one click, but it
  routes around the sandbox mkit is designed around (protected region, `allowWrite`,
  `TestWriteSitesAreOnTheReviewedAllowlist`). If ever done: ADR first, write only what
  `sandbox-audit` proposed, after showing the diff, on an explicit press.
- **Logic in JS** — see ground rule 2.
- **Live sandbox-event recorder that re-classifies** — that *would* be a second implementation of
  `sessionaudit`'s classifier. Idea 6 records raw decisions; classification stays in Go.
- **Reintroducing a per-turn context injector.** The `Stop` hook was removed in 0.15.0 because its
  `additionalContext` rendered in the transcript every turn. `prompt.submit` `context` is shown in the
  docs as Claude-only (prompt unchanged in the transcript), but that's stated for an example, not as a
  general rule — verify before relying on it.

## Costs

- **Second language.** The repo is Go. A mod is JS/TS with its own test runner (`claude plugin test`)
  and validator; CI would grow a step.
- **Packaging and versioning.** Ships inside the existing plugin (see "Packaging"), so no new channel,
  but the plugin gains a minimum Claude Code version for the mod (v2.1.287 terminal / v2.1.286
  Desktop; older versions ignore it and load the skills). The mod must tolerate an older or newer
  `mkit` (check `mkit version --json` and degrade by not acting) — the binary still versions apart
  from the plugin.
- **Contracts.** Ideas 1, 3, 5 and 6 each turn a `--json` form (or a new command) into a live contract.
  1 and 6 change how skills/audit get their inputs → ADR each.
- **Off-switches.** `--safe-mode`, `disableAllHooks`, org policy, per-behaviour `userConfig` — the mod
  must be pure upside.
- **Unsandboxed process.** Anything the mod runs (gate, gh, mkit) runs outside the Bash sandbox.

## Suggested order

1. **Idea 1** (facts on skill load) and **idea 4** (background gate): use the binary as it is, no UI
   required, help every skill.
2. **Idea 2** (side recap/explain): small, high daily value.
3. **Idea 3** (init/cache-prune panes): needs new `--json` plan/apply forms first.
4. The rest as they earn it.

## Related

- [`statusline-provider.md`](statusline-provider.md) — `$.ui.status` is a ⚠-prefixed line, not a
  statusline; redrawing `PromptHint`/`SessionMode` only partly covers that idea.
- [`recoverability-permission-gate.md`](recoverability-permission-gate.md) — could be built as a mod
  (`tool.check` on `Write`/`Edit`/`NotebookEdit`) instead of a `PreToolUse` settings hook, with a pane
  explaining the ask.
- [`ai-native-sdlc.md`](ai-native-sdlc.md) — SDLC-playbook brainstorm; its **[mod]** items (test
  lock, protected paths, review-capacity band, finding ratings, `/intent`) build on this doc.
- [`bash-approval-mod.md`](bash-approval-mod.md) — verified detail on `tool.check`, `$.ui.ask` and
  why the permission prompt can't be redrawn; a mod that shows the command with highlighting and
  marked risks before approval.
