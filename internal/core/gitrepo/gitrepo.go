// Package gitrepo answers the handful of questions about the surrounding git
// repository that the config and profile surfaces need.
//
// It shells out to git rather than linking a git library: the payload's scripts
// already parse the same plumbing, and matching them exactly is worth more here
// than avoiding a subprocess. Every call uses stable machine output.
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

// Ignored reports whether path (relative to the work tree root) is excluded, and
// which file carries the rule.
//
// The source matters more than it looks: git reads `.gitignore` at a higher
// precedence than the common dir's `info/exclude`, so a negation written into the
// exclude cannot lift a rule that lives in a committed `.gitignore`. A remedy that
// names the wrong file is a remedy that does nothing.
//
// This is pattern matching, not a stat — it answers correctly for a path that does
// not exist yet, which is the case that matters at first run.
//
// Two calls, and the split is not redundant. `check-ignore -v` exits **0 and prints
// a pattern for a path matched by a *negation* too* — measured:
//
//	$ git check-ignore -v .mkit/config.toml
//	.gitignore:22:!.mkit/config.toml	.mkit/config.toml   (exit 0)
//	$ git check-ignore -q .mkit/config.toml               (exit 1)
//
// So -v answers "which rule decided this", not "is it ignored". Reading truth off
// -v reports every deliberately re-included file as excluded, which is exactly
// backwards for the one path this repo negates. `-q` is the boolean; `-v` runs
// only afterwards, to name the file a remedy must edit.
func (r *Repo) Ignored(path string) (ignored bool, source string) {
	ignored, source, _ = r.IgnoreRule(path)
	return ignored, source
}

// IgnoreRule additionally returns the pattern that decided the path. A remedy
// needs both: the file to edit, and which line in it — the fix for a
// directory-only `.mkit/` is not the fix for a stray `*.toml`.
func (r *Repo) IgnoreRule(path string) (ignored bool, source, pattern string) {
	if !r.Excluded(path) {
		return false, "", ""
	}
	// -z, so each field is read whole. Without it the format is
	// `<source>:<line>:<pattern>\t<path>` and splitting at the first colon
	// truncates any source path containing one — a core.excludesFile under a
	// directory with a colon in its name would name the wrong file to edit.
	//
	// git refuses `-z` except with `--stdin` ("-z only makes sense with
	// --stdin"), which is no loss: passing the path as data rather than as an
	// argument is also what makes a leading dash harmless.
	out, err := runStdin(r.Toplevel, path+"\x00", "check-ignore", "-v", "-z", "--stdin")
	if err != nil || out == "" {
		return true, "", ""
	}
	f := strings.Split(out, "\x00")
	if len(f) >= 3 {
		return true, f[0], f[2]
	}
	if len(f) >= 1 {
		return true, f[0], ""
	}
	return true, "", ""
}

// Excluded is the boolean half of Ignored — `check-ignore -q` on its own,
// without the second call that names the rule. Use it where nothing needs a
// remedy: the scratch probes ask twice per call, and doubling that to name a
// file no one reads is two forks on every skill's first call.
func (r *Repo) Excluded(path string) bool {
	_, err := run(r.Toplevel, "check-ignore", "-q", "--", path)
	return err == nil
}

// CommonDir is the absolute `--git-common-dir`: the repository storage shared by
// every linked worktree. It is where the one file mkit writes outside its own
// three locations lives — `info/exclude`.
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

// Tracked reports whether path is in the index. A tracked file is unaffected by
// ignore rules, which is why this is asked separately from Ignored.
func (r *Repo) Tracked(path string) bool {
	_, err := run(r.Toplevel, "ls-files", "--error-unmatch", "--", path)
	return err == nil
}

// Log returns up to n subject lines from HEAD, newest first. Empty on an unborn
// branch, which is a normal state and not an error.
func (r *Repo) Log(n string) []string {
	out, err := run(r.Toplevel, "log", "--no-merges", "--format=%s", "-n", n)
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// LocalConfigValue reads a key from this repository's own config only, ignoring
// the user's global and the system files.
//
// The distinction is load-bearing wherever a value is reported as *discovered from
// this repo*: `git config --get` walks system → global → local, so a developer's
// personal `pull.rebase=true` in ~/.gitconfig would otherwise be presented as a
// property every repository on the machine had declared. A profile that reports a
// user's habit as a repo's convention is worse than reporting nothing.
func (r *Repo) LocalConfigValue(key string) string {
	out, _ := run(r.Toplevel, "config", "--local", "--get", key)
	return out
}

// Remote returns the name of the first configured remote, or "".
func (r *Repo) Remote() string {
	out, err := run(r.Toplevel, "remote")
	if err != nil || out == "" {
		return ""
	}
	return strings.SplitN(out, "\n", 2)[0]
}

// RemoteURL returns the fetch URL of the named remote, or "".
func (r *Repo) RemoteURL(remote string) string {
	out, _ := run(r.Toplevel, "remote", "get-url", remote)
	return out
}

func runStdin(dir, stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	return string(out), err
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}
