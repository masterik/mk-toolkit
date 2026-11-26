---
title: "Other agents — Codex and Copilot support for mkit's skills"
date_created: 2026-11-13
last_updated: 2026-11-25
description: >
  Whether OpenAI Codex and GitHub Copilot can run mkit's SKILL.md skills, and which of the
  capabilities the skills lean on (discovery, frontmatter, bundled scripts, plugins, subagents,
  structured questions, arguments, worktrees, sandbox, Windows) each agent has: yes, no or
  workaround, with sources. Research for epic #59 (#54).
---

# Other agents — Codex and Copilot

**Status:** findings, primary sources checked 2026-11-13 (#59 grilling session). Closes #54. The
decisions they fed are in [Implications](#implications). **UNVERIFIED** = not confirmed by a
source; re-check before relying on it.

**Question.** Porting the skills assumes Codex and GitHub Copilot read SKILL.md-format skills and
offer what the skills use in Claude Code. Is that true, per agent?

**Scope.** Codex = CLI, IDE extension and app (one skill/sandbox model). Copilot = CLI, VS Code
(JetBrains where noted) and the coding agent; columns split where they differ.

**Legend.** **yes** = native · **no** = absent · **workaround** = absent, but the skill can get
the same result another way (named).

## Summary

| Capability | Codex | Copilot CLI | Copilot VS Code | Copilot coding agent |
|---|---|---|---|---|
| Reads SKILL.md | yes | yes | yes | yes |
| Reads `.claude/skills` | **no** | yes | yes | yes |
| Reads `.agents/skills` | yes | yes | yes | yes |
| Bundled scripts | yes | yes | yes | yes |
| Env var for the skill's directory | **no** → relative paths | **no** → relative paths | **no** → relative paths | **no** → relative paths |
| Plugin + marketplace | yes | yes | — | — |
| Subagents | yes, parallel | yes, parallel (`/fleet`) | yes, parallelism undocumented | UNVERIFIED |
| Structured question | workaround (Plan mode only) | yes (`ask_user`) | yes (`#vscode/askQuestions`) | **no** (none documented) |
| `$ARGUMENTS` | **no** → free text | **no** → free text | **no** → free text | **no** → free text |
| Worktree/session (`ExitWorktree`) | UNVERIFIED | UNVERIFIED | UNVERIFIED | UNVERIFIED |
| Sandbox lets `git commit` | **no** without approval (`.git` read-only) | yes (opt-in sandbox, `.git` r/w) | UNVERIFIED | UNVERIFIED |
| Sandbox lets `gh` (network) | **no** without approval (network off) | yes (network on) | UNVERIFIED | UNVERIFIED |
| Windows shell | PowerShell (hardcoded) or WSL | PowerShell (`pwsh` 6+) or WSL | PowerShell or WSL | PowerShell or WSL |

"—" = not researched for that surface.

## Codex (CLI, IDE, app)

- **SKILL.md: yes.** Frontmatter honoured: `name`, `description`. Discovery: `.agents/skills`
  from cwd up to the repo root, and `~/.agents/skills`. **`.claude/skills` is not read.**
  [build skills](https://learn.chatgpt.com/docs/build-skills)
- **Bundled scripts: yes**, through paths relative to SKILL.md's directory. **No skill-body env
  var**: `${CLAUDE_PLUGIN_ROOT}` is expanded only in hooks and MCP config. Workaround: relative
  paths. Source: Codex source, `codex-rs/ext/skills/src/catalog_prompt.rs`.
- **Plugins: yes.** Manifest `.codex-plugin/plugin.json` (`"skills": "./skills/"`); marketplace at
  `.agents/plugins/marketplace.json`, entries `"source": {"source": "local", "path": "./plugin"}`. Verified
  with codex-cli 0.169.0 (#56): `codex plugin marketplace add <path|owner/repo>`, then
  `codex plugin add mkit@masterik`; the install copies the whole tree, `_shared/` included.
  [build plugins](https://developers.openai.com/codex/plugins/build)
- **Subagents: yes**, parallel, on by default; spawned when a skill asks for them in prose (no
  structured briefing API). Custom agents are TOML files in `.codex/agents/` and inherit the
  parent's sandbox. [subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents.md)
- **Structured questions: workaround.** Only in Plan mode (`request_user_input`); the default-mode
  feature flag is off and under development. Elsewhere: ask in plain text and parse the reply.
- **`$ARGUMENTS`: no.** Arguments arrive as free text in the prompt; the skill reads them from
  there.
- **Worktree/session features: UNVERIFIED** — not covered by the research.
- **Sandbox:** `workspace-write` allows cwd and temp dirs. **Network is off** by default (`gh`
  needs approval) and **`.git` is read-only** (`git commit` needs approval).
  [approvals and security](https://learn.chatgpt.com/docs/agent-approvals-security)
- **Windows:** runs natively in **PowerShell**, inside the native Windows sandbox (default); WSL is
  the documented alternative. The agent shell is hardcoded: no setting picks Git Bash (an
  `[windows] agent_shell` proposal, [openai/codex#16717](https://github.com/openai/codex/issues/16717),
  closed unimplemented). Whether the sandbox lets it run Git's `bash.exe`: UNVERIFIED (#58).
  [Windows](https://learn.chatgpt.com/docs/windows/windows-sandbox.md)

## Copilot (CLI, VS Code, coding agent)

- **SKILL.md: yes** in the CLI, the coding agent, VS Code and JetBrains. Discovery, project:
  `.github/skills`, `.claude/skills`, `.agents/skills`; user: `~/.copilot/skills`,
  `~/.agents/skills`. [about agent skills](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills)
- **Frontmatter:** the CLI honours `name`, `description`, `license`, `allowed-tools`. VS Code adds
  `argument-hint`, `user-invocable`, `disable-model-invocation`, `context: fork`.
- **Bundled scripts: yes**, relative to the skill's directory. Plugin hooks get
  `PLUGIN_ROOT`/`CLAUDE_PLUGIN_ROOT`; **skill bodies get nothing documented**. Workaround: relative
  paths.
- **Plugins: yes.** `plugin.json` plus `marketplace.json`, installed with
  `copilot plugin marketplace add`, then `copilot plugin install <name>@<marketplace>`. Legacy
  (no Agent Plugins `$schema`) manifests are found at `.plugin/`, root, `.github/plugin/`, then
  **`.claude-plugin/plugin.json`**; marketplaces likewise accept **`.claude-plugin/marketplace.json`**,
  `source` a repo-relative path (`./plugin`). So Claude Code's files serve Copilot; no third
  manifest (#56, docs checked 2026-11-25). Slash syntax for a plugin skill: UNVERIFIED (`/skills list`).
  [about CLI plugins](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/about-cli-plugins)
- **Subagents:** CLI **yes** — the `task` tool, and `/fleet` for parallel. VS Code **yes** —
  `runSubagent`/`task`; parallelism undocumented. Coding agent **UNVERIFIED**.
- **Structured questions:** CLI **yes** (`ask_user`); VS Code **yes** (`#vscode/askQuestions`);
  coding agent **no** (none documented) — it runs unattended, so a skill must default instead of
  asking.
- **`$ARGUMENTS`: no.** Free text, as in Codex.
- **Worktree/session features: UNVERIFIED** — not covered by the research.
- **Sandbox:** the CLI sandbox is **opt-in**, with `.git` read/write and outbound network on. VS
  Code and coding-agent sandboxes: UNVERIFIED.
- **Windows:** PowerShell (or WSL); the CLI needs `pwsh` 6+. Git Bash isn't mentioned.
  [install](https://docs.github.com/en/copilot/how-tos/copilot-cli/install-copilot-cli)

## Implications

Already decided in #59:

- **Script paths relative to SKILL.md** (#52) — neither agent gives a skill body its directory.
- **One plugin tree, a manifest per agent** (#56) — Codex wants `.codex-plugin/plugin.json`,
  Copilot and Claude Code their own.
- **Dedupe in prose** (#51).
- **From PowerShell, skills run the scripts with Git's own `bash.exe` by path** (#58,
  `_shared/references/powershell.md`) — both agents run PowerShell, and so does Claude Code without
  Git Bash (optional since v2.1.120). A bare `bash` is missing (Git for Windows puts only `Git\cmd` on
  `PATH`; verified) or WSL's (Linux `git`/`gh`, `/mnt/c` paths). `<git-root>` comes from
  `git --exec-path` (`…/mingw64/…` on x64, `…/clangarm64/…` on ARM64). No Git Bash → stop with the
  install remedy. WSL detection: postponed.

Follows from the table, for whoever ports a skill:

- `.agents/skills` is the one project path both agents read; `.claude/skills` misses Codex.
- Under Codex's default sandbox, `commit`, `pr` and `finish` hit an approval on every `git commit`
  and `gh` call; that is the agent's gate, not a failure to report.
- A skill that asks a structured question needs a plain-text fallback in Codex default mode; on the
  Copilot coding agent, which runs unattended, it takes the default instead.
- A skill that takes arguments reads them from the free-text request, never `$ARGUMENTS`.

## Known gaps

Documented, not fixed (#56). Per host, on top of the table above:

- **All other hosts:** `sandbox-audit` is Claude Code only (it reads Claude Code transcripts and
  needs the macOS binary). `model:` is ignored, so `commit`/`pr`/`finish` run on the session's
  model. Skill arguments (`review quick`, `explain eli5`) are read from the request text.
- **Codex and Copilot CLI:** `disable-model-invocation:` and `argument-hint:` are ignored too, so
  `setup` and `cleanup` may trigger on a plain request instead of only by name. Copilot VS Code
  honours both.
- **Codex:** default `workspace-write` sandbox — every `git commit`, `git push`, `git fetch` and `gh`
  call asks for approval (`.git` read-only, network off). Structured questions only in Plan mode,
  so skills ask with a numbered list. `review`'s Claude subagent reviewers are #57 (standalone, outside #59).
- **Copilot CLI:** no known gap beyond the above; not smoke-run (#56 closed without it), support rests on the docs.
- **Copilot coding agent:** unattended — every ask takes the skill's do-nothing default.
- **Windows:** PowerShell on both hosts (and Claude Code without Git Bash); the scripts run under Git's
  `bash.exe`, reached by path (#58). Verified on Windows 11 ARM64 (UTM), Windows PowerShell 5.1 and
  pwsh 7: scripts and bats pass under Git Bash. Windows PowerShell 5.1's `>` writes UTF-16, so skills
  never redirect into a file a tool reads. The default `Restricted` execution policy blocks npm's
  `codex.ps1` shim (install note, not a skill gap). Not handled yet: a WSL `bash`. Agent smoke-runs: pending.
