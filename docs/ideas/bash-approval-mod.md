---
title: "Bash approval mod — highlighted command and marked risks before allowing a call"
date_created: 2026-10-13
last_updated: 2026-10-13
description: >
  Can a Claude Code mod improve the plain-text Bash permission prompt (syntax highlighting,
  dangerous spans marked)? Verified against the mods docs: the prompt itself can't be changed,
  but a mod can intercept "ask" decisions and ask with its own dialog. Records what is
  documented, what isn't, the design that's possible, and why it's security-critical. Not an
  mkit feature; a standalone mod idea. Not scheduled.
---

# Bash approval mod

**Status:** idea only. Verified against the mods docs on 2026-10-13. Nothing built and no probe mod
run. A standalone Claude Code mod, **not mkit scope**: mkit's skills don't drive approvals. It sits
beside [`recoverability-permission-gate.md`](recoverability-permission-gate.md), which also works in
the permission path.

## The problem

When a Bash call needs approval, Claude Code (terminal and Desktop alike) shows:

- a one-line command summary;
- a reason, e.g. "Contains brace with quote character (expansion obfuscation)";
- the full command as **plain text** in a scroll box;
- "Runs outside Claude's sandbox" when that applies;
- Deny / Allow once.

A multi-line script with `&`, `$!`, subshells and `cd … &&` chains is hard to read there. The reason
names a category but doesn't point at the offending part of the command. So the reviewer reads the
whole thing unaided, which pushes them toward rubber-stamping.

Wanted: the command syntax-highlighted, with the risky spans marked and explained.

## What the docs say (verified)

Sources: mods `overview`, `reference`, `events`, `interface`, `gallery`, `admin`, `troubleshoot`,
`create` pages under <https://code.claude.com/docs/en/plugins/mods/>, the `permissions` page, and the
mods type declarations (`claude-code.d.ts`).

1. **The permission prompt can't be changed.** It is not a render site. The interface page: "The
   permission prompt isn't a render site, so a mod can't change what it shows". The d.ts says the
   engine alone draws it "since its answer authorises an action". The only addition allowed is
   `$.ui.notice(tool_use_id, text)`: one **plain-text** line under the open dialog, with no colour
   and no `Code` element.
2. **`tool.check` can make the decision itself.**
   - It fires after rules, `tool.call` hooks and `PreToolUse` hooks, and **before** the mode settles
     an `ask` (dialog, auto-mode classifier, or headless host).
   - It gets `{ tool, input, tool_use_id }`, with `input.command` for Bash. The input is pinned:
     the hook decides about the call but can't rewrite it.
   - `await next(e)` returns the decision so far: `{ decision: allow|ask|deny, reason?, rule?, hook?,
     setting?, decider?, ceiling? }`. Here `reason` is "the rule or check that decided".
   - The mod may return a different decision.
3. **What a mod's allow can and can't override:**
   - It overrides `ask` rules and non-managed `PreToolUse` blocks.
   - It doesn't override managed `PreToolUse` blocks, which are final.
   - It doesn't override deny rules where the admin guard loads (managed settings or Team/Enterprise),
     unless `allowModsToOverrideDenyRules` is set.
   - In auto mode, "a call the mod approves runs without a classifier check".
   - Interaction-requiring tools and network-path reads still prompt.
4. **`$.ui.ask(question, options | { options, header })`** holds a call on the `AskUserQuestion`
   dialog.
   - The dialog itself takes only text and options; it adds a free-text row and a "Chat about this"
     row.
   - **Custom content is possible.** A `ui.render` on the `AskUserQuestion` render site can put the
     mod's own elements *above* the engine's dialog. The tree must contain the dialog reference
     exactly once.
   - Time spent waiting doesn't count against the hook budget.
   - It rejects when dismissed, and in `-p` (no one to ask).
5. **The `Code` element** has `source` (≤ 10,000 chars), `language`, `path`, `startLine`,
   `format: source|diff` and `wrap`.
   - It uses the engine's own highlighter and the user's theme colours.
   - It has **no line or range highlight props**; the only emphasis is word shading in `diff`.
   - `Text` has `color`, `backgroundColor`, `bold`, `underline`, `inverse` and so on. Annotations
     are composed as a `Box` of several `Text` elements.
6. **Hook budget and failure:**
   - Each hook gets 10 s of its own execution time. Waits inside mods API calls don't count;
     your own awaits do.
   - A hook that throws, times out or returns the wrong shape is **skipped**, and core's decision
     stands. For an `ask`, that means the normal built-in prompt: here, failing open *is* the safe
     direction.
   - Three hooks-worker crashes unload all mods until `/reload-plugins`.
7. **No "always allow."** There is no API to write a permission rule. `$.settings.read` is
   read-only, so a mod can only decide one call at a time.
8. **Where mods run:**
   - Hooks and drawing: terminal (v2.1.287+) and the Desktop Code tab (v2.1.286+).
   - Hooks without drawing: the VS Code chat panel, `-p`, the Agent SDK and cloud sessions.
   - Nothing at all: Desktop on WSL.
9. **The `ToolUse` row can be redrawn** (requestId = the tool call id), e.g. with a highlighted
   `Code` element.
10. **No npm.** The only allowed imports are relative files in the plugin plus the bare
    `claude-code`. There's no build step, no Node APIs, and only Web APIs (`URL`, `TextEncoder`,
    `crypto.subtle`, …). A parser must be vendored as plain JS.

### Not documented (needs a probe mod)

- Whether `reason` carries the Bash safety-check text ("expansion obfuscation"), or just a generic
  "ask".
