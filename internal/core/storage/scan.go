package storage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options selects what Scan looks at.
type Options struct {
	Days int
	// Provider is "claude", "codex", or "all".
	Provider string
}

// Scan walks every provider matching Options.Provider and returns what
// Apply would delete. It is read-only: no path named in the result is
// touched.
func Scan(opts Options) (*Report, error) {
	if opts.Days < 0 {
		return nil, fmt.Errorf("storage: days must be non-negative, got %d", opts.Days)
	}
	switch opts.Provider {
	case "claude", "codex", "all":
	default:
		return nil, fmt.Errorf("storage: provider must be claude, codex, or all, got %q", opts.Provider)
	}

	now := time.Now()
	cutoff := now.Add(-time.Duration(opts.Days) * 24 * time.Hour)

	report := &Report{Days: opts.Days, At: now}
	for _, p := range Providers {
		if opts.Provider != "all" && opts.Provider != p.Name {
			continue
		}
		home := p.ResolveHome()
		pr := ProviderReport{Name: p.Name, Home: home, Present: dirExists(home)}
		for _, cat := range p.Categories {
			base := filepath.Join(home, cat.RelBase)
			if !dirExists(base) {
				continue
			}
			pr.Categories = append(pr.Categories, scanCategory(cat, base, cutoff))
		}
		report.Providers = append(report.Providers, pr)
	}
	return report, nil
}

func scanCategory(cat Category, base string, cutoff time.Time) CategoryReport {
	cr := CategoryReport{Label: cat.Label, Base: base, Kind: cat.Kind}
	switch cat.Kind {
	case KindFiles:
		cr.Entries, cr.Errors = scanFiles(base, cat.Pattern, cutoff)
	case KindStaleDirs:
		cr.Entries, cr.Errors = scanStaleDirs(base, cutoff)
	}
	for _, e := range cr.Entries {
		cr.Bytes += e.Bytes
	}
	return cr
}

// scanFiles recursively finds regular files under base, older than cutoff,
// optionally matching pattern. Symlinks and directories are never descended
// beyond what filepath.WalkDir already skips.
func scanFiles(base, pattern string, cutoff time.Time) ([]Entry, []PathError) {
	var entries []Entry
	var errs []PathError

	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, PathError{Path: path, Err: err.Error()})
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular() {
			return nil
		}
		if pattern != "" {
			if ok, _ := filepath.Match(pattern, d.Name()); !ok {
				return nil
			}
		}
		info, ierr := d.Info()
		if ierr != nil {
			errs = append(errs, PathError{Path: path, Err: ierr.Error()})
			return nil
		}
		if !info.ModTime().Before(cutoff) {
			return nil
		}
		entries = append(entries, Entry{Path: path, Bytes: info.Size(), ModTime: info.ModTime()})
		return nil
	})
	if err != nil {
		errs = append(errs, PathError{Path: base, Err: err.Error()})
	}
	return entries, errs
}

// scanStaleDirs finds immediate subdirectories of base — dot-directories and
// symlinks excluded — that contain no regular file at or after cutoff.
func scanStaleDirs(base string, cutoff time.Time) ([]Entry, []PathError) {
	var entries []Entry
	var errs []PathError

	children, err := os.ReadDir(base)
	if err != nil {
		errs = append(errs, PathError{Path: base, Err: err.Error()})
		return entries, errs
	}

	for _, d := range children {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		full := filepath.Join(base, d.Name())
		info, ierr := d.Info()
		var modTime time.Time
		if ierr == nil {
			modTime = info.ModTime()
		}
		bytes, fresh, dirErrs := scanStaleDir(full, cutoff)
		errs = append(errs, dirErrs...)
		if fresh {
			continue
		}
		entries = append(entries, Entry{Path: full, Bytes: bytes, ModTime: modTime, IsDir: true})
	}
	return entries, errs
}

// scanStaleDir walks one candidate directory, exiting as soon as a file at
// or after cutoff is found (the -print -quit equivalent). When it walks to
// completion without finding one, bytes is the sum of every regular file
// found along the way.
func scanStaleDir(dir string, cutoff time.Time) (bytes int64, fresh bool, errs []PathError) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, PathError{Path: path, Err: err.Error()})
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			errs = append(errs, PathError{Path: path, Err: ierr.Error()})
			return nil
		}
		if !info.ModTime().Before(cutoff) {
			fresh = true
			return filepath.SkipAll
		}
		bytes += info.Size()
		return nil
	})
	if err != nil {
		errs = append(errs, PathError{Path: dir, Err: err.Error()})
	}
	return bytes, fresh, errs
}

func dirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
