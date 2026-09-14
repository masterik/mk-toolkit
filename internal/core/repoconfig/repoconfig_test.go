package repoconfig

import (
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
	for _, args := range [][]string{
		{"init", "-q", "."},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
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

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// A global core.excludesFile would otherwise decide the ignore tests below.
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// The four states, and the one that matters. Shadowed is the state a repo
// configured before the config existed lands in: the file can be written and then
// silently never travels, which is the one property it exists for.
func TestStat(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		repo := newRepo(t)
		if got := Stat(repo).State; got != StateAbsent {
			t.Errorf("State = %q, want %q", got, StateAbsent)
		}
	})

	t.Run("untracked", func(t *testing.T) {
		repo := newRepo(t)
		writeFile(t, repo.Toplevel, RelPath, "version = 1\n")
		if got := Stat(repo).State; got != StateUntracked {
			t.Errorf("State = %q, want %q", got, StateUntracked)
		}
	})

	t.Run("tracked", func(t *testing.T) {
		repo := newRepo(t)
		writeFile(t, repo.Toplevel, RelPath, "version = 1\n")
		git(t, repo.Toplevel, "add", RelPath)
		if got := Stat(repo).State; got != StateTracked {
			t.Errorf("State = %q, want %q", got, StateTracked)
		}
	})

	t.Run("shadowed by a legacy directory-only rule", func(t *testing.T) {
		repo := newRepo(t)
		writeFile(t, repo.Toplevel, ".gitignore", ".mkit/\n")
		st := Stat(repo)
		if st.State != StateShadowed {
			t.Fatalf("State = %q, want %q", st.State, StateShadowed)
		}
		if st.IgnoreSource != ".gitignore" {
			t.Errorf("IgnoreSource = %q, want .gitignore", st.IgnoreSource)
		}
	})

	t.Run("the current rule pair leaves it committable", func(t *testing.T) {
		repo := newRepo(t)
		writeFile(t, repo.Toplevel, ".gitignore", ".mkit/*\n!.mkit/config.toml\n")
		writeFile(t, repo.Toplevel, RelPath, "version = 1\n")
		if got := Stat(repo).State; got != StateUntracked {
			t.Errorf("State = %q, want %q — the negation must survive", got, StateUntracked)
		}
	})

	// A tracked file is unaffected by ignore rules, so tracked beats shadowed.
	t.Run("tracked outranks an ignore rule", func(t *testing.T) {
		repo := newRepo(t)
		writeFile(t, repo.Toplevel, RelPath, "version = 1\n")
		git(t, repo.Toplevel, "add", "-f", RelPath)
		writeFile(t, repo.Toplevel, ".gitignore", ".mkit/\n")
		if got := Stat(repo).State; got != StateTracked {
			t.Errorf("State = %q, want %q", got, StateTracked)
		}
	})
}

// Absent is a normal state, never an error: config is an input, never a
// permission (ADR 0001 decision 3).
func TestLoadAbsentIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	cfg, present, err := Load(dir)
	if err != nil {
		t.Fatalf("err = %v, want nil for an absent config", err)
	}
	if present {
		t.Error("present = true for an absent config")
	}
	if !cfg.IsZero() {
		t.Error("want a zero config")
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &Config{
		Gate:   Gate{Commands: map[string]string{"test": "go test ./...", "lint": "golangci-lint run"}},
		Spec:   Spec{Store: "github-issues", Ref: "acme/widget"},
		Commit: Commit{Scopes: []string{"api", "ui"}},
		Review: Review{Reviewers: []string{"@team/core"}},
		Merge:  Merge{Style: "squash"},
	}
	if err := Write(dir, want); err != nil {
		t.Fatal(err)
	}

	got, present, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("present = false after Write")
	}
	if got.Version != Version {
		t.Errorf("Version = %d, want %d", got.Version, Version)
	}
	if got.Gate.Commands["test"] != "go test ./..." || got.Gate.Commands["lint"] != "golangci-lint run" {
		t.Errorf("gate commands = %v", got.Gate.Commands)
	}
	if got.Spec != want.Spec || got.Merge != want.Merge {
		t.Errorf("spec/merge = %+v %+v", got.Spec, got.Merge)
	}
	if strings.Join(got.Commit.Scopes, ",") != "api,ui" {
		t.Errorf("scopes = %v", got.Commit.Scopes)
	}
}

// The file is committed and read in a diff, so it carries comments. Rendered from
// a template for exactly that reason — no Go TOML marshaller preserves them.
func TestWriteIsCommented(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, &Config{Merge: Merge{Style: "squash"}}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "# mkit — per-repo configuration.") {
		t.Errorf("want a comment header, got:\n%s", b)
	}
	if !strings.Contains(string(b), "never a permission") {
		t.Error("want the config-is-an-input note in the header")
	}
}

