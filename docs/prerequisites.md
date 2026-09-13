# Prerequisites

mkit's plugin payload is **Markdown and nothing else** since M5 took the last shell script.
No hooks (removed in 0.15.0), nothing to build, and no *machine* setup step: installing the
plugin is a clone. What follows is what the skills, and the `mkit` binary they call, need.

Per **repo** there is now one optional step, `mkit init`, which writes `.mkit/config.toml`
([ADR 0001](adr/0001-per-repo-config-and-init.md)). It pins what inspection cannot establish and
is never a precondition: every command and every skill runs with no config present.

The `mkit` **binary** is a separate install (`brew install masterik/tap/mkit`), and since M5 it
is **required by every skill** — each one's first call is `mkit facts <skill>`, which opens the run
directory and returns every starting fact. Presence only, with no declared minimum on either side: a
subcommand that does not exist *is* the too-old signal. Absorbing the script layer deleted rows from
this page as it went — `node` left with `findings.mjs` (M4), `shasum` and `jq` with the gate and
facts ports (M5).

> **`mkit doctor` reports on this page's tooling**, plus the sandbox writable set, the plugin
> payload and the allowlist gaps below — on demand, reporting only. It checks *presence on
> `PATH`*, not versions, `gh` authentication or cache state, so a too-old tool still reads as
> healthy here and fails later. Two things it cannot tell
> you, both deliberate: it does not run unprompted at session start, and it cannot report that
> `mkit` itself is missing — a binary cannot report its own absence. That second gap is why
> `review` probes for `mkit findings` itself, at step 0 — and why a skill whose first call is
> `mkit facts` treats `command not found` as its own stop condition, with the `brew` remedy.

**macOS is the supported platform.** Nothing detects an OS or branches on one: the release builds
`darwin` × amd64/arm64 (Intel + Apple Silicon), and the Homebrew cask that installs it is macOS-only
in any case.

## Required

| Tool | Used by | Why |
| --- | --- | --- |
| `git` ≥ 2.30 | everything | `--absolute-git-dir`, `worktree list --porcelain`, `diff --shortstat` |
| `bash` ≥ 3.2 | `mkit gate run` | gate steps run as `bash -c '<command>'`, so `-- sh -c 'a && b'` keeps meaning what it says. macOS ships `/bin/bash` 3.2 and nothing here needs 4.x |
| `mkit` | every skill | the run directory, the starting facts, the gate, the branch classifier, the findings arithmetic |

```bash
brew install git masterik/tap/mkit
```

## Recommended

