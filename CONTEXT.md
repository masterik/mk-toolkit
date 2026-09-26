# mkit

## Language

**Pinned value**:
A value written into the repo config (`.mkit/config.toml`) by a deliberate choice; it replaces discovery for that key.
_Avoid_: setting, configured default

**Discovered value**:
A value mkit reads off the repo on every run (history, git config, CODEOWNERS, manifests) when nothing is pinned.

**Form default**:
The option `mkit init` pre-selects when neither a pinned nor a discovered value exists. A suggestion only — it becomes a **Pinned value** when the user accepts it, and "don't pin" is always one of the options.
_Avoid_: default config

**Scope**:
The area-of-change in a conventional commit's parentheses (`feat(cli): …`). Repo-specific; discovered from history.
_Avoid_: using "scope" for a commit **Type**

**Type**:
The conventional-commit kind (`feat`, `fix`, `docs`, …). A fixed vocabulary from the spec, never pinned per repo.

## Relationships

- Pre-selection in `mkit init` follows **Pinned value** → **Discovered value** → **Form default**.

## Flagged ambiguities

- "default scope types" was used for both **Scope** and **Type** — resolved: `init` offers **Scopes**; **Types** stay fixed.