// A value carrying a quote or a backslash must survive the round trip: a repo
// whose lint command contains one is not exotic.
func TestWriteEscapesAwkwardValues(t *testing.T) {
	dir := t.TempDir()
	awkward := `sh -c 'echo "a\b" && lint'`
	if err := Write(dir, &Config{Gate: Gate{Commands: map[string]string{"lint": awkward}}}); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Gate.Commands["lint"] != awkward {
		t.Errorf("lint = %q, want %q", got.Gate.Commands["lint"], awkward)
	}
}

func TestIsZero(t *testing.T) {
	if !(&Config{}).IsZero() {
		t.Error("an empty config is zero")
	}
	if (&Config{Version: 1}).IsZero() == false {
		t.Error("version alone is not something pinned, so it stays zero")
	}
	if (&Config{Merge: Merge{Style: "squash"}}).IsZero() {
		t.Error("a pinned value is not zero")
	}
}

// The remedy must name the file git reported, because `.gitignore` outranks the
// common dir's `info/exclude` — editing the wrong one changes nothing.
func TestShadowedRemedyNamesTheSource(t *testing.T) {
	got := ShadowedRemedy(Status{IgnoreSource: ".git/info/exclude", IgnorePattern: ".mkit/", ParentExcluded: true})
	if !strings.Contains(got, ".git/info/exclude") {
		t.Errorf("remedy does not name the source: %s", got)
	}
	if !strings.Contains(got, "!.mkit/config.toml") {
		t.Errorf("remedy does not name the negation: %s", got)
	}
	if !strings.Contains(ShadowedRemedy(Status{}), ".gitignore") {
		t.Error("want a sensible default when git named no source")
	}
}

// A rule that is not `.mkit/` needs a different fix, and naming the `.mkit/` one
// sends the reader looking for a line that is not in the file. A directory rule
// must be rewritten (git never descends into an excluded directory); anything
// else is lifted by a negation after it.
func TestShadowedRemedyDependsOnTheRule(t *testing.T) {
	dir := ShadowedRemedy(Status{IgnoreSource: ".gitignore", IgnorePattern: ".mkit/", ParentExcluded: true})
	if !strings.Contains(dir, "replace") || !strings.Contains(dir, ".mkit/*") {
		t.Errorf("directory rule should be rewritten: %s", dir)
	}

	other := ShadowedRemedy(Status{IgnoreSource: ".gitignore", IgnorePattern: "*.toml"})
	if !strings.Contains(other, "*.toml") {
		t.Errorf("remedy does not name the rule that actually matched: %s", other)
	}
	if strings.Contains(other, "replace the `.mkit/` rule") {
		t.Errorf("remedy names a rule that is not in the file: %s", other)
	}
	if !strings.Contains(other, "!.mkit/config.toml") {
		t.Errorf("remedy does not name the negation: %s", other)
	}
}

// The remedy is only correct if ParentExcluded matches what git will actually do,
// so this checks the prediction against the ground truth: append the negation and
// ask git whether the file came back. A wrong answer here is a remedy the reader
// applies and which changes nothing — the exact failure ADR 0002 was written about.
func TestParentExcludedMatchesGit(t *testing.T) {
	// Every shape that reaches this code: directory-only rules, wildcards that
	// swallow the parent, and patterns that catch only the file.
	rules := []string{
		".mkit/", ".mkit/*", ".m*", "*.toml", "*", ".mkit*",
		"**/.mkit/", ".mki?/", "/.mkit", "config.toml", ".mkit/config.toml",
	}
	for _, rule := range rules {
		for _, dirExists := range []bool{false, true} {
			name := rule
			if dirExists {
				name += " (.mkit present)"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				git(t, dir, "init", "-q", ".")
				if dirExists {
					if err := os.MkdirAll(filepath.Join(dir, ".mkit"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				repo, err := gitrepo.Open(dir)
				if err != nil {
					t.Fatal(err)
				}

				writeFile(t, dir, ".gitignore", rule+"\n")
				st := Stat(repo)
				if st.State != StateShadowed {
					t.Skipf("rule %q does not shadow the config", rule)
				}

				// Ground truth: does a negation after the rule bring it back?
				writeFile(t, dir, ".gitignore", rule+"\n!"+RelPath+"\n")
				stillIgnored, _, _ := repo.IgnoreRule(RelPath)

				if st.ParentExcluded != stillIgnored {
					t.Errorf("rule %q: ParentExcluded=%v, but a negation %s",
						rule, st.ParentExcluded,
						map[bool]string{true: "does not work", false: "works"}[stillIgnored])
				}
			})
		}
	}
}
