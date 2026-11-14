# Summary format and decisions

The form of a skill's closing message and how it collects a decision. Each skill keeps its own *content*
list; this file owns the *form* and the *ask*.

## A closing message is a brief, not a receipt

From the message alone the reader must tell **what happened, what was left as it was and why, and what is
open**. A bare count, title or sha is not a summary: anything the reader could ask "what was that?" about
gets a self-contained line — what, where, why.

Order: the **verdict** first, then the detail, then the ask **last**. Print "none" for an empty section a
reader might wonder about (open questions, left alone).

## Form (Claude app and terminal alike)

- Open with the verdict as one bold line, no heading: `**Verdict:** 2 commits made, tree clean.`
- One `##` heading per section, blank line around it; skip headings for a short report.
- **Tables only for short cells** (id, tag, `path:line`, a few words, a status), under ~100 columns. Prose
  never goes in a cell.
- A record with a body is a `###` heading, a `path:line` line, and labelled bullets one level deep.
- Code and paths in backticks; a code fence only for a real snippet, diff or a fixed-format block.
- Plain status words (`fixed`, `kept`, `needs decision`), not colour emoji. One leading `✓` or `?` is fine.
- No HTML, no rule between sections, no bold on whole sentences. Bold marks the verdict, bullet labels and
  the closing ask.

## Collecting a decision

Write the decision into the summary first; the prompt only collects answers.

- **Host has `AskUserQuestion`** → one question per decision, up to four per call. Header ≤12 characters
  (an id or truncated name; the full name goes in the question text). Options: the recommended action first
  and marked, then the alternative, then do-nothing, each with a one-line consequence. A skill may define its
  own choice set; with no recommendation, do-nothing goes first. A diff or config snippet goes in `preview`.
- **No such tool** → end with the same choices as a numbered list per decision, recommended first; ask for
  the numbers.
- **No answer, or an unclear one, is "do nothing"** — never the destructive or outward-facing option. Name
  what was left alone.
- A go-ahead for a plan already on screen is two options; do not invent a third.
