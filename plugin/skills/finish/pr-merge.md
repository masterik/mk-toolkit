# finish: the PR merge path

For `pr=<url>`, `pr_state=OPEN`, not draft: the branch is merged on GitHub, never with a local `git merge`.
Items 1–2 run before `finish`'s plan (step 4); items 3–5 only after its go-ahead.

**Stacked?** Probe once (`../_shared/references/stacked-prs.md`, "Detect"): only `null`, absent or a 404 means
not stacked; any other failure stops there. A stack changes item 3 (**`gh stack merge`, never `gh pr merge`**) and the plan: every unmerged PR
below this one merges with it.

1. **Push what step 1 committed** — `gh pr merge` merges what is on GitHub. `pushed=no` →
   `git push -u <remote> <feature-branch>`; else `git push <remote> <feature-branch>`. Never a bare `git push`
   (`pushed=yes` only proves *some* upstream).
2. **Pick the method.** `gh repo view <remote_repo> --json mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed`
   (repo argument only when `remote_repo=` is not `none`) says what the remote accepts; `merge` →
   `--merge`, `squash` → `--squash`, `rebase` → `--rebase`. In order:
   - **The style is allowed** → use it, **do not ask**, and name where it came from.
   - **The repo's style (from `merge.md`) is not allowed** → **report it, never substitute**, even when only
     one method is allowed: what `merge.md` says, what the remote allows; the user picks.
   - **The default style is not allowed** → exactly one other allowed: use it; several: ask which. Either
     way without reporting a mismatch.

   Never `--admin` unless the user explicitly asks. **Now show `finish`'s plan with this method and get the
   go-ahead.**
3. **Merge.**
   - Not stacked: `gh pr merge <pr-url> --<method> --delete-branch`. `--delete-branch` deletes the **local
     branch too**, and may switch this checkout to `<base>` first. A refusal (failing checks, missing review,
     conflict) → report exactly what it said; never fall back to a local merge.
   - Stacked: `gh stack merge <pr-number> --yes --<method>` (needs `gh stack`; absent → stop with its install
     line, nothing merged). Base requires signed commits → stop before merging and report that the stack
     needs a local `gh stack rebase` + `gh stack push`. A refusal or `enqueued` stops here: report it
     verbatim, delete nothing. The server deletes no branches: item 5 removes them, remote and local,
     **for every PR the merge carried**, each after its own `state=MERGED` check
     (`git push <remote> --delete <branch>`). Never offer auto-merge.
4. **Sync the local base** — from `primary=` if the base is checked out there:

   ```bash
   git switch <base>
   git pull --ff-only <remote> <base>
   ```

5. **Confirm it merged, then remove the local vestiges.** Success can mean "enqueued" or "auto-merge enabled":

   ```bash
   gh pr view <pr-url> --json state,mergeStateStatus
   ```

   Only `state=MERGED` proceeds. Anything else → stop, say the PR hasn't merged yet, leave branch and
   worktree in place; re-run this item once it lands.

   Then remove by `cleanup_path`, with a **forced** delete (a squash/rebase merge leaves the local branch
   looking unmerged; `state=MERGED` is the stated reason). Check first that the branch still exists — item 3
   may have deleted it; "already gone" is not a failure.
   - `git-worktree`, worktrunk available: `wt remove <feature-branch>`; add `-D` if it reports the branch
     unmerged.
   - `exit-worktree`: the **ExitWorktree** tool, `action: "remove"`, `discard_changes: true`. Never
     `git worktree remove` it from inside.
   - `git-worktree`, no worktrunk, each call as `git -C "<primary>"` (the removal may delete the cwd):
     `worktree remove "<toplevel>"`, then `branch -D <feature-branch>` if `branch --list <feature-branch>`
     still shows it, then `worktree prune`.
   - `none`: `git branch -D <feature-branch>` if it still exists.

   **Stacked — every PR the merge carried**, this one included, one at a time, each git call as
   `git -C "<primary>"`: `gh pr view <n> --json state,headRefName,headRefOid` (`--repo <remote_repo>` only
   when `remote_repo=` is not `none`). Only `state=MERGED`, and only if a local `<headRefName>` is absent or
   its tip is `headRefOid` or an ancestor of it (`merge-base --is-ancestor <headRefName> <headRefOid>`):
   `push <remote> --delete <headRefName>` only if `ls-remote <remote> refs/heads/<headRefName>` prints
   `headRefOid` or nothing, then `branch -D <headRefName>` if it still exists and no other worktree has
   it checked out. A failed check (local or remote tip past `headRefOid`) → it holds commits the merge never
   carried: leave both refs, named under "left behind". Not merged → delete nothing of it; say so.

   **Stacked:** the PRs above were rebased and retargeted on GitHub; their local branches are behind. With a
   local stack tracked, ask before `gh stack sync` (it force-pushes), then run it from the surviving root;
   otherwise tell the user those branches need a fetch and rebase. Leave them and their worktrees; name them
   under "left behind".
