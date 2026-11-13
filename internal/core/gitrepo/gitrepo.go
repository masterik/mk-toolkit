// Package gitrepo answers the two questions about the surrounding git
// repository the binary still asks: where the work tree is, and where its
// common dir is. `doctor` reads both for the leftovers older releases wrote;
// `claudecfg` uses Open to find a work tree's settings. Every other repo
// question moved into the plugin's scripts (ADR 0006).
//
// It shells out to git rather than linking a git library, with stable machine
// output only.
//
// Layering: returns data, never prints, never assumes a terminal.
package gitrepo

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotARepo is returned by every function here when there is no work tree.
var ErrNotARepo = errors.New("not inside a git repository")

// Repo is a resolved work tree. Construct it with Open.
type Repo struct {
	// Toplevel is the absolute path of this work tree's root — `--show-toplevel`,
	// so a linked worktree resolves to its own root rather than the main checkout.
	Toplevel string
}

// Open resolves the work tree containing dir ("" means the process working directory).
func Open(dir string) (*Repo, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil || out == "" {
		return nil, ErrNotARepo
	}
	return &Repo{Toplevel: out}, nil
}

// CommonDir is the absolute `--git-common-dir`: the repository storage shared by
// every linked worktree, and where `info/exclude` lives — which `doctor` reads
// for the ignore pair an older mkit wrote there.
//
// git answers relatively when asked from inside the work tree, so the result is
// resolved against Toplevel rather than returned as given.
func (r *Repo) CommonDir() (string, error) {
	out, err := run(r.Toplevel, "rev-parse", "--git-common-dir")
	if err != nil || out == "" {
		return "", ErrNotARepo
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(r.Toplevel, out)
	}
	return filepath.Clean(out), nil
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}
