---
name: commit
description: >-
  Stage and split local changes into logical Conventional Commits. Trigger on "commit", "split into
  commits", or a commit-message question. Commits only — merge is finish, PR is pr.
model: sonnet
---

# Commit work

Commits that are easy to review and safe to ship: only intended changes, logically scoped, messages saying
what changed and why. This skill only commits — merge is `finish`, a PR is `pr`; it runs no quality gate.

Ask only if unclear: one commit or several (unsure → several small ones when changes are unrelated).
Conventional Commits are required. Repo rules (scopes, subject length) are read, not asked.

## Workflow

**0. One call for every read-only fact**, which also opens the run directory. The path is relative to this
skill's directory: resolve it and run it absolute. A key or exit not named here:
`../_shared/references/output-discipline.md`.

```bash
bash ../_shared/scripts/facts.sh commit --run
```

Keep the `run_dir=` literal as `<run-dir>` (no shell variable survives to the next call; re-running `--run`
opens a second directory), `toplevel=` as `<toplevel>`, and `command -v git` as `<git>` — the absolute git
binary for every call whose output you parse or judge (`../_shared/references/git-safety.md`).

Then this repo's commit rules (rules for reading a setting: `../_shared/references/repo-settings.md`):

- **`<toplevel>/docs/agents/commit.md` exists** → its scope list is the repo's: pick from it, or say why none
  fits; never re-derive scopes from history. Its subject limit is the limit.
- **No file** → discover scopes: a repo document naming them (`CONTRIBUTING.md`, `.commitlintrc`), else
  the `type(scope):` parentheses in `"<git>" -C "<toplevel>" log --format=%s -200`, else the changed paths'
  top-level directory. Discovered scopes are a starting set, not a rule. Subject ~72 chars — never derived
  from history.

1. **Inspect before staging** — step 0 returned it all.
   - Confirm `branch=` is the intended one (matters with `linked=yes`).
   - Read **both** `unstaged_stat` and `staged_stat`, and the `untracked` list.
   - Full diff per file, only for files you must judge — never the whole tree at once.
2. **Decide commit boundaries.** Split feature vs refactor, backend vs frontend, formatting vs logic, tests
   vs prod code, dependency bumps vs behaviour changes. Changes mixed within one file → patch staging.
   - **Large mixed tree (>~10 files or >~400 changed lines): delegate the read**
     (`../_shared/references/agent-delegation.md`). The brief carries the branch, the stats, the file lists
     and the split heuristics above. The subagent writes `<run-dir>/commit-plan.md` and returns only the plan
     — per commit: type(scope), subject, paths, one line of rationale, plus any file needing patch staging —
     under ~20 lines. Coverage contract: **every path in exactly one commit**, a hunk assignment for every
     mixed file.
   - Read it here instead when smaller, or when **more than ~3 files are mixed** whatever the size.
   - **Verify the plan by set comparison** against step 0's `*_file_list` blocks: same set, no path twice,
     nothing invented. On a gap, name the missing paths back to the same subagent. Staging, wording and
     splits stay here.
3. **Stage only what belongs in the next commit.** Whole file: `git add -- "<path>"`. Mixed file: read
   [patch-staging.md](patch-staging.md) and follow it. Never `git add -p` / `git restore --staged -p`
   (interactive).
4. **Review what will be committed.** `"<git>" -C "<toplevel>" diff --cached --stat`, then
   `"<git>" -C "<toplevel>" diff --cached -- "<path>"` **file by file, skipping none, never truncated**.
   Check for secrets or tokens, debug logging, unrelated formatting churn.
5. **Describe the staged change in 1–2 sentences** (what, why). Can't do it cleanly → too big or mixed:
   back to step 2.
6. **Write the message** (format below). Single line: `git commit -m "<subject>"`. Multi-line: run
   `mktemp "<run-dir>/msg.XXXXXX"`, write the message to the path it prints, `git commit -F "<msgfile>"`.
   A fresh file per commit; never `-v`, never `-F -` with a heredoc.
7. **Repeat** until the tree is clean.

## Message format

```text
<type>(<scope>): <summary>

<What changed.>
<Why it changed.>

<footer — BREAKING CHANGE: … / Closes #123, if needed>
```

Summary imperative and specific. Types: `feat` · `fix` · `docs` · `style` (formatting, no logic) ·
`refactor` · `perf` · `test` · `build` (build system, dependencies) · `ci` · `chore` · `revert`. Breaking:
`!` after type/scope and/or a `BREAKING CHANGE:` footer. A change spanning several scopes omits the scope.
The diff decides type and scope, not the branch name.

## Final report (always)

Confirm hashes with `"<git>" -C "<toplevel>" log --oneline -n <N>` (N = commits made), then, under a bold
verdict (`**Verdict:** 2 commits, tree clean.`; form per `../_shared/references/summary-format.md`):

```
<short-sha>  <type>(<scope>): <summary>
  <what/why, 1 sentence>
```

One block per commit, in order. Name any change deliberately left uncommitted (path, why). Say under its
block when a split was forced (a file committed whole). Then one line on where the rules came from, when there
is anything to say:

```
Scopes: cli, core (from docs/agents/commit.md) · subject max 72 (from docs/agents/commit.md)
Scopes: derived from the last 200 commits · subject max ~72 (default)
```

A `commit.md` that did not say what it seemed to mean is reported here in one line.

Follow `../_shared/references/git-safety.md`: no config edits, no skipped hooks, no force-push, no AI
attribution; a failed hook → fix it and make a NEW commit.
