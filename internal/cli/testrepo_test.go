package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fixtures shared by the tests that drive the real cobra root over a throwaway repo.

// testGitEnv keeps a test's git away from the developer's config and stops it
// leaving work behind. `commit` and `merge` may start `maintenance --auto` (or
// `gc --auto`) detached, and one still writing into .git when the test ends
// fails t.TempDir's cleanup with "directory not empty" — measured in CI on
// TestScanMergedBranch, a test whose assertions had all passed.
var testGitEnv = []string{
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	"GIT_CONFIG_COUNT=2",
	"GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false",
	"GIT_CONFIG_KEY_1=gc.auto", "GIT_CONFIG_VALUE_1=0",
}

func gateGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	), testGitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// gateRepo is a throwaway repo under $TMPDIR with a run directory open, and the
// process working directory moved into it — never the real home, and never a
// repo a developer is working in.
func gateRepo(t *testing.T) (repo, rd string) {
	t.Helper()
	// Canonicalized: $TMPDIR is a symlink on macOS (/tmp -> /private/tmp) and git
	// reports --show-toplevel resolved, so a raw t.TempDir() path never matches
	// what every fact is reported against.
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gateGit(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gateGit(t, repo, "add", "-A")
	gateGit(t, repo, "commit", "-q", "-m", "base")
	rd = filepath.Join(repo, ".mkit", "run1")
	if err := os.MkdirAll(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	return repo, rd
}

// field reads a `key=value` line out of the output, as the skills do.
func field(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return v
		}
	}
	return ""
}

// put writes name under dir, creating its parents.
func put(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
