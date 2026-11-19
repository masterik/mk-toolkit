---
name: shorten
description: >-
  Rewrite text shorter and tighter under the user's concision rules (report, documentation, plan). Use it as
  /mkit:shorten [report|doc|plan] <text>, and also whenever the user asks to shorten, tighten, condense,
  trim, make concise or brief, "rewrite my way" or "improve the wording", or pastes a status update, doc
  section, README, commit/PR text or plan to clean up, even without saying "concise". Output is only the
  revised text; it never edits a file.
---

# Shorten

Part of the **mkit** bundle, outside the edit → commit → review → finish/pr line, like `explain`. Runs
nothing, opens no run directory, works outside a repository. Cut words, never facts.

## Output

Revised text only, ready to copy: no preamble, no explanation, no change list. Plain text or markdown; keep
the original's structure unless a rule below says otherwise. The user pastes the result straight into place,
so commentary is cleanup work. Never edits a file: a path argument is read, the result is returned.

## What to shorten

The text in the request. None given (bare `/mkit:shorten`, "rewrite my way") → the last response, or the
item named; a file path → that file's text.

## Pick the type

An explicit `report|doc|plan` argument wins. Else infer: **report** = status, findings, answer, message;
**doc** = README, spec, guide, ADR, comments; **plan** = steps, roadmap, task breakdown. Mixed → each part
by its type. Unclear → report.

## Rules

**Report — extremely concise.** Brevity beats grammar and style.
- Result first, then only what changes the reader's next action.
- Drop articles, filler, pleasantries, restating, process narration, and hedges that carry no real
  uncertainty. Keep qualifiers that do ("likely", "suspected", "seems").
- Fragments, `→`, `=`, abbreviations, semicolons OK.
- Keep every number, name, path, ID, blocker, caveat, decision needed.

**Doc — clear, complete, concise.** Concision beats polished grammar; clarity and completeness beat concision.
- Short sentences or bullets, imperative, one idea each. Define a term once.
- Cut filler, intros, repeats.
- Keep all requirements, constraints, reasons, edge cases, meaningful examples.
- Loose grammar fine, ambiguity not: a cut that blurs meaning is restored.

**Plan — thorough, actionable.**
- Keep step order (numbered), dependencies, phases, done-when, required specs, constraints, inputs where
  present; never add any.
- What, not how. No code unless essential for clarity.
- Terse wording, but no step or requirement dropped.

## Before returning

Every fact, number, name and requirement still present? Meaning unchanged, nothing invented? Any word left
that can go without loss? Only the revised text in the output?

## Example

In (report): "Hi! I just wanted to let you know that I finished looking into the failing CI job, and it
seems the problem was caused by an outdated Node version in the workflow file, so I updated it to 20 and the
build is passing again."
Out: CI fail likely outdated Node in workflow. Bumped to 20; build passes.
