package scratch_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/scratch"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// newRepo is a repo with nothing ignored: EnsureIgnored has work to do.
func newRepo(t *testing.T) *gitrepo.Repo {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestEnsureIgnoredWritesThePairIntoTheCommonDirExclude(t *testing.T) {
	repo := newRepo(t)
	if scratch.Ignored(repo) {
		t.Fatal("a fresh repo should not ignore the scratch yet")
	}
	if !scratch.EnsureIgnored(repo) {
		t.Fatal("EnsureIgnored reported the scratch is still not ignored")
	}
	if !scratch.Ignored(repo) {
		t.Error("Ignored disagrees with EnsureIgnored")
	}

	common, err := repo.CommonDir()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(common, "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	// The pair is the unit. `.mkit/*` alone would hide repo config from `git add`;
	// the old directory-only `.mkit/` would make the negation impossible to add.
	for _, want := range []string{"\n.mkit/*\n", "\n!.mkit/config.toml\n", "Added by mkit"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("exclude file is missing %q:\n%s", want, b)
		}
	}
}

// The committed file must stay committable, or `mkit init` writes a config that
// silently never travels.
func TestEnsureIgnoredLeavesConfigCommittable(t *testing.T) {
	repo := newRepo(t)
	scratch.EnsureIgnored(repo)
	cmd := exec.Command("git", "check-ignore", "-q", "--", ".mkit/config.toml")
	cmd.Dir = repo.Toplevel
	if err := cmd.Run(); err == nil {
		t.Error(".mkit/config.toml is ignored — the negation did not take")
	}
}

func TestEnsureIgnoredIsIdempotent(t *testing.T) {
	repo := newRepo(t)
	scratch.EnsureIgnored(repo)
	common, err := repo.CommonDir()
	if err != nil {
		t.Fatal(err)
	}
	exclude := filepath.Join(common, "info", "exclude")
	first, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatal(err)
	}
	scratch.EnsureIgnored(repo)
	second, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("a second call appended again:\n%s", second)
	}
}

// One probe cannot speak for both. An unrelated `*.jsonl` rule hides the ledger
// while leaving every run directory untracked — under which a yes is reported to
// a skill whose worktree teardown then fails.
func TestIgnoredNeedsBothProbes(t *testing.T) {
	repo := newRepo(t)
	if err := os.WriteFile(filepath.Join(repo.Toplevel, ".gitignore"), []byte("*.jsonl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if scratch.Ignored(repo) {
		t.Error("a rule that only hides gate.jsonl must not report the scratch as ignored")
	}
}

// A committed .gitignore carrying the pair is the variant for repos whose
// teammates run mkit from fresh clones — and is what mk-toolkit itself ships.
func TestIgnoredAcceptsACommittedGitignore(t *testing.T) {
	repo := newRepo(t)
	if err := os.WriteFile(filepath.Join(repo.Toplevel, ".gitignore"),
		[]byte(".mkit/*\n!.mkit/config.toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !scratch.Ignored(repo) {
		t.Error("a committed .gitignore pair must count")
	}
	// And the legacy directory-only rule, which every pre-config repo carries.
	if err := os.WriteFile(filepath.Join(repo.Toplevel, ".gitignore"), []byte(".mkit/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !scratch.Ignored(repo) {
		t.Error("the legacy directory-only rule must still count as ignored")
	}
}

func TestDir(t *testing.T) {
	repo := newRepo(t)
	if got, want := scratch.Dir(repo), filepath.Join(repo.Toplevel, ".mkit"); got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
}
