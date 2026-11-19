---
name: shorten
description: >-
  Rewrite text or a doc file shorter. Use as /mkit:shorten <text or file path>, or when the user asks to
  shorten, tighten, condense or trim something.
---

# Shorten

Runs no commands, opens no run directory, works outside a repository.

## Input

Always explicit: pasted text, or a doc file path (any other file → ask). None given (bare `/mkit:shorten`) → ask what to shorten; never guess from the conversation.

- **Text** → output only the revised text, ready to copy: no preamble, no explanation, no change list.
- **File** → rewrite it in place with Edit/Write; reply with one line (file, rough size cut). Touch only
  that file. Keep its format and structure (frontmatter, code blocks, links, tables, headings).

## Rules

- Be clear and complete.
- Be concise: cut words, never facts.
- Brevity beats grammar and style.
- Cut filler, intros, pleasantries, repeats, process narration, hedges with no real uncertainty. Keep
  qualifiers that carry it ("likely", "seems").
- Short sentences or bullets, one idea each; fragments, `→`, `=`, abbreviations fine. Define a term once.
- Keep every number, name, path, ID, requirement, constraint, reason, edge case, caveat, decision, step
  order, dependency, and meaningful example. Never add any.
- Loose grammar fine, ambiguity not: a cut that blurs meaning is restored.
- Never touch code or commands.

## Before returning

Every fact still present? Meaning unchanged, nothing invented? Any word left that can go without loss?
Text input → only the revised text in the output.

## Example

In: "Hi! I just wanted to let you know that I finished looking into the failing CI job, and it seems the
problem was caused by an outdated Node version in the workflow file, so I updated it to 20 and the build is
passing again."
Out: CI fail likely outdated Node in workflow. Bumped to 20; build passes.
