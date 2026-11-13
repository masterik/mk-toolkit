package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newRepo builds a throwaway work tree. Inside the test's own temp directory, so
// nothing here can touch a developer's real state.
func newRepo(t *testing.T) *Repo {
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
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestOpenOutsideARepo(t *testing.T) {
	// t.TempDir() is not a work tree; a parent that happens to be one would make
	// this vacuous, so the check is that Open reports rather than guesses.
	dir := t.TempDir()
	if _, err := Open(dir); err == nil {
		t.Skip("temp dir sits inside a work tree on this machine")
	} else if err != ErrNotARepo {
		t.Errorf("err = %v, want ErrNotARepo", err)
	}
}

// git answers `--git-common-dir` relative to the work tree; CommonDir must hand
// back an absolute path, or doctor's `info/exclude` lookup lands in the cwd.
func TestCommonDirIsAbsolute(t *testing.T) {
	repo := newRepo(t)
	got, err := repo.CommonDir()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("CommonDir = %q, want absolute", got)
	}
	if _, err := os.Stat(filepath.Join(got, "HEAD")); err != nil {
		t.Errorf("CommonDir = %q is not the repository storage: %v", got, err)
	}
}
