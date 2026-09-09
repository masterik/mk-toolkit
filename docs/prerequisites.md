# Prerequisites

mkit's plugin payload is Markdown plus six helpers — five shell scripts and one dependency-free
Node file — and one hook. Nothing in it needs building and there is no setup step: installing the
plugin is a clone. What follows is what those scripts call.

The `mkit` **binary** is a separate, optional install (`brew install masterik/tap/mkit`), and
nothing below requires it. It is [taking over the script layer](backlog.md) one milestone at a
time, and each script it replaces deletes a row from this page: `node` goes with `findings.mjs`
(M4), `jq` and `shasum` with the gate port (M5).

**macOS is the supported platform — for the scripts and for the binary.** Nothing detects an OS or
branches on one; the scripts are simply written to what macOS provides, which is the narrower
target: no GNU-only flags, no `flock`, no bash 4. The binary keeps the same scope: the release
builds `darwin` × amd64/arm64 (Intel + Apple Silicon), and the Homebrew cask that installs it is
macOS-only in any case.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | every `.sh` — five helpers, one hook, `install.sh`, plus the sourced `lib/common.sh` | macOS ships `/bin/bash` 3.2 (frozen there over GPLv3) and `/bin/zsh` 5.9. The scripts run under bash via `#!/usr/bin/env bash`, so **your interactive shell being zsh is irrelevant** — nothing here needs 4.x, and no Homebrew bash is required |
| `node` ≥ 18 | `findings.mjs` | ESM, `node:fs`. No npm install, no dependencies |
| `jq` ≥ 1.6 | `gate-detect.sh`, `gate-run.sh`, `facts.sh`, `branch-scan.sh` | reads `package.json`, `wt list --format=json`, `gh`'s JSON, and the gate ledger's JSONL |

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
| `gh` | `pr`, `facts.sh --gh`, and `branch-scan.sh` (`cleanup`) | `pr` cannot open a PR at all; `facts.sh` prints `pr=gh-missing`; `branch-scan.sh` falls back to git-only classification and reports `gh=gh-missing` |
| `wt` ([worktrunk](https://worktrunk.dev)) | `finish` cleanup, `facts.sh` worktree classification | plain `git worktree remove` |

```bash
brew install ripgrep gh worktrunk/tap/worktrunk
gh auth login
```

## Dev only — running the tests

Contributors need more than users do; none of this is required to *use* the plugin.

```bash
brew install bats-core go golangci-lint    # bats for the shell suites, Go for the binary
./tests/run.sh                             # node --test findings.mjs, then bats tests/bats/
go build ./... && go vet ./... && go test ./...   # the binary — what CI runs
golangci-lint run                          # CI pins v2.12
```

**Under the sandbox the Go gate needs its caches redirected**, or it fails on `~/Library/Caches/
go-build` and `~/go` rather than on your code — see [Running under the OS sandbox](#running-under-the-os-sandbox):

```bash
export GOCACHE="$TMPDIR/go-build" GOMODCACHE="$TMPDIR/go-mod" GOLANGCI_LINT_CACHE="$TMPDIR/golangci"
```

The bats suite needs nothing: `helpers.bash` points `MKIT_HOME` at a throwaway directory under
`$TMPDIR` for every test, which is both the sandbox containment story and the reason a developer's
own state cannot make an assertion pass or fail.

## Optional — extra reviewers for `review`

`review` wants three independent sources. Any one can be missing; the skill redistributes
its lenses and says so in the summary. It never reports a partial review as clean.

- **CodeRabbit** — `coderabbit` CLI, or the `coderabbit` plugin's review skill.
- **Codex** — `codex` CLI, or the `codex` plugin's rescue skill.
- Claude alone still works: `review` runs two subagents with different lens splits so
  corroboration keeps meaning something.

## The `SessionStart` hook — no setup, no action

The plugin ships one hook. `scripts/hooks/session-bootstrap.sh` reports, **once per tool**, any
prerequisite above that is missing — a gap that otherwise surfaces later as a thinner `facts.sh`
block or a `gate_cache=no-jq` annotation, and costs far more to debug than to be told about. It
installs nothing and writes nothing outside `~/.mkit/`.

It needs no user action. Claude Code loads a plugin's `hooks/hooks.json` automatically: plugin
hooks require **no opt-in beyond installing the plugin**, and subagents inherit them. It is gated
accordingly — every session after it has said its piece produces zero bytes, and a tool that
comes back loses its record, so a later removal warns again.

The hook itself depends on nothing external, not even `jq`: a reporter that needs the tool it
reports on is unavailable in exactly the case that matters.

Silencing it for good:

```bash
"${CLAUDE_PLUGIN_ROOT}/install.sh" --uninstall   # writes the tombstone; it sticks
"${CLAUDE_PLUGIN_ROOT}/install.sh" --status      # prerequisites, hook state, gate ledger
```

`--uninstall` writes `~/.mkit/bootstrap.disabled`, and the hook honours it forever. That
tombstone is not bookkeeping for its own sake: a deleted file carries no provenance, so "never
warned" and "warned and dismissed" are byte-identical on disk, and without a record of the
*intent* the dismissal would last exactly until the next session. Delete that file to undo.

The hook needs no allowlist entry: Claude Code runs a hook itself, not the agent through `Bash`,
so it never prompts.

## Verify

```bash
for t in git bash node jq rg gh wt coderabbit codex mkit; do
	printf '%-12s %s\n' "$t" "$(command -v "$t" || echo '— not found')"
done
git --version; node --version; jq --version
```

Then check the plugin itself, from any repo:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh" commit --no-run   # prints a fact block
"${CLAUDE_PLUGIN_ROOT}/scripts/gate-detect.sh"             # prints fast= and full=
node "${CLAUDE_PLUGIN_ROOT}/scripts/findings.mjs" schema    # prints the JSONL shape
"${CLAUDE_PLUGIN_ROOT}/install.sh" --status                # prerequisites, hook, gate ledger
"${CLAUDE_PLUGIN_ROOT}/scripts/hooks/session-bootstrap.sh" </dev/null   # silent once it has spoken
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
      "Bash(*/mkit/scripts/run-open.sh:*)",
      "Bash(*/mkit/scripts/branch-scan.sh:*)",
      "Bash(node */mkit/scripts/findings.mjs:*)"
    ]
  }
}
```

Adjust the path fragment to wherever the plugin is installed — under
`~/.claude/plugins/cache/<marketplace>/mkit/<version>/` for a marketplace install, or your
checkout for a local one. `gate-run.sh` runs the repo's own lint/test/build, so allowlisting
it delegates that trust; leave it out if you would rather approve each gate.

## Running under the OS sandbox

Claude Code can run Bash tool calls inside an OS sandbox (Seatbelt on macOS). Everything below was
measured on 2026-09-09 with `sandbox.enabled: true`. **One entry is all mkit itself needs**; the rest
of this section is what the tools mkit *composes* need, and the two facts that no script may forget.

### The one grant mkit needs

```json
{
  "permissions": {
    "additionalDirectories": ["~/.mkit"]
  }
}
```

`~/.mkit/` holds `bootstrap.state` (which one-time prerequisite messages have been said) and
`bootstrap.disabled` (the uninstall tombstone). Without the entry, mkit still works: the session hook
goes silent rather than repeating itself, and `install.sh --status` reports the directory as not
writable with this same remedy.

`additionalDirectories` rather than `sandbox.filesystem.allowWrite` deliberately — it grants the
sandbox write *and* makes the path a working directory, which also satisfies the auto-mode
classifier's "no writes outside the working directories" rule. `allowWrite` alone leaves that rule
biting.

**Nothing else of mkit's needs a grant.** Run directories and `gate.jsonl` live in
`<toplevel>/.mkit/`, inside the working directory the sandbox already writes; ephemeral files live in
`$TMPDIR`. See [ADR 0002](adr/0002-state-locations-under-a-sandbox.md).

> **`~/.claude/…` cannot be granted.** Anything under `~/.claude` (or `CLAUDE_CONFIG_DIR`) is a
> *protected* path: an `allowWrite` entry or an `Edit` allow rule covering it does not lift the
> protection, and the only lever is `filesystem.disabled`, which turns filesystem isolation off
> everywhere. Measured: a path in that region already listed in `allowWrite` still failed with
> `Operation not permitted`. This is why mkit's user-scoped state is not there — and why no mkit
> surface will ever tell you to allowlist a path under it.

### What the composed tools need

| tool | needs | without it |
| --- | --- | --- |
| `go build` / `go vet` / `go test` | `GOCACHE`, `GOMODCACHE` (and `GOLANGCI_LINT_CACHE` for the linter) redirected into `$TMPDIR` — the defaults under `$HOME` are denied | the Go gate fails on cache writes, not on your code |
| `gh run view --log` | `~/.cache/gh` in `additionalDirectories` | the log fetch fails |
| `git push` / `fetch` over HTTPS | nothing — the credential-helper *store* write is denied and prints on stderr, but exit status and parsed output are unaffected | noise only |
| `codex` CLI | `~/.codex` in `additionalDirectories` | `could not create PATH aliases`, `failed to initialize in-process app-server client` |
| `coderabbit` CLI | `~/.coderabbit` in `additionalDirectories` | its own state writes fail |

For the Go gate, either export the redirection in the session or wrap the commands:

```bash
export GOCACHE="$TMPDIR/go-build" GOMODCACHE="$TMPDIR/go-mod" GOLANGCI_LINT_CACHE="$TMPDIR/golangci"
```

### Network, for the remote-facing skills

Sandboxed egress goes through a filtering proxy, so the network side is an allowlist too. What the
payload itself reaches for:

| skill / script | host | for |
| --- | --- | --- |
| `pr`, `facts.sh --gh`, `branch-scan.sh` | `api.github.com`, `github.com` | `gh pr view`, `gh pr list`, `gh pr create` |
| `pr`, `finish`, `cleanup` | your remote's host (`git remote -v`) | `fetch`, `push` |
| `review`'s external reviewers | whatever the `codex` / `coderabbit` CLI calls | those are their own tools; check their docs for the hosts |

`cleanup` and `branch-scan.sh` degrade rather than fail when GitHub is unreachable: `fetch=failed`
and `gh=gh-error`, with every branch still classified from git alone. `pr` cannot open a PR without
`api.github.com` — there is no local substitute for that one.

Attempt the call and read the error rather than predicting reachability; a denied connection is
reported as such.

### Two facts no script may forget

**`ps` and `pgrep` cannot list processes at all** under the sandbox — `operation not permitted: ps`,
not an empty result. No script may depend on either, and a "is it still running?" check built on one
reads as *not running*.

**`mktemp` with no template ignores `$TMPDIR`.** On macOS the bare and `-t` forms resolve the Darwin
per-user temp directory (`/var/folders/…/T/`), which the sandbox denies. It is not fixable by
environment. Always pass a template: `mktemp "$TMPDIR/name.XXXXXX"`. Both forms are banned from the
payload and a test asserts it.

Also protected *inside* the working directory: `.git/config` and `.git/hooks`. So `git config` fails,
and a nested `git init` under the project half-fails.

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
