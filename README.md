# mkit

A **personal toolkit for agentic coding**: the skills and tools I use daily with Claude Code.
Opinionated, built for one setup (Claude Code, macOS, git + GitHub), shared as-is.

- **The plugin** (the product): Markdown skills that take work from edits to committed, reviewed and
  merged, plus two small bash scripts they share. Needs nothing but `git`, `bash` and (for PRs) `gh`.
- **The `mkit` binary** (optional): macOS + Claude Code power tools: a prerequisite report, a
  sandbox audit and storage cleanup. No skill needs it but `sandbox-audit`.

Skills orchestrate `git`, `gh`, CodeRabbit, Codex and, if installed, [Worktrunk](https://worktrunk.dev)
(`wt`) rather than reimplementing them. Judgement lives in Markdown. Background: [concept](docs/concept.md).

## Skills

| Skill | Does |
| --- | --- |
| `commit` | Stage intentionally; split into logical Conventional Commits. |
| `review` | Review the diff or recent commits with CodeRabbit, Codex and Claude (`quick` uses the first two); verify findings; apply worthwhile fixes. |
| `pr` | Commit, push, open a GitHub PR, request reviewers. |
| `finish` | Commit, merge into base (the open PR, or locally), delete the branch and worktree. |
| `cleanup` | Delete branches and worktrees that are provably merged; ask about the rest. |
| `setup` | Optional, once per repo: record non-default settings as prose in `docs/agents/`; converts an old `.mkit/config.toml`. Run as `/mkit:setup`. |
| `sandbox-audit` | Turn sandbox blocks and permission denials into a proposed settings diff. Report-only. Needs the binary. |
| `explain` | Re-explain the last answer or one session item in plain English (`simpler`, `eli5`, `deeper`). |
| `recap` | Branch status in plain English: done, changed, left, and waiting on you. |
| `shorten` | Rewrite text shorter under report/doc/plan concision rules; returns only the revised text. |

Each skill runs alone, in any order.

**Usage.** Ask in plain words ("commit this", "review my changes", "open a PR", "where are we?"),
or call a skill directly as `/mkit:<skill>`, for example `/mkit:review quick`. A typical branch:

```
edit → /mkit:commit → /mkit:review → /mkit:pr   (or /mkit:finish to merge locally)
```

Skills discover gate commands, commit scopes, reviewers and the push remote from the repo (git history,
`CODEOWNERS`, task runners, manifests); merge style defaults to a merge commit. To change a default,
`/mkit:setup` writes short prose files in `docs/agents/` (`commit.md`, `review.md`, `merge.md`, `remote.md`,
`gate.md`, `cleanup.md`); write or edit them by hand just as well. Each is optional. Skills keep per-run files in a temp dir; only `setup` writes into your work tree, and only `docs/agents/`.

## Install

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

That is all the skills need. For optional tools, sandbox setup and a permission allowlist, see
[Prerequisites](docs/prerequisites.md).

## The binary (optional)

macOS only, for Claude Code users:

```bash
brew install masterik/tap/mkit
```

```
mkit doctor              prerequisites, plugin state, sandbox grants, leftovers from old releases
mkit audit sandbox       sandbox and permission-gate events across sessions (sandbox-audit runs it)
mkit cache prune         prune stale Claude Code / Codex storage
```

Every command takes `--json`. On a terminal, the reports are styled and `cache prune --apply` shows a
tick-list. Plugin and binary are released together under one version tag.

## Docs

For users:
- [Prerequisites](docs/prerequisites.md): install, optional tools, sandbox, permissions, Gatekeeper fix, gotchas
- [Concept](docs/concept.md): what mkit is, its design principles, how the parts fit

For contributors:
- [CONTRIBUTING.md](CONTRIBUTING.md): setup, layout, rules, tests, PRs, releases
- [AGENTS.md](AGENTS.md): full architecture and conventions
- [Backlog](docs/backlog.md): what's next, invariants, what's done
- [ADRs](docs/adr/): decisions that are hard to reverse
- [Ideas](docs/ideas/README.md): researched but not scheduled

## Contributing

Direction follows my own workflow, but [issues](https://github.com/masterik/mk-toolkit/issues) and PRs
are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE). The vendored Go skills under `.agents/skills/` are MIT-licensed by
[samber/cc-skills-golang](https://github.com/samber/cc-skills-golang).
