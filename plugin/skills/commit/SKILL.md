---
name: commit
description: >-
  Stage and split local changes into logical Conventional Commits. Trigger on "commit", "make a commit",
  "split into commits", or what the commit message should be. Commits only — merge is finish, PR
  is pr.
model: sonnet
---

# Commit work

Part of the **mkit** bundle (`commit` · `finish` · `pr` · `review`); shared references
live in `../_shared/references/`. This skill only commits — merge-and-cleanup is `finish`, a PR is
`pr`. How the steps compose — and why this one runs alone, in any order, with the rest skipped — is
`../_shared/references/workflow-contract.md`.

Keep output bounded throughout. For this skill that is two rules, restated below where they apply:
**`--stat` before any diff**, **never truncate a staged diff you are about to approve**.
`../_shared/references/output-discipline.md` has the reasoning; read it only for a case not covered here.
`commit` is the front-end both finishers call, so what it loads, they load. `commit` does not run the
quality gate — that starts at `pr` and `finish` (`../_shared/references/quality-gate.md`).

## Goal

Commits that are easy to review and safe to ship:

- only intended changes included
- logically scoped (split when needed)
- messages saying what changed and why

## Ask if missing

- One commit or several? (Unsure → default to several small ones when changes are unrelated.)
- Conventional Commits are required.
- Repo rules — required scopes and max subject length — are **read, not asked for**: step 0's
  `mkit repo profile --json` answers both. Ask only for what it leaves unanswered.

## Workflow

**Step 0 — one call for everything read-only**, which also opens this run's directory
(`../_shared/references/output-discipline.md`):

```bash
mkit facts commit
```

`mkit facts` **is** this skill's dependency check. If it fails with `command not found` **or**
`unknown command "facts"` — absent and too old are the same answer here — **stop** and say:

> This skill runs on the `mkit` binary. Install it with `brew install masterik/tap/mkit` (or upgrade
> with `brew upgrade mkit`), then run it again.

Presence only, no declared minimum on either side — a subcommand that does not exist *is* the too-old
signal.

Then read what ran on this branch before, and what each step concluded — never a stop:
`../_shared/references/workflow-contract.md`, "Reading it".

```bash
mkit work show --json --limit 20
```

A `spec` or `implement` gist is the scope hint step 2 splits against — it says what this branch was
*for*, which the diff alone never does.

Keep the `run=` and `refs=` literals it prints; this file writes the first as `<run-dir>`. There is no
`$RUN_DIR` — a shell variable does not survive to the next Bash call — and re-running `mkit facts` opens a
second directory instead of returning the first.

Then how this repo words a commit — one more read-only call, cheap enough to send in the same
message as the one above:

```bash
mkit repo profile --json
```

Two values, each tagged `pinned` or `discovered`:

- **`commit_scopes`** — the scopes this repo uses. `pinned` is the repo's own list: use it as given and
  **do not re-derive scopes from history** (`../_shared/references/conventional-commits.md`'s scope
  detection is the fallback, not a cross-check). `discovered` is that same history read for you, so it is
  a starting set, not a rule.
- **`commit_subject_max`** — the longest subject this repo accepts. Only ever `pinned`: nothing discovers
  it, because history shows what past subjects happened to be, not what is required. Absent → the
  conventional ~72.

**Rule 3 applies to what you used** (`../_shared/references/workflow-contract.md`): a pinned value is
named as pinned in the final report, a discovered one is named as derived.

**Silence, with one exception.** If this call fails, or a value comes back `unavailable` with nothing but
"not discoverable" behind it, carry on exactly as this skill did before it existed and **say nothing about
it** — the profile is enrichment, not a prerequisite, and there is no version of this skill that stops on
it. The exception is a **pin that went nowhere**: a non-empty `config_problems`, or an `unavailable` whose
`cause` names `.mkit/config.toml`. That is the repo asking for something mkit could not honour — report the
`detail` in one line, verbatim, and carry on with what is left.

