package profile

import (
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// Issue #18: `commit` and `pr` read these values off the profile. What the skills
// need from this package is the *tag* — a value they may state as pinned — and a
// cause where a pin was rejected, which is not the same answer as "no answer".

func write(t *testing.T, toplevel string, c *repoconfig.Config) {
	t.Helper()
	if err := repoconfig.Write(toplevel, c); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedScopesAndReviewersAreTaggedPinned(t *testing.T) {
	repo := newRepo(t)
	write(t, repo.Toplevel, &repoconfig.Config{
		Commit: repoconfig.Commit{Scopes: []string{"cli", "core"}},
		Review: repoconfig.Review{Reviewers: []string{"@a"}},
	})
	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	// Pinned, and not re-derived: the values are the pinned ones verbatim, which
	// is what lets `commit` and `pr` skip history and CODEOWNERS entirely.
	if p.Scopes.Source != Pinned || strings.Join(p.Scopes.Values, ",") != "cli,core" {
		t.Errorf("commit_scopes = %+v, want the pinned list tagged pinned", p.Scopes)
	}
	if p.Review.Source != Pinned || strings.Join(p.Review.Values, ",") != "@a" {
		t.Errorf("reviewers = %+v, want the pinned list tagged pinned", p.Review)
	}
}

func TestSubjectMaxIsPinnedOrAbsentAndNeverDiscovered(t *testing.T) {
	repo := newRepo(t)
	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	// No discovery arm on purpose: the longest subject in history is what the
	// repo happened to write, not what it requires.
	if p.SubjectMax.Source != Unavailable || p.SubjectMax.Value != "" {
		t.Fatalf("subject max with nothing pinned = %+v, want unavailable", p.SubjectMax)
	}
	if !strings.Contains(p.SubjectMax.Cause, "not discoverable") {
		t.Errorf("cause does not say why there is no discovered answer: %q", p.SubjectMax.Cause)
	}

	n := 72
	write(t, repo.Toplevel, &repoconfig.Config{Commit: repoconfig.Commit{SubjectMax: &n}})
	p, err = Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.SubjectMax.Source != Pinned || p.SubjectMax.Value != "72" {
		t.Errorf("subject max = %+v, want 72 pinned", p.SubjectMax)
	}
}

func TestReviewModeIsPinnedOrAbsent(t *testing.T) {
	repo := newRepo(t)

	// The absent arm first, on a repo with no config at all: nothing in a repo is
	// evidence of the roster a team wants, so there is no discovery arm to fall
	// back to and `review` keeps its own default. Asserted because the name
	// promises it — a test covering only the pinned half cannot tell a missing
	// discovery arm from a broken one.
	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.ReviewMode.Source != Unavailable || p.ReviewMode.Value != "" {
		t.Errorf("review mode = %+v, want unavailable with no value", p.ReviewMode)
	}
	if p.ReviewMode.Cause == "" {
		t.Error("an unavailable value with no cause presents an empty answer as an answer")
	}

	write(t, repo.Toplevel, &repoconfig.Config{Review: repoconfig.Review{Mode: "quick"}})
	p, err = Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.ReviewMode.Source != Pinned || p.ReviewMode.Value != "quick" {
		t.Errorf("review mode = %+v, want quick pinned", p.ReviewMode)
	}
}

// A rejected pin is `unavailable` **with a cause naming the key and the file** —
// the state a skill surfaces to the user. A plain absent answer carries a cause
// too, but never one that names the config file, which is how the skills tell the
// two apart without parsing prose.
func TestARejectedPinIsUnavailableWithTheConfigFileNamed(t *testing.T) {
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, repoconfig.RelPath,
		"version = 1\n\n[commit]\nsubject_max = 0\n\n[review]\nmode = \"fast\"\n")
	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		v    Value
		key  string
	}{
		{"commit subject max", p.SubjectMax, "commit.subject_max"},
		{"review mode", p.ReviewMode, "review.mode"},
	} {
		if tc.v.Source != Unavailable {
			t.Errorf("%s: source = %q, want unavailable — a rejected pin must never wear a valid tag", tc.name, tc.v.Source)
		}
		if !strings.Contains(tc.v.Cause, tc.key) || !strings.Contains(tc.v.Cause, repoconfig.RelPath) {
			t.Errorf("%s: cause does not name the key and the file: %q", tc.name, tc.v.Cause)
		}
	}
	// And the problems are on the profile as a whole as well, which is what
	// `doctor` and `repo profile`'s header print.
	if len(p.ConfigProblems) != 2 {
		t.Errorf("config_problems = %+v, want both rejected pins", p.ConfigProblems)
	}
}
