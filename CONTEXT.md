# mkit

## Language

**Pinned value**:
A value written into the repo config (`.mkit/config.toml`) by a deliberate choice; it replaces discovery for that key.
_Avoid_: setting, configured default

**Discovered value**:
A value mkit reads off the repo (history, git config, CODEOWNERS, manifests, task runners) when nothing is pinned. `mkit init` pins what it discovers — gate commands and history scopes included — so re-discovering is a user act (`mkit init --force`), never a side effect of a run.

**Form default**:
The option `mkit init` pre-selects when neither a pinned nor a discovered value exists. A suggestion only — it becomes a **Pinned value** when the user accepts it, and "don't pin" is always one of the options.
_Avoid_: default config

**Optional page**:
A page of the `mkit init` wizard that is not walked — the gate and the cleanup keep list. Its pre-selection is written as it stands; the review page opens it for a change.

**Remote**:
The one git remote a branch is pushed to and whose GitHub repo its PR is about — a single choice, not a push/PR pair. A **Pinned value** when the repo config names it; otherwise `origin` if it exists. With several remotes and none pinned, the skills ask once and offer to pin the answer.
_Avoid_: upstream (a branch's tracking ref, or the project in a fork workflow), push target

**Scope**:
The area-of-change in a conventional commit's parentheses (`feat(cli): …`). Repo-specific; discovered from history.
_Avoid_: using "scope" for a commit **Type**

**Type**:
The conventional-commit kind (`feat`, `fix`, `docs`, …). A fixed vocabulary from the spec, never pinned per repo.

## Relationships

- Pre-selection in `mkit init` follows **Pinned value** → **Discovered value** → **Form default**.
- A gate step's **Discovered value** prefers the repo's own task-runner recipe (just, make, Taskfile, deno tasks) over the language's standard command.
- A **Remote** is a **Form default** (`origin`) only where several exist: discovery then names no value, so accepting `mkit init` pins it. The same **Remote** also decides which HEAD names the default branch.
- Reviewers are the one discovered list `init` does not pin: pinning CODEOWNERS' owners would replace `pr`'s per-path match with a flat list.

## Flagged ambiguities

- "default scope types" was used for both **Scope** and **Type** — resolved: `init` offers **Scopes**; **Types** stay fixed.
