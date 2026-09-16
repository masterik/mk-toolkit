package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #18's "done when", through the interface `commit` and `pr` call: a repo
// with scopes and reviewers pinned reports them as `pinned` in
// `mkit repo profile --json`, which is what lets those skills use them without
// re-deriving either.

type profileJSON struct {
	Scopes struct {
		Values []string `json:"values"`
		Source string   `json:"source"`
	} `json:"commit_scopes"`
	Reviewers struct {
		Values []string `json:"values"`
		Source string   `json:"source"`
	} `json:"reviewers"`
	SubjectMax value `json:"commit_subject_max"`
	ReviewMode value `json:"review_mode"`
}

type value struct {
	Value  string `json:"value"`
	Source string `json:"source"`
	Cause  string `json:"cause"`
}

func profileOf(t *testing.T) profileJSON {
	t.Helper()
	res := run(t, "repo", "profile", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d: %s%s", res.code, res.stdout, res.stderr)
	}
	var p profileJSON
	if err := json.Unmarshal([]byte(res.stdout), &p); err != nil {
		t.Fatalf("repo profile --json is not JSON: %v\n%s", err, res.stdout)
	}
	return p
}

// configRepo is a throwaway repo under $TMPDIR with every environment lookup
// pointed inside it — never the developer's home.
func configRepo(t *testing.T) string {
	t.Helper()
	repo, _ := gateRepo(t)
	t.Setenv("MKIT_HOME", filepath.Join(repo, ".mkit-home"))
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	put(t, repo, ".gitignore", ".mkit/*\n!.mkit/config.toml\n")
	return repo
}

func writeConfig(t *testing.T, repo, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".mkit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".mkit", "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedScopesAndReviewersComeBackTaggedPinned(t *testing.T) {
	repo := configRepo(t)
	writeConfig(t, repo, "version = 1\n\n[commit]\nscopes = [\"cli\", \"core\"]\nsubject_max = 72\n\n"+
		"[review]\nreviewers = [\"@a\", \"@b\"]\nmode = \"quick\"\n")
	p := profileOf(t)
	if p.Scopes.Source != "pinned" || strings.Join(p.Scopes.Values, ",") != "cli,core" {
		t.Errorf("commit_scopes = %+v", p.Scopes)
	}
	if p.Reviewers.Source != "pinned" || strings.Join(p.Reviewers.Values, ",") != "@a,@b" {
		t.Errorf("reviewers = %+v", p.Reviewers)
	}
	if p.SubjectMax.Source != "pinned" || p.SubjectMax.Value != "72" {
		t.Errorf("commit_subject_max = %+v", p.SubjectMax)
	}
	if p.ReviewMode.Source != "pinned" || p.ReviewMode.Value != "quick" {
		t.Errorf("review_mode = %+v", p.ReviewMode)
	}
}

// With nothing pinned the profile still answers — that is the fallback the
// skills degrade to, and it is today's discovery rather than an error.
func TestNothingPinnedStillAnswers(t *testing.T) {
	configRepo(t)
	p := profileOf(t)
	for _, tc := range []struct {
		name, source string
	}{
		{"commit_scopes", p.Scopes.Source},
		{"reviewers", p.Reviewers.Source},
		{"commit_subject_max", p.SubjectMax.Source},
		{"review_mode", p.ReviewMode.Source},
	} {
		if tc.source == "pinned" {
			t.Errorf("%s: tagged pinned with no config file", tc.name)
		}
	}
	// A plain absent answer must not name the config file: that is how a skill
	// tells "no answer" (stay silent) from "your pin was rejected" (say so).
	for _, c := range []string{p.SubjectMax.Cause, p.ReviewMode.Cause} {
		if strings.Contains(c, "config.toml") {
			t.Errorf("an absent answer names the config file, which reads as a rejected pin: %q", c)
		}
	}
}

func TestARejectedPinIsReportedNotSilentlyReplaced(t *testing.T) {
	repo := configRepo(t)
	writeConfig(t, repo, "version = 1\n\n[commit]\nsubject_max = -1\n\n[review]\nmode = \"fast\"\n")
	p := profileOf(t)
	for _, tc := range []struct {
		name string
		v    value
		key  string
	}{
		{"commit_subject_max", p.SubjectMax, "commit.subject_max"},
		{"review_mode", p.ReviewMode, "review.mode"},
	} {
		if tc.v.Source != "unavailable" || tc.v.Value != "" {
			t.Errorf("%s = %+v, want unavailable with no value", tc.name, tc.v)
		}
		if !strings.Contains(tc.v.Cause, tc.key) || !strings.Contains(tc.v.Cause, "config.toml") {
			t.Errorf("%s: cause does not name the key and the file: %q", tc.name, tc.v.Cause)
		}
	}
	// And nothing failed: config is an input, never a permission.
	if res := run(t, "doctor"); res.code != 0 {
		t.Errorf("doctor exited %d on a rejected pin", res.code)
	}
}

// The init front end, since a skill can drive it: both new fields are flags, and
// both are validated before anything reaches a committed file.
func TestInitPinsAndValidatesTheNewFields(t *testing.T) {
	repo := configRepo(t)
	if res := run(t, "init", "--yes", "--subject-max", "72", "--review-mode", "quick",
		"--scope", "cli", "--reviewer", "@a"); res.code != 0 {
		t.Fatalf("init exited %d: %s%s", res.code, res.stdout, res.stderr)
	}
	b, err := os.ReadFile(filepath.Join(repo, ".mkit", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	// Quoting is the marshaller's business, so the assertion is on the key and
	// the value, not on which quote character it chose.
	for _, want := range []string{"subject_max = 72", "mode = ", "quick"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("written config lacks %q:\n%s", want, b)
		}
	}
	p := profileOf(t)
	if p.SubjectMax.Source != "pinned" || p.ReviewMode.Source != "pinned" {
		t.Errorf("what init wrote did not come back pinned: %+v %+v", p.SubjectMax, p.ReviewMode)
	}
}

func TestInitRefusesAValueItWouldHaveToWriteAndThenReject(t *testing.T) {
	configRepo(t)
	for _, args := range [][]string{
		{"init", "--yes", "--subject-max", "-5"},
		{"init", "--yes", "--review-mode", "fast"},
	} {
		res := run(t, args...)
		if res.code == 0 {
			t.Errorf("mkit %s succeeded; a typo pinned into a committed file has a long life",
				strings.Join(args, " "))
		}
	}
}
