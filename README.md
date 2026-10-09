# mkit

A **personal toolkit for agentic coding** — the skills and tools I use every day to work with
Claude Code. Shared as-is: it's opinionated, built around one workflow (Claude Code, macOS, git +
GitHub), and may be useful to you in parts.

Two halves:

- **A Claude Code plugin** — Markdown skills that take work from **edits → committed → reviewed →
  integrated**, plus a few that help you understand and tune the agent itself.
- **`mkit`, a Go binary** — the mechanical layer under the skills (starting facts, quality gate,
  branch classification, review-finding arithmetic) and some standalone housekeeping for the
  agent's environment.

The skills orchestrate `git`, `gh`, [Worktrunk](https://worktrunk.dev) (`wt`), CodeRabbit and
Codex rather than reimplementing them. Judgement lives in Markdown; anything that must be identical
every run lives in the binary. Why and how: [concept](docs/concept.md).

## Skills

| Skill | Does |
| --- | --- |
| `commit` | Inspect the tree, stage intentionally, split into logical Conventional Commits. |
| `review` | Review the local diff or recent commits with CodeRabbit + Codex + Claude (or quick: the first two), verify findings, fix what's worth fixing. |
| `pr` | Commit → push → open a GitHub PR → request reviewers. |
| `finish` | Commit → merge into base (the open PR, or locally) → delete the branch and worktree. |
| `cleanup` | Sweep every local branch and worktree; delete what's provably merged, ask about the rest. |
| `sandbox-audit` | Turn recent sessions' sandbox blocks and permission denials into a proposed settings diff. Report-only; run as `/mkit:sandbox-audit`. |
| `explain` | Re-say the last answer, or one item from the session, in plain English (`simpler`, `eli5`, `deeper`). |
| `recap` | Plain-English status of the branch: done, changed, left, waiting on you. Survives compaction. |

Each skill runs on its own, in any order — there's no required pipeline. Planned: `brainstorm`,
`spec`, `implement` ([backlog](docs/backlog.md)).

## The binary

```
mkit facts <skill>       every starting fact a skill needs, in one call
mkit gate detect | run   find and run the repo's checks; remember what passed over which content
mkit findings …          reconcile and group a multi-reviewer code review
mkit branch status       classify every local branch and worktree
mkit worklog show        what each step concluded on this branch
mkit repo profile        discovered vs pinned repo config;  mkit init  pins it
mkit doctor              prerequisites, sandbox writability, plugin state
mkit audit sandbox       sandbox / permission-gate events across sessions
mkit cache prune         prune stale Claude Code / Codex local storage
```

Every command takes `--json`; on a terminal, some get a TUI.

## Install

```bash
brew install masterik/tap/mkit
```

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

Both are required — the plugin is Markdown, and every skill but `explain` calls the binary. Then run
`mkit doctor`.
`gh` and `wt` are recommended; sandbox setup, the Gatekeeper workaround for the unsigned binary,
and a permission allowlist are in [Prerequisites](docs/prerequisites.md).

## Docs

- [Concept](docs/concept.md) — what this is, design principles, how it fits together
- [Prerequisites](docs/prerequisites.md) — tooling, sandbox, permissions
- [Backlog](docs/backlog.md) — what's next, invariants, what's done
- [ADRs](docs/adr/) — decisions that were hard to reverse
- [AGENTS.md](AGENTS.md) — architecture and conventions, for contributors and coding agents

## Contributing

It's a personal toolkit, so the direction follows my own workflow — but issues and PRs are
welcome ([GitHub Issues](https://github.com/masterik/mk-toolkit/issues)). Conventional Commits;
run `just ci` before opening a PR.

## License

[MIT](LICENSE). The vendored Go skills under `.agents/skills/` are MIT-licensed by
[samber/cc-skills-golang](https://github.com/samber/cc-skills-golang).
