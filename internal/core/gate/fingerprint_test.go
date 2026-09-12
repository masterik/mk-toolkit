package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// git, run against a throwaway repo under $TMPDIR. Never the real home directory,
// and never a repo the developer is working in: every fixture here commits.
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

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo returns a committed base repo: two tracked files and a .gitignore.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "a.txt", "alpha\n")
	write(t, dir, "dir/b.txt", "beta\n")
	write(t, dir, ".gitignore", "ignored.txt\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	return dir
}

// emptyRepo has no commits: an unborn HEAD is a normal state, not an error.
func emptyRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	return dir
}

func fingerprintOf(t *testing.T, dir string) string {
	t.Helper()
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatalf("open %s: %v", dir, err)
	}
	fp, err := Fingerprint(repo)
	if err != nil {
		t.Fatalf("Fingerprint(%s): %v", dir, err)
	}
	return fp
}

// The fixture matrix. Each mutates a base repo into one of the shapes the
// fingerprint has to get right; `setup` is nil where the base repo is the case.
var fixtures = []struct {
	name  string
	base  func(t *testing.T) string
	setup func(t *testing.T, dir string)
}{
	{name: "clean", base: newRepo},
	{name: "unborn-head", base: emptyRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "a.txt", "alpha\n")
	}},
	{name: "staged-only", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "a.txt", "ALPHA\n")
		git(t, dir, "add", "a.txt")
	}},
	{name: "unstaged-only", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "a.txt", "ALPHA\n")
	}},
	{name: "staged-and-unstaged", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "a.txt", "ALPHA\n")
		git(t, dir, "add", "a.txt")
		write(t, dir, "dir/b.txt", "BETA\n")
	}},
	{name: "untracked", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "new.txt", "new\n")
	}},
	{name: "untracked-directory", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "pkg/one.txt", "1\n")
		write(t, dir, "pkg/two.txt", "2\n")
	}},
	{name: "ignored-file-present", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "ignored.txt", "noise\n")
	}},
	{name: "tracked-symlink", base: newRepo, setup: func(t *testing.T, dir string) {
		if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		git(t, dir, "add", "link")
		git(t, dir, "commit", "-q", "-m", "link")
		// Retarget it: a symlink's blob is its target path, so this is a change
		// even though the file it points at is untouched.
		if err := os.Remove(filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("dir/b.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
	}},
	{name: "untracked-symlink", base: newRepo, setup: func(t *testing.T, dir string) {
		if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
	}},
	{name: "deleted-tracked-file", base: newRepo, setup: func(t *testing.T, dir string) {
		if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
			t.Fatal(err)
		}
	}},
	{name: "tracked-file-replaced-by-directory", base: newRepo, setup: func(t *testing.T, dir string) {
		if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
			t.Fatal(err)
		}
		write(t, dir, "a.txt/inner.txt", "inner\n")
	}},
	{name: "path-with-a-space", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, "two words.txt", "spaced\n")
	}},
	{name: "tracks-dot-mkit", base: newRepo, setup: func(t *testing.T, dir string) {
		write(t, dir, ".mkit/leftover.txt", "committed before the ignore rule existed\n")
		git(t, dir, "add", "-f", ".mkit/leftover.txt")
		git(t, dir, "commit", "-q", "-m", "legacy scratch")
		write(t, dir, ".mkit/leftover.txt", "changed under the gate's feet\n")
		write(t, dir, ".mkit/gate.jsonl", "{}\n")
	}},
}

// The port must agree with `mkit_tree_fingerprint` byte for byte. Not for
// cross-version ledger compatibility — a mismatch degrades safely to `drifted`,
// i.e. "run the gate" — but because it is the cheapest possible proof the port is
// right. Deleted with the script it compares against; TestFingerprintGolden is
// what survives.
func TestFingerprintParityWithShell(t *testing.T) {
	common := commonSh(t)
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			dir := f.base(t)
			if f.setup != nil {
				f.setup(t, dir)
			}
			want := shellFingerprint(t, common, dir)
			if got := fingerprintOf(t, dir); got != want {
				t.Errorf("fingerprint = %q, shell says %q", got, want)
			}
		})
	}
}

