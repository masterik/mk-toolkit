package cli

import (
	"bytes"
	"strings"
	"testing"
)

// Tests in this package drive the real cobra root: argv in, stdout and exit
// code out — the interface a person or a skill calls, not the commands' internals.

type result struct {
	stdout string
	stderr string
	code   int
}

func run(t *testing.T, args ...string) result {
	t.Helper()
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	res := result{}
	if err := root.Execute(); err != nil {
		var errBuf bytes.Buffer
		res.code = Fail(&errBuf, err)
		res.stderr = errBuf.String()
	}
	res.stdout = out.String()
	return res
}

// The binary is optional power tools (#53, ADR 0006): `doctor`, `audit
// sandbox`, `cache prune`, `version`. Every verb a skill or the TOML config once
// called is gone, and an old skill calling one must fail loudly with a usage
// error, never a silent success. Where each went: gate (#48, prose in the shared
// quality-gate reference), worklog (#49, recap reads git log and the PR),
// scratch (#50, per-run files live in the temp dir facts.sh creates), findings
// (#51, review merges in prose), facts and branch (#52, plugin scripts), init
// and repo (#53, settings live in docs/agents/, written by the setup skill).
func TestRemovedCommandsAreUnknown(t *testing.T) {
	for _, args := range [][]string{
		{"init"}, {"init", "--yes"},
		{"repo"}, {"repo", "profile", "--json"},
		{"gate"}, {"gate", "detect"}, {"gate", "run", "--", "true"},
		{"worklog", "show"}, {"worklog", "append", "--step", "commit"},
		{"findings"}, {"findings", "schema", "--json"},
		{"scratch"}, {"scratch", "open", "review"}, {"scratch", "prune"},
		{"facts", "commit"},
		{"branch", "status", "--default", "main"},
	} {
		got := run(t, args...)
		if got.code != 2 {
			t.Errorf("%v: exit %d, want 2 (usage)\nstdout:\n%s\nstderr:\n%s", args, got.code, got.stdout, got.stderr)
			continue
		}
		if want := `unknown command "` + args[0] + `"`; !strings.Contains(got.stderr, want) {
			t.Errorf("%v: stderr does not name the unknown command:\n%s", args, got.stderr)
		}
	}
}

// What stays still answers, so the test above cannot pass by breaking the root.
func TestRemainingCommandsAreKnown(t *testing.T) {
	for _, args := range [][]string{
		{"version", "--json"}, {"doctor", "--help"}, {"audit", "sandbox", "--help"}, {"cache", "prune", "--help"},
	} {
		if got := run(t, args...); got.code != 0 {
			t.Errorf("%v: exit %d\n%s%s", args, got.code, got.stdout, got.stderr)
		}
	}
}
