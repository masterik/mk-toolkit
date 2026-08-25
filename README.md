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
| `cleanup` | Sweep every local branch and worktree: delete what's merged, keep only the default branch and a local `develop`-like one, switch to one and pull it current. Local-only — never touches a remote branch. |

`_shared/` is the shared **references** bundle (git safety, Conventional Commits, quality
gate, worktree detection, branching, the commit journal) that the six skills link into — not a
triggerable skill. `scripts/` holds seven helpers the skills call for the mechanical steps:
opening a run directory, gathering the starting facts, detecting and running the quality gate,
the arithmetic over a review's findings, the commit journal, and classifying every local
branch/worktree for `cleanup`.

## Install

```
/plugin marketplace add masterik/workflow_tool
/plugin install mkit@masterik
```

Nothing to build. The scripts need `git`, `bash`, `node` and `jq`; `rg`, `gh` and `wt` are
recommended — see [Prerequisites](PREREQUISITES.md).

## Journaling intent (on by default)

`commit` normally has to reverse-engineer *why* each hunk exists out of the diff. Journaling
records that while the session still knows it. **It sets itself up**: on your first session
after installing mkit, a `SessionStart` hook writes the one user-scoped marker that turns
journaling on for every repo — present and future — and tells you once that it did. There is
no install step.

Opting out, at either scope:

```bash
"${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh" disable   # this repo only — beats the default
"${CLAUDE_PLUGIN_ROOT}/install.sh" --uninstall       # everywhere, and it sticks
```

With it on, and nothing else to configure:

- a `Stop` / `SubagentStop` hook works out which dirty paths no entry covers and asks the agent
  that did the work to record one — its paths, a type/scope/subject proposal, one line of `why`.
  At most one nudge per prompt per agent — a subagent and its parent each get one; it never
  writes the record itself.
- `commit` reads the entries, skips the exploratory whole-tree diff read for the paths they still
  describe accurately, and takes the `why` lines as message bodies. It still reads every staged
  hunk before committing — an entry is a proposal, never a decision.
- `note` ("note this") records one by hand.

Entries live in `<git-dir>/mkit/journal.jsonl` — never committed, never in `git status`, and
per-worktree, so they die with the worktree. `status` / `uncovered` / `drop` / `compact` inspect
and prune. A repo's own `disable` always beats the global default, and `install.sh --uninstall`
leaves a tombstone the hook honours — so an opt-out at either scope sticks rather than being
re-asserted next session. Details:
[`skills/_shared/references/journal.md`](skills/_shared/references/journal.md).

## Not re-proving the same tree (the gate ledger)

`review` runs the quality gate, then `finish` or `pr` runs it again over content that never
changed. Every step the gate finishes is recorded against a fingerprint of the content it read
— staging- and commit-invariant, so committing the reviewed tree does not invalidate the proof.
The next skill sees `fast_cache=fresh exit=0 age=6m` and can skip a 90-second suite.

**Wall-clock only — there are no token savings here.** Gate output already goes to a log rather
than into context. Nothing is automatic and nothing is silent: the scripts never skip a step,
the skill decides, and a step served from the ledger is reported as `cached (6m ago)`, never as
a pass. `gate.jsonl` lives beside the journal, on by default, with `--no-cache` to ignore it and
`--no-ledger` to stop writing it. Details:
[`skills/_shared/references/quality-gate.md`](skills/_shared/references/quality-gate.md).

## Testing the scripts

`tests/run.sh` runs the script layer's own suite: `node --test` over `findings.mjs`, `bats`
over the eight shell scripts. Dev-only — see [Prerequisites](PREREQUISITES.md#dev-only--running-tests).

## Docs

- [Concept](concept.md) — direction and design principles
- [Prerequisites](PREREQUISITES.md) — required tooling, setup, permission allowlist
