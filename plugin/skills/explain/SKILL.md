---
name: explain
description: >-
  Re-explain the last response, or one item from this session (a term, file, finding, error, or a decision
  and why it was made), in plain English, with a diagram when a picture helps. Use it as
  /mkit:explain [simpler|eli5|deeper] [<item>], and also whenever the user signals confusion about something
  already said: "wait what", "I don't get it", "say that again simply", "explain that in plain English",
  "what does X mean", "why did you do that". Not for fresh questions about code nobody has discussed yet
  ("explain how this function works"); answer those normally.
---

# Explain it again, in plain English

Part of the **mkit** bundle, but outside the edit → commit → review → finish/pr line, like `cleanup`. This
is a reading aid, not a workflow step. It is **the one mkit skill that needs no binary**: it does no
mechanical work, so it makes no `mkit` call and opens no run directory. It works without `mkit` installed
and outside a repository.

How the answer reads (levels, writing rules, visuals, words used, the closing line):
`../_shared/references/plain-english.md`. Read it before answering. This file only says *what* to explain.

## What this does and does not touch

- Reads this session. Reads a file only to **confirm** something the session refers to, such as a line it
  quoted or a function it named, never to widen the topic.
- Never edits, stages, commits or runs anything with side effects. Explaining a command means describing
  it, not running it.

## 1. Parse the argument

Strip a level word first (`simpler`, `eli5`, `deeper`, with or without `--`). What's left decides the
mode:

- **Empty:** the **last response.** That's the most recent assistant answer before this request, not this
  skill's own output.
- **Anything else:** an **item.** The user may refer to it loosely ("finding #2", "the shadowed thing",
  "that error", "why squash"), so match it to the session by meaning, not by exact words. A **decision** is
  an item too: "why X" and "why did you X" are items whose answer needs the reasoning (step 3).

If the item matches two different things in the session, pick the more recent one, and name the other in
one line so the user can ask for it.

## 2. Find it, or say it isn't there

Find the item in the session. If it isn't there (never discussed, or lost when earlier turns were
compacted), **say so plainly** and stop: "I can't see `<item>` in this session. It may have been in an
earlier part that was summarized. Paste the bit you mean, or ask me fresh." Never rebuild a past answer
from guesswork. A confident explanation of something that wasn't said is worse than no explanation.

A general term that the session *used* but never explained (for example "fast-forward") is in the session.
Explain it the way the session used it.

## 3. Explain

Follow `plain-english.md` at the chosen level. The shape:

1. **One line:** the whole point, as one sentence a reader could stop after.
2. **The explanation.** For the **last response**: its key points in order, with whatever the user must
   do or decide listed first. For an **item**: what it is, why it matters *here*, and what it affects in
   this session. For a **decision**, add what was chosen, why, what was rejected, and why not. A decision
   explained without its alternatives is just a restated conclusion.
3. **A visual,** only when it shows a flow, structure, before/after or comparison (`plain-english.md`,
   Visuals).
4. **Words used,** if the answer defined any terms.
5. **One closing line** offering the other levels.

Length: shorter than the original for the last response, unless the level is `deeper`. For an item, aim for
what fits on one screen.
