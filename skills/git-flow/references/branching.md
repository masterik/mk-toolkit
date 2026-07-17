# Branching strategy

Shared by `commit`, `create-pr`, and `finish-feature`. Covers the default branch model to assume when a repo doesn't document its own.

## Discover first

Check for a documented convention before defaulting — a `CONTRIBUTING.md`, a docs/guidelines page, or the prevailing pattern in `git branch -a` / `git log --all --oneline --graph`.

## Common default (when nothing is documented)

```
main           # production-ready / default branch
├── feat/*     # feature branches
├── bugfix/*   # bug fix branches
└── hotfix/*   # emergency fixes
```

## Rules

- The default/base branch (`main`, `master`, or whatever the repo uses) is production-ready — never commit to it directly, and never push to it directly (see `git-safety.md`).
- A branch's prefix (`feat/`, `bugfix/`, `hotfix/`, or the repo's equivalent) is a useful signal for telling a feature branch from the base branch, and can hint at commit type — but the actual diff is what decides the commit type/scope, not the branch name.
