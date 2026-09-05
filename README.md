# mkit

A **Go binary (`mkit`) plus a Claude Code plugin** — a cohesive kit of agent coding-workflow
skills that take work from **edits → committed → reviewed → integrated**. Composition over
replacement: the skills orchestrate `git`, GitHub CLI (`gh`), Worktrunk (`wt`), and code-review
tools (CodeRabbit/Codex); they don't reimplement them.

The skills are the product; the binary is the mechanical layer beneath them, currently
[replacing the shell scripts](docs/backlog.md) one milestone at a time. Claude-only for now;
other agents (Codex, opencode, …) are a later, thin packaging step.

## Skills

| Skill | Does |
|-------|------|
| `commit` | Inspect the tree, stage intentionally, split into logical Conventional Commits. |
| `review` | Review the local diff/commits — full (CodeRabbit + Codex + Claude, all lenses) or quick (CodeRabbit + Codex, bugs/impl only) — verify the findings, fix what's worth fixing, summarize. |
| `finish` | Commit → merge the branch back into its base → delete branch / remove worktree (local, no PR). |
| `pr` | Commit → push → open a GitHub PR → assign reviewers (remote review path). |
| `cleanup` | Sweep every local branch and worktree: delete what's merged, keep only the default branch and a local `develop`-like one, switch to one and pull it current. Local-only — never touches a remote branch. |

`_shared/` is the shared **references** bundle (git safety, Conventional Commits, quality
gate, worktree detection, branching) that the five skills link into — not a triggerable skill.
`scripts/` holds six helpers the skills call for the mechanical steps: opening a run directory,
gathering the starting facts, detecting and running the quality gate, the arithmetic over a
review's findings, and classifying every local branch/worktree for `cleanup`.

## Install

The plugin — the five skills and the scripts they call:

```
/plugin marketplace add masterik/mk-toolkit
/plugin install mkit@masterik
```

The `mkit` binary, via Homebrew:

```bash
brew install masterik/tap/mkit
```

Nothing to build either way. The binary answers `mkit version` today and is
[absorbing the script layer](docs/backlog.md) one milestone at a time; M3 will let it register
the plugin itself, making `brew install` the only step. Until then the scripts need `git`,
`bash`, `node` and `jq`; `rg`, `gh` and `wt` are recommended — see
[Prerequisites](docs/prerequisites.md).

## Not re-proving the same tree (the gate ledger)

`review` runs the quality gate, then `finish` or `pr` runs it again over content that never
changed. Every step the gate finishes is recorded against a fingerprint of the content it read
— staging- and commit-invariant, so committing the reviewed tree does not invalidate the proof.
The next skill sees `fast_cache=fresh exit=0 age=6m` and can skip a 90-second suite.

**Wall-clock only — there are no token savings here.** Gate output already goes to a log rather
than into context. Nothing is automatic and nothing is silent: the scripts never skip a step,
the skill decides, and a step served from the ledger is reported as `cached (6m ago)`, never as
a pass. `gate.jsonl` lives in `<git-dir>/mkit/`, on by default, with `--no-cache` to ignore it
and `--no-ledger` to stop writing it. Details:
[`skills/_shared/references/quality-gate.md`](plugin/skills/_shared/references/quality-gate.md).

## Testing

`go build ./... && go vet ./... && go test ./...` covers the binary — that is what CI runs.
`tests/run.sh` covers the script layer: `node --test` over `findings.mjs`, then `bats` over the
shell-script suites. Dev-only — see
[Prerequisites](docs/prerequisites.md#dev-only--running-tests).

## Docs

- [Concept](docs/concept.md) — direction and design principles
- [Backlog](docs/backlog.md) — the Go-binary migration: ordered milestones and their invariants
- [Prerequisites](docs/prerequisites.md) — required tooling, setup, permission allowlist
