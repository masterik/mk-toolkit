package doctor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/buildinfo"
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
	// MKIT_HOME too, and this one is not about measurement: the user-state check
	// calls userdir.Writable, which *creates* the directory when it is
	// absent and removes it again. Unset, that probe ran in the developer's real
	// home on every run of this suite.
	t.Setenv("MKIT_HOME", filepath.Join(t.TempDir(), ".mkit"))
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

// The TOML config is gone (#53): repo settings live in docs/agents/. An old
// `.mkit/config.toml` is a leftover with the `setup` remedy, never a config row
// that calls it fine — the two used to contradict each other in one report.
func TestOldConfigIsOnlyALeftover(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, ".gitignore", ".mkit/\n")
	writeFile(t, repo.Toplevel, filepath.Join(".mkit", "config.toml"), "version = 1\n[merge]\nstyle = \"sqaush\"\n")

	r := Run(Options{Repo: repo})
	for _, c := range r.Checks {
		if c.Name == "config" || c.Name == "config values" {
			t.Errorf("config check still reported: %+v", c)
		}
	}
	c := find(t, r, "old repo config")
	if c.Status != Warn || !strings.Contains(c.Remedy, "`setup`") {
		t.Errorf("old repo config = %+v, want a warn naming the setup skill", c)
	}
}

// The scripts the skills run need bash 3.2+, so the row names the version, not
// just a path.
func TestBashReportsItsVersion(t *testing.T) {
	isolate(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH here")
	}
	c := find(t, Run(Options{}), "bash")
	if c.Status != OK {
		t.Fatalf("bash on PATH but not OK: %+v", c)
	}
	if !strings.Contains(c.Detail, "bash ") || !strings.ContainsAny(c.Detail, "0123456789") {
		t.Errorf("bash detail must name the version: %q", c.Detail)
	}
}

// Below 3.2 is a Fail, and a bash whose version cannot be read is never OK.
func TestBashCheckJudgesTheVersion(t *testing.T) {
	dir := t.TempDir()
	fake := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, tc := range []struct {
		name, body string
		want       Status
	}{
		{"old", "printf 3.1.17", Fail},
		{"min", "printf 3.2.57", OK},
		{"new", "printf 5.2.37", OK},
		{"mute", "exit 0", Unknown},
		{"broken", "exit 1", Unknown},
	} {
		if c := bashCheck(fake(tc.name, tc.body)); c.Status != tc.want {
			t.Errorf("%s: status = %v, want %v (%+v)", tc.name, c.Status, tc.want, c)
		}
	}
}

