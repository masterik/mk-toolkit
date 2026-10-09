# Prerequisites

What the skills and the `mkit` binary need, and how to set them up under Claude Code's sandbox.
**macOS only** (Intel and Apple Silicon).

## Install

```bash
brew install git masterik/tap/mkit
```

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

Both are required: the plugin is Markdown, and every skill but `explain` starts by calling `mkit`.
A missing or too-old binary stops the skill with a `brew install` / `brew upgrade mkit` remedy.

Then check everything at once:

```bash
mkit doctor
```

`doctor` reports tool presence on `PATH`, the sandbox writable set, plugin state and permission
allowlist gaps. It reports only — it never changes anything — and it checks presence, not versions
or `gh` auth.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | `mkit gate run` | gate steps run as `bash -c '<command>'`; macOS's `/bin/bash` is enough |
| `mkit` | every skill but `explain` | run directory, starting facts, quality gate, branch classifier, findings arithmetic |

## Recommended

| Tool | Used by | Without it |
| --- | --- | --- |
| `gh` | `pr`, `finish`, `cleanup` | `pr` can't open a PR; `cleanup` classifies from git alone (`gh=gh-missing`) |
| `wt` ([Worktrunk](https://worktrunk.dev)) | `finish`, worktree classification | plain `git worktree remove` |
| `rg` (ripgrep) | `review`'s fix-checks sweep | `grep -E`, slower |
| `gh stack` ([github/gh-stack](https://github.com/github/gh-stack)) | `pr`/`finish` on stacked PRs | stacked PRs can't be created or merged by the skills |

```bash
brew install gh ripgrep worktrunk/tap/worktrunk
gh auth login
gh extension install github/gh-stack
```

### Extra reviewers for `review`

`review` wants three independent sources: **CodeRabbit** (`coderabbit` CLI or plugin), **Codex**
(`codex` CLI or plugin) and Claude. Any can be missing — the skill redistributes its lenses and
says so; it never reports a partial review as clean.

## Optional — repo config

`mkit init` writes a committed `.mkit/config.toml` pinning what discovery can't establish: spec
store, commit scopes and subject length, reviewers, review mode, merge style, gate commands, and
branches `cleanup` must keep. On a terminal it's a short wizard (Spec → Commit → Review → Merge);
every field is also a flag. Nothing requires it — every skill runs with no config
([ADR 0001](adr/0001-per-repo-config-and-init.md)). `mkit repo profile` shows what's discovered vs
pinned.

## Gatekeeper blocks the binary

Release binaries aren't signed or notarized yet ([#28](https://github.com/masterik/mk-toolkit/issues/28)),
so macOS quarantines each download. After every `brew install` / `brew upgrade`:

```bash
xattr -d com.apple.quarantine "$(which mkit)"
```

(Or System Settings → Privacy & Security → Allow Anyway.)

## Fewer permission prompts

Each subcommand is its own Bash pattern. Allow them once in `~/.claude/settings.json` or a repo's
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

`mkit gate run` runs the repo's own lint/test/build — leave it out to approve each gate yourself.

## Running under the OS sandbox

mkit keeps its state inside the working directory (`<toplevel>/.mkit/`) and `$TMPDIR`, which the
sandbox already allows, so **most of it needs no grant**
([ADR 0002](adr/0002-state-locations-under-a-sandbox.md)).

### `~/.mkit` — two steps, in order

Only the `sandbox-audit` skill writes here today. A grant covers a directory's *interior*, so it
can't create the directory — that's a write to `$HOME`, which nothing grants.

1. Create it from your own shell (in Claude Code, `!` runs outside the sandbox):
   ```
   ! mkdir -p ~/.mkit
   ```
2. Grant it:
   ```json
   { "permissions": { "additionalDirectories": ["~/.mkit"] } }
   ```

Skip step 1 and the first write fails with `mkdir: /Users/you/.mkit: Operation not permitted`.
`additionalDirectories` rather than `sandbox.filesystem.allowWrite`, because it also satisfies the
auto-mode classifier's "no writes outside the working directories" rule.

> **`~/.claude/…` can't be granted.** It's a protected region: an allowlist entry there is inert.
> That's why mkit never stores anything under it, and never tells you to allowlist a path there.

### What composed tools need

| Tool | Needs | Without it |
| --- | --- | --- |
| `go build`/`vet`/`test`, `golangci-lint` | `GOCACHE`, `GOMODCACHE`, `GOLANGCI_LINT_CACHE` in `$TMPDIR` | the gate fails on cache writes, not your code |
| `gh run view --log` | `~/.cache/gh` in `additionalDirectories` | log fetch fails |
| `codex` CLI | `~/.codex` in `additionalDirectories` | fails to initialise |
| `coderabbit` CLI | `~/.coderabbit` in `additionalDirectories` | its state writes fail |
| `git push`/`fetch` over HTTPS | nothing | a harmless credential-store warning on stderr |

```bash
export GOCACHE="$TMPDIR/go-build" GOMODCACHE="$TMPDIR/go-mod" GOLANGCI_LINT_CACHE="$TMPDIR/golangci"
```

### Network

| Skill / command | Hosts |
| --- | --- |
| `pr`, `cleanup`, `mkit branch status`, `mkit facts --gh` | `api.github.com`, `github.com` |
| `pr`, `finish`, `cleanup` | your remote's host |
| `review`'s external reviewers | whatever `codex` / `coderabbit` call |

`cleanup` degrades when GitHub is unreachable (`fetch=failed`, `gh=gh-error`) and still classifies
from git; `pr` cannot open a PR without `api.github.com`.

### Gotchas

- **`ps`/`pgrep` fail outright** under the sandbox — don't build "is it running?" checks on them.
- **`mktemp` without a template ignores `$TMPDIR`** on macOS and is denied. Always pass a template.
- **`.git/config` and `.git/hooks` are protected** even inside the working directory, so `git config`
  writes fail.
- **`wt` is usually a shell function**, so `mkit facts` looks for the real binary on `PATH` and
  reports `wt_bin=none` as advisory.
- **`rtk` is never used inside mkit** — it reshapes output for reading, which a parser can't
  tolerate. It stays on the agent's own commands.

## Development

```bash
brew install go golangci-lint just
just ci     # build, vet, test, lint — what CI runs (golangci-lint v2.12)
```

Under the sandbox, export the Go cache variables above first. Tests build throwaway repos under
`$TMPDIR` and point `MKIT_HOME` at a temp directory; none touch your real home. A test suite that
reaches user-scoped state must set `MKIT_HOME` in its own helper (see `internal/cli`'s `factsRepo`,
`internal/core/doctor`'s `isolate`) — `scratch.UserDirWritable` creates the directory if absent.
