package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixtureFile describes one file (or symlink) to materialize under a
// fixture home. age, if non-zero, backdates the file's mtime that far
// before time.Now(). symlinkTo, if set, makes rel a symlink to that target
// instead of a regular file.
type fixtureFile struct {
	rel       string
	age       time.Duration
	size      int64
	symlinkTo string
}

// fixtureDir explicitly creates an (otherwise-empty) directory.
type fixtureDir struct {
	rel string
}

// buildFixtureHome materializes a synthetic provider home under a fresh
// t.TempDir(), deterministically, from the given files and dirs.
func buildFixtureHome(t *testing.T, files []fixtureFile, dirs []fixtureDir) string {
	t.Helper()
	root := t.TempDir()

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d.rel), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d.rel, err)
		}
	}

	for _, f := range files {
		full := filepath.Join(root, f.rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", f.rel, err)
		}
		if f.symlinkTo != "" {
			if err := os.Symlink(f.symlinkTo, full); err != nil {
				t.Fatalf("symlink %s -> %s: %v", f.rel, f.symlinkTo, err)
			}
			continue
		}
		if err := os.WriteFile(full, make([]byte, f.size), 0o644); err != nil {
			t.Fatalf("write %s: %v", f.rel, err)
		}
		if f.age != 0 {
			mtime := time.Now().Add(-f.age)
			if err := os.Chtimes(full, mtime, mtime); err != nil {
				t.Fatalf("chtimes %s: %v", f.rel, err)
			}
		}
	}

	return root
}

func writeFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func chtimesAt(t *testing.T, path string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}
