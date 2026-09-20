package doctor

import (
	"os"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

func findAll(r *Report, name string) []Check {
	var cs []Check
	for _, c := range r.Checks {
		if c.Name == name {
			cs = append(cs, c)
		}
	}
	return cs
}

// Issue #19's doctor half: the human-run report names what the config says that
// mkit could not honour, and the file it is in.
func TestConfigValuesReportsUnknownKeysAndBadEnums(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, repoconfig.RelPath,
		"version = 1\n\n[reviewers]\nreviewers = [\"@a\"]\n\n[merge]\nstyle = \"sqaush\"\n")

	checks := findAll(Run(Options{Repo: repo}), "config values")
	if len(checks) != 2 {
		t.Fatalf("got %d config-value checks, want 2 (the unknown table and the bad style): %+v", len(checks), checks)
	}
	joined := ""
	for _, c := range checks {
		// Warn, never Fail: config is an input, never a permission — nothing here
		// stops a command.
		if c.Status != Warn {
			t.Errorf("status = %q, want warn: %s", c.Status, c.Detail)
		}
		if c.Remedy == "" {
			t.Errorf("no remedy on %q", c.Detail)
		}
		joined += c.Detail + "\n" + c.Remedy + "\n"
	}
	for _, want := range []string{"reviewers", "merge.style", "sqaush", repoconfig.Path(repo.Toplevel), "squash"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the report never mentions %q:\n%s", want, joined)
		}
	}
}

func TestConfigValuesAreOKWhenUnderstood(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	if err := repoconfig.Write(repo.Toplevel, &repoconfig.Config{
		Merge: repoconfig.Merge{Style: "merge"},
	}); err != nil {
		t.Fatal(err)
	}
	if c := find(t, Run(Options{Repo: repo}), "config values"); c.Status != OK {
		t.Errorf("status = %q, want ok: %s", c.Status, c.Detail)
	}
}

func TestNoConfigValuesCheckWithoutAConfig(t *testing.T) {
	isolate(t)
	// Absent is a normal state, not a finding — and not a check with nothing to
	// say either.
	if cs := findAll(Run(Options{Repo: newRepo(t)}), "config values"); len(cs) != 0 {
		t.Errorf("got %+v, want no check at all", cs)
	}
}

// A future version is reported and carries no remedy: there is nothing to edit in
// the file, and offering configuration that changes nothing is the ADR 0002 rule
// this report exists to obey.
func TestNewerConfigVersionWarnsWithoutARemedy(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, repoconfig.RelPath, "version = 99\n")

	checks := findAll(Run(Options{Repo: repo}), "config values")
	if len(checks) != 1 || checks[0].Status != Warn {
		t.Fatalf("got %+v", checks)
	}
	if checks[0].Remedy != "" {
		t.Errorf("remedy = %q, want none", checks[0].Remedy)
	}
	if !strings.Contains(checks[0].Detail, "99") {
		t.Errorf("detail does not name the version: %s", checks[0].Detail)
	}
}

// A remedy for a key with no enumerated set must still name a value. The
// enumerated branch rendered `strings.Join(Allowed(key), ", ")` unconditionally,
// and `commit.subject_max` is a number with a rule rather than a set — so the
// sentence came out as "to one of , or remove it", which tells the reader
// nothing and breaks the rule that a degradation sentence names a remedy that
// works. Raised independently by all three review sources.
func TestRemedyForANonEnumeratedKeyNamesTheRule(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, repoconfig.RelPath,
		"version = 1\n\n[commit]\nsubject_max = 0\n")

	checks := findAll(Run(Options{Repo: repo}), "config values")
	if len(checks) != 1 {
		t.Fatalf("got %d config-value checks, want 1: %+v", len(checks), checks)
	}
	remedy := checks[0].Remedy
	if strings.Contains(remedy, "one of ,") || strings.Contains(remedy, "to one of  ") {
		t.Fatalf("remedy renders an empty allowed-set: %q", remedy)
	}
	// The rule has one producer; the remedy must be quoting it, not rewording it.
	if want := repoconfig.Rule("commit.subject_max"); !strings.Contains(remedy, want) {
		t.Errorf("remedy %q does not name the rule %q", remedy, want)
	}
}

// doctor is the human-run report, and a config file that is present but cannot
// be read is precisely the state a user cannot diagnose from the file's
// contents. Returning on Load's error reported nothing at all.
func TestAnUnreadableConfigIsReportedNotSkipped(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, repoconfig.RelPath, "version = 1\n")
	path := repoconfig.Path(repo.Toplevel)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	checks := findAll(Run(Options{Repo: repo}), "config values")
	if len(checks) != 1 {
		t.Fatalf("got %d config-value checks, want 1: %+v", len(checks), checks)
	}
	if checks[0].Status != Warn {
		t.Errorf("status = %q, want warn: an unreadable config stops nothing", checks[0].Status)
	}
	if checks[0].Remedy == "" {
		t.Errorf("no remedy on %q", checks[0].Detail)
	}
}
