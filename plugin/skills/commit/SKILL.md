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
`pr`.

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
- Any repo rules: max subject length, required scopes.

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

Keep the `run=` and `refs=` literals it prints; this file writes the first as `<run-dir>`. There is no
`$RUN_DIR` — a shell variable does not survive to the next Bash call — and re-running `mkit facts` opens a
second directory instead of returning the first.

1. **Inspect before staging** — the call above returned all of it.
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
