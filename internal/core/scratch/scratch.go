// Package scratch owns <toplevel>/.mkit/ — the scratch root every skill writes
// through, and the only place in the binary that creates a file inside a user's
// work tree.
//
// The single-owner rule is the point. `tests/bats/payload.bats` asserted the
// three-write-locations invariant statically over the shell, because the
// boundaries that enforce it cannot be created inside a test; in Go the same
// invariant gets a real seam for the first time — every write goes through this
// package, and TestNoWritesOutsideScratch keeps it that way.
//
// Layering: returns data, never prints.
package scratch

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// The two probes that together answer "is mkit's scratch ignored here?".
//
// Both, because neither speaks for the other. An unrelated `*.jsonl` rule hides
// the ledger while leaving every `.mkit/<skill>-*/` run directory untracked, and
// a yes on that basis is reported to a skill whose worktree teardown then fails.
//
// Concrete paths inside the scratch, never the bare `.mkit/`: `check-ignore` is
// pattern matching rather than a stat, so these answer correctly before anything
// exists on disk — which is the first call, the one that has to be right — and
// every rule shape matches them, the legacy directory-only `.mkit/` and the
// current `.mkit/*` pair alike. (`check-ignore .mkit/` *with* the trailing slash
// also works, because git reads the slash as naming something inside the
// directory; depending on that subtlety twice, for two different reasons, is a
// trap for whoever edits this next.)
const (
	scratchProbe = ".mkit/gate.jsonl"
	runProbe     = ".mkit/probe-0/log"
)

// Dir is the scratch root of repo.
func Dir(repo *gitrepo.Repo) string { return filepath.Join(repo.Toplevel, ".mkit") }

// Ignored reports whether mkit's scratch is excluded in this repo. Asked of git
// rather than of a file's contents, so a `.gitignore` line, a global excludes
// file and the common-dir exclude all count.
func Ignored(repo *gitrepo.Repo) bool {
	return repo.Excluded(scratchProbe) && repo.Excluded(runProbe)
}

// EnsureIgnored makes the scratch ignored, once, and reports whether it now is.
// Best effort: it returns false rather than an error and never fails a caller.
//
// Three things break while the scratch is unignored, all measured, none cosmetic:
//
//   - worktree teardown. `git status --porcelain` reports `?? .mkit/`, and
//     `git worktree remove` refuses with "contains modified or untracked files"
//     — which breaks `finish`/`cleanup` and Claude Code's own sweep.
//   - `git add -A`, which commits run artefacts into the user's project. Already
//     happened once, with an improvised helper script.
//   - the gate cache. Fingerprint enumerates with `ls-files --others
//     --exclude-standard`, so an unignored scratch enters the fingerprint and
//     then changes while the gate runs — a run invalidating its own cache entry.
//
// Two lines, and the pair is the unit: `.mkit/*` excludes the scratch while
// leaving `.mkit/` itself includable, and `!.mkit/config.toml` re-includes the
// one committed file. Writing only the first would hide repo config from
// `git add`; writing the old directory-only `.mkit/` would make the negation
// impossible to add later.
//
// Written into the common dir's `info/exclude`: shared by every worktree,
// uncommitted, no diff noise, and writable — only `.git/config` and `.git/hooks`
// are protected inside a working directory. It cannot be written from a
// worktree-isolated session, since that file lives in the main checkout, which is
// why the answer is reported as a starting fact rather than assumed.
func EnsureIgnored(repo *gitrepo.Repo) bool {
	if Ignored(repo) {
		return true
	}
	common, err := repo.CommonDir()
	if err != nil || common == "" {
		return false
	}
	if err := os.MkdirAll(filepath.Join(common, "info"), 0o755); err != nil {
		return false
	}
	exclude := filepath.Join(common, "info", "exclude")
	f, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	// Appended with a comment naming the writer, because an unexplained line in
	// someone else's exclude file is indistinguishable from cruft.
	_, err = fmt.Fprint(f, "\n# mkit scratch root (run directories + gate.jsonl). Added by mkit.\n"+
		"# config.toml is repo config and stays committable — the pair is the rule.\n"+
		".mkit/*\n!.mkit/config.toml\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return false
	}
	return Ignored(repo)
}
