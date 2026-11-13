package cli

import (
	"strings"
	"testing"
)

// The worklog was dropped (#49): recap reads git log and the PR instead. An old
// skill calling `mkit worklog` must get a usage error, not a silent success.
func TestWorklogCommandIsGone(t *testing.T) {
	for _, args := range [][]string{{"worklog", "show"}, {"worklog", "append", "--step", "commit"}} {
		res := run(t, args...)
		if res.code == 0 || !strings.Contains(res.stderr+res.stdout, "unknown command") {
			t.Errorf("mkit %s: exit %d, want an unknown-command error\n%s%s",
				strings.Join(args, " "), res.code, res.stdout, res.stderr)
		}
	}
}

// `mkit facts` and `mkit branch status` moved into the plugin as facts.sh and
// branch-status.sh (#52). An old skill calling either must fail loudly.
func TestFactsAndBranchStatusAreGone(t *testing.T) {
	for _, args := range [][]string{{"facts", "commit"}, {"branch", "status", "--default", "main"}} {
		res := run(t, args...)
		if res.code == 0 || !strings.Contains(res.stderr+res.stdout, "unknown command") {
			t.Errorf("mkit %s: exit %d, want an unknown-command error\n%s%s",
				strings.Join(args, " "), res.code, res.stdout, res.stderr)
		}
	}
}
