# Skill invariants: checklist and rationale

Skills hold instructions; the *why* lives here (#47). Each row is one rule the skills must keep, and the
line that still enforces it, as `file` · "a phrase on that line" (paths under `plugin/skills/`). A row
whose phrase no longer matches is a rule that was cut or moved: fix the row or restore the rule — never
drop it silently. Check: every phrase greps (`grep -F`) in its file.

Size budget (#47): each `SKILL.md` ≤ ~8 KB; a happy-path `commit`/`pr`/`finish` run loads ≤ ~6K tokens of
skill text (SKILL.md plus the references it reads). Links sit at the step that uses them; no up-front
reference lists.

## From AGENTS.md

| # | invariant | enforced at |
| --- | --- | --- |
| A1 | Steps are entry-capable: any order, any subset | `_shared/references/workflow-contract.md` · "it runs first, in" |
| A2 | Discover, derive a thin version, never demand | `_shared/references/workflow-contract.md` · "Discover, then derive. Never demand." |
| A3 | Never send the user to another step; never run a downstream one | `_shared/references/workflow-contract.md` · "No step sends the user to another step." |
| A4 | Name what was assumed | `_shared/references/workflow-contract.md` · "Say what you assumed" |
| A5 | A missing source is one fewer input, never a stop | `_shared/references/workflow-contract.md` · "A missing source is one fewer input, never a" |
| A6 | Config is an input, never a permission; missing file never stops or is mentioned | `_shared/references/repo-settings.md` · "**Never stop**, never mention the missing file" |
| A7 | A settings file that does not say → fall back and say so | `_shared/references/repo-settings.md` · "fall back to rule 2 for that setting and **say so in one line**" |
| A8 | Spec store is `issue-tracker.md`, read never written | `_shared/references/repo-settings.md` · "mkit reads it, never writes it" |
| A9 | `setup` never edits mattpocock's files | `setup/SKILL.md` · "**Never edits mattpocock's files**" |
| A10 | No skill needs the binary but `sandbox-audit` | `_shared/references/workflow-contract.md` · "No skill needs the `mkit` binary but `sandbox-audit`." |
| A11 | `sandbox-audit`: absent ≡ too old → stop with a `brew` remedy | `sandbox-audit/SKILL.md` · "absent and too old are the same answer" |
| A12 | Scripts run as `bash <path>`, resolved against the SKILL.md directory | `_shared/references/output-discipline.md` · "against the directory of the `SKILL.md` you" |
| A13 | Shared links stay relative | `_shared/README.md` · "Keep the relative links intact" |
| A14 | Both diff stats are read | `commit/SKILL.md` · "Read **both** `unstaged_stat` and `staged_stat`" |
| A15 | Unresolvable `--base`/`--range` exits 1 | `_shared/references/output-discipline.md` · "an unresolvable `--base`/`--range`" |
| A16 | `notes:` carries causes; report it as given | `_shared/references/output-discipline.md` · "a cause and its remedy" |
| A17 | Never hardcode `origin`; one resolver | `_shared/references/remote.md` · "never hardcode" |
| A18 | `remote_choice=ask` → ask once, re-run facts with `--remote`, without `--run` | `_shared/references/remote.md` · "**Then re-run the facts for the answer**" |
| A19 | A `--remote` the clone lacks falls back, reported verbatim | `_shared/references/remote.md` · "report it verbatim and carry on" |
| A20 | `gh` gets `--repo <remote_repo>` unless `none` | `_shared/references/remote.md` · "**Name the repo to `gh` whenever `remote_repo=` is not `none`**" |
| A21 | Nothing writes into the work tree; run files in `<run-dir>` under `$TMPDIR` | `_shared/references/output-discipline.md` · "no skill writes its own files into the work tree" |
| A22 | No `$RUN_DIR`; carry the literal; never re-run `--run` | `_shared/references/output-discipline.md` · "**There is no `$RUN_DIR`.**" |
| A23 | `mktemp` always gets a template | `_shared/references/output-discipline.md` · "always gets a template" |
| A24 | Never `/tmp`, never `~/.claude` | `_shared/references/output-discipline.md` · "never the literal `/tmp`, never `~/.claude/…`" |
| A25 | `user_dir_writable=no` → remedy from `notes:`, never allowlist `~/.claude` | `_shared/references/output-discipline.md` · "`user_dir_writable=no` → report it" |
| A26 | Gate commands discovered, never hardcoded | `_shared/references/quality-gate.md` · "**Nothing is hardcoded**" |
| A27 | Scopes discovered; a file's list is never re-derived | `commit/SKILL.md` · "never re-derive scopes from" |
| A28 | Reviewers discovered, never hardcoded | `pr/SKILL.md` · "Never hardcode handles" |
| A29 | Default branch always protected; kept names add, never substitute | `cleanup/SKILL.md` · "the default branch (always, whatever" |
| A30 | A kept branch is never a merge target | `cleanup/SKILL.md` · "a kept branch is never a merge target" |
| A31 | `clean=error` is not clean | `cleanup/SKILL.md` · "unreadable is not clean" |
| A32 | `merged-pr` is verified against the branch's content | `cleanup/SKILL.md` · "verified against this branch's content" |
| A33 | Parsed git output uses the absolute `<git>` | `_shared/references/git-safety.md` · "**absolute path of the real git binary**" |
| A34 | Inline shell: plain calls, no `$(…)`/pipes/heredocs/`&&` | `_shared/references/git-safety.md` · "no `$(…)`, no pipes, no heredocs, no `&&` chains" |
| A35 | Never assemble a program at runtime | `_shared/references/git-safety.md` · "Never assemble a program at runtime" |
| A36 | `sandbox-audit` never writes a settings file | `sandbox-audit/SKILL.md` · "**Never writes a settings file**" |
| A37 | `sandbox-audit`'s query file keeps its name (not re-counted) | `sandbox-audit/SKILL.md` · "Keep the `sandbox-audit.` name" |
| A38 | `explain` runs nothing; `recap` writes nothing | `recap/SKILL.md` · "No `--run`: recap writes nothing." |
| A39 | No AI attribution anywhere | `_shared/references/git-safety.md` · "add AI/Claude co-authorship" |

## Edge cases

| # | rule | enforced at |
| --- | --- | --- |
| C1 | A staged diff is read file by file, never truncated | `commit/SKILL.md` · "**file by file, skipping none, never truncated**" |
| C2 | No interactive `add -p`; no `-F -` heredoc | `commit/SKILL.md` · "never `-v`, never `-F -` with a heredoc" |
| C3 | Patch and message files are allocated fresh, never named after the source | `commit/patch-staging.md` · "never named after the source file" |
| C4 | Hunks sharing lines are not split; say so | `commit/patch-staging.md` · "**Hunks that share lines are not split.**" |
| C5 | Delegated plan: coverage contract, checked by set comparison | `commit/SKILL.md` · "**Verify the plan by set comparison**" |
| C6 | A gap goes back to the same subagent | `_shared/references/agent-delegation.md` · "**On a gap, send the missing paths back to the same subagent**" |
| C7 | Hunk-level judgement is never delegated (>~3 mixed files) | `commit/SKILL.md` · "**more than ~3 files are mixed**" |
| C8 | Subject length never derived from history | `commit/SKILL.md` · "Subject ~72 chars — never derived" |
| C9 | `pr=` is tested for a URL, not non-emptiness | `pr/SKILL.md` · "Test for a URL, not non-emptiness." |
| C10 | Re-count commits after an inline commit | `pr/SKILL.md` · "If step 1 committed, re-count" |
| C11 | Never load the branch diff to write prose | `_shared/references/output-discipline.md` · "**Never load a full branch diff to write prose.**" |
| C12 | `merge.md` reviewers → skip `CODEOWNERS` | `pr/SKILL.md` · "skip `CODEOWNERS` (no cross-check)" |
| C13 | Gate reuse: one rule, never reported as `gate=ok` | `_shared/references/quality-gate.md` · "### Reusing a pass — one rule" |
| C14 | A `check` recipe covering less falls through | `_shared/references/quality-gate.md` · "**Row 2 — read the recipe first.**" |
| C15 | Unparseable manifest = undetected, never a pass | `_shared/references/quality-gate.md` · "the gate is **undetected**, never a pass" |
| C16 | Diagnosis subagent is read-only, ≤15 lines | `_shared/references/quality-gate.md` · "**returns at most 15 lines**" |
| C17 | Never merge with uncommitted changes | `finish/SKILL.md` · "**Never merge with uncommitted changes.**" |
| C18 | Remote resolved before routing on `pr=` | `finish/SKILL.md` · "**before routing**" |
| C19 | Default merge style gives way silently; never overrides worktrunk | `finish/SKILL.md` · "It is not a repo statement" |
| C20 | Unknown style in `merge.md` → one line, then default | `finish/SKILL.md` · "A style `finish` doesn't know" |
| C21 | Repo style refused by the remote → report, never substitute | `finish/pr-merge.md` · "**report it, never substitute**" |
| C22 | Push explicitly to `<remote>`, never a bare `git push` | `finish/pr-merge.md` · "Never a bare `git push`" |
| C23 | No `--admin` unasked | `finish/pr-merge.md` · "Never `--admin` unless the user explicitly asks." |
| C24 | PR refusal → never a local-merge fallback | `finish/pr-merge.md` · "never fall back to a local merge" |
| C25 | Only `state=MERGED` licenses deletion | `finish/pr-merge.md` · "Only `state=MERGED` proceeds." |
| C26 | `--delete-branch` may already have removed the local branch | `finish/pr-merge.md` · "\"already gone\" is not a failure" |
| C27 | Harness worktree: ExitWorktree, `discard_changes: true` | `finish/pr-merge.md` · "`discard_changes: true`" |
| C28 | Stack: `gh stack merge`, never `gh pr merge` | `finish/pr-merge.md` · "**`gh stack merge`, never `gh pr merge`**" |
| C29 | Stack: signed commits required → stop | `finish/pr-merge.md` · "Base requires signed commits" |
| C30 | Stack: delete every merged layer after its own check | `finish/pr-merge.md` · "**for every PR the merge carried**" |
| C31 | Stack: `gh stack sync` force-pushes → ask first | `finish/pr-merge.md` · "ask before `gh stack sync`" |
| C32 | `gh stack` absent → stop with its install line | `_shared/references/stacked-prs.md` · "never fall back to" |
| C33 | Verify against the surviving root after a worktree removal | `finish/SKILL.md` · "`<surviving-root>` once (`primary=`)" |
| C34 | `wt merge` flags only turn defaults off; never `--config-set` | `_shared/references/worktree.md` · "`--config-set` accepts unknown keys silently" |
| C35 | Never `git worktree remove` the harness's own worktree | `_shared/references/worktree.md` · "never `git worktree remove` it from inside itself" |
| C36 | Review: never read a diff into the main session | `review/SKILL.md` · "**Never read a diff into this session.**" |
| C37 | Review: an empty findings file means "looked, found nothing" | `review/reviewers.md` · "**Nothing at or above the bar → write the file empty and say so.**" |
| C38 | Codex: `--wait --write`, launched inside the reviewed tree | `review/reviewers.md` · "**`--wait --write`, appended to the task text.**" |
| C39 | Silent reviewer: ask once, ~10 min, then missing | `review/reviewers.md` · "Still no file after ~10 minutes" |
| C40 | Expected sources = launched; never fake an empty file | `review/SKILL.md` · "expected = reviewers **launched**" |
| C41 | Every `F` id needs exactly one verdict | `review/SKILL.md` · "`F` id needs exactly one verdict**" |
| C42 | Quick is by design, never described as degraded | `review/closing.md` · "never a partial run" |
| C43 | Second round: narrow, never sees the last round's findings | `review/SKILL.md` · "**Optional second round**" |
| C44 | Review never commits unasked | `review/SKILL.md` · "Do not commit unless asked." |
| C45 | Cleanup: every `-D` states its proof | `cleanup/SKILL.md` · "stating the proof as you run it" |
| C46 | Cleanup: `origin=claude-code` worktree → ask | `cleanup/SKILL.md` · "a worktree with `origin=claude-code`" |
| C47 | Cleanup: a missing remote ref is not divergence | `cleanup/SKILL.md` · "never report \"couldn't find remote ref\" as divergence" |
| C48 | Audit: unreadable transcripts → counts are a lower bound | `sandbox-audit/SKILL.md` · "**lower bound**" |
| C49 | Audit: exclude is proposed, never recommended | `sandbox-audit/SKILL.md` · "**proposed, never recommended**" |
| C50 | Audit: never `allowWrite` on credentials, rc files, `~/.claude` | `sandbox-audit/SKILL.md` · "Never propose `allowWrite` on a credentials directory" |
| C51 | Recap: `pushed=` only says an upstream is set | `recap/SKILL.md` · "**`pushed=` only says whether an upstream is set.**" |
| C52 | Recap: a review covers only the commits it saw | `recap/SKILL.md` · "**A PR review is current only for the commits it saw.**" |
| C53 | No answer to a question = do nothing | `_shared/references/summary-format.md` · "**No answer, or an unclear one, is \"do nothing\"**" |

## Why (moved out of the skills)

- **Pinned `<git>` (A33).** A `PreToolUse` hook (`rtk`) rewrites `git status`/`git diff` into a compacted
  summary that reads exactly like the tree; approving a commit off it is the failure. An absolute git path
  is not rewritten, and the worktree-isolation guard refuses wrappers (`rtk git …`) outright. Quotes stay
  because `~/My Projects` splits `-C`.
- **Shell shape (A34, A35, C2).** The isolation guard refuses `python3 - <<'PY'`, `node -e`, `bash -c` over
  generated text ("cannot be shown not to be git"); a permission gate was measured denying `git commit -F -`
  with a heredoc body. Plain single calls also run under PowerShell (#58). An improvised splitter once nearly
  got committed into a user's repo.
- **Both stats (A14).** A bare `git diff --shortstat` is empty on a fully staged tree, which reads as clean.
- **Fresh temp files (C3, A23).** A reused patch name staged another file's hunks; a reused message file
  committed the previous message. `mktemp` with no template resolves the Darwin per-user temp dir and
  ignores `$TMPDIR`.
- **Plan coverage (C5–C7).** A delegated commit plan once dropped a changed file; re-reading the diff in the
  main session paid for the delegation twice. Per-hunk assignment does not survive a 20-line budget.
- **`pr=` sentinels (C9).** Every sentinel is non-empty; a non-empty test showed `none` as a PR. A failed
  lookup is `gh-error`, never `none`: `none` is what `pr` opens a PR on, and what `finish` merges locally on.
- **Explicit push (C22).** `pushed=yes` only proves *some* upstream, possibly on another remote.
- **`state=MERGED` (C25).** `gh pr merge` success can mean "enqueued" or "auto-merge enabled"; deleting the
  only copy of a branch on that signal risks losing it. A squash/rebase merge leaves the local branch looking
  unmerged, so the confirmed state is the stated reason for `-D`.
- **Report, never substitute (C21).** A wrong setting is worth one interruption; a silent substitution
  repeats on every run.
- **Unknown style (C20) and unknown review mode.** Falling back silently leaves the only symptom a merge
  shape or roster the team thought they had changed.
- **worktrunk (C34).** `wt merge` flags are negative only (checked against worktrunk 0.77), so `squash` with
  a user's `merge.squash = false` cannot be forced; `wt --config-set merge.bogus=true` exits 0, a remedy that
  only looks applied.
- **Stacks (C28–C32).** GitHub refuses the synchronous merge for a stacked PR; merging layer *k* merges every
  layer below atomically; the server-side rebase is unsigned. The REST async endpoint
  (`PUT …/pulls/{n}/merge-async`, states `pending`/`merged`/`enqueued`/`failed`) is what `gh stack` wraps.
- **Surviving root (C33).** The feature worktree directory is gone after removal; a call pinned to it, or a
  cwd inside it, fails instead of verifying.
- **Review independence (C36–C44).** A full review is tens of thousands of tokens; reviewers that see each
  other anchor; a reviewer that fixes anchors on itself. An absent findings file and an empty one mean
  different things downstream. `codex:codex-rescue` must not poll a job it backgrounded and, without
  `--wait`, ends its turn while Codex runs; without `--write` Codex runs read-only and the write fails with
  "session filesystem policy prohibits writes". A `docs` pass over fresh prose finds defects in the last
  round's sentences forever.
- **Cleanup `-D` (C45).** `-d` checks against whatever is checked out — a `merged` branch may be an
  ancestor of `develop` while you stand on `main`; a `merged-pr` branch is by definition not a local ancestor.
- **Protected paths (A24, C50).** `~/.claude` is a protected region: an allowlist entry there looked right and
  did nothing.
- **Bundle design.** Nothing project-specific is hardcoded; compose `git`/`gh`/`wt`, never re-encode git
  logic; a missing review tool's lenses go to a Claude subagent; the main session holds decisions, evidence
  lives in the run directory; a script only for a mechanical invariant.
