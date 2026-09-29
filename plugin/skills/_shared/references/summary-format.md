# Summary format and decisions

How a skill's closing message is shaped, and how it collects a decision. Used by every mkit skill that
ends with a report or a question. Each skill keeps its own *content* list (what its report must say);
this file owns the *form* and the *ask*, so they do not drift apart.

## A closing message is a brief, not a receipt

The reader has not watched the run. From the message alone they must be able to tell **what happened, what
was left as it was and why, and what is open**. A bare count, a bare title or a bare sha is not a summary.
Anything the reader could ask "what was that?" about gets a self-contained line: what, where, why.

Order: the **verdict** first, then the detail, then the ask **last** — so it is what is left on screen.
Print "none" for an empty section that a reader might otherwise wonder about (open questions, left alone);
its absence must never be ambiguous.

## Form: the same in the Claude app and in a terminal

Both render Markdown; a terminal does not reflow wide or nested structure.

- Open with the verdict as one bold line, no heading: `**Verdict:** 2 commits made, tree clean.`
- One `##` heading per section, blank line around it. Skip headings for a report short enough to read
  without them.
- **Tables only for short cells** (id, tag, `path:line`, a few words, a status). Prose never goes in a
  cell. Keep a table under about 100 columns; drop a column before wrapping one.
- A record with a body is a `###` heading, a `path:line` line, and labelled bullets one level deep.
- Code and paths in backticks. A code fence only for a real snippet, diff or a fixed-format block a skill
  defines — never to make text stand out.
- Plain status words (`fixed`, `kept`, `needs decision`), not colour emoji. One leading `✓` or `?` is fine.
- No HTML, no rule between every section, no bold on whole sentences. Bold marks the verdict, bullet
  labels and the closing ask.

## Collecting a decision

Write the decision into the summary first — the summary is complete without the prompt. Then collect the
answers:

- **Host has `AskUserQuestion`** → one question per decision, up to four per call (more in further
  calls). Header = a short id of at most 12 characters (finding id, a truncated branch name or setting); the full name goes in the question text. Options: the recommended action first
  and marked so, then the alternative, then the do-nothing choice, each with a one-line consequence. A skill
  may define its own choice set (`take it` / `decline`); mark a recommendation only where the skill has one,
  and put the do-nothing choice first when none is recommended. A diff
  or a config snippet goes in `preview`. The question text names the subject, so it reads alone.
- **No such tool** (another host, or a non-interactive run) → end with the same choices as a numbered list
  per decision, recommended first, and ask the user to reply with the numbers.
- **No answer, or an unclear one, is "do nothing"** — never a default to the destructive or outward-facing
  option. Name what was left alone in the report.
- A decision that is really a go-ahead for something already on screen (`finish`'s plan) is the same
  prompt with two options; do not invent a third.
