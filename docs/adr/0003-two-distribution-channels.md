# ADR 0003 — Two distribution channels: Homebrew ships the binary, GitHub ships the plugin

**Status:** accepted · **Date:** 2026-09-09 ·
**Supersedes** the "Homebrew ships binary *and* plugin payload" decision in
[`backlog.md`](../backlog.md) and the same claim in `concept.md`, `README.md` and `AGENTS.md`.
**Withdraws** M3.

## Context

M1 shipped `v0.12.0` as a Homebrew **cask** and recorded the intent that Homebrew would ship the
plugin payload too, so one `brew upgrade` updated both halves. M3 was written against that intent:
`mkit install` would register the Homebrew-provided payload as a `directory` marketplace in
`~/.claude/settings.json`.

Inspecting the shipped cask — rather than re-reading the config that produced it — turned up two
gaps, and a third came out of implementing [ADR 0002](0002-state-locations-under-a-sandbox.md):

1. **The payload is not in the archive.** `.goreleaser.yaml` declares `archives: [formats:
   [tar.gz]]` with no `files:`, so the tarball carries the binary plus goreleaser's default
   `LICENSE` and `README.md`. The installed cask contains exactly those three entries.
2. **A cask has no stable path to register.** A cask is not a keg: it installs to
   `/opt/homebrew/Caskroom/mkit/<version>/` and never creates `/opt/homebrew/opt/<name>`.
   Verified — `/opt/homebrew/opt/mkit` does not exist and `/opt/homebrew/bin/mkit` is a symlink
   straight into the versioned directory. Casks also have no artifact stanza for "install this
   directory into `share/`". So the only available path is version-pinned, which rots on the next
   upgrade, and the alternative was reworking the release chain into a formula.
3. **The file `mkit install` would have to write cannot be written.** `~/.claude/settings.json` is
   inside the sandbox's protected region *and* appears explicitly in the session's write-deny
   list. No `permissions.additionalDirectories` or `sandbox.filesystem.allowWrite` entry lifts
   that — the same class of finding as ADR 0002's, one file over. M3 had already conceded this by
   specifying its own registration step as human-run (`! mkit install`).

Meanwhile the GitHub marketplace path already worked, and is what every machine actually uses:
`/plugin marketplace add masterik/mk-toolkit` against the root `.claude-plugin/marketplace.json`,
with the payload resolved from a checkout Claude Code maintains.

## Decision

**1. Homebrew ships the binary and nothing else.** The cask installs one executable, which is
what casks do well. No `files:` entry, no formula rework, no payload in the archive.

**2. The plugin payload ships from the GitHub marketplace, as it does today.** `marketplace.json`
stays at the repo root with `source: "./plugin"`. This is not a fallback or an interim measure; it
is the channel.

**3. The two channels are independent, permanently.** Nothing synchronises them and nothing is
expected to. `brew upgrade mkit` moves the binary; the marketplace's own update mechanism moves
the payload.

**4. Installation is manual in this phase, and M3 is withdrawn.** Adding a marketplace and
enabling a plugin are two lines a human runs once. Silencing the `SessionStart` hook is creating
one file. A command that wraps either buys nothing while it cannot write the file it would need to
write, so `plugin/install.sh` is deleted rather than ported, and `install`/`uninstall` are
re-specified later from what the binary actually needs.

**5. Skew is a condition to report, not a state to eliminate.** Because (3) is permanent, a plugin
from GitHub can meet a binary from Homebrew that is too old for it. The guard belongs to M4 — the
first port that makes a skill call the binary, and therefore the first point at which skew can
hurt anything. Today the payload invokes no binary at all, so there is nothing to guard.

## Consequences

- **Both M3 blockers are dissolved rather than solved**, which is the whole value here. The
  cask-vs-formula decision, the archive `files:` entry, the version-pinned registration path and
  the settings write all stop being work because nothing needs them.
- **The cask stays honest.** Its description and contents now match: a Go CLI. The claim that
  Homebrew ships the payload was an intent stated as fact in four documents; it is corrected in
  all of them.
- **`mkit init` becomes the near-term priority**, since nothing in the port line blocks it and it
  is what a user actually wants per project. M7 moves ahead of the remaining ports.
- **Version skew becomes a standing condition.** This is the real cost of the decision, and it is
  accepted deliberately: an independently-updating payload and binary will drift, so the payload
  must eventually detect and report drift rather than assume a matched pair. Bounded by M4 owning
  the guard, and by the fact that the payload calls no binary until then.
- **Two things `install.sh` owned need homes.** Writing the tombstone becomes a documented
  one-liner. Being the loud diagnostic for an unwritable user directory falls to `facts.sh`'s
  `user_dir_writable=` starting fact until M7's `doctor` restores a human-run surface — a real
  gap between the two, named here so it is not discovered later.
- **A one-time migration script is required, not merely convenient.** ADR 0002 moved user scope
  to `~/.mkit/`, and a grant on that path covers the directory's *interior* — `mkit` cannot
  create it, because that is a write to `$HOME`. Nothing inside a sandboxed session can, which
  makes a human-run script the only thing that can.

## Alternatives rejected

- **Switch the tap entry from a cask to a formula.** Kegs get `opt/`, and `share/mkit/plugin`
  installs naturally, so M3's registration would have worked as written. Rejected because it buys
  a payload channel that already exists at the price of redoing M1's release chain — and it would
  still not have solved (3), the settings write, which is the part no packaging choice can reach.
- **Keep the cask and have `mkit install` copy the payload out of Caskroom to a stable path.** No
  tap rework, but `mkit install` becomes load-bearing for every `brew upgrade` and a stale copy is
  a silent failure mode. Rejected: it makes a command mandatory in order to serve a payload the
  marketplace serves for free.
- **Ship the payload through both channels.** Two sources for one set of skills, differing by
  machine, with no way to tell from inside a session which one is loaded. This is the "two
  implementations of one invariant" failure the porting rules exist to prevent, applied to
  distribution.
- **Keep M3 and solve the blockers.** Defensible six weeks ago. Rejected now because the
  registration step was already conceded as human-run, which left the milestone delivering a
  wrapper around two lines of manual configuration.