1. **Inspect before staging** — step 0's `mkit facts` returned all of it.
   - Confirm `branch=` is the intended one; matters inside a worktree (`linked=yes`).
   - **`run_ignored=no` stops this skill before step 3.** mkit's own scratch root is not ignored here,
     so staging would sweep run artefacts into your commit. Report the `notes:` remedy — it has to be
     applied from the main checkout — and do not stage.
   - Read **both** stats: `unstaged_stat` and `staged_stat`. They are separate because a bare
     `git diff --stat` reports nothing when the work is already fully staged, which reads exactly like a
     clean tree.
   - Then the full diff per file, only for files you must judge — never the whole tree at once.
   - Large mixed tree: stop after the `--stat` and let step 2 delegate the read.
2. **Decide commit boundaries.**
   - Split by: feature vs refactor, backend vs frontend, formatting vs logic, tests vs prod code, dependency
     bumps vs behavior changes.
   - Changes mixed within one file → plan patch staging.
   - **Large mixed tree (>~10 files or >~400 changed lines): delegate the read**
     (`../_shared/references/agent-delegation.md`). The subagent does not have this skill loaded, so its brief
     carries the branch, the stats, the file list and the split heuristics above — all of which step 0
     returned. It reads the diff, writes the plan to `<run-dir>/commit-plan.md` (the file is the completion
     signal) and **returns a commit plan and nothing else** — per proposed commit: type and scope, one-line
     subject, the paths it covers, one line of rationale, plus any file needing patch staging because it is
     mixed. Under ~20 lines; no diff, no file contents. The brief states the coverage contract: **every path
     in the file list in exactly one proposed commit**, and a hunk assignment for every file it calls mixed.
   - **Two exclusions.** Below that size, read it here — the `--stat` plus a few targeted per-file diffs
     costs less than the round trip. And **more than roughly three mixed files is read here regardless of
     size**: a per-hunk assignment does not survive a twenty-line return budget.
   - **The plan is a proposal, not a decision.** Verify coverage by comparing its path list against the
     `*_file_list` blocks step 0 returned — a set comparison, not a re-read: same set, no path twice,
     nothing invented. **On a gap, name the missing paths back to the same subagent** and ask only where
     they go; its context still holds the diff. Then do the staging, wording and splits here — the rationale
     is what lets you answer "why is X with Y?" without re-reading the diff.
3. **Stage only what belongs in the next commit.** A whole file is `git add -- <path>`. A file whose
   changes belong in more than one commit uses the patch recipe below. Do not reach for `git add -p` or
   `git restore --staged -p`: both are interactive, and there is no terminal here.
4. **Review what will actually be committed.** `"$git_bin" -C "$toplevel" diff --cached --stat` to plan, then
   `"$git_bin" -C "$toplevel" diff --cached -- "<path>"` **file by file, skipping none** — the one read
   that must not be truncated, since the checks below only work on the actual hunks
   (`../_shared/references/output-discipline.md`). Pinned to `git_bin=` because a hook can hand you a
   *summarized* status or diff that reads exactly like the tree, and approving a commit off a digest is
   the failure that guarantees (`../_shared/references/git-safety.md`). Check for: secrets or tokens,
   accidental debug logging, unrelated formatting churn.
5. **Describe the staged change in 1–2 sentences** before writing the message: what changed, why. If you
   cannot describe it cleanly the commit is too big or mixed — back to step 2.
6. **Write the message.** Conventional Commits required (`../_shared/references/conventional-commits.md`).
   The scope comes from step 0's `commit_scopes` — a scope outside a **pinned** list is a scope this repo
   does not use, so pick from the list or say why none fits; keep the subject within
   `commit_subject_max` where one is pinned.
   Single line: `git commit -m '<subject>'`. Multi-line: allocate the file with
   `mktemp "<tmp>/mkit-msg.XXXXXX"`, write the message to the path it prints, then
   `git commit -F "<msgfile>"` — not `-v` (interactive) and not `-F -` with the body on a heredoc,
   which a permission gate has been measured denying because it leaves nothing to judge. **Allocate,
   never a fixed name**: `<tmp>` is shared by every session on the machine, so a second `commit` run
   writing `mkit-msg.txt` would overwrite this one's message between writing it and committing it.
7. **Repeat** until the working tree is clean.

## Patch staging, non-interactively

`git add -p` needs a terminal. This does not, and it uses no shape a permission gate refuses — in
particular **no heredoc'd interpreter**: `python3 - <<'PY'` is refused outright in an isolated session,
because a program assembled at runtime cannot be shown not to be git
(`../_shared/references/git-safety.md`). Improvising a splitter into the user's working tree is how a
helper script nearly got committed into someone else's project.

