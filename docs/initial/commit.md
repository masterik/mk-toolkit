---
name: commit
description: Shortcut for git-commit. Switches to Sonnet for speed and always displays the final commit message(s).
allowed-tools:
  - Skill
  - AskUserQuestion
model: claude-sonnet-4-6
---

# Commit Shortcut

Thin wrapper around the `git-commit` skill. This command exists to run on Sonnet for speed and to guarantee the commit message is shown afterward.

## Rules

- Never include co-author attribution or Claude/AI mentions in commits.

## Steps

1. Invoke the `git-commit` skill using the Skill tool, passing along any arguments the user provided after `/commit`.
   - `/commit` with no arguments → invoke `git-commit` with no args.
   - `/commit fix typo in readme` → invoke `git-commit` with args `"fix typo in readme"`.
2. After the skill completes, display every commit message it created as a quote block:

```
> <short-hash> <commit message first line>
```

If the skill did not show them, ask it to display the commit hash and message before finishing.
