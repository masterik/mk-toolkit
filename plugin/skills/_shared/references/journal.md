# Commit journal

How an implementation session records **why** each unit of work exists, so `commit` stops
reverse-engineering intent from the diff. Written by the `Stop` / `SubagentStop` hook (the normal path)
or by the `note` skill (when the user asks); read by `commit`.

**On by default, per user.** The `SessionStart` hook (`scripts/hooks/session-bootstrap.sh`) writes the
user-scoped default on the first session after install, so journaling applies everywhere without a setup
step. Enablement still resolves repo-first, so a repo can always overrule it:

| | file | means |
| --- | --- | --- |
| repo, off | `<git-dir>/mkit/journal.disabled` | never journal here, whatever the default says (`journal.sh disable`) |
| repo, on | `<git-dir>/mkit/journal.enabled` | journal here (`journal.sh enable`) |
| user | `~/.claude/mkit/journal.default` | journal in every repo that has said neither (the `SessionStart` hook, or `install.sh`) |
| neither | — | off — reachable only by `install.sh --uninstall` |

Checked in that order. The repo always outranks the user default, and a tombstone outranks a marker —
precedence never resolves *toward* writing. `journal.sh enabled` prints the one-word verdict the hook
compares against; `enabled --why` adds where it came from (`repo` / `user` / `none`).

`disable` writes a tombstone **only** when a user default is in play; otherwise it just removes the marker,
leaving a pristine repo byte-identical to one that never opted in.

Turning it off for the *user* needs `install.sh --uninstall`, which writes its own tombstone
(`~/.claude/mkit/bootstrap.disabled`) one scope out, for exactly the reason this file's repo tombstone
exists: without a record of the intent, the hook would re-establish the default next session and the
opt-out would silently expire.

## The two governing rules

> **1. The journal is a changelog of intent, not a commit plan.** It answers "why does this hunk exist?"
> It never answers "what should the commits be?"

> **2. The hook names the gap; the agent supplies the judgement.** The hook computes set difference —
> which dirty paths have no entry — and hands that back to the model. It never authors a record.

Rule 2 is the `gate-detect.sh` shape (`fast=` *proposed* beside `docs_candidates:`) applied to a lifecycle
event instead of a command: a script for a mechanical invariant, never for a decision.

Rule 1 has one load-bearing consequence: **the journal can never put a message on a commit whose staged
hunks `commit` has not read.** That safety read stays mandatory and untruncated however fresh the entries
look — a stale entry that wrote a plausible-but-wrong message into permanent history is a worse failure
than the tokens it saved.

## The record

**One kind: `unit`**, always agent-authored, appended by `journal.sh add`. The `Stop` nudge names that
invocation without its flags — it is one transcript line, every turn, in every journaling repo — so the
flags live here instead:

```
journal.sh add --paths <a,b|repeatable> --type <feat|fix|docs|refactor|test|chore|perf> \
               --scope <s> --subject "..." --why "..." [--source note|stop|subagent-stop]
```

| Field | |
| --- | --- |
| `seq` | 1-based, append order — what preserves dependency order |
| `branch`, `head` | `branch` filters on read; `head` anchors classification |
| `paths[]` / `blobs{}` | repo-relative paths, and path → blob sha (`""` for a deletion): the freshness check |
| `type`, `scope`, `subject` | a *proposal* to `commit` (`conventional-commits.md`) |
| `why` | one line: why this unit exists. The perishable half — unrecoverable from a diff |
| `source` | `note` \| `stop` \| `subagent-stop` |

Plus `kind` and a UTC `ts`. The script records all of that itself; the caller supplies only judgement —
`paths`, `type`, `scope`, `subject`, `why`.

**`source` is diagnostic only** — `commit` treats all three identically. It exists so "is the unattended
path actually firing?" is answerable without instrumenting a session.

## Classification

`journal.sh status` classifies each entry against current state:

| Class | Test | `commit` does |
| --- | --- | --- |
| `fresh` | every path still dirty, every blob hash matches | trust the intent; stage, then the safety read |
| `drifted` | paths still dirty, ≥1 hash changed — **or** only some of its paths are still dirty (a partly consumed entry) | re-read **only those paths** |
| `committed` | paths no longer dirty, touched by a commit in `<head>..HEAD` | drop the entry |
| `orphaned` | paths no longer dirty, not committed since `<head>` (reverted) | drop, mention it |
| `unknown-head` | recorded `head` no longer resolves — gc after a rebase, a fresh clone. An amend or reset usually leaves the old commit reachable as a dangling object, so it does *not* fire | treat as `drifted` — **never** `fresh` |

A path gone from disk but showing as a deletion in porcelain is still valid intent → `fresh`. Untracked
files are in scope. `status` also emits coverage arithmetic (`journal_covered` / `journal_uncovered` + the
uncovered paths) and `overlap:` — a path in more than one `fresh` entry, the patch-staging signal.

## Who decides what

| Decision | Owner |
| --- | --- |
| Why a unit exists (`why`) | journal — perishable |
| Paths in a unit, their content at note time | journal records; the script verifies freshness |
| Unit ordering | journal (append order) |
| Type / scope / subject per unit | journal *proposes*; `commit` may override |
| **Whether a record is needed at all** | **the hook** — coverage arithmetic |
| **What the record says** | **the agent being nudged** — it has the session context |
| **How units group into commits** | **`commit`** — needs the whole tree and the user's answer |
| Overlapping paths → patch staging | script *reports*; `commit` decides the hunks |
| Final subject wording | `commit` — must match what was actually staged |
| Drop / defer a unit | `commit` + the user |

## Storage, and what it costs

One file per repo: `<git-dir>/mkit/journal.jsonl`, append-only JSONL, under the same root `run-open.sh`
owns — never committed, never in `git status`.

- **One file for the whole repo, not one per branch.** Records carry their `branch`, filtered on read. No
  branch-name slugging, so no `feat/a` vs `feat-a` collision.
- **Entries die with the worktree.** A linked worktree has its own git dir, so its journal goes when the
  worktree does — correct, since `commit` spends the entries before `finish` tears it down, but it means
  journaling does not survive `wt`-style removal. Includes a **subagent under `isolation: "worktree"`**,
  which journals into a git dir the parent never reads.
- **Renames are not tracked.** A renamed path degrades to `orphaned` + the new path `uncovered` — i.e. to
  today's behavior.

Accepted, documented gaps — not bugs to work around.

## The escape hatch mkit does not ship

A **`PostToolUse` hook with `type: "agent"`** is the maximum-fidelity capture — an LLM with tools
inspecting every `Edit` as it lands, so nothing is missed and no `Stop` nudge is needed. It is also the
wrong default: an extra model call on every edit, in every repo. A power user who wants it registers it in
their own `settings.json`; mkit will not ship it.

Same for **`asyncRewake`** (background hook, wakes the model on exit 2) — it keeps the nudge off the turn's
critical path, but decouples it from the turn whose context makes `why` answerable. Reconsider only if the
synchronous hook measures slow.
