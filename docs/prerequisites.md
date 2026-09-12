# Prerequisites

mkit's plugin payload is Markdown plus five shell helpers — bash only since M4 took the last
Node file with it. No hooks (removed in 0.15.0), nothing to build, and no *machine* setup step:
installing the plugin is a clone. What follows is what those scripts call.

Per **repo** there is now one optional step, `mkit init`, which writes `.mkit/config.toml`
([ADR 0001](adr/0001-per-repo-config-and-init.md)). It pins what inspection cannot establish and
is never a precondition: every command and every skill runs with no config present.

The `mkit` **binary** is a separate install (`brew install masterik/tap/mkit`). It is
[taking over the script layer](backlog.md) one milestone at a time, and each script it replaces
deletes a row from this page — `node` left with `findings.mjs` (M4); `jq` and `shasum` go with the
gate port (M5). One skill now **requires** it: `review` does its findings arithmetic with
`mkit findings` and stops at step 0 if the binary is absent or too old to know that subcommand.
Everything else still runs without it.

> **`mkit doctor` reports on this page's tooling**, plus the sandbox writable set, the plugin
> payload and the allowlist gaps below — on demand, reporting only. It checks *presence on
> `PATH`*, not versions, `gh` authentication or cache state, so a too-old tool still reads as
> healthy here and fails later. Two things it cannot tell
> you, both deliberate: it does not run unprompted at session start, and it cannot report that
> `mkit` itself is missing — a binary cannot report its own absence. That second gap is why
> `review` probes for `mkit findings` itself, at step 0, and why `facts.sh` reports `mkit=` and
> `mkit_bin=` as raw starting facts without comparing them to anything.

**macOS is the supported platform — for the scripts and for the binary.** Nothing detects an OS or
branches on one; the scripts are simply written to what macOS provides, which is the narrower
target: no GNU-only flags, no `flock`, no bash 4. The binary keeps the same scope: the release
builds `darwin` × amd64/arm64 (Intel + Apple Silicon), and the Homebrew cask that installs it is
macOS-only in any case.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | every `.sh` — five helpers plus the sourced `lib/common.sh` | macOS ships `/bin/bash` 3.2 (frozen there over GPLv3) and `/bin/zsh` 5.9. The scripts run under bash via `#!/usr/bin/env bash`, so **your interactive shell being zsh is irrelevant** — nothing here needs 4.x, and no Homebrew bash is required |
| `jq` ≥ 1.6 | `facts.sh`, `branch-scan.sh` | reads `wt list --format=json` and `gh`'s JSON |

```bash
brew install git jq
```

## Recommended

| Tool | Used by | Degrades to |
| --- | --- | --- |
| `rg` (ripgrep) | the `fix-checks` sweep | `grep -E` (same output, slower) |
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
./tests/run.sh                             # bats tests/bats/
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

## No hook, and no setup step

The plugin ships **no hooks** and no installer. Both existed once: a `SessionStart` hook
(`session-bootstrap.sh`) named a missing prerequisite once per tool, and `install.sh` provided
`--status` and an `--uninstall` tombstone to silence it. Both were removed in 0.15.0 — the
prerequisite report belongs to the binary, and `mkit doctor` (M7) owns it now.

**The gap that leaves, stated plainly:** nothing tells you unprompted that a tool from the table
above is missing. It surfaces later — a thinner `facts.sh` block, a `pr=gh-missing`
annotation on a gate report — which is exactly the debugging cost the hook existed to avoid.
`mkit doctor` is the check, and it is human-run: you have to think to run it. It does not fully
replace the hook and never will — a binary cannot report its own absence, and cannot speak at
session start. That is accepted.

Nothing needs silencing any more, so `~/.mkit/bootstrap.disabled` no longer does anything; delete
it if you have one.

## Verify

```bash
for t in git bash jq rg gh wt coderabbit codex mkit; do
	printf '%-12s %s\n' "$t" "$(command -v "$t" || echo '— not found')"
done
git --version; jq --version; mkit version
```

Then check the plugin itself, from any repo:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/facts.sh" commit --no-run   # prints a fact block
mkit gate detect                                           # prints full= and its sources
mkit findings schema                                       # prints the JSONL shape
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
      "Bash(*/mkit/scripts/run-open.sh:*)",
      "Bash(*/mkit/scripts/branch-scan.sh:*)",
      "Bash(mkit findings:*)",
      "Bash(mkit gate detect:*)",
      "Bash(mkit gate run:*)"
    ]
  }
}
```

Adjust the path fragment to wherever the plugin is installed — under
`~/.claude/plugins/cache/<marketplace>/mkit/<version>/` for a marketplace install, or your
checkout for a local one. `mkit gate run` runs the repo's own lint/test/build, so allowlisting
it delegates that trust; leave it out if you would rather approve each gate.

## Running under the OS sandbox

Claude Code can run Bash tool calls inside an OS sandbox (Seatbelt on macOS). Everything below was
measured on 2026-09-09 with `sandbox.enabled: true`. **One directory is all mkit itself needs** —
though opening it takes two steps, not one; the rest of this section is what the tools mkit
*composes* need, and the two facts that no script may forget.

### The two steps mkit needs

**Both, in this order.** A grant covers a directory's *interior*, so it cannot bring the directory
into existence — and creating it is a write to `$HOME`, which nothing grants. The grant alone
produces a configuration that looks correct and still fails.

**1. Create the directory — from your own shell, not from a session.** In Claude Code, the `!`
prefix runs a command outside the sandbox:

```
! mkdir -p ~/.mkit
```

Skipping this and adding only the grant below gets you
`mkdir: /Users/mk/.mkit: Operation not permitted` at the first write — measured, and the reason
this section is two steps instead of one.

**2. Grant it:**

```json
{
  "permissions": {
    "additionalDirectories": ["~/.mkit"]
  }
}
```

`~/.mkit/` is **empty today** — its two files went with the hook — but it stays the declared home
for user-scoped state, and it is what `MKIT_HOME` redirects. Nothing writes there yet, so neither
step is required for anything the payload currently does; `facts.sh` reports `user_dir_writable=no`
without them, with this same two-part remedy, so the first user-scoped write the binary makes does
not fail as a surprise. `mkit doctor` reports the same thing on demand, from the same producer —
`mkit_user_dir_remedy()` in `lib/common.sh` is the only place this sentence is written.

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
