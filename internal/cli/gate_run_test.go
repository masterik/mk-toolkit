package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The ported cases from tests/bats/gate-run.bats, which was `gate-run.sh`'s
// spec. They exercise the interface the skills call — argv in, stdout, exit code,
// the run directory's logs and the ledger out.

func gateGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// gateRepo is a throwaway repo under $TMPDIR with a run directory open, and the
// process working directory moved into it — never the real home, and never a
// repo a developer is working in.
func gateRepo(t *testing.T) (repo, rd string) {
	t.Helper()
	repo = t.TempDir()
	gateGit(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gateGit(t, repo, "add", "-A")
	gateGit(t, repo, "commit", "-q", "-m", "base")
	rd = filepath.Join(repo, ".mkit", "run1")
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	return repo, rd
}

func ledgerPath(repo string) string { return filepath.Join(repo, ".mkit", "gate.jsonl") }

func ledgerRecords(t *testing.T, repo string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(ledgerPath(repo))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("ledger line is not JSON: %q (%v)", line, err)
		}
		out = append(out, m)
	}
	return out
}

func steps(recs []map[string]any) string {
	var s []string
	for _, r := range recs {
		s = append(s, r["step"].(string))
	}
	return strings.Join(s, " ")
}

func TestGateRunPassingStep(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "lint", "--", "true")
	if res.code != 0 {
		t.Fatalf("exit %d: %s%s", res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "lint ok") || !strings.Contains(res.stdout, "gate=ok steps=lint") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
}

// The whole point of the log: the command's full output goes there, and what
// reaches the agent is a verdict plus a bounded excerpt.
func TestGateRunFailingStepExitsWithTheCommandsOwnCode(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "test", "--", "bash", "-c", "echo boom; exit 7")
	if res.code != 7 {
		t.Fatalf("exit = %d, want 7", res.code)
	}
	if !strings.Contains(res.stdout, "test FAIL exit=7") ||
		!strings.Contains(res.stdout, "gate=FAILED step=test") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
	b, err := os.ReadFile(filepath.Join(rd, "gate-test.log"))
	if err != nil || !strings.Contains(string(b), "boom") {
		t.Errorf("log = %q, %v", b, err)
	}
	// Nothing the verdict says may reach stderr: a skill parses stdout.
	if res.stderr != "" {
		t.Errorf("stderr = %q, want empty — the verdict is already on stdout", res.stderr)
	}
}

// Outside ExitError's 0/1/2 vocabulary, which is the point: `mkit gate run`
// exits with whatever the step exited with.
func TestGateRunPassesThroughUnusualExitCodes(t *testing.T) {
	_, rd := gateRepo(t)
	for _, want := range []int{3, 127} {
		res := run(t, "gate", "run", rd, "s", "--", "bash", "-c", "exit "+strconv.Itoa(want))
		if res.code != want {
			t.Errorf("exit = %d, want %d", res.code, want)
		}
	}
	// And a command that does not exist at all is bash's own 127.
	res := run(t, "gate", "run", rd, "s", "--", "no-such-command-xyz")
	if res.code != 127 {
		t.Errorf("missing command exit = %d, want 127", res.code)
	}
}

func TestGateRunSurfacesAFailureLine(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "test", "--", "bash", "-c", `echo "AssertionError: nope"; exit 1`)
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.stdout, "AssertionError: nope") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
}

func TestGateRunRejectsAnEmptyCommand(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "--chain", "lint=")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2", res.code)
	}
	if !strings.Contains(res.stderr, "empty command") {
		t.Errorf("stderr = %q", res.stderr)
	}
}

func TestGateRunChainStopsAtTheFirstFailure(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "--chain", "a=true", "b=exit 1", "c=touch should-not-run")
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	for _, want := range []string{"a ok", "b FAIL"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, res.stdout)
		}
	}
	if strings.Contains(res.stdout, "c ") {
		t.Errorf("a step after the failure ran:\n%s", res.stdout)
	}
	if _, err := os.Stat(filepath.Join(rd, "should-not-run")); err == nil {
		t.Error("the third step ran")
	}
}

func TestGateRunKeepGoingRunsEveryStep(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "--keep-going", "--chain", "a=true", "b=exit 3", "c=true")
	if res.code != 3 {
		t.Fatalf("exit = %d, want 3", res.code)
	}
	for _, want := range []string{"a ok", "b FAIL", "c ok", "gate=FAILED step=b exit=3"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, res.stdout)
		}
	}
}

// `$*` would flatten argv on spaces and bash would re-split it, so
// `-- printf '[%s]\n' 'foo bar'` used to run as two arguments.
func TestGateRunQuotingSurvivesArgv(t *testing.T) {
	_, rd := gateRepo(t)
	for _, arg := range []string{"foo bar", "it's fine"} {
		res := run(t, "gate", "run", rd, "step", "--", "printf", `[%s]\n`, arg)
		if res.code != 0 {
			t.Fatalf("exit = %d: %s%s", res.code, res.stdout, res.stderr)
		}
		b, err := os.ReadFile(filepath.Join(rd, "gate-step.log"))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.TrimRight(string(b), "\n"), "["+arg+"]"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	}
}

