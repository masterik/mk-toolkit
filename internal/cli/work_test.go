package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runIn is findings_test.go's run, with a working directory: `work` answers about
// the repository the process is standing in, so the test has to stand somewhere.
func runIn(t *testing.T, dir string, args ...string) (stdout string, code int) {
	t.Helper()
	t.Chdir(dir)
	res := run(t, args...)
	return res.stdout, res.code
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main", "."}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// No payload in reach, so the fingerprint is a reported absence rather than
	// whatever happens to be installed on the machine running the suite.
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "no-such-config"))
	return dir
}

func TestWorkShowJSONShape(t *testing.T) {
	dir := newRepo(t)
	if _, code := runIn(t, dir, "work", "append", "--step", "spec", "--gist", "the plan"); code != 0 {
		t.Fatalf("append exited %d", code)
	}

	out, code := runIn(t, dir, "work", "show", "--json")
	if code != 0 {
		t.Fatalf("show exited %d", code)
	}
	var got struct {
		Branch      string `json:"branch"`
		Path        string `json:"path"`
		Fingerprint string `json:"fingerprint"`
		Records     []struct {
			Step        string   `json:"step"`
			TS          string   `json:"ts"`
			Fingerprint string   `json:"fingerprint"`
			Gist        string   `json:"gist"`
			Assumptions []string `json:"assumptions"`
			Schema      int      `json:"schema"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got.Branch != "main" || !strings.Contains(got.Path, filepath.Join(".mkit", "work", "main-")) {
		t.Errorf("branch/path: %+v", got)
	}
	if len(got.Records) != 1 || got.Records[0].Step != "spec" || got.Records[0].Gist != "the plan" {
		t.Fatalf("records: %+v", got.Records)
	}
	// Present-and-empty, never omitted: a reader must be able to tell "no
	// fingerprint" from "a field I do not know about".
	if !strings.Contains(out, `"fingerprint"`) || !strings.Contains(out, `"assumptions": []`) {
		t.Errorf("a degraded field was omitted rather than reported:\n%s", out)
	}
	// The envelope carries the *current* tree's fingerprint, which is what makes a
	// record's own fingerprint answerable: "does this gist still describe the tree?"
	if !strings.Contains(out, `"cause"`) {
		t.Errorf("no current fingerprint and no cause for its absence:\n%s", out)
	}
	if got.Records[0].Schema != 1 || got.Records[0].TS == "" {
		t.Errorf("binary-set fields: %+v", got.Records[0])
	}
}

// Being first on a branch is the normal case for an entry-capable step.
func TestWorkShowOnAnEmptyBranchExitsZero(t *testing.T) {
	out, code := runIn(t, newRepo(t), "work", "show", "--json")
	if code != 0 {
		t.Fatalf("exited %d on a branch nothing has run on", code)
	}
	if !strings.Contains(out, `"records": []`) {
		t.Errorf("want an empty record list, got:\n%s", out)
	}
}

func TestWorkAppendJSONReportsTheDegradedFingerprint(t *testing.T) {
	out, code := runIn(t, newRepo(t), "work", "append", "--step", "review", "--gist", "two findings",
		"--artifact", "https://example.invalid/pr/1", "--assume", "goal from branch name", "--json")
	if code != 0 {
		t.Fatalf("exited %d\n%s", code, out)
	}
	var got struct {
		Branch string `json:"branch"`
		Cause  string `json:"cause"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got.Branch != "main" {
		t.Errorf("branch: %q", got.Branch)
	}
	// The payload is out of reach here, and that is a named cause rather than a
	// silent empty field.
	if got.Cause == "" {
		t.Error("no fingerprint and no cause — a degradation went unreported")
	}
}

// Exit 2 is "you asked for something that cannot be asked for": retrying cannot
// help, which is the distinction a skill needs from a condition in the repo.
func TestWorkUsageErrorsExitTwo(t *testing.T) {
	dir := newRepo(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unknown step", []string{"work", "append", "--step", "reviw", "--gist", "x"}},
		{"missing gist", []string{"work", "append", "--step", "review"}},
		{"empty gist", []string{"work", "append", "--step", "review", "--gist", ""}},
		{"whitespace gist", []string{"work", "append", "--step", "review", "--gist", "   "}},
	} {
		if _, code := runIn(t, dir, tc.args...); code != 2 {
			t.Errorf("%s exited %d, want 2", tc.name, code)
		}
	}
}

// Exit 1, not 2: no work tree is the environment, not a mistake at the command
// line — the same status `mkit repo profile` already returns there.
func TestWorkOutsideAWorkTreeExitsOne(t *testing.T) {
	outside := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))
	for _, args := range [][]string{
		{"work", "show", "--json"},
		{"work", "append", "--step", "review", "--gist", "x"},
	} {
		if _, code := runIn(t, outside, args...); code != 1 {
			t.Errorf("%v outside a work tree exited %d, want 1", args, code)
		}
	}
}

// The binary is what reads and writes the log; no skill parses the file by hand,
// which only holds if both verbs agree on where it is.
func TestShowAndAppendAgreeOnThePath(t *testing.T) {
	dir := newRepo(t)
	cmd := exec.Command("git", "checkout", "-q", "-b", "feature/a/b")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}

	if _, code := runIn(t, dir, "work", "append", "--step", "commit", "--gist", "on a slashed branch"); code != 0 {
		t.Fatalf("append exited %d", code)
	}
	out, code := runIn(t, dir, "work", "show", "--json")
	if code != 0 || !strings.Contains(out, "on a slashed branch") {
		t.Fatalf("show did not read back what append wrote (exit %d):\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".mkit", "work", "feature")); err == nil {
		t.Error("a slash in the branch name became a directory")
	}
}
