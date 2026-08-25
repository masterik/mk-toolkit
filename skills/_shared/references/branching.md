# Branching strategy

Shared by `commit`, `pr`, `finish`, `cleanup`. The branch model to assume when a repo documents none.

## Discover first

Check for a documented convention before defaulting: `CONTRIBUTING.md`, a docs/guidelines page, or the
prevailing pattern in `git branch -a` / `git log --all --oneline --graph`.

## Common default

```
main           # production-ready / default branch
├── feat/*     # feature branches
├── bugfix/*   # bug fix branches
└── hotfix/*   # emergency fixes
```

## Rules

- The default/base branch (`main`, `master`, whatever the repo uses) is production-ready: never commit to it
  directly, never push to it directly (`git-safety.md`).
- A branch prefix (`feat/`, `bugfix/`, `hotfix/`, or the repo's equivalent) tells a feature branch from the
  base and can hint at commit type — but **the diff decides the type/scope**, not the branch name.
