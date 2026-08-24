# Prerequisites

mkit is Markdown plus six small scripts and two hooks. There is nothing to build and nothing to
put on `PATH` — installing the plugin is a clone, and the one piece of user-scoped setup happens
by itself on the next session (see *the commit journal*, below). What follows is what the
scripts call.

**macOS is the supported platform.** Nothing here detects an OS or branches on one; the scripts
are simply written to what macOS provides, which is the narrower target: no GNU-only flags, no
`flock`, no bash 4.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | every `.sh` — six scripts plus the sourced `lib/common.sh` | macOS ships `/bin/bash` 3.2 (frozen there over GPLv3) and `/bin/zsh` 5.9. The scripts run under bash via `#!/usr/bin/env bash`, so **your interactive shell being zsh is irrelevant** — nothing here needs 4.x, and no Homebrew bash is required |
| `node` ≥ 18 | `findings.mjs` | ESM, `node:fs`. No npm install, no dependencies |
| `jq` ≥ 1.6 | `gate-detect.sh`, `gate-run.sh`, `facts.sh`, `journal.sh`, the hook | reads `package.json`, `wt list --format=json`, and the journal's and gate ledger's JSONL |

```bash
brew install git jq node
```

Claude Code now ships as a native binary, so **it no longer guarantees a `node` on the
machine** — install one even if Claude Code runs fine without it.

## Recommended

| Tool | Used by | Degrades to |
| --- | --- | --- |
| `rg` (ripgrep) | `gate-run.sh` failure digest, `gate-detect.sh` doc scan, the `fix-checks` sweep | `grep -E` (same output, slower) |
| `shasum` | the gate ledger's content fingerprint | `gate_cache=no-hash` — the gate runs every step, exactly as before. Never a hard requirement: a latency optimization may not add a prerequisite. macOS ships it, but it is a Perl script, so a stripped environment can lack it |
| `gh` | `pr`, and `facts.sh --gh` | `pr` cannot open a PR at all; `facts.sh` prints `pr=gh-missing` |
| `wt` ([worktrunk](https://worktrunk.dev)) | `finish` cleanup, `facts.sh` worktree classification | plain `git worktree remove` |

```bash
brew install ripgrep gh worktrunk/tap/worktrunk
gh auth login
```

## Dev only — running `tests/`

Contributors testing the scripts themselves need `bats-core`; users of the plugin never do.

```bash
brew install bats-core
./tests/run.sh                             # node --test findings.mjs, then bats tests/bats/
```

## Optional — extra reviewers for `review`

`review` wants three independent sources. Any one can be missing; the skill redistributes
its lenses and says so in the summary. It never reports a partial review as clean.

- **CodeRabbit** — `coderabbit` CLI, or the `coderabbit` plugin's review skill.
- **Codex** — `codex` CLI, or the `codex` plugin's rescue skill.
- Claude alone still works: `review` runs two subagents with different lens splits so
  corroboration keeps meaning something.

## The commit journal — on by default, no setup

Journaling records *why* each unit of work exists, for `commit` to spend later instead of
re-deriving intent from the diff. **It configures itself.** On your next session after
installing the plugin, the `SessionStart` hook (`scripts/hooks/session-bootstrap.sh`) writes
`~/.claude/mkit/journal.default` — the user-scoped marker that makes journaling apply to every
repo, including ones you clone tomorrow — and tells you once that it did. Nothing to run.

It needs nothing beyond the `jq` already required above, and the hook itself needs not even
that: it reports a missing `jq` rather than depending on one.

Opting out, at either scope:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh" disable    # this repo only — beats the default
"${CLAUDE_PLUGIN_ROOT}/install.sh" --uninstall        # everywhere, and it sticks
"${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh" enabled --why   # → enabled user | disabled repo | …
```

`--uninstall` writes `~/.claude/mkit/bootstrap.disabled`, and the hook honours it forever.
That tombstone is not bookkeeping for its own sake: a deleted file carries no provenance, so
"never set up" and "deliberately removed" are byte-identical on disk, and without a record of
the *intent* an uninstall would last exactly until the next session. Re-running `install.sh`
clears it. `install.sh --status` reports which state you are in.

**Both hooks need no user action.** Claude Code loads a plugin's `hooks/hooks.json`
automatically: plugin hooks require **no opt-in beyond installing the plugin**, and subagents
inherit them. That is why each one is gated on something:

- `SessionStart` → `session-bootstrap.sh` acts once and then produces zero bytes on every
  later session, and stays silent forever once the tombstone exists.
- `Stop` / `SubagentStop` → `journal-nudge.sh` exits 0 with no output outside a git repo, or in
  a repo whose own `journal.sh disable` tombstone outranks the default.

Neither needs an allowlist entry: Claude Code runs a hook itself, not the agent through `Bash`,
so neither ever prompts.

## Verify

```bash
for t in git bash node jq rg gh wt coderabbit codex; do
	printf '%-12s %s\n' "$t" "$(command -v "$t" || echo '— not found')"
done
git --version; node --version; jq --version
```

Then check the plugin itself, from any repo:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh" commit --no-run   # prints a fact block
"${CLAUDE_PLUGIN_ROOT}/scripts/gate-detect.sh"             # prints fast= and full=
node "${CLAUDE_PLUGIN_ROOT}/scripts/findings.mjs" schema    # prints the JSONL shape
"${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh" enabled         # prints enabled or disabled
"${CLAUDE_PLUGIN_ROOT}/install.sh" --status                # what setup is in place
"${CLAUDE_PLUGIN_ROOT}/scripts/hooks/session-bootstrap.sh" </dev/null   # silent once set up
```

Empty `${CLAUDE_PLUGIN_ROOT}` fails as `/scripts/facts.sh: not found`. That is intended:
find the plugin checkout and call the script by its real path rather than working around it.

## Fewer permission prompts

Each new script is a new Bash pattern, so the first run of each asks. Allow them once, in
`~/.claude/settings.json` (user-wide) or a repo's `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": [
      "Bash(*/mkit/scripts/facts.sh:*)",
      "Bash(*/mkit/scripts/gate-detect.sh:*)",
      "Bash(*/mkit/scripts/gate-run.sh:*)",
      "Bash(*/mkit/scripts/journal.sh:*)",
      "Bash(*/mkit/scripts/run-open.sh:*)",
      "Bash(node */mkit/scripts/findings.mjs:*)"
    ]
  }
}
```

Adjust the path fragment to wherever the plugin is installed — under
`~/.claude/plugins/cache/<marketplace>/mkit/<version>/` for a marketplace install, or your
checkout for a local one. `gate-run.sh` runs the repo's own lint/test/build, so allowlisting
it delegates that trust; leave it out if you would rather approve each gate.

## Two things worth knowing

**`wt` is usually a shell function.** Worktrunk's shell integration has to be a function to
`cd` the parent shell, and it shadows the binary. `command -v wt` then answers `wt` with no
path, so the scripts walk `PATH` for the real executable instead, and report `wt_bin=none`
as *advisory* — the agent's own shell may still have `wt` when a script does not.

**`rtk` is deliberately not used inside the scripts.** It reshapes command output for an
agent to read — it strips the leading space from `git diff --stat`, for one — which is
exactly what a parser must not tolerate. The scripts consume `--porcelain`,
`--shortstat`/`--name-only` and `--format=json`, and do their own compaction. rtk stays where it belongs:
on the agent's own direct commands, via the user's hook.
