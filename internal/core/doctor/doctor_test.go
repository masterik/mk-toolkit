package doctor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

func newRepo(t *testing.T) *gitrepo.Repo {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "."}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			// The developer's own git config must not reach these repos: a global
			// commit.gpgsign, an init.templateDir hook or a commit.template would
			// otherwise make the suite pass or fail by whose machine it runs on.
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolate points every environment lookup at throwaway state, so a run measures
// the code and not the developer's machine.
func isolate(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	return cfg
}

func find(t *testing.T, r *Report, name string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check named %q in %+v", name, r.Checks)
	return Check{}
}

// The finding M7 exists to produce: a config path that is shadowed would let
// `mkit init` write a file that never travels.
func TestShadowedConfigIsAFailWithAWorkingRemedy(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, ".gitignore", ".mkit/\n")

	c := find(t, Run(Options{Repo: repo}), "config")
	if c.Status != Fail {
		t.Errorf("status = %q, want fail", c.Status)
	}
	if !strings.Contains(c.Remedy, "!.mkit/config.toml") {
		t.Errorf("remedy must name the negation: %q", c.Remedy)
	}
	if !strings.Contains(c.Remedy, ".gitignore") {
		t.Errorf("remedy must name the file carrying the rule: %q", c.Remedy)
	}
}

// Absent is a normal state, not a finding: config is an input, never a
// permission (ADR 0001 decision 3).
func TestAbsentConfigIsNotAFinding(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, ".gitignore", ".mkit/*\n!.mkit/config.toml\n")

	if c := find(t, Run(Options{Repo: repo}), "config"); c.Status != OK {
		t.Errorf("status = %q, want ok — running with no config is supported", c.Status)
	}
}

func TestUnignoredScratchIsAFail(t *testing.T) {
	isolate(t)
	repo := newRepo(t)

	c := find(t, Run(Options{Repo: repo}), "scratch ignored")
	if c.Status != Fail {
		t.Errorf("status = %q, want fail", c.Status)
	}
	if c.Remedy == "" {
		t.Error("want a remedy")
	}
}

// Doctor is useful outside a work tree; the repo group is skipped, not fatal.
func TestRunsWithoutARepo(t *testing.T) {
	isolate(t)
	r := Run(Options{Repo: nil})
	if c := find(t, r, "work tree"); c.Status != Unknown {
		t.Errorf("status = %q, want unknown", c.Status)
	}
	if len(r.Checks) < 5 {
		t.Error("the machine-level checks must still run")
	}
}

// A missing payload takes out the skills, so it is a Fail that names how to fix
// it. Since M5 it takes out nothing else — every remedy sentence has a producer
// in the binary now, so the other checks no longer degrade with it.
func TestMissingPayloadIsNamedWithARemedy(t *testing.T) {
	isolate(t)
	r := Run(Options{Repo: newRepo(t)})

	c := find(t, r, "plugin payload")
	if c.Status != Fail {
		t.Skipf("a payload was found in this environment (%s)", c.Detail)
	}
	if !strings.Contains(c.Remedy, "marketplace add") {
		t.Errorf("remedy must name the install step: %q", c.Remedy)
	}
	// The user-dir check no longer depends on it: both the probe and its remedy
	// live in internal/core/scratch.
	if u := find(t, r, "user state dir"); u.Status == Unknown {
		t.Error("the user dir check should answer without a payload")
	}
}

// The two grant keys are not equivalent: additionalDirectories also satisfies the
// auto-mode classifier, allowWrite does not. Reporting the narrow one as a clean
// pass would hide a failure it still produces.
func TestAllowlistDistinguishesTheNarrowGrant(t *testing.T) {
	cfg := isolate(t)
	// Pick a tool that exists here, so the check is not skipped as noise.
	var tool, dir string
	for _, g := range grants {
		if _, err := exec.LookPath(g.tool); err == nil {
			tool, dir = g.tool, g.dir
			break
		}
	}
	if tool == "" {
		t.Skip("none of the composed tools are installed")
	}

	settings := map[string]any{
		"sandbox": map[string]any{"filesystem": map[string]any{"allowWrite": []string{dir}}},
	}
	b, _ := json.Marshal(settings)
	writeFile(t, cfg, "settings.json", string(b))

	c := find(t, Run(Options{Repo: newRepo(t)}), "allowlist")
	if c.Status != Warn {
		t.Fatalf("status = %q, want warn", c.Status)
	}
	if !strings.Contains(c.Detail, "allowWrite") {
		t.Errorf("detail must name the narrower grant: %q", c.Detail)
	}
	if !strings.Contains(c.Remedy, "additionalDirectories") {
		t.Errorf("remedy must name the grant that does both: %q", c.Remedy)
	}
}

func TestWorstAndCounts(t *testing.T) {
	r := &Report{Checks: []Check{{Status: OK}, {Status: Warn}, {Status: Unknown}}}
	if got := r.Worst(); got != Warn {
		t.Errorf("Worst = %q, want warn", got)
	}
	r.Checks = append(r.Checks, Check{Status: Fail})
	if got := r.Worst(); got != Fail {
		t.Errorf("Worst = %q, want fail", got)
	}
	if c := r.Counts(); c[OK] != 1 || c[Warn] != 1 || c[Fail] != 1 || c[Unknown] != 1 {
		t.Errorf("Counts = %v", c)
	}
}

// Every finding must be actionable or explicitly say it is not — a check that
// reports a problem and offers nothing is the shape ADR 0002 forbids.
func TestEveryNonOKCheckSaysWhat(t *testing.T) {
	isolate(t)
	for _, c := range Run(Options{Repo: newRepo(t)}).Checks {
		if c.Status == OK {
			continue
		}
		if c.Detail == "" {
			t.Errorf("check %q has status %q and no detail", c.Name, c.Status)
		}
	}
}
