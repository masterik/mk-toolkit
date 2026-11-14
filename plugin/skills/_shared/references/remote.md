# Which remote

The one place the remote rule is written. `pr`, `finish` and `cleanup` never hardcode `origin` and never
re-derive the answer: `facts.sh` resolves it (`--remote <name>` → `origin` → the lone remote → git's first).

| Key | Meaning |
| --- | --- |
| `remote=` | the resolved remote, or `none` |
| `remote_source=` | `chosen` (`--remote`) · `origin` · `only` · `first` (several, no `origin`: a guess) · `none` |
| `remote_repo=` | the GitHub `owner/repo` of `remote=`, or `none` |
| `remotes=` | every configured remote, comma-joined, or `none` |
| `remote_choice=` | `ask` when several remotes and none chosen; else `no` |

`pr=` and `default_branch=` are computed **for `remote=`**. A `notes:` entry naming `remote_source` is a
`--remote` this checkout lacks: report it verbatim and carry on with the fallback `remote=`.

## The rule

0. **The repo names one.** `docs/agents/remote.md` exists and names a remote → re-run the skill's own
   `facts.sh` call with `--remote <name>` before anything uses the remote; say where the name came from.
1. **`remote_choice=no`** — use `remote=`. Do not ask.
2. **`remote_choice=ask`** — ask once, before the first push or remote call, listing `remotes=` with their
   URLs (`git remote -v`), `remote=` first as the default. **Then re-run the facts for the answer**, even
   the default: repeat the skill's `facts.sh` call **without `--run`**, with `--remote <answer>` (and `--gh`
   / `--base` as before). `cleanup` passes the same `--remote` to `branch-status.sh`. Unattended or "just do
   it" → use `remote=` and call it a fallback in the report.
3. **`remote=none`** — `pr` stops; `finish` takes the local path; `cleanup` skips its network calls.

## Using it

- `git push -u <remote> <branch>` · `git push <remote> --delete <branch>` · `git fetch <remote>`.
- **Name the repo to `gh` whenever `remote_repo=` is not `none`**: `gh pr create --repo <remote_repo> --head
  <branch>`, `gh repo view <remote_repo> --json …`. Left alone, `gh` can silently pick another repo. A command
  taking a PR URL needs neither; with `remote_repo=none`, leave `--repo` off.
