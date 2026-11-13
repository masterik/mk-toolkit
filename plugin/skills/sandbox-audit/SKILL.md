---
name: sandbox-audit
disable-model-invocation: true
description: >-
  Turn recent sandbox and auto-mode friction, user-wide, into a paste-ready settings diff and the guards
  that should stay. Run it as /mkit:sandbox-audit; report-only, it never edits a settings file.
---

# Audit the sandbox and permission gate across sessions

Machine gardening, **not repo-scoped**: it reads every project's transcripts under the Claude home, from any
directory.

**It needs the `mkit` binary** (`brew install masterik/tap/mkit`) — the one mkit skill that does — so it is
Claude-only and macOS-only.

- Reads transcripts and settings (both through `mkit audit sandbox`), and `CLAUDE.md` when the report says one
  exists. Nothing else.
- **Never writes a settings file** (sandbox-protected, and each permission is the user's call). The output is
  a paste-ready diff.
- Writes one file: its ledger, `${MKIT_HOME:-$HOME/.mkit}/sandbox-audit.md` (step 5).

## Preconditions

**One call, and it is the dependency check** (no `facts.sh`, no run directory):

```bash
mkit audit sandbox --days <N, default 14>
```

- `command not found` **or** `unknown command` (`"audit"`, or `"sandbox"` on a binary with only `audit
  sessions`) — absent and too old are the same answer → **stop**: "This skill runs on the `mkit` binary.
  Install it with `brew install masterik/tap/mkit` (or `brew upgrade mkit`), then run it again."
- Non-zero exit with **`no such file or directory`** for the projects directory → no transcripts to read
  (fresh machine, or `CLAUDE_HOME` elsewhere): say so and stop; not a finding.
- **Any other error** → a failed audit, not an empty one: report it verbatim and stop.
- **`unreadable=N`** on exit 0 → partial counts. All unreadable (`transcripts=0`, `unreadable` > 0) → failed
  audit: report the paths, stop. Otherwise list them at the top, call every count a **lower bound**, and never
  treat a finding's absence as proof.

Read `${MKIT_HOME:-$HOME/.mkit}/sandbox-audit.md` if it exists (last run's date, counts, **stay-blocked**
and **applied** lists). Its absence is the ordinary first run, never mentioned.

## Workflow

### 1. Read the report

Headline counts as `key=value`, four groupings (blocks by target, by command, overrides by command, auto-mode
denials by reason), a per-project table, then the config. Drill into a bucket via JSON — two calls, carrying
the printed path as a literal:

```bash
mktemp "$TMPDIR/sandbox-audit.XXXXXX"
mkit audit sandbox --days <N> --json --events > "<file>"
```

Query the file with a short script, never read it whole; `rm` it when done. Keep the `sandbox-audit.` name:
the command skips calls naming it, so the next audit doesn't count these queries as blocks.

- **`sandbox_blocks`** — sandboxed Bash calls denied. `network-outbound <host>` targets are exact; other
  targets are a normalized EPERM line (`~`, `.claude/worktrees/*`, `<file>`, `#`) — evidence, not a path to
  allowlist verbatim.
- **`overrides`** — `dangerouslyDisableSandbox` calls. `overrides_preemptive` (no block before them in their
  session) and `overrides_read_only` (reads the sandbox allows) are **behaviour**, not config gaps.
- **`automode_denials`** — classifier refusals, with its reason (a refused override counts here too).
- `rule_denials`, `user_denials` — a `permissions.deny` rule, or the user at the prompt.

### 2. Read the current config

From the report's config section (never read settings files yourself): `config_dir=`, then per root its
`settings=` files (`user`, `project`, `local`) with their keys.

- **`covered=<file>:<entry>`** — matches an existing entry by the command's reading of the pattern; confirm
  wildcards before calling it a regression.
- **`protected=yes`** — Claude Code's own region; an allowlist entry there is inert.
- **`config_unreadable:`** — what those files hold is **unknown**, not empty: propose nothing they might hold,
  say so at the top. `unknown_keys` → check each against the docs (a misspelling is silently ignored).
- `claude_md=yes` → read `<config_dir>/CLAUDE.md` for step 3's behaviour rules.

**Check every key's exact name and semantics against the current docs** before recommending it
(`https://code.claude.com/docs/en/sandboxing`, `…/settings-reference`); flag anything you could not confirm.

### 3. Decide each finding

Group by root cause, not command (toolchain caches and temp dirs, network hosts, protected paths, unix
sockets, non-HTTP network, nested sandboxes, no cause). One disposition each, in this preference order:

| disposition | when | the change |
| --- | --- | --- |
| **covered** | marked `covered=` | none — still occurring is a regression worth one line |
| **stop the traffic** | telemetry the task never needed | an `env` opt-out (`DO_NOT_TRACK`, `HOMEBREW_NO_ANALYTICS`, the tool's own) — never allowlist a tracker |
| **allowlist** | a host or path a tool legitimately needs | `sandbox.network.allowedDomains`, `sandbox.filesystem.allowWrite`, or an `env` redirect of a temp/cache dir |
| **pre-approve** | a routine classifier refusal the user always approves | a narrow `permissions.allow` rule or a scoped `autoMode.allow` sentence |
| **exclude — the user's call** | a named tool that cannot run sandboxed by design and whose own job is the blocked write (`git push -u` writing `.git/config`, `git worktree remove`, a tool starting its own sandbox, docker, ssh) | **proposed, never recommended**: `sandbox.excludedCommands` unsandboxes *every* run — show the narrowest pattern, what it unsandboxes, whether an `ask` rule keeps it prompting; names only |
| **stay blocked** | the guard did its job: an unasked merge, a sandbox-bypass env var, **any credential, secret or `.env` path**, a write to Claude Code's own config by anything but its owning tool, destructive git during a merge | none — name it so the next run doesn't propose it |
| **behaviour** | preemptive or read-only overrides, an override carried forward | a rule for `~/.claude/CLAUDE.md`, not a setting |

A protected path is **stay blocked** by default; **exclude** only when the writer is that path's own tool.
Never propose `allowWrite` on a credentials directory, a shell rc file, or anything under `~/.claude`. The
ledger's **stay-blocked** entries stay unless the user says otherwise: report the count, don't re-propose.

### 4. Report

Bold verdict (`**Verdict:** 40 blocks, 6 changes proposed, 2 stay blocked.`), form per
`../_shared/references/summary-format.md`, then only:

1. **Headline table** — the counts, against the ledger's last run (`sandbox_blocks 105 → 40`); a delta only
   over the same `--days`, else say the windows differ.
2. **Settings diff** — per file, only the changed keys, as fenced paste-ready JSON; one line under each: the
   finding and how many events it removes.
3. **Stay blocked** — what, how often, why.
4. **Behaviour** — CLAUDE.md wording, with the counts behind it.
5. **Undiagnosed** — buckets you could not attribute, one example command each.

Then ask which changes the user takes: `AskUserQuestion`, one question per change (header ≤12 chars, the full
key in the text, options **take it** / **decline**, the diff in `preview`), numbered-list fallback. An
**exclude** change lists `decline` first. They paste the accepted ones.

### 5. Update the ledger

After the answers, overwrite `${MKIT_HOME:-$HOME/.mkit}/sandbox-audit.md` whole (a snapshot), carrying every
earlier entry forward:

```markdown
# sandbox-audit ledger
last_run: <UTC date> · days: <N>
counts: sandbox_blocks=… overrides=… overrides_preemptive=… overrides_read_only=… automode_denials=… rule_denials=… user_denials=…

## Applied
- <change> — <finding> (<date>)

## Stay blocked
- <operation> — <why> (<date>)

## Declined
- <change> — <user's reason, if given> (<date>)
```

Directory not writable → say so with the remedy `mkit doctor` reports, print the ledger instead, and finish.
