---
name: recap
description: >-
  Plain-English status of the work on this branch: what was done, what changed (committed vs not), what
  is left, and what is waiting on the user. Built from the session, commit messages, the PR and git state, so
  it still holds after the context was compacted. Use it as /mkit:recap [simpler|eli5|deeper], and also
  when the user asks "where are we", "what did we do", "catch me up", "what's left", or wants a summary to
  hand the work to someone else.
---

# Recap the work, in plain English

Part of the **mkit** bundle, but outside the edit → commit → review → finish/pr line, like `cleanup`. This
is a status read, not a step. It is **read-only**: it opens no run directory, records nothing and
changes nothing in the repo. Running it leaves no trace.

How the answer reads (levels, writing rules, visuals, words used, the closing line):
`../_shared/references/plain-english.md`. Read it before answering. A level word (`simpler`, `eli5`,
`deeper`, with or without `--`) is the only argument.

## Preconditions

**First call, on its own:**

```bash
bash ../_shared/scripts/facts.sh recap --gh
```

The path is relative to this skill's directory (`../_shared/references/output-discipline.md` says how to
resolve it). No `--run`: recap writes nothing.

If it exits 1 because this isn't a git work tree, stop and say that recap reports on a repository's
branch, so it needs to run inside one. `/mkit:explain` covers the session alone.

From `facts.sh`: `branch=`, `default_branch=`, `detached=`, `upstream=`, `pushed=`, `ahead=`/`behind=`,
`clean=`, the `staged= unstaged= untracked= conflicted=` counts, `status:`, the stats, `pr=` with
`pr_state=`/`pr_draft=`, `remote_repo=`, and `toplevel=` (`<toplevel>` below; `<git>` is the absolute git
binary, `../_shared/references/git-safety.md`).
**`pushed=` only says whether an upstream is set.** With an upstream, commits are published only when
`ahead=0`; with `ahead=2`, say two commits are still local. With `upstream=none`, say no upstream is set,
so whether the branch was pushed is unknown — never "not pushed".

**The base this branch is measured against** is `default_branch`, unless the session, a commit message or
the PR's base says the branch is stacked on another one. Then use that parent, because measuring from the
default would list the parent's commits as this branch's work. Skip this step when the branch **is** the
base, when `detached=yes`, or when `default_branch=unknown`. Otherwise read what this branch has committed,
subjects **and bodies** — the body is where a commit says why — bounded so a
long-lived branch can't flood the context:

```bash
"<git>" -C "<toplevel>" log --no-decorate -n 30 --format='%h %s%n%b' <base>..HEAD
```

If that fails (a base that exists only on the remote, for instance), or the step was skipped, say in one
line that there was no base to compare against. Then build "Done" from the PR and the session alone.

**When `pr=` is a URL**, read the PR's own account of the work and where its review stands, in the same
message as the log:

```bash
gh pr view <pr-url> --json title,body,baseRefName,reviewDecision,latestReviews
```

Any other `pr=` value (`none`, `gh-missing`, `gh-unauthenticated`, `no-remote`, or absent on a detached
HEAD) is **one fewer input, never a stop**; a failed `gh pr view` is the same. Say in one line that the
recap rests on the session and git alone, and name the `pr=` value only when it means the user could fix
it (`gh-unauthenticated`).

## Build the recap

Merge three sources, and when they disagree, trust them in this order: **git** (what is actually true
now: commits, staged, unstaged and untracked), then **what was published** (commit messages and the PR's
body and review state — what was concluded and why), then the **session** (what was said, intended and
decided). When the session claims something git contradicts (for example "I committed that" while the
change is still unstaged), report what git shows and name the gap.

**After the context was compacted, the session is thin** — and the reasons behind uncommitted work went
with it. Describe uncommitted changes from `status:` and the stats (what changed, in which files), and say
plainly that why they were made can't be recovered from git. Never invent a reason. Committed work keeps its
reasons in its messages and the PR body.

**A PR review is current only for the commits it saw.** A `latestReviews` entry or `reviewDecision` describes
the branch as it was when the reviewer read it; commits pushed since, or uncommitted changes now, are not
covered. Say "approved before the last two commits", never "approved", when the log shows later work.

## Shape

Follow `plain-english.md` at the chosen level, in this order:

1. **One line:** where things stand, as the very first line. No title heading above it, and one sentence,
   not a paragraph. For example: "The feature works and is committed; two review fixes are
   still uncommitted, and the PR isn't open yet."
2. **Waiting on you:** decisions, approvals or questions the user still owes, and review comments asking
   for changes. Put it first when it isn't empty, because it is the part that blocks. Leave the heading out
   when nothing is waiting.
3. **Done:** what was finished, in plain words, with the commits or PR that show it.
4. **Changed:** committed vs uncommitted (staged, unstaged, untracked), and whether the branch is pushed.
   Name files only where a name helps. Counts are enough for the rest.
5. **Left:** what's still to do, as the session, the PR body or its review left it. Don't invent a plan. If
   nothing says what is left, say that.
6. **A timeline visual** when there were several steps (`plain-english.md`, Visuals). It's usually a short
   left-to-right flow of what ran, with a marker for where things are now.
7. **Words used,** then the closing line.

The recap should read on its own, so the user can paste it to a colleague without rewriting it. Don't
refer to "above" or "earlier in this chat".
