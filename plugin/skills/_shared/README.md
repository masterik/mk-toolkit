# _shared: reference bundle

The shared library for mkit's skills; not a skill (no `SKILL.md`). Skills link into `references/` by relative
path (`../_shared/references/…`), each at the step that uses it, so each rule lives in one place. `scripts/`
holds the two bundled scripts, `facts.sh` (every skill's starting facts) and `branch-status.sh` (`cleanup`'s
classifier), run as `bash ../_shared/scripts/<name>.sh` resolved against the skill's directory; their tests
are `tests/*.bats` at the repo root.

> Keep the relative links intact: they make the bundle portable.

## References

| file | what | read by |
| --- | --- | --- |
| `workflow-contract.md` | how the steps compose; the five rules; what each step owes | `review`, `recap` |
| `git-safety.md` | the never-list, pinned `<git>`, inline shell shape, branches | every repo-scoped skill |
| `output-discipline.md` | `facts.sh` contract, run dir, where a write lands, bounded output | every repo-scoped skill, for the `facts.sh` call |
| `repo-settings.md` | reading a `docs/agents/<area>.md` setting; the areas and defaults | any skill reading a setting; `setup` |
| `summary-format.md` | the closing message's form and how a decision is asked | every skill with a report |
| `quality-gate.md` | find the gate, run it, the one reuse rule, a delegated diagnosis | `pr`, `finish`, `setup` |
| `worktree.md` | local merge and teardown per `cleanup_path`; `wt merge` style flags | `finish` (local path), `cleanup` |
| `remote.md` | which remote: `facts.sh` keys, when to ask, `gh --repo` | `pr`, `finish`, `cleanup` (several remotes, or `docs/agents/remote.md`) |
| `stacked-prs.md` | detecting a GitHub stack and the calls that change on one | `pr`, `finish` (stacked only) |
| `agent-delegation.md` | briefs, return budgets, one writer per file, model per stage | `commit` (large tree), `review` |
| `review-severity.md`, `lenses-correctness.md`, `lenses-craft.md` | the severity bar and the eight lenses | `review`'s reviewers |
| `triage-reconcile.md`, `triage-verify.md`, `fix-checks.md` | dedupe rule, verdicts, the three checks per fix | `review` steps 3–5 |
| `plain-english.md` | register, levels, visuals | `explain`, `recap` |

Skill-local files beside a `SKILL.md` hold a branch only that skill takes: `finish/pr-merge.md`,
`review/reviewers.md`, `review/closing.md`, `setup/migration.md` and `setup`'s templates.

Why the rules say what they say, and where each is enforced: `docs/skill-invariants.md` in the repo (not
shipped).
