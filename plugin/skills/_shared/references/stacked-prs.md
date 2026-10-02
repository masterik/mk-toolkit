# Stacked PRs (GitHub native, public preview since 2026-07-30)

A stack is an ordered chain of PRs in one repo, each targeting the branch below it. GitHub owns the
stack object, retargeting and rebasing; `pr` and `finish` only detect it and route around the calls
that break on it. No stack detected → the skill runs exactly as before. A stacks API that 404s
(feature not enabled for the repo) is the same answer.

**Stacked work depends on the `gh stack` extension** (`gh extension install github/gh-stack`;
`mkit doctor` reports it). Probe with `gh extension list`. Stack detected but the extension absent →
**stop** and give that install line; never fall back to `gh pr merge` or hand-built stack calls.
Ordinary PRs never need it. The REST endpoints below are what the extension calls — kept for reading
a failure and for the detection probe, not as a second path.

Sources: GitHub Docs — "Merging stacked pull requests", "REST API endpoints for stacked pull
requests", "REST API endpoints for pull requests" (async merge). Send `-H 'X-GitHub-Api-Version: 2026-03-10'`
on the calls below.

## Detect

A PR is stacked when its REST resource carries a non-null `stack` object (number, size, position, base):

```bash
gh api -H 'X-GitHub-Api-Version: 2026-03-10' repos/{owner}/{repo}/pulls/<n> --jq '.stack'
```

`null`/absent → not stacked. A PR whose `base` is another feature branch rather than the default branch is
a **stack candidate** even without the object (see `pr`).

## Why the normal merge path breaks

- `gh pr merge` and `PUT …/pulls/{n}/merge` are the legacy **synchronous** endpoints; GitHub refuses
  them for a stacked PR. Use the async endpoint below.
- Merging PR *k* merges (or queues) **every unmerged PR below it too**, atomically: if one cannot merge,
  none do. A mid-stack PR can never merge alone. Say so in the confirmation — the user is approving
  *N* PRs, not one.
- **Auto-merge is unsupported** for stacks. Never `--auto`.
- Branch protection / rulesets are evaluated *when the merge runs*, not at submission; a failure shows
  up while polling. `bypass_rules` is not to be set (the `--admin` rule from `finish` applies).
- After a merge, GitHub rebases the next PR up onto the base and retargets it. The rebase is
  server-side and **unsigned**; a repo requiring signed commits needs `gh stack rebase` + `gh stack push`
  locally instead — report it, do not work around it.

## Merge

```bash
gh stack merge <pr-number> --yes --<merge|squash|rebase>    # everything up to and including that PR
```

Atomic, merge-queue aware, polls for you; exit 4 = GitHub API failure, report its message verbatim.
Without `--yes` it opens a wizard — always pass it, the skill has already confirmed. The mechanics it
wraps:

### Async endpoint (reference)

```bash
# 1. submit — 202 + a UUID; 200 = already merged / in queue; 409 = a merge request is already enqueued
gh api -X PUT -H 'X-GitHub-Api-Version: 2026-03-10' repos/{owner}/{repo}/pulls/<n>/merge-async \
  -f sha=<head-sha> -f merge_method=<merge|squash|rebase>      # merge_action defaults to `default`

# 2. poll the UUID from the 202 body until it leaves `pending` (the UUID expires after 24 h)
gh api -H 'X-GitHub-Api-Version: 2026-03-10' repos/{owner}/{repo}/pulls/<n>/merge-async/<uuid>
```

`sha` is the PR's current head (`gh pr view <n> --json headRefOid`) — a stale value is a refusal, which
is the point. `merge_method` applies to direct merges only; with a merge queue the queue's own method
wins. Result `state`:

| state      | meaning                         | do                                                          |
| ---------- | ------------------------------- | ----------------------------------------------------------- |
| `pending`  | running                         | poll again (few seconds apart, bounded — ~2 min, then stop and say so) |
| `merged`   | landed (includes commit OID)    | continue to the confirm step                                |
| `enqueued` | in the merge queue              | **not merged** — leave branch/worktree, say so              |
| `failed`   | carries a message               | report it verbatim; never fall back to a local merge        |

Whatever the state, still confirm with `gh pr view <n> --json state` before deleting anything.

## Sync

After the merge, the layers above are rebased and retargeted **on GitHub**; local copies are behind.

```bash
gh stack sync          # fetch, ff trunk, cascade-rebase, push (--force-with-lease --atomic), relink
```

Run it when the user has a local stack tracked (`gh stack view` exits 2 when there is none — then tell
them the upper branches need `git fetch` + a rebase onto the new base). Exit 3 = rebase conflict (state is
restored; resolve with `gh stack rebase`). Sync never opens PRs.

## Create (for `pr`)

`gh stack submit` pushes the branches and opens/links the PRs (`--auto` creates them as drafts unless
`--open`). To link PRs that already exist: `gh stack link`. A merged or queued PR cannot be unstacked.
Same-repo branches only.
