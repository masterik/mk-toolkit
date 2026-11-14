# mkit: backlog

This file holds the ordered work list and the rules every change must keep. Rationale is in
[`concept.md`](concept.md), unscheduled ideas in [`ideas/`](ideas/README.md), and open work in
[Issues](https://github.com/masterik/mk-toolkit/issues).

## Decisions

- **The plugin is the product; the binary is optional.** Every skill but `sandbox-audit` runs on the plugin
  alone (its Markdown plus two bash 3.2 scripts). The binary is macOS + Claude Code power tools: `doctor`,
  `audit sandbox`, `cache prune` ([ADR 0006](adr/0006-standalone-skills-optional-binary.md)).
- **Go, one binary, subcommand tree.** It ships as a single static binary. The stdlib covers the
  workload, and Go has the Charm TUI stack and GoReleaser's Homebrew support.
- **The binary is macOS only.** Builds are darwin amd64 and arm64, and a cask can't install on Linux
  anyway. The plugin's portability (Codex, Copilot, Windows via Git Bash) is the scripts' and the
  prose's job, never the binary's.
- **Two channels, one tag.** Homebrew ships the binary and the marketplace ships the plugin
  ([ADR 0003](adr/0003-two-distribution-channels.md)); each release bumps every agent manifest and
  builds the binary from the same tag. There is no installer: each channel is a one-time human step.
- **Presence, not versions.** Only `sandbox-audit` calls the binary; `unknown command` is its too-old
  signal, and the remedy is `brew upgrade mkit`.

## Invariants

Breaking one is a design error, not a trade-off.

1. **Layering.** `core` returns data and never prints or assumes a terminal. `cli` formats; `tui`
   renders. There is no logic in a Bubble Tea `Update`.
2. **No TUI off a TTY.** If stdout is not a terminal, emit no ANSI and no alt-screen.
3. **Every command is non-interactive-capable and takes `--json`.** Only `sandbox-audit` parses the
   binary's output (`audit sandbox`). The skills' facts and branch classifier are the plugin's bundled
   scripts (#52), whose `key=value` text changes in lockstep with the skills that read it.
4. **Commands and scripts report and run; they never integrate.** No staging, merging, pushing, or
   editing your files. Writes are bounded: `cache prune --apply` deletes stale Claude Code / Codex
   storage, and only with `--apply`; `branch-status.sh` runs `git fetch --prune`.

5. **Judgement stays in Markdown.** The binary owns mechanical invariants only. When the split is
   unclear, the binary reports candidates and the skill chooses.
6. **Skills stay as files.** They are Markdown in this repo, served from the marketplace, and never
   embedded in the binary.
7. **Every workflow step is entry-capable.** A step runs alone, in any order, derives what's missing,
   names its assumptions, and never sends the user to another step
   ([workflow contract](../plugin/skills/_shared/references/workflow-contract.md)).
8. **Write locations, chosen by lifetime:** `$TMPDIR` (per-run files) and `~/.mkit/`. Nothing in the
   work tree, no `info/exclude` write. No state is kept in `~/.claude` or your files
   (`cache prune --apply` only deletes there)
   ([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 5).
9. **Degradation is named, never hit.** A path the sandbox would deny is reported with a working
   remedy, or the command says it is human-run. Each such sentence has one producer.
10. **Config is an input, never a permission.** Everything runs without `docs/agents/*.md`; a missing
    file means discover, default or ask
    ([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 7).
11. **No skill needs the binary but `sandbox-audit`.** A skill that stops because `mkit` is missing or
    too old is a bug ([ADR 0006](adr/0006-standalone-skills-optional-binary.md) decision 1).

## Next

### M8: the `spec` and `implement` skills ([#9](https://github.com/masterik/mk-toolkit/issues/9))

This is the workflow's front half. `brainstorm` can land at any time. Like every skill, none of the
three may need the binary (invariant 11), so the planned `mkit plan` verb is out.

- **Frontier arithmetic** (unblocked slices, blockers, cycles, dangling edges): prose in a shared
  reference, or a third bundled script if it replaces many calls (that needs its own ADR,
  [ADR 0006](adr/0006-standalone-skills-optional-binary.md)). It never picks or sizes a slice.
- **`spec`**: turns the discussion so far into a spec plus a task graph, without re-interviewing the
  user. It publishes to the store `docs/agents/issue-tracker.md` names, falling back to the run dir.
- **`implement`**: works the frontier one slice at a time, running the gate between slices. It runs
  sequentially in place by default, for two reasons: `wt` is fragile under the sandbox (it was seen
  failing to `mktemp`), and two editors in one tree collide. Parallel worktrees would be a
  `docs/agents/` setting.

**Done when** `implement`, in a fresh session, can work a spec from `spec` using only the artifact and
the branch's commits.

### Also open

- **Rest of epic [#59](https://github.com/masterik/mk-toolkit/issues/59)**:
  Codex and Copilot ports (#54–#57), Windows via Git Bash (#58).
- **Signing and notarization**: [#28](https://github.com/masterik/mk-toolkit/issues/28). Until then,
  the cask strips the quarantine flag on install
  ([fallback](prerequisites.md#gatekeeper-blocks-the-binary)).

## Later

- **Hunk staging helper**: replaces `commit`'s Markdown patch-staging recipe
  ([#11](https://github.com/masterik/mk-toolkit/issues/11)). Not a binary verb: `commit` must not need
  the binary.
- **`mkit session show`**: reads earlier Claude Code transcript turns back, so `explain` can cover
  compacted context when the binary is present, never requiring it.
- **TUIs for `cleanup` and `review`**: [#12](https://github.com/masterik/mk-toolkit/issues/12),
  [#13](https://github.com/masterik/mk-toolkit/issues/13).
- **`install` / `uninstall`**: only if a job appears that manual steps can't do
  ([#10](https://github.com/masterik/mk-toolkit/issues/10)).

## Done

Milestone numbers are stable IDs, referenced from code comments and `AGENTS.md`. Details are in the
linked issues, the [Releases](https://github.com/masterik/mk-toolkit/releases) and
[`CHANGELOG.md`](../CHANGELOG.md) (frozen at 0.19.0).

| | What | Notes |
| --- | --- | --- |
| M1 | Scaffold + release chain | Go module, cobra, GoReleaser, Homebrew cask (`v0.12.0`). |
| M2 | `mkit cache prune` | Ported from shell; TUI tick-list on `--apply`. |
| M3 | ~~`install`/`status`/`uninstall`~~ | Withdrawn ([ADR 0003](adr/0003-two-distribution-channels.md)); `status` folded into `doctor`. |
| M4 | `mkit findings` ([#6](https://github.com/masterik/mk-toolkit/issues/6)) | Review arithmetic ported from JS. |
| M5 | The `jq` consumers ([#7](https://github.com/masterik/mk-toolkit/issues/7)) | `facts`, `gate`, `branch status`, `scratch`; the payload became Markdown only. |
| M6 | `mkit worklog` + workflow contract ([#8](https://github.com/masterik/mk-toolkit/issues/8)) | Worklog dropped in [#49](https://github.com/masterik/mk-toolkit/issues/49) (epic #59): `recap` reads `git log` and the PR; the contract stays. |
| M7 | `repo profile`, `init`, `doctor` ([#3](https://github.com/masterik/mk-toolkit/issues/3)) | `init`, `repo profile` and `.mkit/config.toml` removed in #53; `doctor` stays. |
| — | `audit sandbox` + `sandbox-audit` ([#36](https://github.com/masterik/mk-toolkit/issues/36)) | |
| — | `explain` + `recap` ([#43](https://github.com/masterik/mk-toolkit/issues/43)) | |
| — | Standalone skills, optional binary (epic [#59](https://github.com/masterik/mk-toolkit/issues/59): #48–#53, #60, #61, #47) | [ADR 0006](adr/0006-standalone-skills-optional-binary.md). Gate, worklog, findings, `.mkit/`, `facts`/`branch status`, `init`/`repo profile` left the binary; `setup` skill and two bundled scripts in; skill prose cut to ≤ ~8 KB per `SKILL.md` ([checklist](skill-invariants.md)). |

## Accepted gaps

- **Nothing reports a missing tool unprompted.** The `SessionStart` hook that did this was removed in
  0.15.0. `mkit doctor` is human-run and optional. Re-adding a hook is fine if this gap proves costly,
  but do it deliberately.