| Tool | Used by | Degrades to |
| --- | --- | --- |
| `rg` (ripgrep) | the `fix-checks` sweep | `grep -E` (same output, slower) |
| `gh` | `pr`, `mkit facts --gh`, and `mkit branch scan` (`cleanup`) | `pr` cannot open a PR at all; `mkit facts` prints `pr=gh-missing`; `mkit branch scan` falls back to git-only classification and reports `gh=gh-missing` |
| `wt` ([worktrunk](https://worktrunk.dev)) | `finish` cleanup, `mkit facts` worktree classification | plain `git worktree remove` |

```bash
brew install ripgrep gh worktrunk/tap/worktrunk
gh auth login
```

## Dev only — running the tests

Contributors need more than users do; none of this is required to *use* the plugin.

```bash
brew install go golangci-lint
go build ./... && go vet ./... && go test ./...   # what CI runs
golangci-lint run                                # CI pins v2.12
```

**Under the sandbox the Go gate needs its caches redirected**, or it fails on `~/Library/Caches/
go-build` and `~/go` rather than on your code — see [Running under the OS sandbox](#running-under-the-os-sandbox):

```bash
export GOCACHE="$TMPDIR/go-build" GOMODCACHE="$TMPDIR/go-mod" GOLANGCI_LINT_CACHE="$TMPDIR/golangci"
```

Every test that touches state points `MKIT_HOME` at a throwaway directory under `$TMPDIR` and
builds its repo there, which is both the sandbox containment story and the reason a developer's own
state cannot make an assertion pass or fail.

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
above is missing. It surfaces later — a thinner `mkit facts` block, a `pr=gh-missing`
annotation — which is exactly the debugging cost the hook existed to avoid.
`mkit doctor` is the check, and it is human-run: you have to think to run it. It does not fully
replace the hook and never will — a binary cannot report its own absence, and cannot speak at
session start. That is accepted.

Nothing needs silencing any more, so `~/.mkit/bootstrap.disabled` no longer does anything; delete
it if you have one.

## Verify

```bash
for t in git bash rg gh wt coderabbit codex mkit; do
	printf '%-12s %s\n' "$t" "$(command -v "$t" || echo '— not found')"
done
git --version; mkit version
```

Then check the toolkit itself, from any repo:

```bash
mkit facts commit --no-run   # prints a fact block, and where the payload is
mkit gate detect             # prints the full: block and each step's source
mkit findings schema         # prints the JSONL shape
mkit doctor                  # this page's tooling, the sandbox, the payload, the allowlist
```

## Fewer permission prompts

Each subcommand is its own Bash pattern, so the first run of each asks. Allow them once, in
`~/.claude/settings.json` (user-wide) or a repo's `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": [
      "Bash(mkit facts:*)",
      "Bash(mkit run:*)",
      "Bash(mkit findings:*)",
      "Bash(mkit branch scan:*)",
      "Bash(mkit gate detect:*)",
      "Bash(mkit gate run:*)"
    ]
  }
}
```

`mkit gate run` runs the repo's own lint/test/build, so allowlisting it delegates that trust;
leave it out if you would rather approve each gate.

## Running under the OS sandbox

Claude Code can run Bash tool calls inside an OS sandbox (Seatbelt on macOS). Everything below was
measured on 2026-09-09 with `sandbox.enabled: true`. **One directory is all mkit itself needs** —
though opening it takes two steps, not one; the rest of this section is what the tools mkit
*composes* need, and the two facts nothing may forget.

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
step is required for anything mkit currently does; `mkit facts` reports `user_dir_writable=no`
without them, with this same two-part remedy, so the first user-scoped write the binary makes does
not fail as a surprise. `mkit doctor` reports the same thing on demand, from the same producer —
`scratch.UserDirRemedy` is the only place this sentence is written.

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

Sandboxed egress goes through a filtering proxy, so the network side is an allowlist too. What mkit
itself reaches for:

| skill / command | host | for |
| --- | --- | --- |
| `pr`, `mkit facts --gh`, `mkit branch scan` | `api.github.com`, `github.com` | `gh pr view`, `gh pr list`, `gh pr create` |
| `pr`, `finish`, `cleanup` | your remote's host (`git remote -v`) | `fetch`, `push` |
| `review`'s external reviewers | whatever the `codex` / `coderabbit` CLI calls | those are their own tools; check their docs for the hosts |

`cleanup` and `mkit branch scan` degrade rather than fail when GitHub is unreachable: `fetch=failed`
and `gh=gh-error`, with every branch still classified from git alone. `pr` cannot open a PR without
`api.github.com` — there is no local substitute for that one.

Attempt the call and read the error rather than predicting reachability; a denied connection is
reported as such.

### Two facts nothing may forget

**`ps` and `pgrep` cannot list processes at all** under the sandbox — `operation not permitted: ps`,
not an empty result. Nothing may depend on either, and a "is it still running?" check built on one
reads as *not running*.

**`mktemp` with no template ignores `$TMPDIR`.** On macOS the bare and `-t` forms resolve the Darwin
per-user temp directory (`/var/folders/…/T/`), which the sandbox denies. It is not fixable by
environment; always pass a template. The binary uses Go's own temp-file API, which honours
`$TMPDIR`, so this now bites only a command *you* write.

Also protected *inside* the working directory: `.git/config` and `.git/hooks`. So `git config` fails,
and a nested `git init` under the project half-fails.

## Two things worth knowing

**`wt` is usually a shell function.** Worktrunk's shell integration has to be a function to
`cd` the parent shell, and it shadows the binary. `command -v wt` then answers `wt` with no
path, so `mkit facts` looks for the real executable on `PATH` instead, and reports `wt_bin=none`
as *advisory* — the agent's own shell may still have `wt` when the binary does not find one.

**`rtk` is deliberately not used inside mkit.** It reshapes command output for an
agent to read — it strips the leading space from `git diff --stat`, for one — which is
exactly what a parser must not tolerate. mkit consumes `--porcelain`,
`--shortstat`/`--name-only` and `--format=json`, and does its own compaction. rtk stays where it belongs:
on the agent's own direct commands, via the user's hook.
