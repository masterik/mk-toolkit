package initplan

import (
	"os"
	"regexp"
	"slices"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// ScopeParents are the directories whose children are offered as scopes as well
// as the directory itself — where Go and monorepo layouts keep their units.
var ScopeParents = []string{"internal", "cmd", "pkg", "src", "packages", "apps"}

// dirNoise are directory names that are build output or vendored code, never an
// area of change a commit would name.
var dirNoise = []string{"node_modules", "vendor", "dist", "build", "target", "out", "bin", "tmp"}

// scopeName is a directory name usable as a scope, per the characters the
// history matcher in profile accepts.
var scopeName = regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`)

// Gather collects the candidates the form offers. Read-only: it lists remotes,
// branches and directories, and writes nothing.
func Gather(repo *gitrepo.Repo) Candidates {
	var c Candidates
	for _, name := range repo.Remotes() {
		c.Remotes = append(c.Remotes, Remote{Name: name, URL: repo.RemoteURL(name)})
	}
	c.Branches = repo.LocalBranches()
	c.Dirs = DirCandidates(repo)
	return c
}

// DirCandidates lists the top-level directories, plus one level under each of
// ScopeParents, without duplicates. Hidden, underscored, noise and git-ignored
// directories are skipped.
func DirCandidates(repo *gitrepo.Repo) []string {
	var out []string
	add := func(name string) {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	for _, top := range subdirs(repo, "") {
		add(top)
		if slices.Contains(ScopeParents, top) {
			for _, child := range subdirs(repo, top) {
				add(child)
			}
		}
	}
	return out
}

// subdirs lists rel's directories usable as a scope, asking git once for the
// whole listing which of them are ignored.
func subdirs(repo *gitrepo.Repo, rel string) []string {
	dir := repo.Toplevel
	if rel != "" {
		dir += string(os.PathSeparator) + rel
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names, paths []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name[0] == '.' || name[0] == '_' ||
			slices.Contains(dirNoise, name) || !scopeName.MatchString(name) {
			continue
		}
		path := name + "/"
		if rel != "" {
			path = rel + "/" + path
		}
		names, paths = append(names, name), append(paths, path)
	}
	ignored := repo.ExcludedSet(paths)
	var out []string
	for i, name := range names {
		if !ignored[paths[i]] {
			out = append(out, name)
		}
	}
	return out
}
