// Package branchstatus is what is left of `mkit branch status` once `cleanup`'s
// classifier moved to the plugin's `branch-status.sh` (#52): the protected-set rule
// `mkit repo profile` reports. It goes with `profile` (#53).
//
// Layering: returns data, never prints, never assumes a terminal.
package branchstatus

import (
	"os/exec"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// DevelopCandidates are the develop-like names, in the order the first local one
// wins.
var DevelopCandidates = []string{"develop", "development", "dev"}

// Develop returns the first of DevelopCandidates that exists as a local branch,
// or "none".
//
// Only a *local* branch counts. A develop that exists only as origin/develop is
// not one of this repo's branches to keep — nothing here creates one, and this
// only ever reports on what is already checked out somewhere.
//
// Exported because `mkit repo profile` reports the set cleanup protects with
// nothing pinned, and that set has to be the one this package actually computes.
func Develop(repo *gitrepo.Repo, def string) string {
	for _, cand := range DevelopCandidates {
		if cand == def {
			continue
		}
		if err := run(repo, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+cand); err == nil {
			return cand
		}
	}
	return "none"
}

// ProtectedSet unions the default branch, the develop-like branch and a pinned keep
// list into the branches cleanup must never delete. The one producer of that set.
//
// **The default branch comes first and unconditionally.** A keep list that omits
// it is a mistake, not an instruction: config is an input, and this key only ever
// adds to what cleanup already protects. Order is default, develop, then the
// pinned names as written — stable, so two runs on one repo report the same list
// and a diff of it means something. Blank entries are dropped; they pin nothing.
func ProtectedSet(def, develop string, keep []string) []string {
	out := make([]string, 0, len(keep)+2)
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	add(def)
	if develop != "none" {
		add(develop)
	}
	for _, k := range keep {
		if name, ok := keepName(k); ok {
			add(name)
		}
	}
	return out
}

// keepName trims a pinned entry and says whether anything is left.
//
// **ASCII whitespace only, never `strings.TrimSpace`.** Git forbids the ASCII
// space and control characters in a ref name, so trimming those can only remove
// padding a human left in the TOML — but it *allows* Unicode whitespace, so
// TrimSpace rewrites a legal branch name (`release\u00a02026`) into a different
// one. The pin then matches no branch, the branch is not protected, and this is
// the code path that decides what gets deleted.
//
// So both cases stay true: `" staging "` pins `staging`, and a name whose own
// characters include U+00A0 is looked up exactly as written.
const asciiSpace = " \t\n\r\v\f"

func keepName(k string) (string, bool) {
	k = strings.Trim(k, asciiSpace)
	return k, k != ""
}

func run(repo *gitrepo.Repo, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = repo.Toplevel
	return cmd.Run()
}
