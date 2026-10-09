# mkit

A **Go binary (`mkit`) plus a Claude Code plugin** — a cohesive kit of agent coding-workflow
skills that take work from **edits → committed → reviewed → integrated**. Composition over
replacement: the skills orchestrate `git`, GitHub CLI (`gh`), Worktrunk (`wt`), and code-review
tools (CodeRabbit/Codex); they don't reimplement them.

The skills are the product; the binary is the mechanical layer beneath them — `plugin/` is
Markdown only. `mkit` owns the shared mechanical invariants (starting facts, the quality gate,
branch classification, review-finding arithmetic); the skills run `git`, `gh` and `wt` themselves.
**macOS only** and **Claude Code only** for now; other platforms and agents are tracked as
later work ([backlog](docs/backlog.md)).

## Skills

| Skill | Does |
|-------|------|
| `commit` | Inspect the tree, stage intentionally, split into logical Conventional Commits. |
| `review` | Review the local diff/commits — full (CodeRabbit + Codex + Claude, all lenses) or quick (CodeRabbit + Codex, bugs/impl only) — verify the findings, fix what's worth fixing, summarize. |
| `finish` | Commit → merge the branch back into its base → delete branch / remove worktree (local, no PR). |
| `pr` | Commit → push → open a GitHub PR → assign reviewers (remote review path). |
| `cleanup` | Sweep every local branch and worktree: delete what's merged, keep only the default branch and a local `develop`-like one, switch to one and pull it current. Local-only — never touches a remote branch. |
| `sandbox-audit` | Scan every project's recent sessions for sandbox blocks, sandbox-disabled calls and auto-mode/rule/user denials, and propose a paste-ready settings diff plus the guards that should stay. Report-only — never edits a settings file. **Slash-only**: run it as `/mkit:sandbox-audit`; it is never triggered by a phrase. |
| `explain` | Say the last answer again, or explain one item from the session (a term, finding, error, or a decision and what it rejected), in plain English with a diagram where a picture helps. `simpler`/`eli5` drop the jargon, `deeper` goes under the hood. Needs no binary. |
| `recap` | Plain-English status of the branch: what was done, what changed, what's left, what's waiting on you. Built from the session, the worklog and git, so it survives a compacted context. Read-only. |

`_shared/` is the shared **references** bundle (git safety, Conventional Commits, quality
gate, worktree detection, branching) that the skills link into — not a triggerable skill.
The mechanical steps are the binary's: `mkit facts` opens a run directory and returns every
starting fact, `mkit gate detect|run` detects and runs the quality gate, `mkit branch status`
classifies every local branch and worktree for `cleanup`, `mkit audit sandbox` reads past
transcripts and the settings that govern them for `sandbox-audit`, and `mkit findings` does the arithmetic
over a review's findings.

## Install

The plugin — the eight skills and their shared references:

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

The `mkit` binary, via Homebrew:

```bash
brew install masterik/tap/mkit
```

Nothing to build either way, and **both steps are required** — the marketplace ships the skills,
Homebrew ships the binary, and the two version independently
([ADR 0003](docs/adr/0003-two-distribution-channels.md)). Every repo-scoped skill's first call is
`mkit facts <skill>` (`review` runs a one-line `mkit findings` compatibility probe just before it;
`sandbox-audit`, which is user-wide, starts with `mkit audit sandbox`; `explain` makes no `mkit` call at all), so a missing binary stops a skill at step 0 with a `brew` remedy rather than
degrading. Presence only, with no declared minimum on either side. `commit`, `review`, `pr` and
`finish` also write a per-branch **worklog** through `mkit worklog` (`recap` reads it) — what ran on this branch and what it
concluded — which is an optional *input*: a worklog a step cannot read costs it one input and never
stops it. Beyond `git` and `bash`, `rg`, `gh` and `wt` are recommended — see
[Prerequisites](docs/prerequisites.md).

## Not re-proving the same tree (the gate ledger)

`pr` gates before opening a PR; `finish` gates again before a local merge. Every step the gate
finishes is recorded against a fingerprint of the content it read — staging- and commit-invariant,
so committing the gated tree does not invalidate the proof. A later run over the same content sees
`full_cache=fresh exit=0 age=6m` and can skip a re-run of that step.

**Wall-clock only — there are no token savings here.** Gate output already goes to a log rather
than into context. Nothing is automatic and nothing is silent: `mkit gate run` never skips a step,
the skill decides, and a step served from the ledger is reported as `cached (6m ago)`, never as
a pass. `gate.jsonl` lives in `<toplevel>/.mkit/`, on by default, with `--no-cache` to ignore it
and `--no-ledger` to stop writing it. Details:
[`skills/_shared/references/quality-gate.md`](plugin/skills/_shared/references/quality-gate.md).

## Testing

`just ci` (or `go build ./... && go vet ./... && go test ./... && golangci-lint run`) — what CI
runs. Tests live beside their packages and build throwaway repos under `$TMPDIR`; none touch your
home directory. Dev-only — see [Prerequisites](docs/prerequisites.md#dev-only--running-tests).

## Docs

- [Concept](docs/concept.md) — direction and design principles
- [Backlog](docs/backlog.md) — ordered milestones and their invariants
- [Prerequisites](docs/prerequisites.md) — required tooling, setup, permission allowlist
- [ADRs](docs/adr/) — decisions that were hard to reverse
- [AGENTS.md](AGENTS.md) — architecture and conventions, for contributors and coding agents

## Contributing

Issues and PRs welcome — [GitHub Issues](https://github.com/masterik/mk-toolkit/issues). Commits
follow [Conventional Commits](https://www.conventionalcommits.org/); run `just ci` before opening a PR.

## License

[MIT](LICENSE). The vendored Go skills under `.agents/skills/` are MIT-licensed by
[samber/cc-skills-golang](https://github.com/samber/cc-skills-golang).
