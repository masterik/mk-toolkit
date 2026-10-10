# Contributing

This is a personal toolkit, so its direction follows one workflow. Issues and PRs are welcome. For
a large change, open an issue first so we can agree on the direction.

This page is for changing the repo. For using mkit, see the [README](README.md) and
[Prerequisites](docs/prerequisites.md). The full architecture reference is [`AGENTS.md`](AGENTS.md).

## Setup

macOS only.

```bash
brew install go golangci-lint just
just ci          # build, vet, test, lint: what CI runs (golangci-lint v2.12)
just run doctor  # run the binary from source, no install
```

Run `just --list` to list every recipe. Bare `just` runs `ci`.

When running under Claude Code's sandbox, first move the Go caches into `$TMPDIR`. Otherwise the gate
fails on cache writes, not on your code:

```bash
export GOCACHE="$TMPDIR/go-build" GOMODCACHE="$TMPDIR/go-mod" GOLANGCI_LINT_CACHE="$TMPDIR/golangci"
```

To try plugin changes, load the work tree's `plugin/` instead of the installed copy:
`claude --plugin-dir plugin`. When run inside this repo, the binary also prefers the work tree's
`plugin/`.

## Layout

| Path | What | Ships in |
| --- | --- | --- |
| `cmd/mkit/` | entrypoint only | binary |
| `internal/cli/` | cobra commands; formatting | binary |
| `internal/core/` | logic that returns data and never prints | binary |
| `internal/tui/` | Bubble Tea rendering, terminal only | binary |
| `plugin/skills/<name>/SKILL.md` | skills | plugin |
| `plugin/skills/_shared/references/` | shared references | plugin |
| `docs/` | concept, backlog, prerequisites, ADRs, ideas | — |
| `tools/` | maintainer scripts (`release.sh`) | — |

## Rules

These are the ones changes break most often. The full list is the
[invariants](docs/backlog.md#invariants).

- **Judgement in Markdown, mechanics in Go.** Add a command only for something that must run the same
  way every time, never for a decision. When the line is unclear, the binary reports the
  candidates and the skill chooses.
- **The plugin is Markdown only.** No scripts, no hooks.
- **Layering.** `core` returns data, `cli` formats it, `tui` renders it. No logic in a Bubble Tea `Update`.
- **`--json` on every command, and no TUI off a terminal.** Skills parse the `key=value` text output,
  so treat it as an API.
- **Commands report and run.** They never stage, merge, push, or edit your files.
- **State goes to three places only:** `$TMPDIR`, `<toplevel>/.mkit/` and `~/.mkit/`. The bounded
  exceptions: the `.mkit/` ignore pair in `info/exclude`, `git fetch --prune` (`branch status`),
  `.mkit/config.toml` (`init`), and `cache prune --apply` deleting stale Claude Code / Codex storage.
  `TestWriteSitesAreOnTheReviewedAllowlist` checks every Go file that writes against a reviewed list;
  a new write site needs a reviewed edit to it.
- **Skills are composable.** Each runs alone, in any order, and never sends the user to another
  skill first ([workflow contract](plugin/skills/_shared/references/workflow-contract.md)).
- **Nothing project-specific is hardcoded.** Gate commands, commit scopes and reviewers are discovered
  from the target repo.

## Tests

Tests sit beside their packages and use `go test`. A test that touches state builds a throwaway repo
under `$TMPDIR` and points `MKIT_HOME` inside it.

- **Never touch the real home directory.** Don't point `HOME`, `MKIT_HOME` or any other variable at
  it. A suite that reaches user-scoped state sets `MKIT_HOME` in its own helper (see `factsRepo` in
  `internal/cli` and `isolate` in `internal/core/doctor`).
- `internal/cli` tests drive the real command root: argv goes in; stdout and the exit code come out.
  That is the interface skills call.
- Don't simplify an assertion whose comment names a measured failure.

## Commits and PRs

- Use [Conventional Commits](https://www.conventionalcommits.org/), with the scopes already in
  `git log`. The `commit` skill does this for you.
- Run `just ci` before pushing. CI runs the same steps.
- The PR title becomes the release-note line.
- Merge with a **merge commit**. Never squash or rebase.
- Record a hard-to-reverse decision as an ADR in [`docs/adr/`](docs/adr/). Put a researched but
  unscheduled idea in [`docs/ideas/`](docs/ideas/README.md).

## Releases (maintainer)

`just release` picks the version, shows the PRs since the last tag, and asks for confirmation. Then it
bumps `plugin.json`, commits, tags and pushes. The tag triggers GoReleaser, which builds darwin amd64
and arm64, writes the Release notes from PR titles, and updates the Homebrew cask. The GitHub Release
is the changelog; `CHANGELOG.md` is frozen at 0.19.0.

## License

Contributions are licensed under [MIT](LICENSE).
