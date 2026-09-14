package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/worklog"
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
	// Computed in process since M5 — this used to shell out to the payload and
	// report a cause in a test repo, because there was no payload in reach.
	if got.Fingerprint == "" {
		t.Errorf("no current fingerprint in the envelope:\n%s", out)
	}
	if strings.Contains(out, `"cause"`) {
		t.Errorf("a cause was reported for a fingerprint that succeeded:\n%s", out)
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

// The fingerprint reaches the record. This asserted the opposite until M5 — the
// payload was out of reach in a test repo, so the only observable was the cause —
// and the port turned that degradation into the ordinary path.
func TestWorkAppendJSONCarriesTheFingerprint(t *testing.T) {
	out, code := runIn(t, newRepo(t), "work", "append", "--step", "review", "--gist", "two findings",
		"--artifact", "https://example.invalid/pr/1", "--assume", "goal from branch name", "--json")
	if code != 0 {
		t.Fatalf("exited %d\n%s", code, out)
	}
	var got struct {
		Branch string `json:"branch"`
		Path   string `json:"path"`
		Cause  string `json:"cause"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got.Branch != "main" {
		t.Errorf("branch: %q", got.Branch)
	}
	// `cause` is present only when there is no fingerprint, so its absence is the
	// success signal on this envelope — the fingerprint itself lives on the record.
	if got.Cause != "" {
		t.Errorf("a cause was reported for a fingerprint that succeeds now: %q", got.Cause)
	}
	if !strings.Contains(got.Path, filepath.Join(".mkit", "work", "main-")) {
		t.Errorf("path: %q", got.Path)
	}
	// The record is where the fingerprint has to land: it is what a later reader
	// compares against the tree in front of it.
	b, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var rec struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &rec); err != nil {
		t.Fatalf("record is not JSON: %v\n%s", err, b)
	}
	if rec.Fingerprint == "" {
		t.Errorf("the appended record carries no fingerprint:\n%s", b)
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

// A tree this run could not fingerprint is not evidence that anything is stale.
// The bug this pins was a display placeholder assigned over the comparison value:
// `current` became "-", so no record could ever match it and every one of them
// rendered `(stale)` — an assertion about the tree made from its own absence.
func TestWorkShowUnknownWhenTreeHasNoFingerprint(t *testing.T) {
	dir := newRepo(t)
	if _, code := runIn(t, dir, "work", "append", "--step", "spec", "--gist", "the plan"); code != 0 {
		t.Fatalf("append exited %d", code)
	}

	// Driven through the renderer rather than the command. Until M5 this case was
	// reachable from the CLI because `worklog.Fingerprint` shelled out to the
	// payload and a test repo had none in reach; the port computes it in process,
	// so a valid work tree always has one and the empty-current branch has no
	// command-level scenario left. The branch is still real — an unreadable tree
	// reaches it — and it is the one this test exists for, so it is exercised
	// where it lives.
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	log := worklog.Open(repo, "")
	recs, err := log.Show(worklog.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Fingerprint == "" {
		t.Fatalf("fixture needs one record carrying a fingerprint: %+v", recs)
	}

	var buf bytes.Buffer
	renderWorklog(&buf, log, "", recs)
	out := buf.String()

	if !strings.Contains(out, "(unknown)") || strings.Contains(out, "(stale)") {
		t.Errorf("no current fingerprint must read (unknown), not (stale):\n%s", out)
	}
	if !strings.Contains(out, "tree now fp:-") {
		t.Errorf("the header still reports the absence as -:\n%s", out)
	}
}

// A cross-branch append must not carry this tree's fingerprint: `head()` resolves the
// named branch's tip, so the record would pair one branch's head with another's
// content, and a later reader on that branch compares fingerprints to decide whether
// the gist still describes what it sees.
func TestWorkAppendCrossBranchCarriesNoFingerprint(t *testing.T) {
	dir := newRepo(t)
	cmd := exec.Command("git", "branch", "other")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}

	out, code := runIn(t, dir, "work", "append", "--step", "spec", "--gist", "x", "--branch", "other", "--json")
	if code != 0 {
		t.Fatalf("append exited %d: %s", code, out)
	}
	var got struct {
		Branch string `json:"branch"`
		Cause  string `json:"cause"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if got.Branch != "other" {
		t.Fatalf("recorded against %q, want other", got.Branch)
	}
	if !strings.Contains(got.Cause, "other") || !strings.Contains(got.Cause, "no fingerprint") {
		t.Errorf("the cause does not name the cross-branch reason: %q", got.Cause)
	}

	// The guard below is only worth anything if this tree *can* fingerprint —
	// otherwise it passes for the wrong reason and would keep passing through a
	// regression that recorded the current tree's hash on a cross-branch record.
	// So put a payload in reach and prove the same-branch append gets one.
	t.Setenv("CLAUDE_PLUGIN_ROOT", payloadDir(t))
	mine, code := runIn(t, dir, "work", "append", "--step", "spec", "--gist", "y", "--json")
	if code != 0 {
		t.Fatalf("same-branch append exited %d: %s", code, mine)
	}
	if strings.Contains(mine, `"cause"`) {
		t.Skipf("this machine cannot fingerprint, so the cross-branch guard proves nothing:\n%s", mine)
	}

	out2, code := runIn(t, dir, "work", "append", "--step", "spec", "--gist", "z", "--branch", "other", "--json")
	if code != 0 {
		t.Fatalf("append exited %d: %s", code, out2)
	}
	recs, _ := runIn(t, dir, "work", "show", "--branch", "other", "--json")
	var shown struct {
		Records []struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(recs), &shown); err != nil {
		t.Fatalf("bad json: %v\n%s", err, recs)
	}
	if len(shown.Records) == 0 {
		t.Fatalf("no records on the other branch:\n%s", recs)
	}
	for _, r := range shown.Records {
		if r.Fingerprint != "" {
			t.Errorf("a cross-branch record carried this tree's fingerprint: %q", r.Fingerprint)
		}
	}
}

// A detached HEAD is still the tree in front of us. Its log is named
// `detached~<sha>` while Repo.Branch() is "", so a cross-branch test comparing the two
// resolved names would call every detached append cross-branch and throw the
// fingerprint away.
func TestWorkAppendOnDetachedHeadKeepsItsFingerprint(t *testing.T) {
	dir := newRepo(t)
	t.Setenv("CLAUDE_PLUGIN_ROOT", payloadDir(t))

	cmd := exec.Command("git", "checkout", "-q", "--detach", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout --detach: %v\n%s", err, out)
	}

	out, code := runIn(t, dir, "work", "append", "--step", "spec", "--gist", "x", "--json")
	if code != 0 {
		t.Fatalf("append exited %d: %s", code, out)
	}
	var got struct {
		Branch string `json:"branch"`
		Cause  string `json:"cause"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if !strings.HasPrefix(got.Branch, "detached") {
		t.Fatalf("detached append landed on %q", got.Branch)
	}
	if strings.Contains(got.Cause, "not checked out") {
		t.Errorf("a detached HEAD was treated as a cross-branch append: %q", got.Cause)
	}
}

// repoPayload is resolved at package load, before any test can `t.Chdir` into a
// throwaway repo — a relative path resolved later points at the temp dir instead.
var repoPayload = func() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(wd, "..", "..", "plugin")
}()

// payloadDir is the repo's own plugin payload, so a test can exercise the paths that
// need `lib/common.sh` — the fingerprint above all.
func payloadDir(t *testing.T) string {
	t.Helper()
	if repoPayload == "" {
		t.Skip("could not resolve the payload directory")
	}
	if _, err := os.Stat(filepath.Join(repoPayload, "scripts", "lib", "common.sh")); err != nil {
		t.Skipf("payload not in reach: %v", err)
	}
	return repoPayload
}
