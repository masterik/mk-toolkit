package cli

import (
	"bytes"
	"strings"
	"testing"
)

// Tests in this package drive the real cobra root: argv in, stdout and exit
// code out — the interface the skills call, not the commands' internals.

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

// `review` dedupes its findings in prose now (#51); the `findings` pipeline is
// gone, and a binary that still answered it would invite a skill to call it.
func TestFindingsIsAnUnknownCommand(t *testing.T) {
	for _, args := range [][]string{{"findings"}, {"findings", "schema", "--json"}} {
		got := run(t, args...)
		if got.code != 2 {
			t.Fatalf("%v: exit %d, want 2 (usage)\nstdout:\n%s\nstderr:\n%s", args, got.code, got.stdout, got.stderr)
		}
		if !strings.Contains(got.stderr, `unknown command "findings"`) {
			t.Errorf("%v: stderr does not name the unknown command:\n%s", args, got.stderr)
		}
	}
}

// The gate left the binary (#48): skills detect it from the shared
// quality-gate reference and run its commands themselves.
func TestGateIsAnUnknownCommand(t *testing.T) {
	for _, args := range [][]string{{"gate"}, {"gate", "detect"}, {"gate", "run", "--", "true"}} {
		got := run(t, args...)
		if got.code != 2 {
			t.Fatalf("%v: exit %d, want 2 (usage)\nstdout:\n%s\nstderr:\n%s", args, got.code, got.stdout, got.stderr)
		}
		if !strings.Contains(got.stderr, `unknown command "gate"`) {
			t.Errorf("%v: stderr does not name the unknown command:\n%s", args, got.stderr)
		}
	}
}
