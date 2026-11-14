package doctor

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/userdir"
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
	// remedy words the fix for a path; nil means `rm -r <path>`.
	remedy func(p string) string
}

type orphanAnchors struct {
	toplevel  string
	commonDir string
	userDir   string
}

// worklogDropped is why both worklog locations (the pre-M6 `.mkit/work/` and
// the later `.mkit/worklog/`) are leftovers.
const worklogDropped = "the worklog was dropped (recap reads git log and the PR); nothing reads this one"

// legacy is the table to prune. An entry is dead weight once every machine has
// run a release past the one that stopped writing it — the way the one-shot
// migration scripts were deleted — so each `why` names its replacement, which
// is what makes the entry safe to drop later.
var legacy = []orphan{
	{
		name: "old worklog directory",
		path: func(a orphanAnchors) string { return mkitFile(a, "work") },
		why:  worklogDropped,
	},
	{
		name: "worklog directory",
		path: func(a orphanAnchors) string { return mkitFile(a, "worklog") },
		why:  worklogDropped,
	},
	{
		name: "git-dir scratch",
		path: func(a orphanAnchors) string {
			if a.commonDir == "" {
				return ""
			}
			return filepath.Join(a.commonDir, "mkit")
		},
		why: "mkit keeps no per-repo state since `.mkit/` was retired; nothing reads this one",
	},
	{
		name: "old gate ledger",
		path: func(a orphanAnchors) string { return mkitFile(a, "gate.jsonl") },
		why:  "`mkit gate run` and its ledger were removed; skills run the gate commands directly",
	},
	{
		name: "old repo config",
		path: func(a orphanAnchors) string { return mkitFile(a, "config.toml") },
		why:  "repo settings moved to prose files under docs/agents/, which the `setup` skill writes",
		remedy: func(p string) string {
			return "run the `setup` skill to carry its settings into docs/agents/, then git rm " +
				shellQuote(p) + " (plain rm if it was never committed)"
		},
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

// mkitFile is <toplevel>/.mkit/<name>, or "" outside a repo.
func mkitFile(a orphanAnchors, name string) string {
	if a.toplevel == "" {
		return ""
	}
	return filepath.Join(a.toplevel, ".mkit", name)
}

// userFile is <user dir>/<name>, or "" when there is no user directory.
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
	a := orphanAnchors{userDir: userdir.Dir()}
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
		remedy := "rm -r " + shellQuote(p)
		if o.remedy != nil {
			remedy = o.remedy(p)
		}
		r.add(Check{Group: leftoverGroup, Name: o.name, Status: Warn,
			Detail: p + " — " + o.why, Remedy: remedy})
	}
	if r.oldRunDirs(a) {
		found++
	}
	if r.oldIgnorePair(a) {
		found++
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
		if c == "" {
			continue
		}
		// p was built with filepath.Join, so it is already clean; an anchor is not
		// — MKIT_HOME may carry a trailing slash or a `./` — and a raw prefix test
		// against it would call a perfectly ordinary path a symlink escape.
		c = filepath.Clean(c)
		if p == c || strings.HasPrefix(p, c+string(filepath.Separator)) {
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

// runDirName is the exact shape of a run directory an older `mkit facts` or
// `mkit scratch open` created: `<skill>-<UTC second>-<MkdirTemp suffix>`. Matched
// by shape rather than by a list of skills because any slug was accepted, and
// still never a bare pattern: nothing else names itself like this.
var runDirName = regexp.MustCompile(`^[A-Za-z0-9_-]+-[0-9]{8}T[0-9]{6}Z-[A-Za-z0-9]+$`)

// oldRunDirs reports every run directory left under `<toplevel>/.mkit/`, in one
// finding: there can be dozens, and they share one cause and one remedy.
func (r *Report) oldRunDirs(a orphanAnchors) bool {
	if a.toplevel == "" {
		return false
	}
	dir := filepath.Join(a.toplevel, ".mkit")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	var paths []string
	for _, e := range entries {
		// Directories only — Lstat semantics, so a symlink is never followed.
		if e.IsDir() && runDirName.MatchString(e.Name()) {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	if len(paths) == 0 {
		return false
	}
	sort.Strings(paths)
	detail := strings.Join(paths, ", ") +
		" — per-run files live in a temp dir now; nothing reads these"
	if leavesAnchor(paths[0], a) {
		r.add(Check{Group: leftoverGroup, Name: "old run directories", Status: Warn,
			Detail: detail + "; they are reached through a symlink, so no rm is offered: check where it points first"})
		return true
	}
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = shellQuote(p)
	}
	r.add(Check{Group: leftoverGroup, Name: "old run directories", Status: Warn,
		Detail: detail, Remedy: "rm -r " + strings.Join(quoted, " ")})
	return true
}

// mkitExcludeLines are the lines an older mkit appended to the common dir's
// `info/exclude`: the ignore pair, the legacy directory-only rule it replaced,
// and the two comment lines it wrote above the pair. Exact lines only — a
// user's own rule that happens to mention `.mkit` is theirs.
var mkitExcludeLines = []string{
	"# mkit scratch root (run directories + gate.jsonl). Added by mkit.",
	"# config.toml is repo config and stays committable — the pair is the rule.",
	".mkit/",
	".mkit/*",
	"!.mkit/config.toml",
}

// oldIgnorePair reports the ignore rules mkit wrote into `info/exclude`. The
// remedy is an edit, not an rm: the file is git's and holds the user's rules too.
func (r *Report) oldIgnorePair(a orphanAnchors) bool {
	if a.commonDir == "" {
		return false
	}
	path := filepath.Join(a.commonDir, "info", "exclude")
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	present := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		present[strings.TrimRight(l, " \t\r")] = true
	}
	var rules, lines []string
	for _, l := range mkitExcludeLines {
		if !present[l] {
			continue
		}
		lines = append(lines, "`"+l+"`")
		if !strings.HasPrefix(l, "#") {
			rules = append(rules, l)
		}
	}
	// The comments alone are not a rule; a file holding only them ignores nothing.
	if len(rules) == 0 {
		return false
	}
	r.add(Check{Group: leftoverGroup, Name: "old ignore pair", Status: Warn,
		Detail: path + " still holds mkit's " + strings.Join(rules, " + ") +
			" — nothing writes into the work tree any more, so the rule guards nothing",
		Remedy: "delete these lines from " + shellQuote(path) + ": " + strings.Join(lines, ", ")})
	return true
}
