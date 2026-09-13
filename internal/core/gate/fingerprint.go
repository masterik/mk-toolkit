// Package gate is the quality gate: the content fingerprint a proof is keyed on,
// the ledger that records what was proven, the ecosystem discovery that proposes
// commands, and the runner that executes them.
//
// Layering: returns data, never prints, never assumes a terminal.
package gate

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // git's own object hash; not a security choice
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// Fingerprint returns a short hash identifying *the content a quality-gate
// command would read*. An error means "no fingerprint" — callers degrade to
// running the gate, never fail.
//
// The load-bearing property is that it is invariant under staging and
// committing. The flagship flow is `pr` (gate before opening) → `finish` (gate
// again before merge). A key built from HEAD plus the dirty set would classify as
// drifted the instant the commit lands, even though not one byte the gate reads
// changed — and the whole feature would save exactly nothing. So the key is the
// canonical path → blob mapping the commands actually see:
//
//  1. `git ls-tree -r HEAD`                       the committed mapping
//  2. overlay every path that differs from HEAD   with its *worktree* blob sha
//  3. drop paths deleted in the worktree          (a deletion must leave the
//     mapping, not carry its committed blob)
//  4. add untracked-but-not-ignored paths         same overlay, same batch
//  5. sort, sha256, keep 16 hex characters
//
// Staging is invisible because staging does not change a worktree blob, and the
// dirty pre-commit tree and the clean post-commit tree yield the same mapping.
//
// `--no-renames` and `ls-files --others`, rather than parsing `git status
// --porcelain`: porcelain pairs a rename with a second NUL record that a reader
// must consume or desynchronize from, and reports an untracked *directory* as one
// entry whose contents are then invisible. These two plumbing commands have
// neither trap.
//
// What it cannot see, by construction: file mode (`chmod +x` does not change a
// blob sha — a documented gap), dependency installs, tool versions, env vars, and
// anything ignored. A match therefore means "the tracked content is identical",
// not "the environment is identical" — which is why the ledger also carries an
// age bound.
func Fingerprint(repo *gitrepo.Repo) (string, error) {
	root := repo.Toplevel

	// Everything that differs from HEAD, plus everything untracked and not
	// ignored. `:(exclude).mkit` on both sides, belt to the exclude file's
	// braces: mkit's own scratch root must never enter its own fingerprint, or a
	// run invalidates its own cache entry mid-gate. EnsureIgnored normally keeps
	// it out of `--others` already; this holds even in the session where that
	// write was refused.
	//
	// An unborn HEAD makes the first command fail; that is a normal state and
	// not an error, so both enumerations tolerate it and contribute nothing.
	paths := map[string]bool{}
	for _, args := range [][]string{
		{"diff", "--name-only", "-z", "--no-renames", "HEAD", "--", ".", ":(exclude).mkit"},
		{"ls-files", "--others", "--exclude-standard", "-z", "--", ".", ":(exclude).mkit"},
	} {
		out, err := gitOut(root, args...)
		if err != nil {
			continue
		}
		for _, p := range splitNUL(out) {
			paths[p] = true
		}
	}

	// The mapping, built in the order the layers win: HEAD first, worktree
	// overlays over it, deletions last.
	blob := map[string]string{}

	// `ls-tree` refuses pathspec magic outright ("fatal: pathspec magic not
	// supported by this command: 'exclude'"), so the reserved root is dropped
	// here rather than asked for. Without the guard the exclusion above was
	// half-applied — the overlays dropped a tracked `.mkit/` path while this
	// mapping still contributed its HEAD blob, so committing an otherwise
	// identical worktree changed the fingerprint and broke the commit-invariance
	// the whole ledger rests on. Only reachable in a repo that tracked `.mkit/`
	// before upgrading: the ignore rules stop it being tracked from here on, but
	// neither untracks what already is.
	if out, err := gitOut(root, "ls-tree", "-r", "-z", "HEAD"); err == nil {
		for _, ent := range splitNUL(out) {
			// `<mode> SP <type> SP <object> TAB <path>`
			tab := strings.IndexByte(ent, '\t')
			if tab < 0 {
				continue
			}
			meta, path := ent[:tab], ent[tab+1:]
			if path == ".mkit" || strings.HasPrefix(path, ".mkit/") {
				continue
			}
			f := strings.Fields(meta)
			if len(f) < 3 {
				continue
			}
			blob[path] = f[2]
		}
	}

	deleted := map[string]bool{}
	var regular []string
	for p := range paths {
		if p == "" {
			continue
		}
		abs := filepath.Join(root, p)
		fi, err := os.Lstat(abs)
		switch {
		case err != nil:
			// Gone — or no longer reachable. A tracked file replaced by a
			// DIRECTORY is the real case (splitting a module into a package),
			// and it must leave the mapping rather than keep its HEAD blob.
			deleted[p] = true
		case fi.Mode()&os.ModeSymlink != 0:
			// Checked before regular-file, which would follow the link. git
			// stores a symlink as a blob of its *target path*; hashing the
			// target's content would make a repo with any tracked symlink read
			// `drifted` forever.
			target, lerr := os.Readlink(abs)
			if lerr != nil {
				deleted[p] = true
				continue
			}
			blob[p] = blobHash([]byte(target))
		case fi.Mode().IsRegular():
			// Hashed by git, not in process: see hashRegular.
			regular = append(regular, p)
		default:
			deleted[p] = true
		}
	}

	if err := hashRegular(root, regular, blob); err != nil {
		// One answer per path, or no fingerprint at all. Degrading to "run the
		// gate" is free; a wrong hash is not — two different trees hashing alike
		// is the one failure a gate ledger may not have.
		return "", err
	}

	lines := make([]string, 0, len(blob))
	for p, b := range blob {
		if deleted[p] {
			continue
		}
		lines = append(lines, p+"\t"+b)
	}
	// Byte order, matching `LC_ALL=C sort`; Go string comparison already is one.
	sort.Strings(lines)

	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(l))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// hashRegular fills blob[] for the worktree's regular files, batched through one
