package storage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Selection is the set of entry paths Apply should delete. SelectAll builds
// the non-interactive default; the TUI builds a subset by toggling entries.
type Selection struct {
	paths map[string]bool
}

// NewSelection returns an empty selection.
func NewSelection() *Selection {
	return &Selection{paths: map[string]bool{}}
}

// SelectAll returns a selection containing every entry in report.
func SelectAll(report *Report) *Selection {
	sel := NewSelection()
	for _, p := range report.Providers {
		for _, c := range p.Categories {
			for _, e := range c.Entries {
				sel.Add(e.Path)
			}
		}
	}
	return sel
}

// Add marks path selected.
func (s *Selection) Add(path string) { s.paths[path] = true }

// Has reports whether path is selected.
func (s *Selection) Has(path string) bool { return s.paths[path] }

// Skip is one selected path Apply deliberately left alone.
type Skip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Result is what Apply did.
type Result struct {
	Deleted int         `json:"deleted"`
	Bytes   int64       `json:"bytes"`
	Skipped []Skip      `json:"skipped,omitempty"`
	Errors  []PathError `json:"errors,omitempty"`
}

// Apply deletes every path in sel that report named, and nothing else. It
// never re-walks the tree to decide what qualifies — report is the frozen
// set of candidates from Scan.
//
// Every provider's resolved home is validated before any deletion happens:
// empty, "/", or $HOME is a hard error. Every removal is then re-verified to
// live under that home, without following symlinks to establish it. A stale
// directory is re-checked for freshness immediately before removal, since
// time has passed since Scan built the report.
func Apply(report *Report, sel *Selection) (*Result, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		userHome = ""
	}
	for _, pr := range report.Providers {
		if err := guardHome(filepath.Clean(pr.Home), userHome); err != nil {
			return nil, err
		}
	}

	cutoff := time.Now().Add(-time.Duration(report.Days) * 24 * time.Hour)
	result := &Result{}

	for _, pr := range report.Providers {
		home := filepath.Clean(pr.Home)

		for _, cat := range pr.Categories {
			for _, e := range cat.Entries {
				if !sel.Has(e.Path) {
					continue
				}
				deleteEntry(home, cat, e, cutoff, result)
			}
		}

		if p, ok := ByName(pr.Name); ok && p.EmptyDirSweep != "" {
			sweepEmptyDirs(filepath.Join(home, p.EmptyDirSweep))
		}
	}

	return result, nil
}

func deleteEntry(home string, cat CategoryReport, e Entry, cutoff time.Time, result *Result) {
	clean := filepath.Clean(e.Path)
	if !underHome(clean, home) {
		result.Errors = append(result.Errors, PathError{Path: e.Path, Err: "outside provider home, refused"})
		return
	}

	if cat.Kind == KindStaleDirs {
		_, fresh, rcErrs := scanStaleDir(clean, cutoff)
		if fresh {
			result.Skipped = append(result.Skipped, Skip{Path: e.Path, Reason: "acquired a new file since scan"})
			return
		}
		if len(rcErrs) > 0 {
			result.Skipped = append(result.Skipped, Skip{Path: e.Path, Reason: "could not confirm still stale: " + rcErrs[0].Err})
			return
		}
	}

	lst, lerr := os.Lstat(clean)
	if lerr != nil {
		if os.IsNotExist(lerr) {
			result.Skipped = append(result.Skipped, Skip{Path: e.Path, Reason: "already gone"})
			return
		}
		result.Errors = append(result.Errors, PathError{Path: e.Path, Err: lerr.Error()})
		return
	}
	if lst.Mode()&fs.ModeSymlink != 0 {
		result.Skipped = append(result.Skipped, Skip{Path: e.Path, Reason: "symlink, refused"})
		return
	}
	if e.IsDir != lst.IsDir() {
		result.Skipped = append(result.Skipped, Skip{Path: e.Path, Reason: "type changed since scan"})
		return
	}

	var rmErr error
	if e.IsDir {
		rmErr = os.RemoveAll(clean)
	} else {
		rmErr = os.Remove(clean)
	}
	if rmErr != nil {
		result.Errors = append(result.Errors, PathError{Path: e.Path, Err: rmErr.Error()})
		return
	}
	result.Deleted++
	result.Bytes += e.Bytes
}

func guardHome(home, userHome string) error {
	if home == "" || home == "." || home == "/" {
		return fmt.Errorf("storage: refusing to operate on provider home %q", home)
	}
	if userHome != "" && home == filepath.Clean(userHome) {
		return fmt.Errorf("storage: refusing to operate on $HOME (%s)", home)
	}
	return nil
}

// underHome reports whether path is home or a descendant of it, purely
// lexically — it never resolves a symlink to decide.
func underHome(path, home string) bool {
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..")
}

// sweepEmptyDirs removes now-empty descendants of base, deepest first, but
// never base itself.
func sweepEmptyDirs(base string) {
	if !dirExists(base) {
		return
	}
	var dirs []string
	_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == base || !d.IsDir() {
			return nil
		}
		dirs = append(dirs, path)
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		_ = os.Remove(d) // no-op unless d is now empty
	}
}
