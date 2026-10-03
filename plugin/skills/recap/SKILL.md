---
name: recap
description: >-
  Plain-English status of the work on this branch: what was done, what changed (committed vs not), what
  is left, and what is waiting on the user. Built from the session, the branch worklog and git state, so
  it still holds after the context was compacted. Use it as /mkit:recap [simpler|eli5|deeper], and also
  when the user asks "where are we", "what did we do", "catch me up", "what's left", or wants a summary to
  hand the work to someone else.
---

# Recap the work, in plain English

Part of the **mkit** bundle, but outside the edit → commit → review → finish/pr line, like `cleanup`. This
is a status read, not a step. It is **read-only**: it opens no run directory, writes no worklog record and
changes nothing in the repo. Running it leaves no trace.

How the answer reads (levels, writing rules, visuals, words used, the closing line):
`../_shared/references/plain-english.md`. Read it before answering. A level word (`simpler`, `eli5`,
`deeper`, with or without `--`) is the only argument.

## Preconditions

**First call, on its own:**

```bash
mkit facts recap --no-run
```

`mkit facts` **is** this skill's dependency check. If it fails with `command not found` **or**
`unknown command "facts"` (absent and too old are the same answer here), **stop** and say:

> This skill runs on the `mkit` binary. Install it with `brew install masterik/tap/mkit` (or upgrade
> with `brew upgrade mkit`), then run it again.

If it exits 1 because this isn't a git work tree, stop and say that recap reports on a repository's
branch, so it needs to run inside one. `/mkit:explain` covers the session alone.

Once it has succeeded, read the branch's worklog:

```bash
mkit worklog show --json --limit 20
```

What the worklog reports is **one fewer input, never a stop**
(`../_shared/references/workflow-contract.md`, "Reading it"). An empty log, an unreadable one, or a binary
without `worklog` means the recap rests on the session and git alone. Say so in one line.

From `mkit facts`: `branch=`, `default_branch=`, `detached=`, `upstream=`, `pushed=`, `ahead=`/`behind=`,
`clean=`, the `staged= unstaged= untracked= conflicted=` counts, `status:`, and `git_bin=` for the call below.
**`pushed=` only says whether an upstream is set.** With an upstream, commits are published only when
`ahead=0`; with `ahead=2`, say two commits are still local. With `upstream=none`, say no upstream is set,
so whether the branch was pushed is unknown — never "not pushed".

**The base this branch is measured against** is `default_branch`, unless the session or the worklog says
the branch is stacked on another one. Then use that parent, because measuring from the default would list
the parent's commits as this branch's work. Skip this step when the branch **is** the base, when
`detached=yes`, or when `default_branch=unknown`. Otherwise list what this branch has committed, with the
git it named (bounded, so a long-lived branch can't flood the context):

```bash
<git_bin> log --oneline --no-decorate -n 30 <base>..HEAD
```

If that fails (a base that exists only on the remote, for instance), or the step was skipped, say in one
line that there was no base to compare against. Then build "Done" from the worklog and the session alone.

## Build the recap

Merge three sources, and when they disagree, trust them in this order: **git** (what is actually true
now), then the **worklog** (what each step concluded), then the **session**
(what was said, intended and decided). When the session claims something git contradicts (for example "I
committed that" while the change is still unstaged), report what git shows and name the gap.

**A worklog record is current only while the tree is the one it ran over.** Compare each record's
`fingerprint` with the envelope's own `fingerprint`, which is the tree right now. A record that matches is
present-tense evidence. Call a record stale only when both fingerprints are present and differ; an empty
one means the record can't be dated, so report it as "recorded, current state unknown". A stale record describes an older tree: report it in the past tense ("review
passed before the last changes") and never as the current state.

## Shape

Follow `plain-english.md` at the chosen level, in this order:

1. **One line:** where things stand, as the very first line. No title heading above it, and one sentence,
   not a paragraph. For example: "The feature works and is committed; two review fixes are
   still uncommitted, and the PR isn't open yet."
2. **Waiting on you:** decisions, approvals or questions the user still owes. Put it first when it isn't
   empty, because it is the part that blocks. Leave the heading out when nothing is waiting.
3. **Done:** what was finished, in plain words, with the commits or worklog steps that show it.
4. **Changed:** committed vs uncommitted (staged, unstaged, untracked), and whether the branch is pushed.
   Name files only where a name helps. Counts are enough for the rest.
5. **Left:** what's still to do, as the session or worklog left it. Don't invent a plan. If nothing says
   what is left, say that.
6. **A timeline visual** when there were several steps (`plain-english.md`, Visuals). It's usually a short
   left-to-right flow of what ran, with a marker for where things are now.
7. **Words used,** then the closing line.

The recap should read on its own, so the user can paste it to a colleague without rewriting it. Don't
refer to "above" or "earlier in this chat".
