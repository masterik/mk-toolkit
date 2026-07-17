# Conventional Commits reference

Shared commit-message format for the `commit`, `finish-feature`, and `create-pr` skills.

## Format

```text
<type>(<scope>): <summary>

<What changed.>
<Why it changed.>

<footer — BREAKING CHANGE / Closes #123, if needed>
```

- Summary: imperative, specific, ~≤ 72 chars ("Add", "Fix", "Remove", "Refactor").
- Body: what changed and why — not an implementation diary.
- Breaking change: add `!` after the type/scope (`feat(api)!: …`) and/or a `BREAKING CHANGE:` footer.

## Types

| Type       | Purpose                        |
| ---------- | ------------------------------ |
| `feat`     | New feature                    |
| `fix`      | Bug fix                        |
| `docs`     | Documentation only             |
| `style`    | Formatting/style (no logic)    |
| `refactor` | Code refactor (no feature/fix) |
| `perf`     | Performance improvement        |
| `test`     | Add/update tests               |
| `build`    | Build system/dependencies      |
| `ci`       | CI/config changes              |
| `chore`    | Maintenance/misc               |
| `revert`   | Revert a commit                |

## Scope

The scope names the area changed. **Do not hardcode a scope list** — discover the project's convention:

1. If the repo documents scopes (e.g. a scope table in `CONTRIBUTING.md`, a `.commitlintrc`, or similar), use those.
2. Otherwise infer from the changed paths (`git diff --cached --name-only`): a top-level package or directory name is usually the right scope.
3. For changes spanning multiple areas, omit the scope or use `*`.

## Multi-line messages without shell-escaping pain

Write the message to a temp file and commit with `-F`:

```bash
git commit -F "$TMP/commit-msg.txt"
```

Use the session scratch dir for the temp file when one is available; otherwise a normal temp path is fine. Clean it up afterward.
