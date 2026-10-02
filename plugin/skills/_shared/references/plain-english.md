# Plain English

Shared by `explain` and `recap`. Sets how an answer reads: which register to use, how to change it, how to
deal with terms, and when to draw. Neither skill restates these rules. Each one links here, so they cannot
drift apart.

The reader is an engineer, often reading in a second language, who asked because the first answer did not
land. Dense, correct prose is what failed. The goal is to be understood on the first read, not to be
complete.

## Levels

The argument may carry one level word, with or without `--`. Strip it before reading the rest as the
subject.

| level | words | what changes |
|---|---|---|
| plain (default) | none | Short sentences, common words. Jargon stays, but each term is defined once, the first time it appears. |
| simpler | `simpler`, `eli5` | No jargon at all. Use a comparison to something from everyday life, and say where the comparison stops working. |
| deeper | `deeper` | Plain register, plus how it works underneath: the mechanism, the edge cases, and the file or function that does it, when the session named one. |

## Writing rules

- **One idea per sentence.** Aim for under 20 words. Split a sentence at "which", "because" or a semicolon.
- **Common words over precise ones** when the meaning survives: "use" over "leverage", "check" over
  "validate", "stops" over "terminates". Keep a precise word when the meaning would change without it.
- **Define a term once, where it first appears,** in plain words and in passing: "the gate ledger (a log of
  which checks passed for which version of the code)". Don't define it again later in the same answer.
- **Concrete before abstract.** Give the example first, then the rule it shows.
- **Active voice, named actors.** "`finish` deletes the branch", not "the branch is deleted".
- **No copying.** Never reuse the original answer's sentences. If the reader could have understood those
  sentences, they would not have asked.
- **Keep the facts identical.** Simpler wording never changes a number, a name, a path or a conclusion. If
  something can't be simplified without becoming wrong, keep it exact and explain it next to it.

## Visuals

Draw when a picture shows something the text can't: a **flow** (what happens, in what order), a
**structure** (what contains or calls what), a **before/after**, or a **comparison** of options. Don't draw
a single fact, a definition or a list. A diagram that just repeats the text costs reading time and adds
nothing.

Pick the richest medium the session has:

1. **An inline visual tool**, if one is loaded or available to load (a widget or diagram renderer in the
   desktop app). Follow that tool's own setup instructions before the first call.
2. **A ```` ```mermaid ```` block,** for flows and structures in any Markdown-rendering client.
3. **An ASCII box diagram or a table** in a plain terminal. A table is usually the clearest way to show a
   comparison.

Keep a visual small: about 3 to 8 nodes. If it needs more, the explanation is trying to cover too much.
Narrow the subject instead.

## Words used

End the answer with a short **Words used** list: each term the answer defined, one line each. Leave it out
when the answer defined nothing. The list is for looking back, so the definitions there match the ones in
the text.

## Ending

The last line offers the other levels in one sentence, naming only the ones that would actually change the
answer, for example: "Say `simpler` for no jargon, or `deeper` for how it works inside." Never end with a
question the agent could answer itself.
