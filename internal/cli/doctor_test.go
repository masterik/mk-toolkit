package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An old ledger is a leftover `doctor` names, with the `rm` that removes it.
func TestDoctorReportsAnOldGateLedger(t *testing.T) {
	repo, _ := gateRepo(t)
	t.Setenv("MKIT_HOME", filepath.Join(repo, ".mkit-home"))
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