func TestGateRunTailIsBounded(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "--tail", "3", "test", "--",
		"bash", "-c", `for i in $(seq 1 50); do echo "line $i"; done; exit 1`)
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.stdout, "tail -3:") || !strings.Contains(res.stdout, "line 50") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
	if strings.Contains(res.stdout, "line 47") {
		t.Errorf("the tail was not bounded:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "log_lines=50") {
		t.Errorf("the full length was not reported:\n%s", res.stdout)
	}
}

func TestGateRunRejectsARunDirectoryThatDoesNotExist(t *testing.T) {
	repo, _ := gateRepo(t)
	res := run(t, "gate", "run", filepath.Join(repo, "no-such-dir"), "lint", "--", "true")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2", res.code)
	}
	if !strings.Contains(res.stderr, "mkit run open") {
		t.Errorf("the remedy must name the command that opens one: %q", res.stderr)
	}
}

func TestGateRunRejectsAStepNameThatIsNotASlug(t *testing.T) {
	_, rd := gateRepo(t)
	if res := run(t, "gate", "run", rd, "bad step", "--", "true"); res.code != 2 {
		t.Errorf("exit = %d, want 2", res.code)
	}
	if res := run(t, "gate", "run", rd, "--chain", "noequals"); res.code != 2 {
		t.Errorf("a spec with no separator: exit = %d, want 2", res.code)
	}
}

// --- the gate ledger -------------------------------------------------------

func TestGateRunAppendsOneRecordPerFinishedStep(t *testing.T) {
	repo, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "lint", "--", "true")
	if res.code != 0 {
		t.Fatalf("exit %d", res.code)
	}
	recs := ledgerRecords(t, repo)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	r := recs[0]
	if r["kind"] != "gate" || r["step"] != "lint" || r["exit"].(float64) != 0 {
		t.Errorf("record = %v", r)
	}
	if fp, _ := r["fingerprint"].(string); fp == "" {
		t.Error("record carries no fingerprint")
	}
	logPath, _ := r["log"].(string)
	if logPath != filepath.Join(rd, "gate-lint.log") {
		t.Errorf("log = %q, want %q", logPath, filepath.Join(rd, "gate-lint.log"))
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("the log it names does not exist: %v", err)
	}
	// `review-20260819T111347Z-RPfCbj` -> `review`. Diagnostic only.
	if r["skill"] != "run1" {
		t.Errorf("skill = %v, want run1", r["skill"])
	}
}

func TestGateRunRecordsAFailingStepsExitCode(t *testing.T) {
	repo, rd := gateRepo(t)
	run(t, "gate", "run", rd, "test", "--", "bash", "-c", "exit 7")
	recs := ledgerRecords(t, repo)
	if len(recs) != 1 || recs[0]["exit"].(float64) != 7 {
		t.Errorf("records = %v", recs)
	}
}

func TestGateRunLeavesNoRecordForStepsItNeverRan(t *testing.T) {
	repo, rd := gateRepo(t)
	run(t, "gate", "run", rd, "--chain", "a=true", "b=false", "c=true")
	if got := steps(ledgerRecords(t, repo)); got != "a b" {
		t.Errorf("steps = %q, want %q", got, "a b")
	}
}

func TestGateRunKeepGoingRecordsEveryStep(t *testing.T) {
	repo, rd := gateRepo(t)
	run(t, "gate", "run", rd, "--keep-going", "--chain", "a=true", "b=exit 3", "c=true")
	if got := steps(ledgerRecords(t, repo)); got != "a b c" {
		t.Errorf("steps = %q, want %q", got, "a b c")
	}
}

// The flagship cross-skill lookup is keyed on `cmd`: `commit`/`review` call the
// single-step form and `finish`/`pr` call --chain, so if the two forms disagree
// the cache never hits.
func TestGateRunBothCallFormsRecordTheSameCommand(t *testing.T) {
	repo, rd := gateRepo(t)
	run(t, "gate", "run", rd, "lint", "--", "echo", "hello", "world")
	run(t, "gate", "run", rd, "--chain", "lint=echo hello world")
	recs := ledgerRecords(t, repo)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0]["cmd"] != "echo hello world" || recs[1]["cmd"] != "echo hello world" {
		t.Errorf("cmds = %v / %v", recs[0]["cmd"], recs[1]["cmd"])
	}
}

