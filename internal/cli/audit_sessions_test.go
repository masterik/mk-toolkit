package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// auditHome is a Claude home under the test's temp dir with one transcript in
// it, reached through CLAUDE_HOME — never the real ~/.claude.
func auditHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CLAUDE_HOME", home)
	dir := filepath.Join(home, "projects", "-p-app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"cwd":"/r/app","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"bun run test","dangerouslyDisableSandbox":true}}]}}`,
		`{"message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`,
		`{"message":{"content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"go mod download"}}]}}`,
		`{"message":{"content":[{"type":"tool_result","tool_use_id":"t2","content":"<sandbox_violations>\ndeny network-outbound proxy.golang.org:443 (x)\n</sandbox_violations>","is_error":true}]}}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAuditSessionsHuman(t *testing.T) {
	auditHome(t)
	res := run(t, "audit", "sessions")
	if res.code != 0 {
		t.Fatalf("exit %d: %s%s", res.code, res.stdout, res.stderr)
	}
	for _, want := range []string{"transcripts=1\n", "sandbox_blocks=1\n", "overrides=1\n", "overrides_preemptive=1\n",
		"network-outbound proxy.golang.org:443", "bun run"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, res.stdout)
		}
	}
}

func TestAuditSessionsJSONOmitsEventsUnlessAsked(t *testing.T) {
	auditHome(t)
	for _, tc := range []struct {
		args   []string
		events int
	}{{[]string{"audit", "sessions", "--json"}, 0}, {[]string{"audit", "sessions", "--json", "--events"}, 2}} {
		res := run(t, tc.args...)
		if res.code != 0 {
			t.Fatalf("%v: exit %d: %s", tc.args, res.code, res.stderr)
		}
		var rep struct {
			Counts struct {
				Overrides int `json:"overrides"`
			} `json:"counts"`
			Projects []struct {
				Paths []string `json:"paths"`
			} `json:"projects"`
			Events []json.RawMessage `json:"events"`
		}
		if err := json.Unmarshal([]byte(res.stdout), &rep); err != nil {
			t.Fatalf("%v: %v\n%s", tc.args, err, res.stdout)
		}
		if len(rep.Projects) != 1 || strings.Join(rep.Projects[0].Paths, ",") != "/r/app" {
			t.Errorf("%v: projects = %+v, want the transcript's cwd", tc.args, rep.Projects)
		}
		if rep.Counts.Overrides != 1 || len(rep.Events) != tc.events {
			t.Errorf("%v: overrides=%d events=%d, want 1 and %d", tc.args, rep.Counts.Overrides, len(rep.Events), tc.events)
		}
	}
}

func TestAuditSessionsRejectsANonPositiveWindow(t *testing.T) {
	auditHome(t)
	if res := run(t, "audit", "sessions", "--days", "0"); res.code != 2 {
		t.Errorf("exit %d, want 2", res.code)
	}
}
