# Contributing

This is a personal toolkit, so its direction follows one workflow. Issues and PRs are welcome. For
a large change, open an issue first so we can agree on the direction.

This page is for changing the repo. For using mkit, see the [README](README.md) and
[Prerequisites](docs/prerequisites.md). The full architecture reference is [`AGENTS.md`](AGENTS.md).

## Setup

macOS only.

```bash
brew install go golangci-lint just bats-core
just ci          # build, vet, test, lint, bats: what CI runs (golangci-lint v2.12)
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
| `plugin/skills/_shared/scripts/` | `facts.sh`, `branch-status.sh` (bash 3.2) | plugin |
| `tests/` | bats tests for the two scripts | — |
| `docs/` | concept, backlog, prerequisites, ADRs, ideas | — |
| `tools/` | maintainer scripts (`release.sh`) | — |

## Rules

These are the ones changes break most often. The full list is the
[invariants](docs/backlog.md#invariants).

- **Judgement in Markdown, mechanics in code.** Code only where it replaces many tool calls (the two
  scripts) or serves the optional binary's own tools; never for a decision. When the line is unclear,
  the code reports the candidates and the skill chooses.
- **The plugin is the product; the binary is optional.** No skill but `sandbox-audit` may call `mkit`
  ([ADR 0006](docs/adr/0006-standalone-skills-optional-binary.md)).
- **The plugin is Markdown plus exactly two scripts** (`_shared/scripts/facts.sh`, `branch-status.sh`).
  No hooks; a third script needs an ADR.
- **Layering.** `core` returns data, `cli` formats it, `tui` renders it. No logic in a Bubble Tea `Update`.
- **`--json` on every command, and no TUI off a terminal.** Skills parse the bundled scripts'
  `key=value` output (`plugin/skills/_shared/scripts/`, tested by `tests/*.bats`); change a key there and
  in every skill that reads it, in one commit.
- **Commands report and run.** They never stage, merge, push, or edit your files.
- **Nothing writes into the work tree.** Skills keep per-run files in `$TMPDIR` (`facts.sh --run`);
  `~/.mkit/` holds the `sandbox-audit` ledger; `branch-status.sh` runs `git fetch --prune`. The
  binary's only write is `cache prune --apply` deleting stale Claude Code / Codex storage.
- **Skills are composable.** Each runs alone, in any order, and never sends the user to another
  skill first ([workflow contract](plugin/skills/_shared/references/workflow-contract.md)).
- **Nothing project-specific is hardcoded.** Gate commands, commit scopes and reviewers are discovered
  from the target repo.

## Tests

Tests sit beside their packages and use `go test`. A test that touches state builds a throwaway repo
under `$TMPDIR` and points `MKIT_HOME` inside it.

- **Never touch the real home directory.** Don't point `HOME`, `MKIT_HOME` or any other variable at
  it. A suite that reaches user-scoped state sets `MKIT_HOME` in its own helper (see `testRepo` in
  `internal/cli` and `isolate` in `internal/core/doctor`).
- `internal/cli` tests drive the real command root: argv goes in; stdout and the exit code come out.
- `tests/*.bats` test the plugin's two scripts the same way, in a throwaway repo (`bats tests/`).
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
bumps every agent's `plugin.json` that exists, commits, tags and pushes; plugin-only changes still get
a tag. The tag triggers GoReleaser, which builds darwin amd64
and arm64, writes the Release notes from PR titles, and updates the Homebrew cask. The GitHub Release
is the changelog; `CHANGELOG.md` is frozen at 0.19.0.

## License

Contributions are licensed under [MIT](LICENSE).
