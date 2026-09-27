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

**Scope**:
The area-of-change in a conventional commit's parentheses (`feat(cli): …`). Repo-specific; discovered from history.
_Avoid_: using "scope" for a commit **Type**

**Type**:
The conventional-commit kind (`feat`, `fix`, `docs`, …). A fixed vocabulary from the spec, never pinned per repo.

## Relationships

- Pre-selection in `mkit init` follows **Pinned value** → **Discovered value** → **Form default**.
- A gate step's **Discovered value** prefers the repo's own task-runner recipe (just, make, Taskfile, deno tasks) over the language's standard command.
- Reviewers are the one discovered list `init` does not pin: pinning CODEOWNERS' owners would replace `pr`'s per-path match with a flat list.

## Flagged ambiguities

- "default scope types" was used for both **Scope** and **Type** — resolved: `init` offers **Scopes**; **Types** stay fixed.
