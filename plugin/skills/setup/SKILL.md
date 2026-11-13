---
name: setup
disable-model-invocation: true
description: >-
  Record this repo's settings for mkit's skills as prose in docs/agents/ (commit scopes, review mode, merge
  style and reviewers, remote, gate commands, cleanup keep-list), and convert an old .mkit/config.toml. Run it
  as /mkit:setup; optional — every skill works without it.
---

# Set up mkit for this repo

Write the per-repo settings mkit's skills read: short prose files in `docs/agents/`, plus a `### mkit workflow`
sub-block under the `## Agent skills` heading of `CLAUDE.md` or `AGENTS.md` — the same place and form
mattpocock's setup uses, so a repo with both packs has one place to look.

This is a prompt-driven skill, not a script. Explore, present what you found, confirm with the user, then
write. **Optional**: every skill runs without these files, discovering or asking instead
(`../_shared/references/repo-settings.md`, which is also the one place the readers' rules live). A file is
written **only for a non-default choice**, so the folder shows exactly what this repo changed.

Areas and the file each writes, from the seed templates in this folder:

| Area | File | Template | Default (no file) |
| --- | --- | --- | --- |
| Commit scopes, subject length | `docs/agents/commit.md` | [commit.md](./commit.md) | scopes from history · ~72 chars |
| Review mode | `docs/agents/review.md` | [review.md](./review.md) | `full` |
| Merge style, reviewers | `docs/agents/merge.md` | [merge.md](./merge.md) | `merge` · `CODEOWNERS` |
| Remote | `docs/agents/remote.md` | [remote.md](./remote.md) | `origin` → lone remote → ask |
| Gate commands | `docs/agents/gate.md` | [gate.md](./gate.md) | detected |
| Cleanup keep-list | `docs/agents/cleanup.md` | [cleanup.md](./cleanup.md) | default + `develop`/`development`/`dev` |

The spec store is not mkit's: it is `docs/agents/issue-tracker.md`, mattpocock's file. Read it, never write it.

## What this does and does not touch

- Writes only the six files above and the `### mkit workflow` sub-block. **Never edits mattpocock's files**
  (`issue-tracker.md`, `triage-labels.md`, `domain.md`) or the other `###` sub-blocks under `## Agent skills`,
  and never anything else in `CLAUDE.md`/`AGENTS.md`.
- Never commits, stages or deletes `.mkit/config.toml`: it says what to remove; the user (or `commit`) does it.

## Process

### 1. Explore

Read whatever exists; don't assume. From the repo root (`git rev-parse --show-toplevel`):

- `git remote -v`: how many remotes, and which point at GitHub.
- Commit history: `git log --format=%s -200` — the scopes in `type(scope):` and how often each is used; the
  longest subjects. A repo document naming scopes (`CONTRIBUTING.md`, `.commitlintrc*`, `commitlint.config.*`)
  or a subject limit (commitlint `header-max-length`).
- `CODEOWNERS` (`.github/`, root, `docs/`).
- The gate as `pr`/`finish` would find it with no file: walk `../_shared/references/quality-gate.md`, "Find
  the gate" (task runners and manifests). Note any doc naming a different check command.
- Long-lived local branches: `git branch --format='%(refname:short)'` — anything besides the default and
  feature branches (`staging`, `release/*`-style names, an integration branch).
- `docs/agents/`: which of mkit's six files already exist (a re-run), and whether `issue-tracker.md` does.
- `.mkit/config.toml`: the old config (step 2, "Migration").
- `CLAUDE.md` and `AGENTS.md` at the root: which exist, whether one is a symlink to the other, and whether
  either has `## Agent skills` and a `### mkit workflow` sub-block already.

### 2. Present findings and ask

Summarise what's present and what's missing in a few lines. Then take the sections in order: **one section,
one answer, then the next.** Lead each with the recommended answer so the user can accept it in a word —
through `AskUserQuestion` where the host has it, else a numbered list (`../_shared/references/summary-format.md`).
Skip a section exploration already settled. A one-line explainer only where the choice genuinely branches.

An existing file's value (re-run) or a migrated value is the recommended answer for its section. "Keep the
default" is always an option, and choosing it on a re-run means that file is removed.

**A. Commit.** Scopes: recommend the scopes history uses more than once (a documented list wins), and say
that with no file `commit` reads history itself — so a file is worth it only to **fix** the list (drop
one-offs, add a scope history lacks). Subject length: recommend no limit file unless a document or commitlint
names one.

**B. Review mode.** Recommend `full` (no file). Offer `quick` for a team that wants the cheaper roster by default.

**C. Merge.** Style: recommend `merge` (no file) unless evidence says otherwise — repo-local `pull.rebase`,
a `.config/wt.toml` `merge.*` key, or the GitHub repo allowing only one method (`gh repo view --json
mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed`, when `gh` works). Reviewers: skip when
`CODEOWNERS` exists (recommend relying on it); otherwise ask for handles, recommend none.

**D. Remote.** Only with several remotes: recommend `origin` when it exists (no file) — and, in a fork
workflow (`origin` is the fork, another remote the project), ask which one branches go to. One remote or
none: skip, no file.

**E. Gate.** Show what detection finds. Recommend no file when that is the gate the repo actually runs. Offer
a file when it is not — detection found nothing, a doc names a different command, or the user wants another
order.

**F. Cleanup.** Recommend no file unless exploration found long-lived branches; then offer them as the keep
list, and a develop branch only when it is not named `develop`/`development`/`dev`.

**G. Spec store.** Nothing to ask. Report what `docs/agents/issue-tracker.md` says, or that it is absent
(mattpocock's setup writes it; mkit assumes no store without it).

#### Migration — `.mkit/config.toml`

Written by the old `mkit init`. Read it as text — no tool needed — and carry over every value that differs
from its area's default, as that section's recommended answer:

| Old key | Goes to |
| --- | --- |
| `[commit] scopes`, `subject_max` | `commit.md` |
| `[review] mode` (not `full`) | `review.md` |
| `[review] reviewers` | `merge.md`, Reviewers |
| `[merge] style` (not `merge`) | `merge.md`, Merge style |
| `[remote] name` | `remote.md` |
| `[gate.commands]` (step → command) | `gate.md`, in the order `lint`, `typecheck`, `vet`, `test`, `build`, then the rest |
| `[cleanup] keep` | `cleanup.md`, Keep |
| `[spec] store`, `ref` | nothing — compare with `issue-tracker.md` and report a mismatch or absence |
| `version` | nothing |

`mkit init` also copied **discovered** values into the file (history scopes, detected gate commands). Where a
carried value equals what discovery finds today, say so and recommend dropping it — otherwise the file
freezes an answer the repo would give anyway. A key not in this table: show it and ask. A file that won't
read as TOML: show the lines you could read, carry over nothing on a guess, and ask.

### 3. Confirm and edit

Show the drafts together: each `docs/agents/*.md` file to write (filled from its template, placeholders
replaced, sections the user left at default removed), each file to remove, and the `### mkit workflow`
sub-block. Let the user edit before anything is written.

### 4. Write

**Pick the file to edit:** `CLAUDE.md` if it exists, else `AGENTS.md`; neither → ask which to create, don't
pick. Never create one when the other exists. One a symlink to the other → edit the target, once.

**The block.** If `## Agent skills` exists, put `### mkit workflow` under it — replacing an existing one in
place, after the other sub-blocks otherwise — and leave the other sub-blocks untouched. If the heading is
absent, append it with only this sub-block. List only the files that exist after this run:

```markdown
### mkit workflow

Settings for mkit's skills; a file not listed means the default, or what the skill discovers. See:
- `docs/agents/commit.md`: <scopes, subject length>
- `docs/agents/merge.md`: <merge style, reviewers>
- <one line per existing file>

Spec store: `docs/agents/issue-tracker.md`.
```

With no mkit file at all, the body is one line: *Defaults throughout: scopes from history, `full` review,
`merge` style, `CODEOWNERS` reviewers, remote by `origin` → lone remote → ask, gate detected, cleanup keeps
the default and develop branches. Add a `docs/agents/<area>.md` file (or re-run `setup`) to change one.*
Drop the spec-store line when `issue-tracker.md` is absent.

**The files.** Create `docs/agents/` only if a file is written. Write each chosen file from its template;
delete each one the user set back to default.

### 5. Done

Report, per `../_shared/references/summary-format.md`: the files written, removed and left alone, the file
whose block was edited, and which skills read what (`commit` → `commit.md`; `review` → `review.md`; `pr` and
`finish` → `merge.md`, `remote.md`, `gate.md`; `cleanup` → `cleanup.md`, `remote.md`). Say the files can be
edited by hand later; re-running `setup` is optional. After a migration: `.mkit/config.toml` can go —
`git rm .mkit/config.toml` (plain `rm` if it was never committed); no skill reads it any more. Nothing is
committed: say so.
