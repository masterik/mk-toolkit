// Package worklog is the per-branch record of what each workflow step concluded.
//
// `<toplevel>/.mkit/work/<branch>.jsonl` — append-only, one record per finished
// step, never committed, and a linked worktree gets its own. It is the mechanism
// behind the workflow contract's fourth rule: a step records for the next one and
// never gates on the last one. A later step reads the log to be cheaper and better
// informed; a log it cannot read costs it a derivation, never a run.
//
// Per branch, not per invocation. A run directory (`<skill>-<timestamp>/`) belongs
// to one call and holds its working files; the worklog spans every call on a branch,
// which is the unit of work the finishing steps act on.
//
// Layering: returns data, never prints, never assumes a terminal.
package worklog

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// ErrNoGist means a record carried no gist. Rejected at the door: the gist is the
// whole reason a later step reads the log.
var ErrNoGist = errors.New("a record needs a gist")

// ErrBadStep means the step name is outside Steps.
var ErrBadStep = errors.New("unknown step")

// Log is one branch's worklog in one work tree.
type Log struct {
	repo   *gitrepo.Repo
	branch string
}

// Open resolves the log for branch in repo. An empty branch means the current one;
// a detached HEAD has no branch and gets a log named after the head it is sitting
// on, which keeps a detached session recording somewhere rather than nowhere.
func Open(repo *gitrepo.Repo, branch string) *Log {
	if branch == "" {
		branch = repo.Branch()
		if branch == "" {
			// `~` is forbidden in a ref name and escapes to %7E below, so a
			// detached log can never share a file with a branch that merely
			// spells itself "detached-<sha>".
			if head := repo.Head(); len(head) >= 12 {
				branch = detachedPrefix + head[:12]
			} else {
				branch = detachedPrefix + "unborn"
			}
		}
	}
	return &Log{repo: repo, branch: branch}
}

// Branch is the branch this log belongs to.
func (l *Log) Branch() string { return l.branch }

// Dir is the directory holding every branch's log in this work tree.
func Dir(toplevel string) string { return filepath.Join(toplevel, ".mkit", "work") }

// Path is the absolute path of this log.
func (l *Log) Path() string { return filepath.Join(Dir(l.repo.Toplevel), FileName(l.branch)) }

// detachedPrefix names a log with no branch behind it. `~` cannot appear in a ref
// name, so nothing a branch is called can collide with one of these.
const detachedPrefix = "detached~"

// nameMax is the longest filename the supported filesystems accept — 255 bytes on
// APFS, HFS+ and every ext/XFS variant. FileName budgets against it rather than
// letting the branch decide the length.
const nameMax = 255

// FileName maps a branch name to its log file.
//
// One exported function, used by both verbs, because a mapping that show and append
// derive separately is a mapping they will eventually disagree about — and the
// symptom is an empty log, not an error.
//
// `/` is the real hazard: `feature/x.jsonl` would be a *directory* named `feature`,
// so the whole set is escaped rather than just slashes. Anything outside
// [A-Za-z0-9._-] becomes %XX, which is reversible and, more importantly, injective —
// two branches never share a file. A leading dot is escaped too, so a log never
// hides from `ls`.
//
// **Case is the one place the escaping is not enough.** macOS is the supported
// platform and its filesystem is case-insensitive by default, so `JIRA-123` and
// `jira-123` would open the same file and interleave two branches' histories. Case
// is kept rather than escaped — `%4AIRA-123` is unreadable, and ticket-style names
// are common — and every name instead ends with a short digest of the exact branch.
//
// The digest is on **every** name, not only the ones carrying an uppercase letter,
// and that is the whole point: a suffix added selectively is still ordinary branch
// text, so `FOO` → `FOO-<digest>` collides with a real branch called
// `foo-<digest>`. Unconditional, two names collide only if their digests match.
// It is the **full** SHA-256, not a prefix of it: a truncated digest turns
// "the branches were the same string" into "the branches were the same string, or
// unlucky", and the log it silently merges is the record two steps hand each other.
// The readable part stays in front, which is what a human reads in `ls`.
//
// Which is also why the readable part is what gets **truncated**, never the digest.
// git allows branch names far longer than a filename may be, and a name over
// NAME_MAX does not degrade — `open` fails with ENAMETOOLONG and the branch cannot
// record at all. So the escaped text is cut at a whole-token boundary (a `%XX`
// escape is never split) to whatever the digest leaves room for. Injectivity is
// unaffected: it was never carried by the readable half.
func FileName(branch string) string {
	sum := sha256.Sum256([]byte(branch))
	suffix := fmt.Sprintf("-%x.jsonl", sum[:])

	var b strings.Builder
	for i := 0; i < len(branch); i++ {
		c := branch[i]
		safe := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || (c == '.' && i > 0)
		tok := string(c)
		if !safe {
			tok = fmt.Sprintf("%%%02X", c)
		}
		if b.Len()+len(tok)+len(suffix) > nameMax {
			break
		}
		b.WriteString(tok)
	}
	return b.String() + suffix
}