// `git hash-object --stdin-paths`.
//
// It has to be git that hashes them, not blobHash. A path with a clean filter or
// `text` eol normalization is stored by git as the *filtered* bytes, so hashing
// the raw worktree bytes made the overlay disagree with the ls-tree layer for
// exactly those paths: committing a file changed the fingerprint while its
// content did not, and every gate proof taken before that commit was thrown
// away. `hash-object` applies each path's own attributes, which is why the shell
// used it and why an in-process sha1 cannot replace it.
//
// One fork for the whole batch keeps the cost the in-process version was reaching
// for — a per-path fork measured ~55x slower at 1000 dirty files. Paths are
// newline-delimited, so the rare path containing a newline is hashed on its own
// rather than corrupting the batch.
func hashRegular(root string, paths []string, blob map[string]string) error {
	var batch []string
	for _, p := range paths {
		if strings.ContainsAny(p, "\n\r") {
			out, err := gitOut(root, "hash-object", "--", p)
			if err != nil {
				return fmt.Errorf("fingerprint: hash-object %s: %w", p, err)
			}
			blob[p] = strings.TrimSpace(string(out))
			continue
		}
		batch = append(batch, p)
	}
	if len(batch) == 0 {
		return nil
	}

	out, err := gitStdin(root, strings.Join(batch, "\n")+"\n", "hash-object", "--stdin-paths")
	if err != nil {
		return fmt.Errorf("fingerprint: hash-object --stdin-paths: %w", err)
	}
	hashes := strings.Fields(string(out))
	// One hash per path, in order. A short read means git skipped a path, and a
	// mapping missing an entry is a different tree that would hash alike.
	if len(hashes) != len(batch) {
		return fmt.Errorf("fingerprint: hash-object returned %d hashes for %d paths",
			len(hashes), len(batch))
	}
	for i, p := range batch {
		blob[p] = hashes[i]
	}
	return nil
}

// blobHash is `git hash-object` for a blob, in process: sha1 over the object
// header and the content. Used for symlinks only — git stores a symlink as a blob
// of its target path, and no attribute filter applies to it, so there is nothing
// for git to do that this does not.
func blobHash(data []byte) string {
	h := sha1.New() //nolint:gosec // git's own object hash
	// hash.Hash never returns an error from Write, by contract.
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func splitNUL(b []byte) []string {
	b = bytes.TrimSuffix(b, []byte{0})
	if len(b) == 0 {
		return nil
	}
	return strings.Split(string(b), "\x00")
}

// gitOut runs git in dir and returns stdout. Errors are the caller's to tolerate;
// nothing here writes to stderr or the process's own streams.
func gitOut(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Output()
}

// gitStdin is gitOut with input on stdin, for the batched plumbing calls.
func gitStdin(dir, stdin string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.Output()
}
