# Stacked PRs (GitHub native)

A stack is an ordered chain of PRs in one repo, each targeting the branch below it. GitHub owns the stack,
its retargeting and rebasing; `pr` and `finish` only detect it and route around the calls that break on it.
No stack detected → the skill runs as usual.

**Stacked work needs the `gh stack` extension** (`gh extension install github/gh-stack`; probe with
`gh extension list`). Stack detected, extension absent → **stop** with that install line; never fall back to
`gh pr merge` or hand-built API calls. Ordinary PRs never need it.

## Detect

A PR is stacked when its REST resource carries a non-null `stack` object (number, size, position, base):

```bash
gh api -H "X-GitHub-Api-Version: 2026-03-10" repos/<remote_repo>/pulls/<n> --jq .stack
```

`null`, absent, a 404 or a failed probe → not stacked. A PR whose base is another feature branch is a
**stack candidate** even without the object.

## What changes on a stack

- `gh pr merge` is refused for a stacked PR. Merge with `gh stack merge`.
- Merging PR *k* merges **every unmerged PR below it too**, atomically: the user is approving *N* PRs, so the
  confirmation names them all.
- **Auto-merge is unsupported.** Never `--auto`. Never bypass rules (`--admin` stays forbidden).
- After a merge GitHub rebases the next PR up onto the base and retargets it. That rebase is **unsigned**: a
  base requiring signed commits needs a local `gh stack rebase` + `gh stack push` instead — report it, do not
  work around it.

## Merge

```bash
gh stack merge <pr-number> --yes --<merge|squash|rebase>
```

Merges everything up to and including that PR; merge-queue aware; polls for you. Always `--yes` (the skill
already confirmed). Exit 4 = GitHub API failure: report its message verbatim. `enqueued` is **not merged**.
Whatever it says, confirm each PR with `gh pr view <n> --json state` before deleting anything.

## Sync

The layers above are rebased **on GitHub**; local copies are behind. `gh stack sync` fetches, cascade-rebases
and **force-pushes** them, so it needs the user's explicit OK naming the branches. Run it only when a local
stack is tracked (`gh stack view` exits 2 when none — then tell the user the upper branches need a fetch and
rebase). Exit 3 = rebase conflict (state restored; resolve with `gh stack rebase`).

## Create (for `pr`)

Open the layer with `gh pr create --base <lower-branch>` as usual, then `gh stack link` (or `gh stack submit`
when the stack is tracked locally). Same-repo branches only. A merged or queued PR cannot be unstacked.
