# mkit

A **personal toolkit for agentic coding**: the skills and tools I use daily with Claude Code.
Opinionated, built for one setup (Claude Code, macOS, git + GitHub), shared as-is.

- **Claude Code plugin**: Markdown skills that take work from edits to committed, reviewed and merged.
- **`mkit` Go binary**: the mechanical layer under the skills, plus housekeeping for the agent's
  environment.

Skills orchestrate `git`, `gh`, [Worktrunk](https://worktrunk.dev) (`wt`), CodeRabbit and Codex
rather than reimplementing them. Judgement lives in Markdown. Anything that must run identically
every time lives in the binary. Background: [concept](docs/concept.md).

## Skills

| Skill | Does |
| --- | --- |
| `commit` | Stage intentionally; split into logical Conventional Commits. |
| `review` | Review the diff or recent commits with CodeRabbit, Codex and Claude (`quick` uses the first two); verify findings; apply worthwhile fixes. |
| `pr` | Commit, push, open a GitHub PR, request reviewers. |
| `finish` | Commit, merge into base (the open PR, or locally), delete the branch and worktree. |
| `cleanup` | Delete branches and worktrees that are provably merged; ask about the rest. |
| `sandbox-audit` | Turn sandbox blocks and permission denials into a proposed settings diff. Report-only. |
| `explain` | Re-explain the last answer or one session item in plain English (`simpler`, `eli5`, `deeper`). |
| `recap` | Branch status in plain English: done, changed, left, and waiting on you. |

Each skill runs alone, in any order. Planned: `brainstorm`, `spec`, `implement`
([backlog](docs/backlog.md)).

**Usage.** Ask in plain words ("commit this", "review my changes", "open a PR", "where are we?"),
or call a skill directly as `/mkit:<skill>`, for example `/mkit:review quick`. A typical branch:

```
edit → /mkit:commit → /mkit:review → /mkit:pr   (or /mkit:finish to merge locally)
```

Skills discover gate commands, commit scopes, reviewers and merge style from the repo. `mkit init`
can pin them in a committed `.mkit/config.toml`, but that is optional. mkit's scratch state lives in
`.mkit/` and is git-ignored; `.mkit/config.toml` is committed.

## The binary

```
mkit facts <skill>       all starting facts a skill needs, in one call
mkit gate detect | run   find and run the repo's checks; record what passed
mkit findings …          reconcile and group a multi-reviewer review
mkit branch status       classify every local branch and worktree
mkit worklog show        what each step concluded on this branch
mkit repo profile        discovered vs pinned config (mkit init pins it)
mkit doctor              prerequisites, sandbox writability, plugin state
mkit audit sandbox       sandbox and permission-gate events across sessions
mkit cache prune         prune stale Claude Code / Codex storage
```

Every command takes `--json`. On a terminal, some show a TUI.

## Install

```bash
brew install masterik/tap/mkit
```

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

You need both, because every skill except `explain` calls the binary. Then run `mkit doctor`. For
optional tools, sandbox setup and a permission allowlist, see [Prerequisites](docs/prerequisites.md).

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