// Known-good hashes over the same fixtures, so the matrix outlives the shell.
// A change here is a change to the ledger's key: every existing record in every
// repo reclassifies as `drifted` and every gate re-runs once. That is safe, and
// it is never accidental — update these only alongside a deliberate change to
// what the fingerprint covers.
func TestFingerprintGolden(t *testing.T) {
	golden := map[string]string{
		"clean":                              "fab2136b9085eb01",
		"unborn-head":                        "1106be50325aeb9e",
		"staged-only":                        "530def36328952d8",
		"unstaged-only":                      "530def36328952d8",
		"staged-and-unstaged":                "4bf916e4ee926ff3",
		"untracked":                          "3554cbfbcdca7a1f",
		"untracked-directory":                "0e4b0e5de9baacbc",
		"ignored-file-present":               "fab2136b9085eb01",
		"tracked-symlink":                    "c65263b2fe3871ec",
		"untracked-symlink":                  "ab98969177820d28",
		"deleted-tracked-file":               "fb0fb030dbed4ce3",
		"tracked-file-replaced-by-directory": "9230e934805b3948",
		"path-with-a-space":                  "a669d03a5cec4463",
		"tracks-dot-mkit":                    "fab2136b9085eb01",
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			want, ok := golden[f.name]
			if !ok {
				t.Fatalf("fixture %q has no golden hash", f.name)
			}
			dir := f.base(t)
			if f.setup != nil {
				f.setup(t, dir)
			}
			if got := fingerprintOf(t, dir); got != want {
				t.Errorf("fingerprint = %q, want %q", got, want)
			}
		})
	}
}

// The property the whole ledger rests on: identical content hashes the same
// whether it is dirty, staged, or committed. Without it a `pr` → `finish` cache
// hit is impossible and the feature saves exactly nothing.
func TestFingerprintInvariantUnderStagingAndCommitting(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "ALPHA\n")
	write(t, dir, "new.txt", "new\n")

	dirty := fingerprintOf(t, dir)
	git(t, dir, "add", "-A")
	staged := fingerprintOf(t, dir)
	git(t, dir, "commit", "-q", "-m", "same content")
	committed := fingerprintOf(t, dir)

	if dirty != staged || staged != committed {
		t.Errorf("dirty=%s staged=%s committed=%s — all three must agree", dirty, staged, committed)
	}
}

// mkit's own scratch is never part of its own fingerprint, or a run invalidates
// its own cache entry mid-gate. True even in a repo that tracked `.mkit/` before
// the ignore rules existed — the exclusion covers the HEAD mapping too.
func TestFingerprintExcludesScratch(t *testing.T) {
	dir := newRepo(t)
	before := fingerprintOf(t, dir)

	write(t, dir, ".mkit/gate.jsonl", "{}\n")
	write(t, dir, ".mkit/review-1/gate-lint.log", "noise\n")
	if got := fingerprintOf(t, dir); got != before {
		t.Errorf("untracked scratch changed the fingerprint: %s -> %s", before, got)
	}

	git(t, dir, "add", "-f", ".mkit/gate.jsonl")
	git(t, dir, "commit", "-q", "-m", "legacy tracked scratch")
	if got := fingerprintOf(t, dir); got != before {
		t.Errorf("tracked scratch changed the fingerprint: %s -> %s", before, got)
	}
}

func TestFingerprintNotARepo(t *testing.T) {
	if _, err := gitrepo.Open(t.TempDir()); err == nil {
		t.Fatal("expected a plain directory not to open as a repo")
	}
}

// commonSh locates the shell library this package is replacing.
func commonSh(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../../plugin/scripts/lib/common.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("shell layer gone: %v", err)
	}
	return p
}

func shellFingerprint(t *testing.T, common, dir string) string {
	t.Helper()
	cmd := exec.Command("bash", "-c", `. "$1" && cd "$2" && mkit_tree_fingerprint`, "bash", common, dir)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("mkit_tree_fingerprint: %v", err)
	}
	return strings.TrimRight(string(out), "\n")
}
