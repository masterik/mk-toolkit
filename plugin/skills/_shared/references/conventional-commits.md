# Conventional Commits reference

Commit-message format for `commit`, `finish`, `pr`.

## Format

```text
<type>(<scope>): <summary>

<What changed.>
<Why it changed.>

<footer — BREAKING CHANGE / Closes #123, if needed>
```

- Summary: imperative, specific, ~≤ 72 chars ("Add", "Fix", "Remove", "Refactor").
- Body: what changed and why — not an implementation diary.
- Breaking change: `!` after the type/scope (`feat(api)!: …`) and/or a `BREAKING CHANGE:` footer.

## Types

| Type       | Purpose                        |
| ---------- | ------------------------------ |
| `feat`     | New feature                    |
| `fix`      | Bug fix                        |
| `docs`     | Documentation only             |
| `style`    | Formatting/style (no logic)    |
| `refactor` | Refactor (no feature/fix)      |
| `perf`     | Performance improvement        |
| `test`     | Add/update tests               |
| `build`    | Build system/dependencies      |
| `ci`       | CI/config changes              |
| `chore`    | Maintenance/misc               |
| `revert`   | Revert a commit                |

## Scope

Names the area changed. **Do not hardcode a scope list** — discover the project's:

1. Repo-documented scopes (a table in `CONTRIBUTING.md`, `.commitlintrc`, similar) win.
2. Otherwise infer from changed paths (`git diff --cached --name-only`) — a top-level package or directory
   name is usually right.
3. Spanning several areas: omit the scope, or use `*`.

## Multi-line messages without shell-escaping pain

Write to a temp file, commit with `-F`:

```bash
git commit -F "$TMP/commit-msg.txt"
```

Use the session scratch dir when there is one; clean up afterward.
