# Prerequisites

This page covers what the skills and the optional `mkit` binary need, and how to set them up under
Claude Code's sandbox.

## Install

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

The plugin is all the skills need. **Optional, macOS only:** the `mkit` binary, which `sandbox-audit`
needs and which adds a prerequisite report and storage cleanup:

```bash
brew install masterik/tap/mkit
mkit doctor
```

`doctor` reports, and never changes anything. It checks:

- whether tools are on `PATH`, and the `bash` version;
- plugin state, including drift between plugin and binary versions;
- the sandbox's writable set;
- missing `additionalDirectories` grants;
- files older mkit releases left behind (`.mkit/`, mkit's lines in `info/exclude`).

It does not check other tool versions, `gh` auth, or Bash allow rules.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | every skill but `explain` and `sandbox-audit` | the bundled `facts.sh`/`branch-status.sh`, and the repo's gate commands; macOS's `/bin/bash` is enough |
| `mkit` | `sandbox-audit` only | reads transcripts and settings (`mkit audit sandbox`) |

## Recommended

| Tool | Used by | Without it |
| --- | --- | --- |
| `gh` | `pr`, `finish`, `cleanup` | `pr` can't open a PR; `cleanup` classifies from git alone (`gh=gh-missing`) |
| `wt` ([Worktrunk](https://worktrunk.dev)) | `finish`, worktree classification | plain `git worktree remove` |
| `rg` (ripgrep) | `review`'s fix-checks sweep | slower `grep -E` |
| `gh stack` ([github/gh-stack](https://github.com/github/gh-stack)) | `pr`/`finish` on stacked PRs | no stacked PRs |

```bash
brew install gh ripgrep worktrunk/tap/worktrunk
gh auth login
gh extension install github/gh-stack
```

**Extra reviewers.** `review` uses three sources: CodeRabbit (CLI or plugin), Codex (CLI or plugin)
and Claude. Any of them can be missing. The skill redistributes the work, says so, and never reports
a partial review as clean.

## Optional: repo config

Run `/mkit:setup` in any agent: it looks at the repo, proposes answers, and writes a short prose file
in `docs/agents/` only for a choice that differs from the default — `commit.md` (scopes, subject
length), `review.md` (default mode), `merge.md` (merge style, reviewers), `remote.md`, `gate.md`,
`cleanup.md` (branches to keep) — plus a `### mkit workflow` block under `## Agent skills` in
`CLAUDE.md`/`AGENTS.md`. Edit the files by hand any time. The spec store is mattpocock's
`docs/agents/issue-tracker.md`. Nothing requires any of it
([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 7).

An old `.mkit/config.toml` (from `mkit init`) is read by no skill: `setup` converts it and says to
remove it; you (or `commit`) run `git rm`.

## Gatekeeper blocks the binary

Releases aren't signed or notarized yet ([#28](https://github.com/masterik/mk-toolkit/issues/28)). The
cask clears the quarantine flag on install. If macOS still blocks `mkit` (an older cask, or a binary
fetched some other way), run:

```bash
xattr -d com.apple.quarantine "$(which mkit)"
```

You can also allow it in System Settings → Privacy & Security → Allow Anyway.

## Fewer permission prompts

Each subcommand is its own Bash pattern. Allow them once in `~/.claude/settings.json` or in a repo's
`.claude/settings.json`:

```json
{
  "permissions": {
    "allow": [
      "Bash(mkit audit sandbox:*)"
    ]
  }
}
```

The gate itself — the repo's own lint, test and build — runs as those commands, so their own allow
rules (or prompts) apply.

## Running under the OS sandbox

Per-run files live in `$TMPDIR`, already writable, and nothing goes in the work tree, so **most of
mkit needs no grant** ([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 5).

### `~/.mkit`: two steps, in order

Today only `sandbox-audit` writes here. A grant covers a directory's interior but can't create the
directory itself, because that would be a write to `$HOME`.

1. Create it outside the sandbox (in Claude Code, `!` runs a command outside it):
   ```
   ! mkdir -p ~/.mkit
   ```
2. Grant it:
   ```json
   { "permissions": { "additionalDirectories": ["~/.mkit"] } }
   ```

If you skip step 1, the first write fails with `mkdir: /Users/you/.mkit: Operation not permitted`. Use
`additionalDirectories` rather than `sandbox.filesystem.allowWrite`, because it also satisfies auto
mode's "no writes outside the working directories" rule.

> **`~/.claude/…` can't be granted.** It is a protected region, so an allowlist entry there has no
> effect. That is why mkit never stores anything there.

### Composed tools

| Tool | Needs | Without it |
| --- | --- | --- |
| `go`, `golangci-lint` | `GOCACHE`, `GOMODCACHE`, `GOLANGCI_LINT_CACHE` in `$TMPDIR` | the gate fails on cache writes |
| `gh run view --log` | `~/.cache/gh` in `additionalDirectories` | the log fetch fails |
| `codex` CLI | `~/.codex` in `additionalDirectories` | it fails to initialise |
| `coderabbit` CLI | `~/.coderabbit` in `additionalDirectories` | its state writes fail |
| `git push`/`fetch` over HTTPS | nothing | a harmless credential-store warning |

```bash
export GOCACHE="$TMPDIR/go-build" GOMODCACHE="$TMPDIR/go-mod" GOLANGCI_LINT_CACHE="$TMPDIR/golangci"
```

### Network

| Skill / command | Hosts |
| --- | --- |
| `pr`, `finish`, `cleanup` (`branch-status.sh`) | `api.github.com`, `github.com` |
| `facts.sh <skill> --gh` | `api.github.com` |
| `pr`, `finish`, `cleanup` | your remote's host |
| `review`'s external reviewers | whatever `codex` and `coderabbit` call |

If GitHub is unreachable, `cleanup` still classifies from git. It reports `fetch=failed` plus
`gh=gh-unauthenticated` or `gh=gh-error`. `pr` can't open a PR without `api.github.com`.

### Gotchas

- **`ps` and `pgrep` fail** under the sandbox.
- **`mktemp` without a template ignores `$TMPDIR`** on macOS, and the write is denied. Always pass a
  template.
- **`.git/config` and `.git/hooks` are protected**, so `git config` writes fail.
- **`wt` is usually a shell function.** A script does not inherit it, so skills check for `wt` in their
  own shell (`type wt`), never through `facts.sh`.
- **mkit never calls `rtk`.** `rtk` reshapes output for reading, which breaks a parser.

## Development

See [CONTRIBUTING.md](../CONTRIBUTING.md).
