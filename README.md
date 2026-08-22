# mkit

A **Claude Code plugin** — a cohesive kit of agent coding-workflow skills that take work from
**edits → committed → reviewed → integrated**. Composition over replacement: the skills
orchestrate `git`, GitHub CLI (`gh`), Worktrunk (`wt`), and code-review tools
(CodeRabbit/Codex); they don't reimplement them.

Claude-only for now; other agents (Codex, opencode, …) are a later, thin packaging step.

## Skills

| Skill | Does |
|-------|------|
| `commit` | Inspect the tree, stage intentionally, split into logical Conventional Commits. |
| `review` | Review the local diff/commits — full (CodeRabbit + Codex + Claude, all lenses) or quick (CodeRabbit + Codex, bugs/impl only) — verify the findings, fix what's worth fixing, summarize. |
| `finish` | Commit → merge the branch back into its base → delete branch / remove worktree (local, no PR). |
| `pr` | Commit → push → open a GitHub PR → assign reviewers (remote review path). |
| `note` | Record why the unit of work just finished exists, into the repo's commit journal, so `commit` doesn't infer intent from the diff. |

`_shared/` is the shared **references** bundle (git safety, Conventional Commits, quality
gate, worktree detection, branching, the commit journal) that the five skills link into — not a
triggerable skill. `scripts/` holds six helpers the skills call for the mechanical steps:
opening a run directory, gathering the starting facts, detecting and running the quality gate,
the arithmetic over a review's findings, and the commit journal.

## Install

```
/plugin marketplace add masterik/workflow_tool
/plugin install mkit@masterik
```

Nothing to build. The scripts need `git`, `bash`, `node` and `jq`; `rg`, `gh` and `wt` are
recommended — see [Prerequisites](PREREQUISITES.md).

## Journaling intent (opt-in)

`commit` normally has to reverse-engineer *why* each hunk exists out of the diff. Journaling
records that while the session still knows it. **Off in every repo by default** — installing
mkit registers the hook but nothing is written until you ask:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh" enable   # per repo, one command
```

From then on, with nothing else to configure:

- a `Stop` / `SubagentStop` hook works out which dirty paths no entry covers and asks the agent
  that did the work to record one — its paths, a type/scope/subject proposal, one line of `why`.
  At most one nudge per prompt per agent — a subagent and its parent each get one; it never
  writes the record itself.
- `commit` reads the entries, skips the exploratory whole-tree diff read for the paths they still
  describe accurately, and takes the `why` lines as message bodies. It still reads every staged
  hunk before committing — an entry is a proposal, never a decision.
- `note` ("note this") records one by hand.

Entries live in `<git-dir>/mkit/journal.jsonl` — never committed, never in `git status`, and
per-worktree, so they die with the worktree. `journal.sh disable` turns it back off;
`status` / `uncovered` / `drop` / `compact` inspect and prune. Details:
[`skills/_shared/references/journal.md`](skills/_shared/references/journal.md).

## Testing the scripts

`tests/run.sh` runs the script layer's own suite: `node --test` over `findings.mjs`, `bats`
over the seven shell scripts. Dev-only — see [Prerequisites](PREREQUISITES.md#dev-only--running-tests).

## Docs

- [Concept](concept.md) — direction and design principles
- [Prerequisites](PREREQUISITES.md) — required tooling, setup, permission allowlist
