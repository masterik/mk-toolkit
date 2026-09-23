package profile

import (
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// With nothing pinned, the reported value is what cleanup already protects —
// read from `branchscan`, so the profile cannot advertise a set the classifier
// does not use.
func TestCleanupKeepIsDiscoveredFromWhatCleanupAlreadyProtects(t *testing.T) {
	repo := newRepo(t)
	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Keep.Source != Discovered {
		t.Fatalf("source = %q (%s), want discovered", p.Keep.Source, p.Keep.Cause)
	}
	if want := repo.DefaultBranch(); strings.Join(p.Keep.Values, ",") != want {
		t.Errorf("keep = %v, want just the default branch %q", p.Keep.Values, want)
	}
}

func TestCleanupKeepIncludesADevelopLikeBranchWhenOneExistsLocally(t *testing.T) {
	repo := newRepo(t)
	def := repo.DefaultBranch()
	run(t, repo.Toplevel, "branch", "develop")

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(p.Keep.Values, ","), def+",develop"; got != want {
		t.Errorf("keep = %q, want %q", got, want)
	}
}

// A pinned list replaces the *reported* value and is tagged `pinned`, exactly as
// commit_scopes, reviewers and merge_style are. It does not replace the
// protection: branchscan.ProtectedSet still unions it with the default branch.
func TestCleanupKeepIsTaggedPinnedWhenTheConfigSaysSo(t *testing.T) {
	repo := newRepo(t)
	if err := repoconfig.Write(repo.Toplevel, &repoconfig.Config{
		Cleanup: repoconfig.Cleanup{Keep: []string{"staging"}},
	}); err != nil {
		t.Fatal(err)
	}
	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Keep.Source != Pinned || strings.Join(p.Keep.Values, ",") != "staging" {
		t.Errorf("keep = %+v, want the pinned list", p.Keep)
	}
}
