# review step 6: the closing message

A **decision brief**: from this message alone the reader must tell what was found, what was done about each
finding, and what is still open. Name the run directory once; never shrink a finding to a bare title or a
count. Form rules: `../_shared/references/summary-format.md` — this is its fullest instance.

## Sections, in this order

Open with the verdict as one bold line under no heading (`**Verdict:** 3 findings — 2 fixed, 1 needs your
decision.`), then one `##` heading per section:

1. **Verdict** — real findings, fixed, needing a decision, anything blocking.
2. **Mode** — which ran and what was **not run by design** ("Mode: quick — CodeRabbit + Codex (bugs, impl);
   architecture/quality/tests/docs/comments and adversarial not run"). Worded unlike item 3: a deliberate
   scope, never a partial run.
3. **Completeness** — sources expected vs reported. A failed or missing source, and its uncovered lenses, go
   above every finding.
4. **Scope** — range, shortstat, reviewers that actually ran, lenses no source carried.
5. **Counts** — one line by tag: "2 `[code, major]`, 1 `[docs, minor]`". Never a bare "3 findings".
6. **Findings** — a table first, one row per finding: `id | [surface, severity] | file:line | title |
   status` (`fixed`, `needs decision`, `skipped`), by severity then file. Then each finding as
   `### F<id> [surface, severity] Title`, a `path:line · conf N` line, and three bullets — **Trigger**,
   **Consequence**, **Fix** (done or proposed) — one or two sentences each. Name the reviewers behind any
   finding more than one raised; mark `refined` ones. The only place a body appears; later sections cite ids.
7. **Fixed automatically** — per fix: id, file, what changed, why it was safe unasked.
8. **Needs your decision** — each risky finding as a choice: the proposed fix, the alternative, what happens
   if left alone.
9. **Considered, not changed** — each skipped finding and `immaterial` verdict: id, title, the concrete
   reason.
10. **Open questions** — what the review could not settle: what is unknown, why it matters, who could
    answer. Always printed; "none" when empty. **Pre-existing** problems get their own section, out of the
    counts, one entry each with file:line.
11. **Run directory** — the `<run-dir>` path; whether a second round ran (and its result). Say a fixed tree is
    unverified here: `review` does not build or test; that runs at `pr`/`finish`.

End on the decision the user has to make, or "nothing needed from you", as its own short paragraph.

## The ask

After the message is printed, never instead of it. **`AskUserQuestion`** → one question per finding needing a
decision (up to four per call): header = the finding id; options = the proposed fix (recommended, first), the
alternative, `leave as is`, each with a one-line consequence; the fix's diff in `preview` where it fits; the
question text names the finding and file. **No such tool** → the same choices as a numbered list per finding,
recommended first; ask for the numbers.
