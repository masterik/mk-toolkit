package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An old ledger is a leftover `doctor` names, with the `rm` that removes it.
func TestDoctorReportsAnOldGateLedger(t *testing.T) {
	repo := testRepo(t)
	ledger := filepath.Join(repo, ".mkit", "gate.jsonl")
	put(t, repo, filepath.Join(".mkit", "gate.jsonl"), `{"step":"test"}`+"\n")

	res := run(t, "doctor", "--json")
	if res.code != 0 {
		t.Fatalf("doctor exited %d: %s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "old gate ledger") ||
		!strings.Contains(res.stdout, "rm -r '"+ledger+"'") {
		t.Errorf("doctor did not report the ledger with an rm:\n%s", res.stdout)
	}
	if _, err := os.Stat(ledger); err != nil {
		t.Errorf("doctor must not remove anything: %v", err)
	}
}

// `.mkit/` left the design (#50). Everything an older mkit put there, and the
// ignore pair it wrote into info/exclude, is a leftover `doctor` names with its
// remedy — and leaves exactly where it was.
func TestDoctorReportsEveryOldMkitLeftover(t *testing.T) {
	repo := testRepo(t)
	runA := filepath.Join(repo, ".mkit", "review-20260819T111347Z-RPfCbj")
	runB := filepath.Join(repo, ".mkit", "commit-20260820T090000Z-abc123")
	put(t, runA, "findings-claude.md", "x\n")
	put(t, runB, "gate-test.log", "x\n")
	put(t, repo, filepath.Join(".mkit", "worklog", "main.jsonl"), "{}\n")
	put(t, repo, filepath.Join(".mkit", "gate.jsonl"), "{}\n")
	put(t, repo, filepath.Join(".mkit", "config.toml"), "version = 1\n")
	exclude := filepath.Join(repo, ".git", "info", "exclude")
	excludeBody := "# git ls-files --others --exclude-from=.git/info/exclude\n" +
		"\n# mkit scratch root (run directories + gate.jsonl). Added by mkit.\n" +
		"# config.toml is repo config and stays committable — the pair is the rule.\n" +
		".mkit/*\n!.mkit/config.toml\n"
	put(t, repo, filepath.Join(".git", "info", "exclude"), excludeBody)
	before := gateGit(t, repo, "status", "--porcelain", "--ignored")

	res := run(t, "doctor", "--json")
	if res.code != 0 {
		t.Fatalf("doctor exited %d: %s", res.code, res.stderr)
	}
	for _, want := range []string{
		`"old run directories"`, "rm -r '" + runB + "' '" + runA + "'",
		`"worklog directory"`, "rm -r '" + filepath.Join(repo, ".mkit", "worklog") + "'",
		`"old gate ledger"`, "rm -r '" + filepath.Join(repo, ".mkit", "gate.jsonl") + "'",
		`"old repo config"`, "run the `setup` skill",
		"git rm '" + filepath.Join(repo, ".mkit", "config.toml") + "'",
		`"old ignore pair"`, "delete these lines from '" + exclude + "'",
		"`.mkit/*`", "`!.mkit/config.toml`",
	} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("doctor --json lacks %q:\n%s", want, res.stdout)
		}
	}

	if got := gateGit(t, repo, "status", "--porcelain", "--ignored"); got != before {
		t.Errorf("doctor changed the work tree:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	if b, err := os.ReadFile(exclude); err != nil || string(b) != excludeBody {
		t.Errorf("doctor must not edit info/exclude (err %v):\n%s", err, b)
	}
}

// A user's own rules in info/exclude are theirs: only the lines mkit wrote count.
func TestDoctorIgnoresAnUnrelatedExcludeFile(t *testing.T) {
	repo := testRepo(t)
	put(t, repo, filepath.Join(".git", "info", "exclude"), "*.log\n.idea/\n")

	res := run(t, "doctor", "--json")
	if res.code != 0 {
		t.Fatalf("doctor exited %d: %s", res.code, res.stderr)
	}
	if strings.Contains(res.stdout, "old ignore pair") {
		t.Errorf("doctor reported a user's own exclude rules as mkit's:\n%s", res.stdout)
	}
}
