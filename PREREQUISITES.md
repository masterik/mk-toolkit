# Prerequisites

mkit is Markdown plus six small scripts and one hook. There is nothing to build and nothing to
put on `PATH` — installing the plugin is a clone. What follows is what the scripts call.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | every `.sh` — six scripts plus the sourced `lib/common.sh` | macOS's system bash is 3.2; nothing here needs 4.x |
| `node` ≥ 18 | `findings.mjs` | ESM, `node:fs`. No npm install, no dependencies |
| `jq` ≥ 1.6 | `gate-detect.sh`, `facts.sh`, `journal.sh`, the hook | reads `package.json`, `wt list --format=json`, and the journal's JSONL |

```bash
# macOS
brew install git jq node
# Debian / Ubuntu
sudo apt install -y git jq
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt install -y nodejs
```

Claude Code now ships as a native binary, so **it no longer guarantees a `node` on the
machine** — install one even if Claude Code runs fine without it.

## Recommended

| Tool | Used by | Degrades to |
| --- | --- | --- |
| `rg` (ripgrep) | `gate-run.sh` failure digest, `gate-detect.sh` doc scan, the `fix-checks` sweep | `grep -E` (same output, slower) |
| `gh` | `pr`, and `facts.sh --gh` | `pr` cannot open a PR at all; `facts.sh` prints `pr=gh-missing` |
| `wt` ([worktrunk](https://worktrunk.dev)) | `finish` cleanup, `facts.sh` worktree classification | plain `git worktree remove` |

```bash
brew install ripgrep gh worktrunk/tap/worktrunk   # macOS
sudo apt install -y ripgrep && gh auth login      # Debian/Ubuntu
```

## Dev only — running `tests/`

Contributors testing the scripts themselves need `bats-core`; users of the plugin never do.

```bash
brew install bats-core                     # macOS
npm install -g bats                        # or via npm, any OS
./tests/run.sh                             # node --test findings.mjs, then bats tests/bats/
```

## Optional — extra reviewers for `review`

`review` wants three independent sources. Any one can be missing; the skill redistributes
its lenses and says so in the summary. It never reports a partial review as clean.

- **CodeRabbit** — `coderabbit` CLI, or the `coderabbit` plugin's review skill.
- **Codex** — `codex` CLI, or the `codex` plugin's rescue skill.
- Claude alone still works: `review` runs two subagents with different lens splits so
  corroboration keeps meaning something.

## Opt-in — the commit journal

Journaling records *why* each unit of work exists, for `commit` to spend later. It is off in
every repo until you turn it on there — one command, no other setup:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh" enable    # writes <git-dir>/mkit/journal.enabled
"${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh" enabled   # → enabled | disabled
```

Needs nothing beyond the `jq` already required above.

**The hook needs no user action, and ships inert.** Claude Code loads a plugin's
`hooks/hooks.json` automatically: plugin hooks require **no opt-in beyond installing the
plugin**, and subagents inherit them. That is exactly why the marker gate is non-negotiable —
mkit's `Stop` / `SubagentStop` hook is registered from the moment the plugin is installed, so
without the gate it would start writing in every repo you touch. Outside a git repo, or in a
repo with no marker, it exits 0 with no output and writes nothing.

It also needs no allowlist entry: Claude Code runs the hook itself, not the agent through
`Bash`, so it never prompts.

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
