package initplan

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// A throwaway repo under $TMPDIR — never the developer's own.
func tempRepo(t *testing.T) *gitrepo.Repo {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// The developer's own git config stays out: a global commit.gpgsign or an
		// init.templateDir hook would fail or alter these commits.
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "main")
	for _, d := range []string{"docsite", "internal/cli", "internal/core", "cmd/mkit", "node_modules/x",
		".hidden", "gen", "plugin/skills", "cli"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("gen/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "--allow-empty", "-m", "base")
	git("branch", "release")
	git("remote", "add", "origin", "git@github.com:o/r.git")
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestDirCandidatesAreTopLevelPlusOneUnderParents(t *testing.T) {
	got := DirCandidates(tempRepo(t))
	// Sorted by ReadDir; internal's children follow it; `cli` appears once;
	// plugin is not a parent, so skills is not offered; ignored, hidden and
	// noise directories are skipped.
	want := []string{"cli", "cmd", "mkit", "docsite", "internal", "core", "plugin"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates %q, want %q", got, want)
	}
}

func TestGatherReadsRemotesBranchesAndProtected(t *testing.T) {
	c := Gather(tempRepo(t))
	if len(c.Remotes) != 1 || RemoteSlug(c.Remotes[0].URL) != "o/r" {
		t.Errorf("remotes %+v", c.Remotes)
	}
	if !reflect.DeepEqual(c.Branches, []string{"main", "release"}) {
		t.Errorf("branches %v", c.Branches)
	}
	if !reflect.DeepEqual(c.Protected, []string{"main"}) {
		t.Errorf("protected %v, want [main]", c.Protected)
	}
}

// A default branch known only from the remote's HEAD is not shown as always
// kept: there is no local branch for cleanup to delete, or for the user to see.
func TestGatherLocksOnlyALocalDefaultBranch(t *testing.T) {
	repo := tempRepo(t)
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo.Toplevel
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("update-ref", "refs/remotes/origin/trunk", "HEAD")
	git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	git("checkout", "-q", "release")
	git("branch", "-q", "-D", "main")
	c := Gather(repo)
	if len(c.Protected) != 0 {
		t.Errorf("protected %v, want none — trunk exists only on the remote", c.Protected)
	}
}
