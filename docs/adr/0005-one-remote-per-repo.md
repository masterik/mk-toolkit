# ADR 0005 — One remote per repo, pinned by name

**Status:** accepted · **Date:** 2026-10-15

## Context

`pr` and `finish` hardcoded `origin`; the binary took git's first remote, which is alphabetical, so a
`fork` remote beat `origin`. A checkout with several remotes needs a choice that is made per run and
can be pinned, so it stops being asked.

## Decision

**1. One remote, for push and for PRs.** `[remote] name` in `.mkit/config.toml` names the remote a
branch is pushed to *and* whose GitHub repo `gh` is told about. Not a push remote plus a PR target.

**2. Resolution lives in `gitrepo.ResolveRemote`, in one order:** a pin this clone has → `origin` →
the lone remote → git's first (a guess). `facts`, `branchstatus`, `profile` and `DefaultBranch` all
call it. A pin this clone lacks falls back and is reported; config is an input, never a permission.

**3. Asked only when it is a guess.** Several remotes and no pin (or a pin this clone lacks) →
`remote_choice=ask`; the skill asks once, defaulting to `origin` when it exists, else git's first, and
offers `mkit init --force --remote <name>` in its report. The answer is per run; only the pin persists.
`mkit init` pre-selects `origin` as the form default where several exist and `origin` is one of them;
with no `origin` nothing is pre-selected, so a guess is never pinned by accepting the form.

**4. `cleanup` fetches the resolved remote only.** A branch tracking another remote keeps a stale
ref and reads as `tracking`, not `gone` — the safe direction. It asks like `pr` and `finish` do.

**5. An answer is passed, never pinned mid-run.** `mkit facts <skill> --remote <name>` and `mkit branch status
--remote <name>` replace the pin for one call (`remote_source=chosen`); a name that is not configured is
a usage error, not a fallback. A skill re-runs its facts with it after asking, because `pr=` and
`default_branch=` are computed for the resolved remote. `mkit init` writes an un-ignored file, so a skill
never runs it mid-run — it offers the command in its final report.

**6. `gh` is always told the repo.** `facts` reports `remote_repo=` (the GitHub `owner/repo`, one
producer: `gitrepo.GitHubRepo`) and `pr view` / `pr list` pass it with `--repo`. Left alone `gh` uses a set
default or its own remote ranking and can pick another repo without refusing. A non-GitHub remote keeps
`gh`'s own pick.

## Consequences

- A fork workflow (push to `origin`, PR against `upstream`) is **not modelled**. `gh --repo` covers a
  one-off; a real fix is a second key (push remote vs PR target), a schema addition made when a repo
  needs it. Do not read the single key as an oversight.
- Once `[remote] name` is in committed configs, splitting it later means migrating files already
  written — the reason this is recorded.
