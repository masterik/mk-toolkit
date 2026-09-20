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

// AliveCommits fails **open**, and the worklog's rotation is why that matters: it
// drops every record whose head this map does not call alive. Invert the fallback and
// the first rotation past Keep*2 discards the whole log instead of nothing — a silent
// data loss with no test failing anywhere. So the fallback is pinned here, at the one
// place that decides it.
func TestAliveCommitsFailsOpen(t *testing.T) {
	heads := []string{"deadbeef", "cafebabe"}

	// A Repo whose work tree does not exist: `git cat-file` cannot run at all, which
	// is the shape of every failure the fallback is for — no git, no repo, no HEAD.
	broken := &Repo{Toplevel: filepath.Join(t.TempDir(), "no-such-work-tree")}
	alive := broken.AliveCommits(heads)
	for _, h := range heads {
		if !alive[h] {
			t.Errorf("a batch that could not run reported %s dead; rotation would drop it", h)
		}
	}

	// The empty case stays empty — nothing asked about, nothing to keep alive.
	if got := broken.AliveCommits(nil); len(got) != 0 {
		t.Errorf("AliveCommits(nil) = %v, want empty", got)
	}
}

// The counterpart to the fail-open test: a batch that *did* answer must still tell
// alive from dead. A guard that fails open too eagerly is invisible — rotation simply
// stops dropping anything, and the log grows with records pointing at gone commits.
func TestAliveCommitsDistinguishesWhenTheBatchAnswers(t *testing.T) {
	repo := newRepo(t)
	head := repo.Head()
	if head == "" {
		t.Fatal("no HEAD in the throwaway repo")
	}
	const gone = "0000000000000000000000000000000000000001"

	alive := repo.AliveCommits([]string{head, gone})
	if !alive[head] {
		t.Errorf("HEAD %s reported dead", head)
	}
	if alive[gone] {
		t.Error("a commit that does not exist reported alive")
	}
}

// git runs a command in the repo, failing the test on error.
func git(t *testing.T, repo *Repo, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo.Toplevel
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// A stale `origin/HEAD` — or one naming a branch that exists only as a
// remote-tracking ref — must not beat a local branch that is actually here.
// `mkit branch scan` refuses a --default absent from refs/heads before it
// fetches, so returning the non-local name stops the whole of `cleanup` on a
// branch this checkout has never had.
func TestDefaultBranchPrefersALocalBranchOverAStaleRemoteHead(t *testing.T) {
	repo := newRepo(t)
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "remote", "add", "origin", "https://example.invalid/r.git")
	// origin/HEAD -> origin/develop, with no local develop anywhere.
	git(t, repo, "update-ref", "refs/remotes/origin/develop", "HEAD")
	git(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")

	if got := repo.DefaultBranch(); got != "main" {
		t.Errorf("DefaultBranch() = %q, want main: the remote's answer names no local branch", got)
	}

	// With the branch actually present, the remote's answer is the right one.
	git(t, repo, "branch", "develop")
	if got := repo.DefaultBranch(); got != "develop" {
		t.Errorf("DefaultBranch() = %q, want develop once it exists locally", got)
	}
}
