package profile

import (
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// Issue #19's worst failure shape: a rejected pin used to fall through to
// discovery and arrive tagged `discovered`, so the config looked applied.

const badConfig = `version = 1

[reviewers]
reviewers = ["@nobody"]

[merge]
style = "sqaush"

[spec]
store = "gh-issues"
`

func TestRejectedPinIsUnavailableNotDiscovered(t *testing.T) {
	repo := newRepo(t)
	// git config, so there *is* a discovered answer to fall through to — without
	// one this test would pass for the wrong reason.
	run(t, repo.Toplevel, "config", "mkit.mergeStyle", "rebase")
	writeFile(t, repo.Toplevel, repoconfig.RelPath, badConfig)

	p, err := Build(repo)
	if err != nil {
		t.Fatalf("a malformed config must never fail a command: %v", err)
	}
	if p.Merge.Source != Unavailable {
		t.Fatalf("merge style = %q [%s], want unavailable — a rejected pin replaced by a "+
			"discovered value reads as a correct answer", p.Merge.Value, p.Merge.Source)
	}
	for _, want := range []string{"merge.style", "sqaush", repoconfig.Path(repo.Toplevel)} {
		if !strings.Contains(p.Merge.Cause, want) {
			t.Errorf("cause does not name %q: %s", want, p.Merge.Cause)
		}
	}
	if p.Spec.Store.Source != Unavailable || !strings.Contains(p.Spec.Store.Cause, "spec.store") {
		t.Errorf("spec store = %+v, want unavailable naming the key", p.Spec.Store)
	}
}

func TestProfileReportsUnknownKeysAndNamesTheFile(t *testing.T) {
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, repoconfig.RelPath, badConfig)

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	// Both problems the issue's "done when" names, from one profile.
	var unknown, invalid bool
	for _, pb := range p.ConfigProblems {
		if pb.Path != repoconfig.Path(repo.Toplevel) {
			t.Errorf("problem does not name the config file: %+v", pb)
		}
		switch {
		case pb.Kind == repoconfig.ProblemUnknownKey && pb.Key == "reviewers":
			unknown = true
		case pb.Kind == repoconfig.ProblemInvalidValue && pb.Key == "merge.style":
			invalid = true
		}
	}
	if !unknown {
		t.Errorf("the misspelled table is not in the profile: %+v", p.ConfigProblems)
	}
	if !invalid {
		t.Errorf("the invalid merge.style is not in the profile: %+v", p.ConfigProblems)
	}
	// An unknown key belongs to no value, so the reviewers list is untouched —
	// which is exactly why the problem has to be reported at the top level too.
	if p.Review.Source == Pinned {
		t.Error("`[reviewers]` was honoured; it is not a key mkit knows")
	}
}

func TestGoodConfigCarriesNoProblems(t *testing.T) {
	repo := newRepo(t)
	if err := repoconfig.Write(repo.Toplevel, &repoconfig.Config{
		Merge:  repoconfig.Merge{Style: "squash"},
		Review: repoconfig.Review{Reviewers: []string{"@a"}},
	}); err != nil {
		t.Fatal(err)
	}
	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ConfigProblems) != 0 {
		t.Fatalf("a config mkit itself wrote reported problems: %+v", p.ConfigProblems)
	}
	if p.Merge.Source != Pinned || p.Merge.Value != "squash" {
		t.Errorf("merge style = %+v, want pinned squash", p.Merge)
	}
}