// Binary and payload drift is reported with the binary named optional: nothing
// in the plugin but sandbox-audit needs it.
func TestDriftNamesTheBinaryOptional(t *testing.T) {
	isolate(t)
	payload := t.TempDir()
	writeFile(t, payload, filepath.Join(".claude-plugin", "plugin.json"), `{"name":"mkit","version":"0.0.1"}`)
	if err := os.MkdirAll(filepath.Join(payload, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MKIT_PLUGIN_ROOT", payload)
	old := buildinfo.Version
	buildinfo.Version = "9.9.9"
	t.Cleanup(func() { buildinfo.Version = old })

	c := find(t, Run(Options{}), "plugin payload")
	if c.Status != Warn || !strings.Contains(c.Detail, "optional") {
		t.Errorf("drift = %+v, want a warn calling the binary optional", c)
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
	// live in internal/core/userdir.
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

// Orphans are found by their known location and reported with an rm the user
// runs; doctor itself removes nothing.
func TestOrphansAreReportedAndLeftInPlace(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	old := filepath.Join(repo.Toplevel, ".mkit", "work")
	writeFile(t, old, "main.jsonl", "{}\n")
	writeFile(t, os.Getenv("MKIT_HOME"), "bootstrap.state", "x")

	r := Run(Options{Repo: repo})
	c := find(t, r, "old worklog directory")
	if c.Status != Warn || c.Remedy != "rm -r '"+old+"'" {
		t.Errorf("worklog orphan = %+v", c)
	}
	if find(t, r, "bootstrap.state").Status != Warn {
		t.Error("user-dir orphan not reported")
	}
	if _, err := os.Stat(old); err != nil {
		t.Errorf("doctor must not remove anything: %v", err)
	}
}

// The worklog was dropped: a `.mkit/worklog/` from an older release is a
// leftover like any other.
func TestDroppedWorklogIsALeftover(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	old := filepath.Join(repo.Toplevel, ".mkit", "worklog")
	writeFile(t, old, "main.jsonl", "{}\n")

	c := find(t, Run(Options{Repo: repo}), "worklog directory")
	if c.Status != Warn || c.Remedy != "rm -r '"+old+"'" {
		t.Errorf("worklog leftover = %+v", c)
	}
}

// The gate ledger went with `gate run` (#48); an old one is a leftover.
func TestOldGateLedgerIsAnOrphan(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	ledger := filepath.Join(repo.Toplevel, ".mkit", "gate.jsonl")
	writeFile(t, filepath.Dir(ledger), "gate.jsonl", "{}\n")

	c := find(t, Run(Options{Repo: repo}), "old gate ledger")
	if c.Status != Warn || c.Remedy != "rm -r '"+ledger+"'" {
		t.Errorf("gate ledger orphan = %+v", c)
	}
}

func TestNoOrphansIsOneOKRow(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	if c := find(t, Run(Options{Repo: repo}), "old-version leftovers"); c.Status != OK {
		t.Errorf("status = %q, want ok", c.Status)
	}
}

// A path with a quote must survive being pasted into a shell.
func TestShellQuote(t *testing.T) {
	if got := shellQuote("a b'c"); got != `'a b'\''c'` {
		t.Errorf("got %s", got)
	}
}

// `.mkit` may be a symlink out of the repo. The leftover is still reported, but
// no `rm -r` is offered: it would delete wherever the link points.
func TestOrphanBehindASymlinkedAncestorGetsNoRemedy(t *testing.T) {
	isolate(t)
	repo := newRepo(t)
	outside := t.TempDir()
	writeFile(t, outside, "work/main.jsonl", "{}\n")
	if err := os.Symlink(outside, filepath.Join(repo.Toplevel, ".mkit")); err != nil {
		t.Fatal(err)
	}

	c := find(t, Run(Options{Repo: repo}), "old worklog directory")
	if c.Status != Warn || c.Remedy != "" {
		t.Errorf("want a warning with no remedy, got %+v", c)
	}
	if !strings.Contains(c.Detail, "symlink") {
		t.Errorf("detail must say why: %q", c.Detail)
	}
}

// An MKIT_HOME with a trailing slash or redundant components is an ordinary
// value; it must not be mistaken for a symlink escape and lose its remedy.
func TestOrphanRemedySurvivesAnUnnormalisedUserDir(t *testing.T) {
	isolate(t)
	home := filepath.Join(t.TempDir(), ".mkit")
	writeFile(t, home, "bootstrap.state", "x")
	t.Setenv("MKIT_HOME", home+string(filepath.Separator)+".")

	c := find(t, Run(Options{}), "bootstrap.state")
	if c.Remedy == "" {
		t.Errorf("remedy dropped for an unnormalised MKIT_HOME: %+v", c)
	}
}

func TestHasExtensionMatchesTheExactRepo(t *testing.T) {
	list := "gh stack\tgithub/gh-stack\tv0.1.1\ngh dash\tdlvhdr/gh-dash\tv4.25.2\n"
	if !hasExtension(list, "github/gh-stack") {
		t.Fatal("exact repo not found")
	}
	if hasExtension("gh stack\tgithub/gh-stack-fork\tv1\n", "github/gh-stack") {
		t.Fatal("a fork matched")
	}
}
