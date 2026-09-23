package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #19's "done when", end to end through the interface the skills call: a
// config.toml with a misspelled table and an invalid merge.style runs every
// command successfully, and `repo profile --json` and `doctor` each name both
// problems and the file they are in.

// brokenConfigRepo is a throwaway repo under $TMPDIR carrying the two defects,
// with every environment lookup pointed inside it — never the developer's home.
func brokenConfigRepo(t *testing.T) string {
	t.Helper()
	repo, _ := gateRepo(t)
	t.Setenv("MKIT_HOME", filepath.Join(repo, ".mkit-home"))
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	put(t, repo, ".gitignore", ".mkit/*\n!.mkit/config.toml\n")
	if err := os.WriteFile(filepath.Join(repo, ".mkit", "config.toml"), []byte(
		"version = 1\n\n[reviewers]\nreviewers = [\"@a\"]\n\n[merge]\nstyle = \"sqaush\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestEveryCommandRunsWithAMalformedConfig(t *testing.T) {
	brokenConfigRepo(t)
	// Config is an input, never a permission: not one of these may exit non-zero
	// because a human mistyped a table name.
	for _, args := range [][]string{
		{"version"},
		{"doctor"},
		{"repo", "profile"},
		{"repo", "profile", "--json"},
		{"gate", "detect"},
		{"facts", "commit", "--no-run"},
		{"work", "show"},
		{"init", "--yes"},
	} {
		if res := run(t, args...); res.code != 0 {
			t.Errorf("mkit %s: exit %d\n%s%s", strings.Join(args, " "), res.code, res.stdout, res.stderr)
		}
	}
}

func TestRepoProfileJSONNamesBothProblemsAndTheFile(t *testing.T) {
	repo := brokenConfigRepo(t)
	res := run(t, "repo", "profile", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	var p struct {
		ConfigProblems []struct {
			Kind, Key, Path, Detail string
		} `json:"config_problems"`
		MergeStyle struct{ Value, Source, Cause string } `json:"merge_style"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &p); err != nil {
		t.Fatalf("%v\n%s", err, res.stdout)
	}
	want := filepath.Join(repo, ".mkit", "config.toml")
	var keys []string
	for _, pb := range p.ConfigProblems {
		if pb.Path != want {
			t.Errorf("problem names %q, want %q", pb.Path, want)
		}
		keys = append(keys, pb.Key)
	}
	for _, k := range []string{"reviewers", "merge.style"} {
		if !strings.Contains(strings.Join(keys, " "), k) {
			t.Errorf("%q is not among the reported problems %v", k, keys)
		}
	}
	// The value itself, not just the list: falling through to `discovered` here is
	// the silent-drop failure this issue exists to remove.
	if p.MergeStyle.Source != "unavailable" || !strings.Contains(p.MergeStyle.Cause, "merge.style") {
		t.Errorf("merge_style = %+v", p.MergeStyle)
	}
}

func TestRepoProfileTextNamesBothProblems(t *testing.T) {
	repo := brokenConfigRepo(t)
	out := run(t, "repo", "profile").stdout
	for _, want := range []string{"reviewers", "merge.style", "sqaush", filepath.Join(repo, ".mkit", "config.toml")} {
		if !strings.Contains(out, want) {
			t.Errorf("`repo profile` never mentions %q:\n%s", want, out)
		}
	}
}

func TestDoctorNamesBothProblemsAndStaysExitZero(t *testing.T) {
	repo := brokenConfigRepo(t)
	res := run(t, "doctor")
	if res.code != 0 {
		t.Fatalf("exit %d — a report with findings has succeeded", res.code)
	}
	for _, want := range []string{"reviewers", "merge.style", "sqaush", filepath.Join(repo, ".mkit", "config.toml")} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("`doctor` never mentions %q:\n%s", want, res.stdout)
		}
	}
}

// `init` must not treat a config it could not fully honour as empty space: the
// file is one typo away from correct, and overwriting it loses the rest.
func TestInitDoesNotOverwriteAConfigItCouldNotHonour(t *testing.T) {
	repo := brokenConfigRepo(t)
	path := filepath.Join(repo, ".mkit", "config.toml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res := run(t, "init", "--yes", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	var got struct{ Written bool }
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, res.stdout)
	}
	if got.Written {
		t.Error("init rewrote a config that was merely malformed")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Errorf("the file changed: %v\n%s", err, after)
	}
}
