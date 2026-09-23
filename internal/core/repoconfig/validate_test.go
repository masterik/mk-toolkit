package repoconfig

import (
	"strings"
	"testing"
)

// Issue #19: a plain toml.Unmarshal dropped unknown keys and never checked the
// enumerated fields, so a misspelled table vanished and `style = "sqaush"` was
// accepted by the parser and ignored forever. These cases are that gap.

// loadFile writes a config into a throwaway work tree and loads it. Nothing here
// touches the real home: the tree is t.TempDir().
func loadFile(t *testing.T, body string) *Config {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, RelPath, body)
	c, present, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an error for a file it should have reported on: %v", err)
	}
	if !present {
		t.Fatal("present = false for a file that exists")
	}
	return c
}

func TestLoadCollectsEveryUnknownKey(t *testing.T) {
	// Three offenders, not one: the whole point of accumulating them is that
	// fixing a hand-edited file is not a game of whack-a-mole.
	c := loadFile(t, `
version = 1

[reviewers]
reviewers = ["@a"]

[bogus]
x = 1

[merge]
style = "merge"
stile = "squash"
`)
	var keys []string
	for _, p := range c.Problems {
		if p.Kind != ProblemUnknownKey {
			t.Errorf("unexpected problem kind %q: %s", p.Kind, p.Detail)
			continue
		}
		keys = append(keys, p.Key)
	}
	for _, want := range []string{"reviewers", "bogus", "merge.stile"} {
		if !contains(keys, want) {
			t.Errorf("unknown key %q not reported; got %v", want, keys)
		}
	}
	// The valid neighbours still decoded — a strict decode that also discarded
	// the good values would be a worse answer than the silent drop it replaces.
	if c.Merge.Style != "merge" {
		t.Errorf("merge.style = %q, want the valid sibling to survive the strict decode", c.Merge.Style)
	}
}

func TestLoadNamesTheFileInEveryProblem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, RelPath, "[typo]\nx = 1\n")
	c, _, _ := Load(dir)
	if len(c.Problems) != 1 {
		t.Fatalf("got %d problems, want 1: %+v", len(c.Problems), c.Problems)
	}
	// "ignored" without "where" is not actionable; the path is the remedy.
	if c.Problems[0].Path != Path(dir) || !strings.Contains(c.Problems[0].Detail, Path(dir)) {
		t.Errorf("problem does not name the config file: %+v", c.Problems[0])
	}
}

func TestLoadRejectsAndClearsBadEnumValues(t *testing.T) {
	c := loadFile(t, "[merge]\nstyle = \"sqaush\"\n\n[spec]\nstore = \"gh-issues\"\n")
	for _, tc := range []struct{ key, bad string }{
		{"merge.style", "sqaush"},
		{"spec.store", "gh-issues"},
	} {
		pb := c.Problem(tc.key)
		if pb == nil {
			t.Fatalf("%s: no problem reported for %q", tc.key, tc.bad)
		}
		if pb.Kind != ProblemInvalidValue || pb.Value != tc.bad {
			t.Errorf("%s: got %+v", tc.key, *pb)
		}
		// The allowed set has to appear, or the reader is told "no" with no "then what".
		for _, a := range Allowed(tc.key) {
			if !strings.Contains(pb.Detail, a) {
				t.Errorf("%s: detail does not name allowed value %q: %s", tc.key, a, pb.Detail)
			}
		}
	}
	// Cleared, not kept: a skill branching on merge.style must never be handed
	// "sqaush" to act on.
	if c.Merge.Style != "" || c.Spec.Store != "" {
		t.Errorf("invalid values survived: merge=%q spec=%q", c.Merge.Style, c.Spec.Store)
	}
}

func TestLoadAcceptsEveryAllowedEnumValue(t *testing.T) {
	// Guards the one-producer move: if `init`'s vocabulary and Load's ever drift,
	// a value `mkit init` writes would come back reported as invalid.
	for _, s := range SpecStores {
		if c := loadFile(t, "[spec]\nstore = \""+s+"\"\n"); c.Spec.Store != s || len(c.Problems) != 0 {
			t.Errorf("spec.store %q rejected: %+v", s, c.Problems)
		}
	}
	for _, s := range MergeStyles {
		if c := loadFile(t, "[merge]\nstyle = \""+s+"\"\n"); c.Merge.Style != s || len(c.Problems) != 0 {
			t.Errorf("merge.style %q rejected: %+v", s, c.Problems)
		}
	}
}

func TestWrittenConfigLoadsWithNoProblems(t *testing.T) {
	// The round trip is the real contract: `mkit init` renders from a template
	// rather than marshalling, so nothing but a test proves the strict reader
	// still accepts what the writer emits.
	dir := t.TempDir()
	if err := Write(dir, &Config{
		Gate:   Gate{Commands: map[string]string{"test": "go test ./...", "odd key": "x"}},
		Spec:   Spec{Store: "github-issues", Ref: "o/r"},
		Commit: Commit{Scopes: []string{"cli", "core"}},
		Review: Review{Reviewers: []string{"@a"}},
		Merge:  Merge{Style: "merge"},
	}); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Problems) != 0 {
		t.Fatalf("the file mkit writes does not load cleanly: %+v", c.Problems)
	}
	if c.Version != Version || c.Merge.Style != "merge" || c.Gate.Commands["odd key"] != "x" {
		t.Errorf("round trip lost values: %+v", c)
	}
}

func TestUnparsableFileIsReportedNotReturnedAsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, RelPath, "this is not = = toml\n")
	c, present, err := Load(dir)
	if err != nil || !present {
		t.Fatalf("err = %v, present = %v; config is an input, never a permission", err, present)
	}
	if len(c.Problems) != 1 || c.Problems[0].Kind != ProblemUnparsable {
		t.Fatalf("got %+v", c.Problems)
	}
	if !c.IsZero() {
		t.Error("an unparsable document must decode to nothing pinned")
	}
}

func TestNewerVersionIsReportedNeverRefused(t *testing.T) {
	// The decision, written down in Version's doc comment: refusing would turn a
	// colleague's upgrade into a broken checkout for everyone behind them, and
	// reading is safe because an unknown key is already reported as one.
	c := loadFile(t, "version = 99\n\n[merge]\nstyle = \"squash\"\n")
	pb := c.Problem("version")
	if pb == nil || pb.Kind != ProblemNewerVersion {
		t.Fatalf("a future version was not reported: %+v", c.Problems)
	}
	if c.Merge.Style != "squash" {
		t.Errorf("a future version must not stop the values that parse being used; merge.style = %q", c.Merge.Style)
	}
}

func TestCurrentAndOlderVersionsAreSilent(t *testing.T) {
	for _, v := range []string{"", "version = 1\n", "version = 0\n"} {
		if c := loadFile(t, v+"[merge]\nstyle = \"merge\"\n"); len(c.Problems) != 0 {
			t.Errorf("version %q reported %+v", v, c.Problems)
		}
	}
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
