// Package scratch owns <toplevel>/.mkit/ — the scratch root every skill writes
// through, and the only place in the binary that creates a file inside a user's
// work tree.
//
// The single-owner rule is the point. `tests/bats/payload.bats` asserted the
// three-write-locations invariant statically over the shell, because the
// boundaries that enforce it cannot be created inside a test; in Go the same
// invariant gets a real seam for the first time — every write goes through this
// package, and TestWriteSitesAreOnTheReviewedAllowlist keeps it that way.
//
// Layering: returns data, never prints.
package scratch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// RunDir opens a fresh run directory under <toplevel>/.mkit/ and returns its
// absolute path.
//
// Four invariants, all mechanical, all of which fail silently or destructively
// when they are hand-rolled at the start of a skill:
//
//   - a unique directory, never a plain MkdirAll: the timestamp is
//     second-resolution, so two runs in one checkout can pick the same name, and
//     MkdirAll would merge them and let each clobber the other's logs.
//   - absolute, from --show-toplevel: the path is handed to subagents and reused
//     across shells, where a relative .mkit/… would resolve somewhere else. A
//     linked worktree gets its own, so no write of a worktree-isolated session
//     targets the shared checkout.
//   - ignored *before* the first write, or the first thing that walks the tree —
//     `git worktree remove`, `git add -A`, the gate fingerprint — sees run
//     artefacts. Best effort: the write lands in the main checkout's exclude
//     file, which a worktree-isolated session cannot reach, so the answer is
//     reported (`run_ignored=`) rather than enforced. Opening the run directory
//     is every skill's first call and may not fail over an ignore rule.
func RunDir(repo *gitrepo.Repo, skill string) (string, error) {
	if err := CheckSlug(skill); err != nil {
		return "", err
	}
	EnsureIgnored(repo)

	dir := Dir(repo)
	if err := checkNotSymlink(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("%s-%s-", skill, time.Now().UTC().Format("20060102T150405Z"))
	return os.MkdirTemp(dir, prefix)
}

// prunedSkills is the set `Prune` walks. One pass per skill, so a busy `review`
// never evicts the only `pr` run.
var prunedSkills = []string{"commit", "review", "finish", "pr", "cleanup"}

// liveWindow is how recently a run directory must have been touched to be treated
// as still in use. Age-ranked eviction alone cannot see a run still being written:
// a long review holding an older directory would be removed underneath itself.
const liveWindow = 60 * time.Minute

// PruneResult is what one prune did.
type PruneResult struct {
	Removed int
	Kept    int
	// Live counts directories skipped because something may still be writing them.
	Live int
}

// Prune removes all but the newest keep run directories per skill.
//
// Only `<skill>-*` **directories** are in range, which is what keeps `gate.jsonl`
// out of it.
func Prune(repo *gitrepo.Repo, keep int) (*PruneResult, error) {
	// `0` is a perfectly good number and would evict every run directory —
	// including the live one belonging to the caller doing the pruning.
	if keep < 1 {
		return nil, fmt.Errorf("--keep must keep at least 1 run directory")
	}
	res := &PruneResult{}
	dir := Dir(repo)
	if err := checkNotSymlink(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return res, nil
	}
	if err != nil {
		return nil, err
	}

	for _, skill := range prunedSkills {
		var names []string
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), skill+"-") {
				names = append(names, e.Name())
			}
		}
		// Newest first: the name's UTC timestamp sorts lexicographically.
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
		for i, name := range names {
			path := filepath.Join(dir, name)
			switch {
			case i < keep:
				res.Kept++
			case recentlyTouched(path):
				res.Live++
			default:
				if os.RemoveAll(path) == nil {
					res.Removed++
				}
			}
		}
	}
	return res, nil
}

func recentlyTouched(path string) bool {
	newest, ok := newestMTime(path)
	return ok && time.Since(newest) < liveWindow
}

// newestMTime is the most recent mtime of a run directory or anything inside it.
//
// The directory's own mtime is not enough: it moves when an entry is created or
// removed, and not when an existing file is rewritten. A stage that overwrites
// the output it already wrote — which is most of them — left an active run
// looking idle, and prune would remove it while it was still being written.
func newestMTime(path string) (time.Time, bool) {
	var newest time.Time
	var found bool
	err := filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			// An unreadable entry is not evidence of idleness. Treat the walk as
			// inconclusive and let the caller keep the directory.
			return err
		}
		fi, ferr := d.Info()
		if ferr != nil {
			return ferr
		}
		if m := fi.ModTime(); m.After(newest) {
			newest = m
		}
		found = true
		return nil
	})
	if err != nil {
		// Inconclusive: report "touched just now" so the directory is kept.
		return time.Now(), true
	}
	return newest, found
}

// checkNotSymlink refuses a scratch root that is a symlink. Following one would
// put every create and every RemoveAll on the other side of it — outside the
// work tree, which is the one place this package may never write.
func checkNotSymlink(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return nil // absent is fine; the caller creates it
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; mkit will not read or write scratch "+
			"through one, because every path under it leaves the work tree", dir)
	}
	return nil
}

// SlugError is a rejected name — a usage mistake, which callers map to exit 2
// rather than to the operational failures that share the same return.
type SlugError struct {
	Name   string
	Reason string
}

func (e *SlugError) Error() string { return e.Reason }

// CheckSlug rejects a path component that could traverse or glob.
func CheckSlug(name string) error {
	if name == "" {
		return &SlugError{Name: name, Reason: "empty name where a name is required"}
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return &SlugError{Name: name,
				Reason: "name may only contain [a-zA-Z0-9_-], got: " + name}
		}
	}
	return nil
}
