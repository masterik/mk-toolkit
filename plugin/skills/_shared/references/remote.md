# Which remote

The one place the remote rule is written. `pr`, `finish` and `cleanup` push, fetch and ask GitHub through a
remote; none hardcodes `origin`, and none re-derives the answer. `facts.sh` resolves it, so it is one
implementation: `--remote <name>` → `origin` → the lone remote → git's first.

## What `facts.sh` says

| Key | Meaning |
| --- | --- |
| `remote=` | the resolved remote, or `none` |
| `remote_source=` | `chosen` (`--remote` on this call) · `origin` (`origin` exists) · `only` (the lone remote) · `first` (several, no `origin`: git's alphabetical first — a guess) · `none` |
| `remote_repo=` | the GitHub `owner/repo` of `remote=`, or `none` (not a GitHub remote) |
| `remotes=` | every configured remote, comma-joined, or `none` |
| `remote_choice=` | `ask` when there are several remotes and none was chosen; otherwise `no` |

`pr=` and `default_branch=` are computed **for `remote=`**. A trailing `notes:` entry naming `remote_source` is a
`--remote` that named no remote of this checkout: report that sentence verbatim, then carry on with the
fallback `remote=` already shows.

## The rule

0. **The repo names one.** If `docs/agents/remote.md` exists and names a remote (`repo-settings.md`), re-run
   the skill's own `facts.sh` call with `--remote <name>` added before anything uses the remote, and say
   where the name came from. A name this clone lacks falls back (the `notes:` line says so) and the rule below applies.
1. **`remote_choice=no`** — use `remote=`. Do not ask.
2. **`remote_choice=ask`** — ask once, before the first push or remote call: *which remote?* List `remotes=`
   with their URLs (`git remote -v`), `remote=` first and marked as the default (it is `origin` when `origin`
   exists).
   - **Then re-run the facts for the answer**, even when it is the default: the first run's `pr=`,
     `default_branch=` and `remote_repo=` describe the fallback remote. Repeat the skill's own `facts.sh`
     call **without `--run`** and with `--remote <answer>` added (and `--gh` / `--base` as before), so the
     original run directory stays the run's. `cleanup` passes the same `--remote <answer>` to
     `branch-status.sh`.
   - The user cannot be asked (they said "just do it", or this is unattended) → use `remote=` and name it in
     the report as a fallback, never as a settled answer.
3. **`remote=none`** — nothing to push to. `pr` stops; `finish` takes the local path; `cleanup` skips its
   network calls.

## Using it

- Push: `git push -u <remote> <branch>`; delete: `git push <remote> --delete <branch>`; sync:
  `git fetch <remote>`.
- **Name the repo to `gh` — always, when `remote_repo=` is not `none`** (when it is `none`, leave `--repo` off
  and `gh` chooses as before). Left to itself `gh` uses a set
  default (`gh repo set-default`) or its own remote ranking, and with several remotes it can pick another repo
  *without* refusing — a PR opened or looked up in the wrong place, silently. So `gh pr create --repo
  <remote_repo> --head <branch>` and `gh repo view <remote_repo> --json …`. A command that takes a PR URL
  needs neither. `facts.sh` and `branch-status.sh` already do this for their own `gh` calls.