Four steps per mixed file. `<tmp>` is the `tmp=` from `mkit facts`.

**Allocate the patch file; never name it after the source file.** `<tmp>` is one directory shared by
every session on this machine — two `commit` runs over the same path would otherwise pick the same
`mkit-<f>.patch`, and one would stage the other's selected hunks. Step 1 prints the path it allocated;
carry that literal into steps 2–4, exactly as you carry `run=` — a shell variable does not survive to
the next Bash call (`../_shared/references/output-discipline.md`, "carry the path, not a variable").

```bash
# 1. the file's unstaged diff, on its own, to a freshly allocated file that dies with this run.
#    Run both; the mktemp prints <patch>, which is what steps 2-4 use.
mktemp "<tmp>/mkit-patch.XXXXXX"
"$git_bin" -C "$toplevel" diff -- "<path>" > "<patch>"

# 2. read it, then DROP the hunks that belong in a later commit by editing <patch>:
#    delete whole `@@ … @@` blocks — header lines (diff/index/---/+++) stay, hunk headers are
#    left exactly as they are. Never renumber and never hand-edit a `@@` line.

# 3. apply what is left to the index only
"$git_bin" -C "$toplevel" apply --cached -- "<patch>"

# 4. verify, always — a silent partial apply is the failure mode
"$git_bin" -C "$toplevel" diff --cached --stat -- "<path>"
```

- `--cached` touches **only the index**; the working tree keeps every hunk, so the leftovers are still
  there for the next commit. If step 3 fails, nothing was staged: re-cut the patch from step 1 rather
  than editing further.
- **Hunks that share lines are not split.** Two edits inside one `@@` block go in one commit, or you
  produce an intermediate commit that does not build. When a split would require cutting inside a hunk,
  commit the file whole and say so in the final report.
- `git apply --cached -R -- "<patch>"` unstages exactly what a patch staged. To unstage a whole file:
  `"$git_bin" -C "$toplevel" restore --staged -- "<path>"`.
- The patch file lives in `tmp=`, never in the repo. Delete it once step 4 has verified the staging, and
  nothing of this survives the run.

## Final report (always)

Prune with `mkit run prune` on the way out, folded into another call.
After the last commit run `"$git_bin" -C "$toplevel" log --oneline -n <N>` (N = commits made this run) to
confirm hashes, then report every commit — never skip this, even for one:

```
<short-sha>  <type>(<scope>): <summary>
  <what/why, 1 sentence>
```

One block per commit, in order. Say so if staged changes were deliberately left out.

Then one line for what the repo told you, where it told you anything — rule 3, and the shortest form
that satisfies it:

```
Scopes: cli, core (pinned in .mkit/config.toml) · subject max 72 (pinned)
Scopes: derived from the last 200 commits · no subject limit pinned (~72 assumed)
```

Nothing pinned and nothing to say → no line. A rejected pin is reported here too, in the words the
profile's `cause` used.

Then record the run:

```bash
mkit work append --step commit --gist '<one line: what this run concluded>' \
  [--artifact '<first-sha>^..<last-sha>'] [--assume '<what this run derived rather than found>']...
```

The range is inclusive of the first commit — `<first-sha>..<last-sha>` excludes it, and names nothing at
all on a one-commit run. For a single commit pass the sha itself.

After the report is produced, and it never changes the report: a failed append is one line of note, not a
failed run. **A run that made no commits still records**, with no `--artifact` and a gist naming the state
it actually found: `working tree clean; nothing to commit` only where that is true, and otherwise what was
left behind and why (`changes left unstaged at the user's request`). One record per finished step, and a
checked no-op is a fact — it tells the next step the tree was looked at, which is not what an absent record
says. A gist that reports a clean tree over a dirty one is worse than no record at all.

## Conventional Commit format

```text
<type>(<scope>): <summary>

<What changed.>
<Why it changed.>
```

Summary imperative and specific. Type table and scope detection:
`../_shared/references/conventional-commits.md`.

## Git safety

Follow `../_shared/references/git-safety.md`: never update git config, never skip hooks unless asked, never
force-push, never add AI co-authorship, and if a hook fails, fix it and make a NEW commit.
