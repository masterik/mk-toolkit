package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/scratch"
	"github.com/masterik/mk-toolkit/internal/core/worklog"
)

// leftoverGroup is the report group the orphan checks live in.
const leftoverGroup = "leftovers"

// orphan is a path an older mkit wrote and no current version reads. The list
// is closed on purpose: doctor names exactly these, never a pattern, so a
// finding is always something a version of mkit really did produce.
type orphan struct {
	name string
	// path resolves the location, or "" when its anchor (repo, common dir, user
	// directory) is unknown — an orphan that cannot be located is not reported.
	path func(a orphanAnchors) string
	why  string
}

type orphanAnchors struct {
	toplevel  string
	commonDir string
	userDir   string
}

// legacy is the table to prune. An entry is dead weight once every machine has
// run a release past the one that stopped writing it — the way the one-shot
// migration scripts were deleted — so each `why` names its replacement, which
// is what makes the entry safe to drop later.
var legacy = []orphan{
	{
		name: "old worklog directory",
		path: func(a orphanAnchors) string {
			if a.toplevel == "" {
				return ""
			}
			return filepath.Join(a.toplevel, ".mkit", "work")
		},
		why: "worklogs moved to " + filepath.Join(".mkit", filepath.Base(worklog.Dir("x"))) + "; nothing reads this one",
	},
	{
		name: "git-dir scratch",
		path: func(a orphanAnchors) string {
			if a.commonDir == "" {
				return ""
			}
			return filepath.Join(a.commonDir, "mkit")
		},
		why: "scratch moved to <toplevel>/.mkit/ (ADR 0002); nothing reads this one",
	},
	{
		name: "bootstrap.state",
		path: func(a orphanAnchors) string { return userFile(a, "bootstrap.state") },
		why:  "written by the SessionStart hook, removed in 0.15.0",
	},
	{
		name: "bootstrap.disabled",
		path: func(a orphanAnchors) string { return userFile(a, "bootstrap.disabled") },
		why:  "written by the SessionStart hook, removed in 0.15.0",
	},
}

func userFile(a orphanAnchors, name string) string {
	if a.userDir == "" {
		return ""
	}
	return filepath.Join(a.userDir, name)
}

// orphans reports what an older mkit left behind. Report-only: each finding
// carries the `rm` the user runs, and nothing is removed here. Warn, never
// Fail — a leftover breaks nothing, it only takes space and misleads a reader.
func (r *Report) orphans(repo *gitrepo.Repo) {
	a := orphanAnchors{userDir: scratch.UserDir()}
	if repo != nil {
		a.toplevel = repo.Toplevel
		a.commonDir, _ = repo.CommonDir()
	}

	found := 0
	for _, o := range legacy {
		p := o.path(a)
		if p == "" {
			continue
		}
		// Lstat, not Stat: a symlink is a leftover in itself, and following it
		// would report — and `rm -r` would then be aimed at — its target.
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		found++
		// The `rm` is only offered for a path that really lives under its anchor.
		// Lstat above looks at the last element only; a symlinked ancestor —
		// `.mkit` pointing elsewhere — would aim `rm -r` outside the repo.
		if leavesAnchor(p, a) {
			r.add(Check{Group: leftoverGroup, Name: o.name, Status: Warn,
				Detail: p + " — " + o.why + "; it is reached through a symlink, so no rm is offered: check where it points first"})
			continue
		}
		r.add(Check{Group: leftoverGroup, Name: o.name, Status: Warn,
			Detail: p + " — " + o.why,
			Remedy: "rm -r " + shellQuote(p)})
	}
	if found == 0 {
		r.add(Check{Group: leftoverGroup, Name: "old-version leftovers", Status: OK,
			Detail: "none found in this repo or the user directory"})
	}
}

// leavesAnchor reports whether p, with every symlink in its parent chain
// resolved, falls outside the (resolved) anchor directory it was derived from.
// A path whose anchor cannot be resolved counts as leaving: no remedy is safer
// than one aimed somewhere unverified.
func leavesAnchor(p string, a orphanAnchors) bool {
	var anchor string
	for _, c := range []string{a.toplevel, a.commonDir, a.userDir} {
		if c != "" && (p == c || strings.HasPrefix(p, c+string(filepath.Separator))) {
			anchor = c
			break
		}
	}
	if anchor == "" {
		return true
	}
	root, err := filepath.EvalSymlinks(anchor)
	if err != nil {
		return true
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return true
	}
	rel, err := filepath.Rel(root, dir)
	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// shellQuote single-quotes a path for the remedy line, so a path with a space
// or a quote is still one argument when pasted.
func shellQuote(s string) string {
	out := "'"
	for _, c := range s {
		if c == '\'' {
			out += `'\''`
			continue
		}
		out += string(c)
	}
	return out + "'"
}
