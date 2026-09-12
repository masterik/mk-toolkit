package profile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

func newRepo(t *testing.T) *gitrepo.Repo {
	t.Helper()
	// Neither a payload nor a harness plugin root, so gate discovery reports its
	// cause instead of picking up whatever is installed on the developer's
	// machine — a test that reads the real environment measures the developer.
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

	dir := t.TempDir()
	run(t, dir, "init", "-q", ".")
	run(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
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

func commit(t *testing.T, dir, subject string) {
	t.Helper()
	run(t, dir, "commit", "-q", "--allow-empty", "-m", subject)
}

// ADR 0001 decision 4: a store the user already declared beats a heuristic over
// `git remote`. The doc is read; the remote only supplies the ref.
func TestSpecStoreComesFromTheTrackerDoc(t *testing.T) {
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, "docs/agents/issue-tracker.md", "# Issue tracker: GitHub\n\nbody\n")
	run(t, repo.Toplevel, "remote", "add", "origin", "git@github.com:acme/widget.git")

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.Store.Value != "github-issues" || p.Spec.Store.Source != Discovered {
		t.Errorf("store = %+v, want github-issues/discovered", p.Spec.Store)
	}
	if p.Spec.Ref.Value != "acme/widget" || p.Spec.Ref.Source != Discovered {
		t.Errorf("ref = %+v, want acme/widget/discovered", p.Spec.Ref)
	}
}

// A gh binary and a GitHub remote are not evidence the team tracks work there —
// so with no doc and nothing pinned, the answer is Unavailable with a cause, not
// a confident guess.
func TestSpecStoreIsUnavailableRatherThanGuessed(t *testing.T) {
	repo := newRepo(t)
	run(t, repo.Toplevel, "remote", "add", "origin", "git@github.com:acme/widget.git")

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.Store.Source != Unavailable {
		t.Errorf("store = %+v, want unavailable", p.Spec.Store)
	}
	if p.Spec.Store.Cause == "" {
		t.Error("an unavailable value must carry a cause")
	}
}

func TestPinnedBeatsDiscovered(t *testing.T) {
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, "docs/agents/issue-tracker.md", "# Issue tracker: GitHub\n")
	if err := repoconfig.Write(repo.Toplevel, &repoconfig.Config{
		Spec:  repoconfig.Spec{Store: "files", Ref: "docs/specs"},
		Merge: repoconfig.Merge{Style: "squash"},
	}); err != nil {
		t.Fatal(err)
	}

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.Store.Value != "files" || p.Spec.Store.Source != Pinned {
		t.Errorf("store = %+v, want files/pinned", p.Spec.Store)
	}
	if p.Merge.Value != "squash" || p.Merge.Source != Pinned {
		t.Errorf("merge = %+v, want squash/pinned", p.Merge)
	}
}

// History is what the repo actually does, so scopes are discovered from it and
// ranked by use. A stable order matters: two runs on one repo must produce the
// same profile, or a diff of it means nothing.
func TestScopesAreDiscoveredFromHistoryMostUsedFirst(t *testing.T) {
	repo := newRepo(t)
	commit(t, repo.Toplevel, "feat(api): one")
	commit(t, repo.Toplevel, "fix(api): two")
	commit(t, repo.Toplevel, "docs(ui): three")
	commit(t, repo.Toplevel, "chore: no scope here")

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Scopes.Source != Discovered {
		t.Fatalf("source = %q, want discovered", p.Scopes.Source)
	}
	if got := strings.Join(p.Scopes.Values, ","); got != "api,ui" {
		t.Errorf("scopes = %q, want api,ui", got)
	}
}

func TestReviewersComeFromCodeowners(t *testing.T) {
	repo := newRepo(t)
	writeFile(t, repo.Toplevel, ".github/CODEOWNERS",
		"# comment @not-an-owner\n*.go @team/core @alice\ndocs/ @alice\n")

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Review.Source != Discovered {
		t.Fatalf("source = %q, want discovered", p.Review.Source)
	}
	if got := strings.Join(p.Review.Values, ","); got != "@team/core,@alice" {
		t.Errorf("reviewers = %q, want @team/core,@alice (deduped, comments ignored)", got)
	}
}

// Discovery no longer needs the payload — a reachable payload changes nothing
// about the gate, and the pinned half answers either way.
func TestGateHonoursPinsWithoutAPayload(t *testing.T) {
	repo := newRepo(t)
	if err := repoconfig.Write(repo.Toplevel, &repoconfig.Config{
		Gate: repoconfig.Gate{Commands: map[string]string{"test": "go test ./..."}},
	}); err != nil {
		t.Fatal(err)
	}

	p, err := Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	if p.Gate.Cause != "" {
		t.Errorf("cause = %q, want none — discovery ran", p.Gate.Cause)
	}
	if len(p.Gate.Steps) != 1 || p.Gate.Steps[0].Source != Pinned {
		t.Fatalf("steps = %+v, want the one pinned step", p.Gate.Steps)
	}
}

// Config is an input, never a permission: a profile with no config at all is
// complete and usable.
func TestProfileIsCompleteWithNoConfig(t *testing.T) {
	repo := newRepo(t)
	p, err := Build(repo)
	if err != nil {
		t.Fatalf("Build must not fail without a config: %v", err)
	}
	if p.Config.State != repoconfig.StateAbsent {
		t.Errorf("config state = %q, want absent", p.Config.State)
	}
}
