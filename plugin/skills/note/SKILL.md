---
name: note
description: >-
  Record why the unit of work just finished exists, into the repo's commit journal, so a later
  commit does not have to infer intent from the diff. Trigger on "note this", "record what I just
  did", "journal this", "log the intent". Capture only — committing is commit, reviewing is review.
model: haiku
---

# Note the intent

Part of the **mkit** bundle; the journal's rules, record shape and known gaps are in
`../_shared/references/journal.md`.

**This is not how records normally get written.** The `Stop` / `SubagentStop` hook is the automatic path:
it names the dirty paths no entry covers and the agent that did the work records them before its turn ends.
`note` exists for two cases — the user explicitly asks ("note this"), and journaling is enabled but hooks
are unavailable.

## The job

One judgement, one call.

1. **Name the paths** of the unit just finished — repo-relative, only the paths belonging to *this* unit.
2. **Decide** `type`, `scope` (`../_shared/references/conventional-commits.md`), a one-line **imperative**
   `subject`, and one line of `why`: why this unit exists, not what the code does. Omit `--scope` when the
   unit spans areas.
3. **One call**:

   ```bash
   ${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh add --source note \
     --paths scripts/journal.sh,tests/bats/journal.bats \
     --type feat --scope journal \
     --subject "Add journal.sh with add/status/uncovered subcommands" \
     --why "commit had to re-derive intent from the diff; capture it while the session still knows it."
   ```

   The script records `head`, `branch`, `seq`, `ts` and the blob hashes itself — you supply only judgement.

4. **Confirm in one line**: the `seq` it printed, the type/scope/subject, the path count.

If the script refuses — not a git repo, journaling not enabled — relay its one line and offer
`${CLAUDE_PLUGIN_ROOT}/scripts/journal.sh enable`. Do not work around it.

## Non-goals

**No diff read. No staging. No quality gate. No run directory.** The judgement comes from the session
context already loaded; re-reading the diff to write a `why` defeats the entire point of capturing it now.
This is the cheapest skill in the bundle — keep it that way.

One unit per call. Two unrelated things finished → two `add` calls, in the order they were done: append
order is the dependency order `commit` reads.

**It records intent; it never proposes commits.** `type`, `scope` and `subject` are a proposal `commit`
may override, and the entry is worthless as a commit message on its own — `commit` authors the final
wording against the hunks it actually staged.