// The join is lossy for an argument with a space — these two execute differently
// and must not share a key, or one could be served the other's proof. A false
// `fresh` is the one direction a ledger may never be wrong in.
func TestGateRunAnArgumentWithASpaceRecordsTheQuotedForm(t *testing.T) {
	repo, rd := gateRepo(t)
	run(t, "gate", "run", rd, "a", "--", "printf", "[%s]", "foo bar")
	run(t, "gate", "run", rd, "b", "--", "printf", "[%s]", "foo", "bar")
	recs := ledgerRecords(t, repo)
	withSpace, without := recs[0]["cmd"].(string), recs[1]["cmd"].(string)
	if withSpace == without {
		t.Fatalf("both recorded %q", withSpace)
	}
	if without != "printf [%s] foo bar" {
		t.Errorf("joinable argv = %q", without)
	}
	if !strings.Contains(withSpace, `'foo bar'`) {
		t.Errorf("quoted form = %q", withSpace)
	}
}

// Losing a cache entry is nothing; failing a gate over its own bookkeeping is
// unacceptable.
func TestGateRunLedgerChangesNeitherExitCodeNorOutput(t *testing.T) {
	_, rd := gateRepo(t)
	with := run(t, "gate", "run", rd, "test", "--", "bash", "-c", "echo boom; exit 7")
	without := run(t, "gate", "run", rd, "--no-ledger", "test", "--", "bash", "-c", "echo boom; exit 7")
	if with.code != without.code || with.stdout != without.stdout {
		t.Errorf("--no-ledger changed the result:\n%d %q\n%d %q",
			with.code, with.stdout, without.code, without.stdout)
	}
}

func TestGateRunNoLedgerWritesNothing(t *testing.T) {
	repo, rd := gateRepo(t)
	run(t, "gate", "run", rd, "--no-ledger", "lint", "--", "true")
	if _, err := os.Stat(ledgerPath(repo)); !os.IsNotExist(err) {
		t.Errorf("a ledger exists: %v", err)
	}
}

func TestGateRunAnUnwritableLedgerDoesNotFailTheGate(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	repo, rd := gateRepo(t)
	mk := filepath.Join(repo, ".mkit")
	if err := os.Chmod(mk, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(mk, 0o700) })

	res := run(t, "gate", "run", rd, "lint", "--", "true")
	if res.code != 0 {
		t.Fatalf("exit = %d, want 0", res.code)
	}
	if !strings.Contains(res.stdout, "gate=ok steps=lint") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
}

// Otherwise the second record claims proof over content that step never read.
func TestGateRunComputesTheFingerprintOncePerInvocation(t *testing.T) {
	repo, rd := gateRepo(t)
	res := run(t, "gate", "run", rd, "--chain", `a=echo x > newfile.txt`, "b=true")
	if res.code != 0 {
		t.Fatalf("exit = %d: %s%s", res.code, res.stdout, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, "newfile.txt")); err != nil {
		t.Fatalf("the first step did not run: %v", err)
	}
	recs := ledgerRecords(t, repo)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0]["fingerprint"] != recs[1]["fingerprint"] {
		t.Errorf("the fingerprint moved mid-run: %v vs %v", recs[0]["fingerprint"], recs[1]["fingerprint"])
	}
}

// The ledger write can be what first creates `.mkit/` in a repo — a caller
// passing its own run directory, or a repo whose run dirs were pruned.
// Unignored, `git worktree remove` refuses and the fingerprint watches a
// directory change mid-gate.
func TestGateRunIgnoresTheScratchWhenItIsTheFirstToCreateIt(t *testing.T) {
	repo, _ := gateRepo(t)
	if err := os.RemoveAll(filepath.Join(repo, ".mkit")); err != nil {
		t.Fatal(err)
	}
	own := t.TempDir()
	res := run(t, "gate", "run", own, "lint", "--", "true")
	if res.code != 0 {
		t.Fatalf("exit = %d: %s%s", res.code, res.stdout, res.stderr)
	}
	if out := gateGit(t, repo, "status", "--porcelain"); out != "" {
		t.Errorf("the ledger write dirtied the tree:\n%s", out)
	}
}

func TestGateRunJSON(t *testing.T) {
	_, rd := gateRepo(t)
	res := run(t, "gate", "run", "--json", rd, "--keep-going", "--chain", "a=true", "b=exit 3")
	if res.code != 3 {
		t.Fatalf("exit = %d, want 3", res.code)
	}
	var got struct {
		Steps []struct {
			Step     string `json:"step"`
			Cmd      string `json:"cmd"`
			Exit     int    `json:"exit"`
			Log      string `json:"log"`
			LogLines int    `json:"log_lines"`
		} `json:"steps"`
		Gate       string `json:"gate"`
		FailedStep string `json:"failed_step"`
		Exit       int    `json:"exit"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, res.stdout)
	}
	if got.Gate != "FAILED" || got.FailedStep != "b" || got.Exit != 3 {
		t.Errorf("verdict = %+v", got)
	}
	if len(got.Steps) != 2 || got.Steps[0].Step != "a" || got.Steps[1].Exit != 3 {
		t.Errorf("steps = %+v", got.Steps)
	}
}
