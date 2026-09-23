---
name: sandbox-audit
description: >-
  Scan recent Claude Code sessions, user-wide, for sandbox and auto-mode friction — what the OS sandbox
  blocked, what ran with the sandbox disabled, what the auto-mode classifier, a permission rule or the user
  refused — and propose paste-ready settings changes (network allowlist, filesystem allowWrite, env,
  excludedCommands, permission and autoMode rules) plus the guards that should stay. Trigger on "sandbox
  audit", "audit my sandbox", "scan sessions for sandbox issues", "why does the sandbox keep blocking",
  "update my allowlist from recent sessions", "review my auto mode denials", "reduce sandbox overrides".
  Report-only: it never edits a settings file.
---

# Audit the sandbox and permission gate across sessions

Part of the **mkit** bundle, but outside the edit → commit → review → finish/pr line, like `cleanup`: this
is machine gardening, not feature work, and it is **not repo-scoped** — it reads every project's transcripts
under the Claude home, so it runs the same from any directory, inside a repository or not.

References: `../_shared/references/output-discipline.md` (bounded output, where a write may land).

## What this does and does not touch

- Reads transcripts (through `mkit audit sessions`) and settings files. Nothing else.
- **Never writes a settings file.** `~/.claude/settings.json` and every `.claude/settings*.json` are
  sandbox-protected, and a changed permission is the user's decision to make, file by file. The output is a
  paste-ready diff.
- Writes one file: its own ledger, `${MKIT_HOME:-~/.mkit}/sandbox-audit.md` (step 5) — the decisions a later run must not
  re-litigate.

## Preconditions

**One call, and it is the dependency check.** This skill has no run directory and no repository, so it does
not start with `mkit facts`; the command it needs *is* the probe:

```bash
mkit audit sessions --days <N, default 14>
```

The default `--top 10` per grouping keeps this bounded; a long tail of one-off targets is step 1's
JSON drill-down, never a reason to print everything here.

If it fails with `command not found` **or** `unknown command "audit"` — absent and too old are the same
answer — **stop** and say:

> This skill runs on the `mkit` binary. Install it with `brew install masterik/tap/mkit` (or upgrade
> with `brew upgrade mkit`), then run it again.

A non-zero exit whose message is **`no such file or directory`** for the projects directory means there
are no transcripts to read (a fresh machine, or `CLAUDE_HOME` pointing somewhere else) — say so and stop;
that is not a finding. **Any other error** — permission denied, not a directory, a walk that failed — is a
failed audit, not an empty one: report it verbatim and stop. Never present it as "nothing found".

**`unreadable=N` on an exit-0 run means the counts are partial.** The command skips a transcript it cannot
open rather than failing the whole scan, and names each one in the trailing `unreadable:` block. If every
transcript is unreadable (`transcripts=0` with `unreadable` above 0), that is a failed audit — report the
paths and stop. Otherwise carry on, but list the unreadable paths at the top of the report, call every
count a **lower bound**, and never present a finding's absence as proof it did not happen.

Read `${MKIT_HOME:-~/.mkit}/sandbox-audit.md` if it exists: the last run's date, counts, and the **stay-blocked** and
**applied** lists. Its absence is the ordinary first run, never mentioned.

## Workflow

### 1. Read the report

The human output is the summary: headline counts as `key=value`, then four groupings — sandbox blocks by
target, by command, overrides by command, auto-mode denials by reason — then a per-project table. Pull the
JSON only when a bucket needs drilling into:

```bash
f=$(mktemp "${TMPDIR:?}/sandbox-audit.XXXXXX") && mkit audit sessions --days <N> --json --events > "$f" && echo "$f"
```

Carry the printed path as a literal into every later call — a shell variable does not survive to the next
one — query the file with a short script, never by reading it whole, and `rm` it when the report is done.
`mktemp` with an explicit template under `$TMPDIR` is what keeps two concurrent runs from sharing a file.
Keep the `sandbox-audit.` name: the command skips any call that names it, which is what stops the next
audit counting these queries' output — EPERM lines, quoted — as fresh sandbox blocks.

`projects[].paths` is every working directory a project's sessions ran in — a checkout, a linked
worktree, or a subdirectory of either. Resolve each to its root with
`git -C <path> rev-parse --show-toplevel` (keep the path as given when it is not a work tree), and read
settings from the root: a session started in `repo/packages/web` still loads `repo/.claude/`. A path that
no longer exists is a removed worktree; skip it.

Know what the counts mean before reasoning from them:

- **`sandbox_blocks`** — sandboxed Bash calls the sandbox denied. A `network-outbound <host>` target came from
  Claude Code's own `<sandbox_violations>` report and is exact. Any other target is the tool's own EPERM
  line, normalized (`~`, `.claude/worktrees/*`, `<file>`, `#` for ids) — read it as evidence, not a path to
  allowlist verbatim.
