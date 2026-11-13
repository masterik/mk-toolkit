# mkit

## Language

**Plugin**:
The product: the skills, their shared references and the two bundled scripts (`facts.sh`, `branch-status.sh`). Every skill runs on it alone.

**Binary**:
The optional `mkit` Go program: macOS + Claude Code power tools (`doctor`, `audit sandbox`, `cache prune`). Only `sandbox-audit` needs it.
_Avoid_: calling it a prerequisite, or "the mechanics under the skills" (its role before ADR 0006)

**Repo setting**:
A per-repo choice written as short prose in `docs/agents/<area>.md` (commit, review, merge, remote, gate, cleanup), by the `setup` skill or by hand. Lazy: the file exists only where the repo chose something other than the **Discovered value** or **Default**. A skill reads it when present and never stops when it is absent.
_Avoid_: pinned value, config (both name the retired `.mkit/config.toml`)

**Discovered value**:
A value a skill reads off the repo (history, `CODEOWNERS`, manifests, task runners) when no **Repo setting** answers. Re-read on every run; nothing freezes it.

**Default**:
What a skill uses when there is neither a **Repo setting** nor a **Discovered value**: `full` review, `merge` style, ~72-char subjects. Not a repo statement: it gives way without comment (a remote refusing `merge`, the user's worktrunk config).
_Avoid_: form default (the retired `mkit init` wizard's pre-selection)

**Remote**:
The one git remote a branch is pushed to and whose GitHub repo its PR is about — a single choice, not a push/PR pair. A **Repo setting** when `docs/agents/remote.md` names it and this clone has it; otherwise `origin` if it exists, else the only remote. When several remotes remain, the skills ask once per run; `setup` can record the answer.
_Avoid_: upstream (a branch's tracking ref, or the project in a fork workflow), push target

**Scope**:
The area-of-change in a conventional commit's parentheses (`feat(cli): …`). Repo-specific; a **Repo setting** in `commit.md`, else discovered from history.
_Avoid_: using "scope" for a commit **Type**

**Type**:
The conventional-commit kind (`feat`, `fix`, `docs`, …). A fixed vocabulary from the spec, never set per repo.

## Relationships

- A setting resolves **Repo setting** → **Discovered value** → **Default**; the rules, once per area, are `plugin/skills/_shared/references/repo-settings.md`.
- A gate's **Discovered value** prefers the repo's own task-runner recipe (just, make, Taskfile, deno tasks) over the language's standard command.
- The same **Remote** also decides which HEAD names the default branch.
- Reviewers: a list in `merge.md` replaces `pr`'s per-path `CODEOWNERS` match; with none, the match runs.
- The spec store is mattpocock's `docs/agents/issue-tracker.md`, not an mkit **Repo setting**.

## Flagged ambiguities

- "default scope types" was used for both **Scope** and **Type** — resolved: a repo sets **Scopes**; **Types** stay fixed.
