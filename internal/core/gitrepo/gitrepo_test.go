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

func write(t *testing.T, repo *Repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo.Toplevel, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The regression this package exists to avoid. `git check-ignore -v` exits 0 and
// prints a pattern for a *negated* path too, so reading truth off -v reports every
// deliberately re-included file as excluded — which is exactly backwards for
// `.mkit/config.toml`, the one path mkit negates.
func TestIgnoredDistinguishesNegationFromExclusion(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, ".gitignore", ".mkit/*\n!.mkit/config.toml\n")

	if ignored, _ := repo.Ignored(".mkit/config.toml"); ignored {
		t.Error("config.toml reported ignored, but a negation re-includes it")
	}
	if ignored, _ := repo.Ignored(".mkit/gate.jsonl"); !ignored {
		t.Error("gate.jsonl reported not ignored, but .mkit/* excludes it")
	}
}

// The answer must be right before anything exists on disk: this is pattern
// matching, and the first call is the one that has to get it right.
func TestIgnoredAnswersForAbsentPaths(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, ".gitignore", ".mkit/*\n!.mkit/config.toml\n")

	if _, err := os.Stat(filepath.Join(repo.Toplevel, ".mkit")); !os.IsNotExist(err) {
		t.Fatal("precondition: .mkit/ must not exist")
	}
	if ignored, _ := repo.Ignored(".mkit/gate.jsonl"); !ignored {
		t.Error("want ignored for a path that does not exist yet")
	}
}

// The source names the file a remedy must edit, and getting it wrong produces a
// remedy that changes nothing.
func TestIgnoredNamesTheRuleSource(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, ".gitignore", ".mkit/\n")

	ignored, source := repo.Ignored(".mkit/config.toml")
	if !ignored {
		t.Fatal("want ignored under a directory-only rule")
	}
	if source != ".gitignore" {
		t.Errorf("source = %q, want .gitignore", source)
	}
}

func TestTracked(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "hi")
	if repo.Tracked("a.txt") {
		t.Error("an unstaged file is not tracked")
	}
	cmd := exec.Command("git", "add", "a.txt")
	cmd.Dir = repo.Toplevel
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if !repo.Tracked("a.txt") {
		t.Error("a staged file is tracked")
	}
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