- **`overrides`** — calls run with `dangerouslyDisableSandbox`, whatever became of them.
  `overrides_preemptive` had no sandbox block before them in their session: the agent guessed the sandbox
  would fail. `overrides_read_only` were reads the sandbox allows anyway. Both are **behaviour**, not a
  config gap.
- **`automode_denials`** — the classifier's refusals, with the reason it gave. An override the classifier
  refused counts here too, and in `override_outcomes`.
- `rule_denials`, `user_denials` — a `permissions.deny` rule, or the user at the prompt.

### 2. Read the current config

The user config directory is **`$CLAUDE_CONFIG_DIR`** when it is set, else `~/.claude` — resolve it once
and use it for every user-level read below. Read `<config dir>/settings.json`, and `.claude/settings.json` +
`.claude/settings.local.json` under each resolved project root that has events. Note `sandbox.*`, `permissions.*` (`allow`, `ask`, `deny`,
`additionalDirectories`), `autoMode.*`, `env`. Also `<config dir>/CLAUDE.md`, for step 3's
behaviour rules.

Before recommending any key, **check its exact name and semantics against the current docs** —
`https://code.claude.com/docs/en/sandboxing` and `…/settings-reference`. Settings keys change between
releases, and a misspelled key is silently ignored: it looks applied and does nothing. Flag anything you
could not confirm, such as how a wildcard inside an `excludedCommands` pattern matches.

### 3. Decide each finding

Group the buckets by root cause, not by command: toolchain caches and temp dirs, network hosts, protected
paths, unix sockets, non-HTTP network, nested sandboxes, no cause at all. Then give each root cause exactly
one disposition, preferring them in this order:

| disposition | when | the change |
| --- | --- | --- |
| **covered** | the current config already handles it | none — but a covered finding still occurring is a regression worth one line |
| **stop the traffic** | telemetry or analytics the task never needed (`telemetry.*`, `analytics.*`, `posthog`) | an `env` opt-out (`DO_NOT_TRACK`, `HOMEBREW_NO_ANALYTICS`, the tool's own) — never allowlist a tracker |
| **allowlist** | a host or a path a tool legitimately needs | `sandbox.network.allowedDomains`, `sandbox.filesystem.allowWrite`, or an `env` redirect of a temp or cache dir into a writable one |
| **pre-approve** | the classifier refused something routine the user always approves | a narrow `permissions.allow` rule, or an `autoMode.allow` sentence scoped to the exact operation |
| **exclude — the user's call** | a named tool that cannot run sandboxed *by design* and whose own job is the blocked write: `git push -u` recording its upstream in `.git/config`, `git worktree remove` deleting a worktree's `.vscode`/`.claude`, a tool that starts its own sandbox (`sandbox_apply`), docker, ssh | **proposed, never recommended.** `sandbox.excludedCommands` turns the sandbox off for *every* run of that command, not just the one write, so present it as a trade-off for the user to accept: the narrowest pattern, what it unsandboxes, and whether an `ask` rule keeps it prompting. Only for commands matched by name — never a pattern broad enough to cover arbitrary scripts |
| **stay blocked** | the guard did its job: a merge nobody asked for, a sandbox-bypass env var, **any read or write of a credential, secret or `.env` path**, a write to Claude Code's own configuration (`settings*.json`, `.claude/hooks`, `.claude/skills`, `~/.claude/plugins`) by anything other than the tool that owns it, destructive git during a merge | none — name it, so the next run does not propose it. A protected path is protected on purpose: its block is only ever an exclude candidate when the command writing it is that path's own tool (git for `.git/config`) |
| **behaviour** | preemptive overrides, overrides on read-only commands, an override carried forward to every later command | a rule for `~/.claude/CLAUDE.md`, not a setting |

A protected path cannot be allowlisted: an `allowWrite` entry covering it is inert. That makes it a
**stay blocked** row by default, and an **exclude** proposal only under that row's narrow condition — never
the reverse. Never propose widening `allowWrite` to a credentials directory, a shell rc file, or
anything under `~/.claude`.

Anything already on the ledger's **stay-blocked** list stays there unless the user says otherwise — report
its count, do not re-propose it.

### 4. Report

In this order, and nothing else:

1. **Headline table** — the counts, and against the ledger's last run where there is one (`sandbox_blocks
   105 → 40`). A delta is only meaningful over the same `--days`; say so if the windows differ.
2. **Settings diff** — per file, only the keys that change, as paste-ready JSON. One line under each change:
   the finding and how many events it removes.
3. **Stay blocked** — what, how often, why.
4. **Behaviour** — any CLAUDE.md wording, with the counts behind it.
5. **Undiagnosed** — buckets you could not attribute, one example command each.

Then ask which changes the user is taking. They paste them; this skill does not.

### 5. Update the ledger

After the user answers, write `${MKIT_HOME:-~/.mkit}/sandbox-audit.md` — overwrite it whole, it is a snapshot, not a log:

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

Carry every earlier entry forward; add this run's. If that directory is not writable, say so with the remedy
`mkit doctor` reports, print the ledger instead, and finish — the audit itself is complete without it.
