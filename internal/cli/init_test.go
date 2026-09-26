package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #31's regression half: the form changed, the non-interactive contract
// did not. Off a TTY and under --yes, `init` applies no form default — the
// `merge = merge` and `review.mode = full` pre-selections exist only in the form.

func initRepo(t *testing.T) string {
	t.Helper()
	repo, _ := gateRepo(t)
	t.Setenv("MKIT_HOME", filepath.Join(repo, ".mkit-home"))
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	put(t, repo, ".gitignore", ".mkit/*\n!.mkit/config.toml\n")
	return repo
}

func initJSON(t *testing.T, args ...string) (map[string]any, result) {
	t.Helper()
	res := run(t, append([]string{"init", "--json"}, args...)...)
	var m map[string]any
	if res.code == 0 {
		if err := json.Unmarshal([]byte(res.stdout), &m); err != nil {
			t.Fatalf("%v\n%s", err, res.stdout)
		}
	}
	return m, res
}

func TestInitWithNoFieldsAppliesNoFormDefault(t *testing.T) {
	for _, args := range [][]string{{"--yes"}, {}} {
		repo := initRepo(t)
		m, res := initJSON(t, args...)
		if res.code != 0 {
			t.Fatalf("init %v: exit %d %s", args, res.code, res.stderr)
		}
		if m["state"] != "nothing-to-pin" || m["written"] != false {
			t.Errorf("init %v: %v, want nothing-to-pin — a form default leaked off the TTY", args, m)
		}
		if _, err := os.Stat(filepath.Join(repo, ".mkit", "config.toml")); !os.IsNotExist(err) {
			t.Errorf("init %v wrote a config", args)
		}
	}
}

func TestInitFlagsPinOnlyWhatWasGiven(t *testing.T) {
	repo := initRepo(t)
	m, res := initJSON(t, "--merge", "squash", "--scope", "cli")
	if res.code != 0 || m["state"] != "written" {
		t.Fatalf("exit %d, %v %s", res.code, m, res.stderr)
	}
	b, err := os.ReadFile(filepath.Join(repo, ".mkit", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`style = 'squash'`, `scopes = ['cli']`} {
		if !strings.Contains(s, want) {
			t.Errorf("config lacks %s:\n%s", want, s)
		}
	}
	if strings.Contains(s, "[review]") {
		t.Errorf("config pins a review mode no flag gave:\n%s", s)
	}
}

func TestInitRefusesAnInvalidFlag(t *testing.T) {
	initRepo(t)
	if _, res := initJSON(t, "--merge", "sqaush"); res.code == 0 {
		t.Error("--merge sqaush accepted")
	}
	if _, res := initJSON(t, "--subject-max", "0"); res.code == 0 {
		t.Error("--subject-max 0 accepted")
	}
}

func TestInitKeepsAnExistingConfigWithoutForce(t *testing.T) {
	repo := initRepo(t)
	if _, res := initJSON(t, "--merge", "squash"); res.code != 0 {
		t.Fatal(res.stderr)
	}
	m, _ := initJSON(t, "--merge", "rebase")
	if m["state"] != "already-configured" {
		t.Errorf("state %v, want already-configured", m["state"])
	}
	m, res := initJSON(t, "--force", "--review-mode", "quick")
	if res.code != 0 || m["state"] != "written" {
		t.Fatalf("--force: exit %d %v", res.code, m)
	}
	b, _ := os.ReadFile(filepath.Join(repo, ".mkit", "config.toml"))
	for _, want := range []string{`style = 'squash'`, `mode = 'quick'`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("--force rewrite lost or lacks %s:\n%s", want, b)
		}
	}
}
