# Which remote

The one place the remote rule is written. `pr`, `finish` and `cleanup` push, fetch and ask GitHub through a
remote; none hardcodes `origin`, and none re-derives the answer. `mkit facts` resolves it, from the repo
config's `[remote] name` pin first, so it is one implementation (`internal/core/gitrepo`, `ResolveRemote`).

## What `mkit facts` says

| Key | Meaning |
| --- | --- |
| `remote=` | the resolved remote, or `none` |
| `remote_source=` | `pinned` · `chosen` (`--remote` on this call) · `origin` (nothing pinned, `origin` exists) · `only` (the lone remote) · `first` (several, no `origin`: git's alphabetical first — a guess) · `none` |
| `remote_repo=` | the GitHub `owner/repo` of `remote=`, or `none` (not a GitHub remote) |
| `remotes=` | every configured remote, comma-joined, or `none` |
| `remote_choice=` | `ask` when there are several remotes and none is pinned or chosen; otherwise `no` |

`pr=` and `default_branch=` are computed **for `remote=`**. A trailing `notes:` entry naming `remote_source` is
a pin that went nowhere (a remote this clone lacks, a name mkit refused): report that sentence verbatim, then
carry on with the fallback `remote=` already shows.

## The rule

1. **`remote_choice=no`** — use `remote=`. Do not ask. When `remote_source=pinned`, say so where the remote
   is shown ("pinned in `.mkit/config.toml`").
2. **`remote_choice=ask`** — ask once, before the first push or remote call: *which remote?* List `remotes=`
   with their URLs (`git remote -v`), `remote=` first and marked as the default (it is `origin` when `origin`
   exists).
   - **Then re-run the facts for the answer**, even when it is the default: the first run's `pr=`,
     `default_branch=` and `remote_repo=` describe the fallback remote. Repeat the skill's own `mkit facts`
     call with `--no-run --remote <answer>` added (and `--gh` / `--base` as before). `--no-run` keeps the
     original run directory; a name that is not a configured remote exits `2`. `cleanup` passes the same
     `--remote <answer>` to `mkit branch status`.
   - **Never run `mkit init` mid-run.** It writes `.mkit/config.toml`, an un-ignored file: in the feature
     worktree it leaves the tree dirty, blocks the worktree removal at the end of `finish`, and never reaches a
     PR. Offer the pin in the **final report** instead, as a command for the user to run and commit on their
     own: `mkit init --force --remote <name>`. After a pin the question never comes back.
   - The user cannot be asked (they said "just do it", or this is unattended) → use `remote=` and name it in
     the report as a fallback, never as a pinned answer.
3. **`remote=none`** — nothing to push to. `pr` stops; `finish` takes the local path; `cleanup` skips its
   network calls.

## Using it

- Push: `git push -u <remote> <branch>`; delete: `git push <remote> --delete <branch>`; sync:
  `git fetch <remote>`.
- **Name the repo to `gh` — always, when `remote_repo=` is not `none`.** Left to itself `gh` uses a set
  default (`gh repo set-default`) or its own remote ranking, and with several remotes it can pick another repo
  *without* refusing — a PR opened or looked up in the wrong place, silently. So `gh pr create --repo
  <remote_repo> --head <branch>` and `gh repo view <remote_repo> --json …`. A command that takes a PR URL
  needs neither. `mkit facts` and `mkit branch status` already do this for their own `gh` calls.
