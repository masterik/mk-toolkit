# Prerequisites

This page covers what the skills and `mkit` need, and how to set them up under Claude Code's sandbox.
**macOS only** (Intel and Apple Silicon).

## Install

```bash
brew install git masterik/tap/mkit
```

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

You need both, because every skill except `explain` calls `mkit`. If the binary is missing or too
old, the skill stops and tells you to run `brew install` or `brew upgrade mkit`.

```bash
mkit doctor
```

`doctor` reports, and never changes anything. It checks:

- whether tools are on `PATH`;
- plugin state, including drift between plugin and binary versions;
- the sandbox's writable set;
- missing `additionalDirectories` grants.

It does not check tool versions, `gh` auth, or Bash allow rules.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | `mkit gate run` | gate steps run as `bash -c '<command>'`; macOS's `/bin/bash` is enough |
| `mkit` | every skill but `explain` | run dir, starting facts, gate, branch classifier, findings |

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

`mkit init` writes a committed `.mkit/config.toml`. It pins two kinds of value:

- **What it discovers:** gate commands, commit scopes, spec store and merge style, so runs stop
  re-discovering them.
- **What discovery can't establish:** subject length, review mode, and the branches `cleanup` must
  keep.

Reviewers stay discovered per path from CODEOWNERS unless you pin a list. On a terminal, `init` runs a
short wizard (Spec → Commit → Review → Merge), and every field is also a flag. Nothing requires the
config ([ADR 0001](adr/0001-per-repo-config-and-init.md)). `mkit repo profile` shows which values are
discovered and which are pinned. `mkit init --force` re-discovers and reopens the wizard.

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
      "Bash(mkit facts:*)",
      "Bash(mkit scratch:*)",
      "Bash(mkit findings:*)",
      "Bash(mkit worklog:*)",
      "Bash(mkit branch status:*)",
      "Bash(mkit gate detect:*)",
      "Bash(mkit gate run:*)"
    ]
  }
}
```

`mkit gate run` runs the repo's own lint, test and build. Leave it out of the list if you want to
approve each gate run.

## Running under the OS sandbox

State lives in `<toplevel>/.mkit/` and `$TMPDIR`, both already writable, so **most of mkit needs no
grant** ([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).

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
| `pr`, `finish`, `cleanup`, `mkit branch status` | `api.github.com`, `github.com` |
| `mkit facts <skill> --gh` | `api.github.com` |
| `pr`, `finish`, `cleanup` | your remote's host |
| `review`'s external reviewers | whatever `codex` and `coderabbit` call |

If GitHub is unreachable, `cleanup` still classifies from git. It reports `fetch=failed` plus
`gh=gh-unauthenticated` or `gh=gh-error`. `pr` can't open a PR without `api.github.com`.

### Gotchas

- **`ps` and `pgrep` fail** under the sandbox.
- **`mktemp` without a template ignores `$TMPDIR`** on macOS, and the write is denied. Always pass a
  template.
- **`.git/config` and `.git/hooks` are protected**, so `git config` writes fail.
- **`wt` is usually a shell function.** `mkit facts` looks for the real binary on `PATH` and reports
  `wt_bin=none` as advisory.
- **mkit never calls `rtk`.** `rtk` reshapes output for reading, which breaks a parser.

## Development

See [CONTRIBUTING.md](../CONTRIBUTING.md).