// Append writes one record and then rotates.
//
// Deliberately *not* best-effort, which is where it parts company with the gate
// ledger: `mkit work append` is a command someone invoked, so a write it could not
// perform is an error with the path and the cause. The best-effort half lives in the
// skills, which append after their report is produced and treat a failure as one line
// of note — contract rule 4 says a recorded fact is an input, never a permission, so
// a failed append must not be able to fail a run.
//
// No lock. A single write(2) under O_APPEND is atomic below PIPE_BUF, and a record is
// far below it; taking a lock here would cost the append the never-blocks property the
// whole side effect rests on.
func (l *Log) Append(rec Record) error {
	if !ValidStep(rec.Step) {
		return fmt.Errorf("%w: %q (one of: %s)", ErrBadStep, rec.Step, strings.Join(Steps, ", "))
	}
	if strings.TrimSpace(rec.Gist) == "" {
		return ErrNoGist
	}
	rec.TS = now()
	rec.Schema = Schema
	if rec.Head == "" {
		rec.Head = l.head()
	}
	if rec.Assumptions == nil {
		rec.Assumptions = []string{}
	}

	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	path := l.Path()
	// Before the MkdirAll, never after — the same ordering, for the same reasons,
	// as run-open.sh's. A standalone `mkit work append` may be the first thing ever
	// to write under `.mkit/` in this repo.
	ensureIgnored(l.repo.Toplevel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("worklog directory %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("worklog %s: %w", path, err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("worklog %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("worklog %s: %w", path, err)
	}

	// Rotation is bookkeeping about a record that is already durable: its failure
	// is not this command's failure.
	_ = l.rotate()
	return nil
}

// head is the commit this record describes: HEAD for the branch actually checked
// out, and the named branch's own tip for a `--branch other` append. Recording the
// current HEAD in another branch's log would hand rotation and every later reader a
// commit that has nothing to do with the work the record is about.
func (l *Log) head() string {
	if l.branch == l.repo.Branch() || strings.HasPrefix(l.branch, detachedPrefix) {
		return l.repo.Head()
	}
	return l.repo.BranchHead(l.branch)
}

// Query narrows what Show returns.
type Query struct {
	// Steps, when non-empty, keeps only records for those steps.
	Steps []string
	// Limit, when > 0, keeps the newest N after filtering.
	Limit int
}

// Show returns the records in file order — oldest first, so the last element is the
// most recent thing that happened on this branch.
//
// A missing file is zero records and no error. A branch nothing has run on is the
// normal case for an entry-capable step, and an error there would turn "nothing to
// read" into something a caller has to handle before it can carry on.
//
// A malformed line is skipped, not fatal, for the same reason: one bad line must not
// cost a reader the records around it.
func (l *Log) Show(q Query) ([]Record, error) {
	f, err := os.Open(l.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("worklog %s: %w", l.Path(), err)
	}
	defer func() { _ = f.Close() }()

	keep := map[string]bool{}
	for _, s := range q.Steps {
		keep[s] = true
	}

	out := []Record{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec Record
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if len(keep) > 0 && !keep[rec.Step] {
			continue
		}
		if rec.Assumptions == nil {
			rec.Assumptions = []string{}
		}
		out = append(out, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("worklog %s: %w", l.Path(), err)
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[len(out)-q.Limit:]
	}
	return out, nil
}