- Whether `input` carries `dangerouslyDisableSandbox`. The Bash input schema in the d.ts has it, but
  `tool.check`'s docs don't say. Without it, the mod can't show "runs outside the sandbox".
- Whether calling `$.ui.ask` *inside* a `tool.check` hook is allowed. The documented example holds
  the call from `tool.call`.
- Whether `bash`/`sh`/`shell` is a valid `Code` language id.
- Whether the `ToolUse` row is visible while the built-in prompt is pending.
- Whether `tool.check` fires for calls that auto mode would approve anyway. This is irrelevant if
  the mod only acts on `ask`.

## Correction to the first take

The first chat answer assumed `tool.check` could return "ask" to trigger the mod's own dialog. It
can't: `ask` hands off to the mode's decider (the built-in dialog). The mod has to **ask
itself** (via `$.ui.ask`) and return `allow` or `deny`.

## Possible designs

### A. Replace: own dialog, then allow/deny (the real answer to the question)

1. Run `tool.check` for Bash with `d = await next(e)`. Anything but `decision: 'ask'` returns `d`
   unchanged, so the mod never loosens an allow or deny.
2. On `ask`, analyse `input.command` (see *Marking risks*) and store the result keyed by
   `tool_use_id`.
3. Call `$.ui.ask` with a header from the analysis and the options **Deny / Allow once**.
4. A `ui.render` on `AskUserQuestion` for that `tool_use_id` draws, above the dialog:
   - the command in `Code` (`language: 'bash'`, if valid);
   - a risk list, one `Text` row per finding: span excerpt, why, and severity colour;
   - core's `reason`, verbatim.
5. Return `allow` only on an explicit "Allow once". Dismissal, free text, "Chat about this" or a
   rejection should **not** return deny. They should return `d` (`ask`), so the built-in prompt
   appears as the fallback.

If `$.ui.ask` turns out not to be allowed inside `tool.check`, split the work:

- `tool.call` asks first. It can't see the core decision yet, so it has to predict "would this ask?"
  from `$.settings.read`, which is a fragile second copy of the rules.
- `tool.check` then reads the stored answer.

That split is markedly worse; probe before building.

### B. Annotate: keep the built-in prompt, add context

- Redraw the `ToolUse` row with highlighted `Code` plus risk rows, so the transcript just above the
  prompt shows the readable version. Whether it's visible while the prompt is pending is
  unverified.
- Add `$.ui.notice(tool_use_id, "⚠ 2 risks: background job (&), cd chain")`: one plain line under
  the real dialog.

B never touches the decision, so **it can't approve anything by mistake**. It is the safer first
build; A only if B proves too weak.

### Marking risks (deterministic, no model)

This needs a vendored tokenizer that understands quoting, heredocs, `$( )`, `{ }`, `&&`/`||`/`;`/`&`/
`|` and redirections. A regex over the raw string misreads quoted text. Each rule maps a span to a
reason:

| Rule | Example |
|---|---|
| Background job / `$!` | `turbo run build &` … `nodered=$!` |
| Pipe to a shell | `curl … \| sh`, `\| bash` |
| Recursive or forced delete | `rm -rf`, `git clean -fdx` |
| Destructive git | `push --force`, `reset --hard`, `branch -D` |
| `eval`, `source` of a variable, `bash -c "$x"` | |
| Expansion inside quotes and braces | the construct core flagged |
| Writes outside cwd | `>`/`tee`/`cp`/`mv` targets that resolve outside the work tree |
| Network | host names in `curl`/`wget`/`git clone` |
| Package install / lifecycle scripts | `npm i`, `bun install`, `pip install` |
| Sandbox | `dangerouslyDisableSandbox` if it's exposed, else core's `reason` |

Show **why**, not just a colour. Unknown constructs are reported as "not analysed", never as safe.

## Risks and costs

- **Security-critical code path.** A bug in design A that returns `allow` approves a command the user
  never saw, and it skips the auto-mode classifier too. Mitigations:
  - allow only on an exact option match;
  - every other path returns core's `d`, never a guess;
  - unit-test the decision function separately from the UI.
- **Information loss.** The built-in prompt's fields (reason, sandbox state, options) must all
  reappear. If core's `reason` or the sandbox flag isn't exposed, design A shows *less* than the
  built-in prompt and shouldn't ship.
- **Trust in the analyser.** A highlighted "no risks found" invites approval. Never render a
  "safe" verdict; render findings, or "nothing flagged — read the command".
- **Coverage.** Only the terminal and the Desktop Code tab can draw. In `-p` `$.ui.ask` rejects, and
  the mod must fall back to `d`. Desktop on WSL loads nothing.
- **Maintenance.** The docs mark the d.ts on `main` as possibly older than a given build. Render
  sites and payloads may move; pin a minimum Claude Code version.
- **Overlap with auto mode.** In auto mode most calls never reach a dialog, so the mod matters only
  in default/ask modes or for calls the classifier escalates.

## Next step (if pursued)

A throwaway probe mod (outside this repo's plugin; `--plugin-dir`) that logs:

- `e.input` and `await next(e)` for a Bash call that triggers an ask;
- whether `$.ui.ask` works inside `tool.check`;
- whether `Code` accepts `language: 'bash'`.

Those answers decide A versus B.

## Related

- [`claude-code-mods.md`](claude-code-mods.md) — mods survey; `tool.check`, `$.ui.ask` and render
  sites in mkit's context.
- [`recoverability-permission-gate.md`](recoverability-permission-gate.md) — another permission-path
  plugin (git recoverability for `Write`/`Edit`). A shared "approval context" mod could host both.
