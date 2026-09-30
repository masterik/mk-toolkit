package cli

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/branchstatus"
	"github.com/masterik/mk-toolkit/internal/core/cache"
	"github.com/masterik/mk-toolkit/internal/core/doctor"
	"github.com/masterik/mk-toolkit/internal/core/profile"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
	"github.com/masterik/mk-toolkit/internal/core/sessionaudit"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

// forceColour makes the renderer emit escapes into a buffer, so the tests can
// tell styled from plain output. CLICOLOR_FORCE is termenv's own switch.
func forceColour(t *testing.T) {
	t.Helper()
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")
}

func show(t *testing.T, out string) {
	t.Helper()
	if os.Getenv("MKIT_SHOW") != "" {
		t.Log("\n" + ansi.ReplaceAllString(out, ""))
	}
}

func TestPrettyDoctorIsStyledAndKeepsEveryFinding(t *testing.T) {
	forceColour(t)
	r := &doctor.Report{Checks: []doctor.Check{
		{Group: "install", Name: "mkit binary", Status: doctor.OK, Detail: "0.20.0 (abc)"},
		{Group: "install", Name: "plugin payload", Status: doctor.Fail, Detail: "not found", Remedy: "brew install x"},
		{Group: "leftovers", Name: "old worklog directory", Status: doctor.Warn, Detail: "/r/.mkit/work", Remedy: "rm -r '/r/.mkit/work'"},
	}}
	var b bytes.Buffer
	prettyDoctor(&b, r)
	if !ansi.MatchString(b.String()) {
		t.Error("expected colour escapes")
	}
	plain := ansi.ReplaceAllString(b.String(), "")
	show(t, b.String())
	for _, want := range []string{"✔", "✖", "▲", "plugin payload", "→ brew install x", "rm -r '/r/.mkit/work'", "1 ok", "1 warn", "1 fail"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
}

func TestPrettyBranchStatusListsBranchesAndWorktrees(t *testing.T) {
	forceColour(t)
	sc := &branchstatus.Scan{
		Default: "main", Protected: []string{"main"}, Remote: "origin", Fetch: "ok", GH: "ok", WorktreesState: "ok",
		Branches: []branchstatus.Branch{
			{Name: "main", Class: branchstatus.Protected, Upstream: "origin/main"},
			{Name: "feat/x", Class: branchstatus.MergedPR, Upstream: "gone", MergedInto: []string{"main"}, PR: "merged#7"},
		},
		Worktrees:      []branchstatus.Worktree{{Branch: "main", Path: "/r", Origin: "primary", Clean: "yes"}},
		ConfigProblems: []string{"a pin that went nowhere"},
	}
	var b bytes.Buffer
	prettyBranchStatus(&b, sc)
	plain := ansi.ReplaceAllString(b.String(), "")
	show(t, b.String())
	for _, want := range []string{"branches (2)", "feat/x", "merged-pr", "merged#7", "worktrees (1)", "primary", "a pin that went nowhere"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
}

func TestPrettyCacheShowsModeAndTotal(t *testing.T) {
	forceColour(t)
	rep := &cache.Report{Days: 7, Providers: []cache.ProviderReport{{
		Name: "claude", Home: "/h/.claude", Present: true,
		Categories: []cache.CategoryReport{
			{Label: "transcripts", Kind: cache.KindFiles, Bytes: 4096, Entries: []cache.Entry{{Path: "a", Bytes: 4096}}},
			{Label: "session dirs", Kind: cache.KindStaleDirs},
		}}}}
	var b bytes.Buffer
	prettyCache(&b, rep, false, nil)
	plain := ansi.ReplaceAllString(b.String(), "")
	show(t, b.String())
	for _, want := range []string{"DRY-RUN", "transcripts", "nothing to prune", "reclaimable", "--apply"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	b.Reset()
	prettyCache(&b, rep, true, &cache.Result{Bytes: 4096})
	if p := ansi.ReplaceAllString(b.String(), ""); !strings.Contains(p, "APPLY") || !strings.Contains(p, "reclaimed") {
		t.Errorf("apply mode not shown:\n%s", p)
	}
}

func TestPrettyAuditSummarisesCountsAndBuckets(t *testing.T) {
	forceColour(t)
	rep := &sessionaudit.Report{Root: "/h/.claude", Days: 14, Transcripts: 3, WithEvents: 2,
		Counts:       sessionaudit.Counts{SandboxBlocks: 4, Overrides: 2},
		BlockTargets: []sessionaudit.Bucket{{Key: "/etc/hosts", Count: 4, Projects: []string{"p"}}},
	}
	var b bytes.Buffer
	prettyAudit(&b, rep, 10)
	plain := ansi.ReplaceAllString(b.String(), "")
	show(t, b.String())
	for _, want := range []string{"last 14d", "sandbox blocks", "/etc/hosts", "[p]"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
}

// The contract that lets skills stay unchanged: a buffer is never Pretty, so
// the plain renderers still run and print no escapes.
func TestPlainOutputHasNoEscapesOffATerminal(t *testing.T) {
	res := run(t, "version")
	if ansi.MatchString(res.stdout) {
		t.Errorf("version printed escapes off a terminal: %q", res.stdout)
	}
	if !strings.HasPrefix(res.stdout, "mkit ") {
		t.Errorf("plain version form changed: %q", res.stdout)
	}
}

func TestPrettyHelpGroupsCommandsAndListsFlags(t *testing.T) {
	forceColour(t)
	root := NewRoot()
	var b bytes.Buffer
	prettyHelp(&b, root)
	plain := ansi.ReplaceAllString(b.String(), "")
	show(t, b.String())
	for _, want := range []string{"── Setup & health", "── Quality gate & review", "doctor", "── flags", "--json"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	sub, _, err := root.Find([]string{"branch", "status"})
	if err != nil {
		t.Fatal(err)
	}
	b.Reset()
	prettyHelp(&b, sub)
	if p := ansi.ReplaceAllString(b.String(), ""); !strings.Contains(p, "--default string") || !strings.Contains(p, "── usage") || !strings.Contains(p, "── global flags") {
		t.Errorf("subcommand help incomplete:\n%s", p)
	}
}

// Off a terminal the stock cobra help is untouched.
func TestHelpIsStockOffATerminal(t *testing.T) {
	res := run(t, "--help")
	if ansi.MatchString(res.stdout) || !strings.Contains(res.stdout, "Available Commands:") && !strings.Contains(res.stdout, "Setup & health:") {
		t.Errorf("help changed off a terminal:\n%s", res.stdout)
	}
}

func fullProfile() *profile.Profile {
	return &profile.Profile{
		Toplevel: "/repo",
		Config:   repoconfig.Status{Path: "/repo/.mkit/config.toml", State: "tracked"},
		Gate: profile.Gate{Ecosystem: "go,just", Steps: []profile.GateStep{
			{Step: "test", Command: "just test", Source: profile.Discovered},
			{Step: "lint", Command: "make lint", Source: profile.Pinned},
		}},
		Spec:       profile.Spec{Store: profile.Value{Value: "github-issues", Source: profile.Discovered}, Ref: profile.Value{Value: "o/r", Source: profile.Discovered}},
		Scopes:     profile.List{Values: []string{"cli", "core"}, Source: profile.Pinned},
		SubjectMax: profile.Value{Value: "72", Source: profile.Pinned},
		Review:     profile.List{Source: profile.Unavailable, Cause: "no CODEOWNERS file and nothing pinned"},
		ReviewMode: profile.Value{Value: "full", Source: profile.Pinned},
		Merge:      profile.Value{Value: "merge", Source: profile.Pinned},
		Keep:       profile.List{Values: []string{"release"}, Source: profile.Pinned},
		Payload:    profile.PayloadInfo{Found: true, Version: "0.20.0", Dir: "/p", Via: "work tree"},
	}
}

// The pretty form must carry every value the plain one prints: a dropped field
// is invisible on a terminal and exactly what nobody would notice.
func TestPrettyProfileKeepsEveryValueThePlainFormPrints(t *testing.T) {
	forceColour(t)
	p := fullProfile()
	var b bytes.Buffer
	prettyProfile(&b, p)
	show(t, b.String())
	plain := ansi.ReplaceAllString(b.String(), "")
	for _, want := range []string{
		"/repo", "/repo/.mkit/config.toml", "tracked", "go,just",
		"just test", "make lint", "pinned", "discovered",
		"github-issues", "o/r", "cli, core", "72", "no CODEOWNERS file and nothing pinned", "unavailable",
		"full", "merge", "release", "0.20.0", "/p (work tree)",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}

	p.Payload = profile.PayloadInfo{Remedy: "install the plugin"}
	p.Gate = profile.Gate{Cause: "no ecosystem recognised"}
	b.Reset()
	prettyProfile(&b, p)
	plain = ansi.ReplaceAllString(b.String(), "")
	for _, want := range []string{"not found", "install the plugin", "no ecosystem recognised"} {
		if !strings.Contains(plain, want) {
			t.Errorf("degraded profile missing %q in:\n%s", want, plain)
		}
	}
}

func TestPrettyInitCarriesEveryLabelThePlainFormPrints(t *testing.T) {
	forceColour(t)
	cases := []struct {
		name string
		res  initResult
		want []string
	}{
		{"wrote", initResult{Path: "/r/.mkit/config.toml", Written: true, State: "tracked"}, []string{"wrote", "/r/.mkit/config.toml"}},
		{"shadowed", initResult{Path: "/r/.mkit/config.toml", State: "shadowed", Detail: "a legacy rule hides it", Remedy: "edit .gitignore"},
			[]string{"/r/.mkit/config.toml", "shadowed", "a legacy rule hides it", "→ edit .gitignore"}},
	}
	for _, c := range cases {
		var b bytes.Buffer
		if err := emitInit(&b, Options{Pretty: true}, c.res); err != nil {
			t.Fatal(err)
		}
		plain := ansi.ReplaceAllString(b.String(), "")
		if !ansi.MatchString(b.String()) {
			t.Errorf("%s: expected colour escapes", c.name)
		}
		for _, w := range c.want {
			if !strings.Contains(plain, w) {
				t.Errorf("%s: missing %q in:\n%s", c.name, w, plain)
			}
		}
	}
	// The pipe path is untouched.
	var b bytes.Buffer
	_ = emitInit(&b, Options{}, initResult{Path: "/p", Written: true})
	if b.String() != "wrote /p\n" {
		t.Errorf("plain init form changed: %q", b.String())
	}
}

func TestPrettyBranchStatusShowsTheKeepList(t *testing.T) {
	forceColour(t)
	sc := &branchstatus.Scan{Default: "main", Protected: []string{"main", "release"}, Keep: []string{"release"}, WorktreesState: "ok"}
	var b bytes.Buffer
	prettyBranchStatus(&b, sc)
	if p := ansi.ReplaceAllString(b.String(), ""); !strings.Contains(p, "keep") || !strings.Contains(p, "release") {
		t.Errorf("keep row missing:\n%s", p)
	}
}

// A scan that could not read a transcript must say its counts are lower bounds,
// and one that read nothing must not print an audit at all.
func TestPrettyAuditFlagsUnreadableTranscripts(t *testing.T) {
	forceColour(t)
	partial := &sessionaudit.Report{Root: "/h", Days: 14, Transcripts: 2, Unreadable: []string{"/h/a.jsonl"},
		Counts: sessionaudit.Counts{SandboxBlocks: 1}}
	var b bytes.Buffer
	prettyAudit(&b, partial, 10)
	p := ansi.ReplaceAllString(b.String(), "")
	if !strings.Contains(p, "lower bounds") || strings.Index(p, "lower bounds") > strings.Index(p, "sandbox blocks") {
		t.Errorf("unreadable warning must come first and say lower bounds:\n%s", p)
	}

	none := &sessionaudit.Report{Root: "/h", Days: 14, Unreadable: []string{"/h/a.jsonl"}}
	b.Reset()
	prettyAudit(&b, none, 10)
	p = ansi.ReplaceAllString(b.String(), "")
	if strings.Contains(p, "sandbox blocks") || !strings.Contains(p, "no audit to show") {
		t.Errorf("an all-unreadable scan must not look like an empty audit:\n%s", p)
	}
}
