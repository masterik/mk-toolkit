# Severity bar & reporting rules

Used by `review` and any skill asking a tool or subagent for findings. "The severity bar" means this
file. Hand it to every reviewer verbatim — one bar for all sources is what makes their findings mergeable.

## The bar

**Severity is what goes wrong when the code runs** — not how wrong a statement is, and not how much effort it
took to find.

- **critical** — data loss or corruption, a security hole, or a crash on a path users reach.
- **major** — wrong runtime behavior, or a broken contract a caller executes against.
- **minor** — a real defect with contained impact.

**Anything that cannot be placed on that bar is not a finding.** Style preferences, hypotheticals and
"consider maybe" notes are noise. Silence beats a finding the reader has to disprove.

### Prose is minor

A defect in prose — comment, doc comment, README, design note — executes nothing, so it is **minor** however
wrong the claim is. Report it; never promote it for being badly wrong.

**One exception: a document a machine or agent executes against as a contract** — rate that by what its
consumer does wrong. Human-facing prose is never that, however prominent. (In a repo whose product *is*
agent-executed markdown — skills, prompts, schema descriptions — the exception is the normal case.)

## Tag every finding `[surface, severity]`

One bracket before the title. Surface says *where the risk is*, severity *how bad*:

| surface | is |
| --- | --- |
| `code` | executable logic |
| `comments` | a comment or doc comment inside a source file |
| `docs` | a project document — README, design note, ADR, skill/prompt file |
| `tests` | test code |
| `config` / `build` | configuration, CI, build scripts — or another lowercase word when none fits |

Derive the surface from the path **and what the finding argues**: a `.go` path whose finding is about a stale
doc comment is `comments`, not `code`.

Carry tags into the counts. "3 findings" says nothing; "2 `[code, major]`, 1 `[docs, minor]`" does.

## The reviewer contract

Every reviewer gets these, whichever tool it is:

- **Read-only.** Read files, run read-only commands (`git diff`, `git log`, `rg`). Do not modify, delete,
  move, stage or commit anything, and do not write through a shell redirect. Reviewers report; the caller fixes.
- **Do not run the tests, build or linter** — they ran before the review and passed.
- **Point at a specific file and line.** A finding with no location cannot be verified.
- **State the failure concretely**: the input or state, and what goes wrong because of it.
- **Report the confidence you actually have**, not the confidence that keeps the finding alive.
- **Say when a problem is pre-existing** rather than introduced by the change.
- **Report one problem once**, naming every lens that raised it — never twice under two lenses.
- **Independence.** A reviewer never sees another's findings and must not guess at them. Two sources
  converging is the signal a multi-source review exists to produce — never hold a finding back because
  someone else will probably have it.

## What not to report

- a defect on a line this change did not touch — **unless the change makes it reachable**
- anything a linter, compiler or type checker catches; all ran before the review and passed
- a lint or vet rule the code silences deliberately, directive visible
- a missing test, missing doc or general-quality observation the project's own rules do not ask for
- a nitpick a senior engineer reading this diff would not raise
- a behaviour change that is plainly the point of the change

**Pre-existing problems are the exception**: report them, and say so, so the reader can weigh them separately.

## A partial review is not a clean review

Track sources **expected** vs sources that actually **reported**.

- Any source that failed, errored or was unavailable **leads the summary**, above any finding.
- **Never turn a partial review's silence into "the code is clean."** A source that never ran and a source
  that found nothing produce identical silence, and the reader takes silence for approval.
- Name a healthy source that reported **zero** findings — that is evidence, and it reads differently from a
  source that fell over.
- Never claim a tool ran if it did not.
