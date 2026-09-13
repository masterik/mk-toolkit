package scratch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UserDir is the user-scoped state directory — where mkit state that outlives a
// repo goes. `MKIT_HOME` overrides it, and tests set it so a run can never read
// or write a developer's real state.
//
// **Empty today.** It held exactly two files, `bootstrap.state` and
// `bootstrap.disabled`, and both died with the `SessionStart` hook. The directory
// keeps its definition anyway: it is the answer to "where does user-scoped state
// go", the path a remedy sentence can point at, and what `MKIT_HOME` redirects.
//
// `~/.mkit`, not `~/.claude/mkit`, and the reason is a hard boundary rather than
// taste (ADR 0002): `~/.claude` is a protected region of the OS sandbox, where an
// allowlist entry is *inert*. Outside it, one `permissions.additionalDirectories`
// entry genuinely opens the path — so this directory is somewhere a remedy can
// point. Not `$TMPDIR` either: that is the location for what dies with the
// command, and sandboxed and unsandboxed commands do not even resolve it to the
// same directory.
func UserDir() string {
	if h := os.Getenv("MKIT_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// $HOME unset or empty. Returning a relative ".mkit" would move
		// user-scoped state into whatever repo the caller happens to be standing
		// in — a fourth write location, and a remedy naming a relative path that
		// no permissions.additionalDirectories entry can grant. The shell failed
		// loudly here (`${MKIT_HOME:-$HOME/.mkit}` expanded to `/.mkit`); an
		// empty string is this package's "no user directory", and every caller
		// reports it as a fact rather than writing to it.
		return ""
	}
	return filepath.Join(home, ".mkit")
}

// UserDirWritable reports whether the user-scoped directory actually takes a
// write. A fact, reported at the first call, so a blocked write is a starting
// fact rather than a mid-run "Operation not permitted".
//
// Probed by writing, not by a mode check: the sandbox denies the write itself
// while leaving the mode bits saying yes, and the case it actually produces is a
// directory that exists and takes a MkdirAll but refuses a create.
//
// Net-zero by construction: the probe file always goes, and every directory this
// had to create is removed again. `mkit facts` calls this, and `facts` writes
// nothing outside the run directory it opens — creating user-scoped state as a
// side effect of reporting on it would make the report the thing that changed the
// answer.
//
// "Every" is load-bearing, and is why this counts absent ancestors rather than
// setting a flag: MKIT_HOME need not have an existing parent, MkdirAll creates
// the whole chain, and one Remove at the end takes only the leaf. That left
// structure behind on disk while still returning success.
func UserDirWritable() bool {
	dir := UserDir()
	if dir == "" {
		return false
	}

	// Every absent ancestor, deepest first — exactly the ones MkdirAll is about
	// to create, and nothing else. A directory that already exists is never ours.
	var created []string
	for d := dir; ; {
		if _, err := os.Stat(d); err == nil {
			break
		}
		created = append(created, d)
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}

	if len(created) > 0 {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false
		}
	}

	ok := true
	probe := filepath.Join(dir, fmt.Sprintf(".writable.%d", os.Getpid()))
	if err := os.WriteFile(probe, []byte("probe\n"), 0o644); err != nil {
		ok = false
	}
	_ = os.Remove(probe)

	// Unwind deepest first, with a directory-only remove, and stop at the first
	// refusal: anything that gained content since the MkdirAll belongs to whoever
	// put it there, and Remove declining is precisely the check for that. A
	// recursive remove here would delete a concurrent run's state.
	for _, d := range created {
		if err := os.Remove(d); err != nil {
			break
		}
	}
	return ok
}

// UserDirRemedy is the one sentence for an unwritable user-scoped directory. One
// producer, so `mkit doctor` and `mkit facts` cannot word it differently.
//
// `permissions.additionalDirectories` rather than `sandbox.filesystem.allowWrite`:
// the former grants the sandbox write *and* makes the path a working directory,
// which is what also satisfies the auto-mode classifier's "no writes outside the
// working directories" rule. allowWrite alone leaves that rule biting, so it is
// named as the narrower alternative and never as the remedy.
//
// **Both halves, always.** The grant covers the directory's *interior*; creating
// the directory is a write to its parent, which nothing grants — measured:
// `mkdir: /Users/mk/.mkit: Operation not permitted`. A sentence naming only the
// grant produced a configuration that looked right and changed nothing, which is
// the exact failure ADR 0002 was written about, one level down. The `mkdir` half
// is human-run by construction: no sandboxed session can perform it, so it is
// spelled with the `!` prefix that runs a command in the user's own shell.
func UserDirRemedy() string {
	dir := UserDir()
	if dir == "" {
		return "$HOME is unset, so there is no user-scoped directory to grant — " +
			"set MKIT_HOME to an absolute path, or run with HOME set"
	}
	return fmt.Sprintf("run `! mkdir -p %s` (a sandboxed session cannot create it), "+
		"then add %s to permissions.additionalDirectories "+
		"(sandbox.filesystem.allowWrite grants the sandbox only)", ShellQuote(dir), dir)
}

// ShellQuote quotes a path for the copy-and-run half of a remedy. Only the
// command is quoted; an allowlist entry is JSON the user types into settings, not
// shell.
//
// MKIT_HOME can point anywhere, including a path with spaces, and a remedy
// printed as `! mkdir -p /Users/x/my state` creates two wrong directories rather
// than failing — the worst kind of wrong, since it looks like it worked.
func ShellQuote(s string) string {
	if strings.IndexFunc(s, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return false
		case r == '/', r == '.', r == '_', r == '-':
			return false
		}
		return true
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// IgnoredRemedy is the one producer of the unignored-scratch sentence, the
// counterpart to UserDirRemedy above.
//
// It has to name `commonDir` rather than a fixed path: under a linked worktree
// the exclude file lives in the main checkout, which is exactly the session that
// cannot reach it. Both callers — `mkit facts`' `run_ignored=no` note and `mkit
// doctor`'s scratch check — read this rather than wording it again; the pair of
// lines is the rule, and a caller that wrote only `.mkit/*` would hide repo
// config from `git add`.
func IgnoredRemedy(commonDir string) string {
	return fmt.Sprintf("from the main checkout, add `.mkit/*` and `!.mkit/config.toml` "+
		"to %s/info/exclude, or to .gitignore", commonDir)
}
