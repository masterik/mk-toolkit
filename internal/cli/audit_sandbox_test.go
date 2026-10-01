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

func TestAuditSandboxHuman(t *testing.T) {
	auditHome(t)
	res := run(t, "audit", "sandbox")
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

func TestAuditSandboxJSONOmitsEventsUnlessAsked(t *testing.T) {
	auditHome(t)
	for _, tc := range []struct {
		args   []string
		events int
	}{{[]string{"audit", "sandbox", "--json"}, 0}, {[]string{"audit", "sandbox", "--json", "--events"}, 2}} {
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

func TestAuditSandboxRejectsBadLimits(t *testing.T) {
	auditHome(t)
	for _, args := range [][]string{{"--days", "0"}, {"--top", "-1"}} {
		if res := run(t, append([]string{"audit", "sandbox"}, args...)...); res.code != 2 {
			t.Errorf("%v: exit %d, want 2", args, res.code)
		}
	}
}

func TestAuditSandboxNamesUnreadableTranscripts(t *testing.T) {
	auditHome(t)
	bad := filepath.Join(os.Getenv("CLAUDE_HOME"), "projects", "-p-app", "locked.jsonl")
	if err := os.WriteFile(bad, []byte("{}\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	res := run(t, "audit", "sandbox")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "unreadable=1\n") || !strings.Contains(res.stdout, "unreadable:\n  "+bad+"\n") {
		t.Errorf("stdout does not name the unreadable transcript:\n%s", res.stdout)
	}
}

// auditSettings puts a user settings.json in the fixture's Claude home.
func auditSettings(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(os.Getenv("CLAUDE_HOME"), "settings.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAuditSandboxConfigMarksCoveredTargets(t *testing.T) {
	auditHome(t)
	auditSettings(t, `{"sandbox":{"network":{"allowedDomains":["*.golang.org"]}},"sandbox_typo":1}`)
	res := run(t, "audit", "sandbox")
	if res.code != 0 {
		t.Fatalf("exit %d: %s%s", res.code, res.stdout, res.stderr)
	}
	for _, want := range []string{
		"network-outbound proxy.golang.org:443", "covered=", "settings.json:*.golang.org",
		"config_dir=", "claude_md=no", "scope=user", "allowed_domains: *.golang.org",
	} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, res.stdout)
		}
	}
}

func TestAuditSandboxJSONCarriesConfigAndCoverage(t *testing.T) {
	auditHome(t)
	auditSettings(t, `{"sandbox":{"network":{"allowedDomains":["proxy.golang.org"]},"typo":true}}`)
	res := run(t, "audit", "sandbox", "--json")
	var rep struct {
		BlockTargets []struct {
			Key       string `json:"key"`
			CoveredBy *struct {
				File  string `json:"file"`
				Entry string `json:"entry"`
			} `json:"covered_by"`
		} `json:"block_targets"`
		Config struct {
			User struct {
				Scope       string   `json:"scope"`
				UnknownKeys []string `json:"unknown_keys"`
			} `json:"user"`
			Projects []any `json:"projects"`
		} `json:"config"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &rep); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, res.stdout)
	}
	if len(rep.BlockTargets) != 1 || rep.BlockTargets[0].CoveredBy == nil || rep.BlockTargets[0].CoveredBy.Entry != "proxy.golang.org" {
		t.Errorf("block_targets = %+v", rep.BlockTargets)
	}
	if rep.Config.User.Scope != "user" || len(rep.Config.User.UnknownKeys) != 1 || rep.Config.Projects == nil {
		t.Errorf("config = %+v", rep.Config)
	}
}

func TestAuditSandboxNoConfig(t *testing.T) {
	auditHome(t)
	auditSettings(t, `{"sandbox":{"network":{"allowedDomains":["proxy.golang.org"]}}}`)
	res := run(t, "audit", "sandbox", "--no-config")
	if strings.Contains(res.stdout, "config_dir=") || strings.Contains(res.stdout, "covered=") {
		t.Errorf("--no-config still read settings:\n%s", res.stdout)
	}
	if j := run(t, "audit", "sandbox", "--no-config", "--json"); strings.Contains(j.stdout, `"config"`) {
		t.Errorf("--no-config --json carries config:\n%s", j.stdout)
	}
}

func TestAuditSandboxUnreadableSettingsAreNotEmpty(t *testing.T) {
	auditHome(t)
	auditSettings(t, `{broken`)
	res := run(t, "audit", "sandbox")
	if !strings.Contains(res.stdout, "config_unreadable:") || strings.Contains(res.stdout, "covered=") {
		t.Errorf("unparseable settings must be named unreadable:\n%s", res.stdout)
	}
}

func TestAuditSessionsStillWorksButIsHidden(t *testing.T) {
	auditHome(t)
	if res := run(t, "audit", "sessions"); res.code != 0 || !strings.Contains(res.stdout, "sandbox_blocks=1\n") {
		t.Errorf("alias exit %d:\n%s%s", res.code, res.stdout, res.stderr)
	}
	help := run(t, "audit", "--help")
	if !strings.Contains(help.stdout, "sandbox") || strings.Contains(help.stdout, "  sessions ") {
		t.Errorf("--help should list sandbox only:\n%s", help.stdout)
	}
}

func TestAuditSandboxUnreadableUserSettingsAreNotAbsent(t *testing.T) {
	auditHome(t)
	auditSettings(t, `{broken`)
	res := run(t, "audit", "sandbox")
	if !strings.Contains(res.stdout, "user_settings=unreadable\n") || strings.Contains(res.stdout, "user_settings=absent") {
		t.Errorf("unparseable user settings must not print as absent:\n%s", res.stdout)
	}
}
