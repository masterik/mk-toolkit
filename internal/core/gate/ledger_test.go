package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

const bogusHead = "0123456789abcdef0123456789abcdef01234567"

func openLedger(t *testing.T, dir string) *Ledger {
	t.Helper()
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return OpenLedger(repo)
}

// seed appends n raw records without going through Append, so a test can build a
// ledger of a given size and shape without tripping rotation on the way there.
func seed(t *testing.T, l *Ledger, n int, head string, mutate func(i int, r *Record)) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	for i := 0; i < n; i++ {
		r := Record{
			Kind: "gate", TS: "2026-01-01T00:00:00Z", Epoch: time.Now().Unix(),
			Fingerprint: fmt.Sprintf("seed%d", i), Step: "seed", Cmd: "true",
			Head: head, Branch: "main", Skill: "seed", Log: "/dev/null",
		}
		if mutate != nil {
			mutate(i, &r)
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
}

func ledgerLines(t *testing.T, l *Ledger) []string {
	t.Helper()
	lines, err := l.lines()
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestAppendWritesOneRecord(t *testing.T) {
	dir := newRepo(t)
	l := openLedger(t, dir)
	l.Append(Record{Fingerprint: "abc123", Step: "lint", Cmd: "true", Exit: 0, Secs: 1, Log: "/dev/null"})

	recs, err := l.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	r := recs[0]
	if r.Kind != "gate" || r.Step != "lint" || r.Cmd != "true" {
		t.Errorf("record = %+v", r)
	}
	if r.Epoch == 0 || r.TS == "" {
		t.Errorf("record was not stamped: ts=%q epoch=%d", r.TS, r.Epoch)
	}
	// The scratch root has to be ignored before the first write, or the very
	// file this records a proof over starts changing under the gate.
	if out := git(t, dir, "status", "--porcelain"); out != "" {
		t.Errorf("ledger write dirtied the tree:\n%s", out)
	}
}

// Nothing to classify it against, and a record that can only ever read `none` is
// noise in a file whose rotation then evicts real proofs.
func TestAppendSkipsRecordWithNoFingerprint(t *testing.T) {
	l := openLedger(t, newRepo(t))
	l.Append(Record{Step: "lint", Cmd: "true"})
	if _, err := os.Stat(l.Path); !os.IsNotExist(err) {
		t.Errorf("ledger exists: %v", err)
	}
}

// Losing a cache entry is nothing; failing a gate over its own bookkeeping is
// unacceptable — so an unwritable scratch root is silent, not an error.
func TestAppendIsSilentWhenTheLedgerCannotBeWritten(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	dir := newRepo(t)
	mk := filepath.Join(dir, ".mkit")
	if err := os.MkdirAll(mk, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(mk, 0o700) })

	l := openLedger(t, dir)
	l.Append(Record{Fingerprint: "abc123", Step: "lint", Cmd: "true"})
	if _, err := os.Stat(l.Path); !os.IsNotExist(err) {
		t.Errorf("ledger exists: %v", err)
	}
}

func TestClassify(t *testing.T) {
	dir := newRepo(t)
	head := git(t, dir, "rev-parse", "HEAD")
	l := openLedger(t, dir)
	now := time.Now()

	rec := func(cmd, fp string, exit int, head string, age time.Duration) {
		seed(t, l, 1, head, func(_ int, r *Record) {
			r.Cmd, r.Fingerprint, r.Exit = cmd, fp, exit
			r.Epoch = now.Add(-age).Unix()
		})
	}
	rec("fresh", "FP", 0, head, time.Minute)
	rec("stale", "FP", 0, head, 2*MaxAge)
	rec("drifted", "OTHER", 0, head, time.Minute)
	rec("failed-and-old", "FP", 1, head, 30*24*time.Hour)
	rec("unknown-head", "FP", 0, bogusHead, time.Minute)
	rec("future", "FP", 0, head, -time.Hour)

	cmds := []string{"fresh", "stale", "drifted", "failed-and-old", "unknown-head", "future", "never-run"}
	want := []Class{Fresh, Stale, Drifted, Failed, UnknownHead, Stale, None}

	got, err := l.Classify(cmds, "FP", now)
	if err != nil {
		t.Fatal(err)
	}
	for i, lk := range got {
		if lk.Class != want[i] {
			t.Errorf("%s: class = %s, want %s", cmds[i], lk.Class, want[i])
		}
	}
	if got[len(got)-1].Found {
		t.Error("a command with no record must not report Found")
	}
	if got[3].Exit != 1 {
		t.Errorf("failed record exit = %d, want 1", got[3].Exit)
	}
}

// A step *name* is not identity — these are different checks, and only the
// command string can be the key.
func TestClassifyKeysOnTheExactCommand(t *testing.T) {
	dir := newRepo(t)
	head := git(t, dir, "rev-parse", "HEAD")
	l := openLedger(t, dir)
	now := time.Now()
	seed(t, l, 1, head, func(_ int, r *Record) {
		r.Step, r.Cmd, r.Fingerprint, r.Epoch = "test", "bun run test", "FP", now.Unix()
	})

	got, err := l.Classify([]string{"bun run test", "bun run test --coverage"}, "FP", now)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Class != Fresh {
		t.Errorf("exact match = %s, want fresh", got[0].Class)
	}
	if got[1].Class != None {
		t.Errorf("different command = %s, want none", got[1].Class)
	}
}

func TestClassifyNewestRecordWins(t *testing.T) {
	dir := newRepo(t)
	head := git(t, dir, "rev-parse", "HEAD")
	l := openLedger(t, dir)
	now := time.Now()
	seed(t, l, 1, head, func(_ int, r *Record) {
		r.Cmd, r.Fingerprint, r.Exit, r.Epoch = "lint", "FP", 1, now.Add(-time.Hour).Unix()
	})
	seed(t, l, 1, head, func(_ int, r *Record) {
		r.Cmd, r.Fingerprint, r.Exit, r.Epoch = "lint", "FP", 0, now.Unix()
	})

	got, err := l.Classify([]string{"lint"}, "FP", now)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Class != Fresh {
		t.Errorf("class = %s, want fresh — the newest record is the answer", got[0].Class)
	}
}

func TestClassifyRejectsAMalformedLedger(t *testing.T) {
	l := openLedger(t, newRepo(t))
	seed(t, l, 1, "", nil)
	if err := os.WriteFile(l.Path, []byte("not json at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Classify([]string{"true"}, "FP", time.Now()); err == nil {
		t.Error("expected an error rather than a confident wrong class")
	}
}

// An upper bound alone would also pass if trim emptied the file, which is the
// failure worth catching: 450 seeded + 1 appended trims to exactly 200, newest
// kept.
func TestRotationKeepsExactlyTheNewestAndTheRecordJustWritten(t *testing.T) {
	dir := newRepo(t)
	l := openLedger(t, dir)
	head := git(t, dir, "rev-parse", "HEAD")
	seed(t, l, 450, head, nil)
	l.Append(Record{Fingerprint: "abc123", Step: "lint", Cmd: "true", Head: head})

	recs, err := l.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != LedgerKeep {
		t.Fatalf("got %d records, want %d", len(recs), LedgerKeep)
	}
	if last := recs[len(recs)-1]; last.Step != "lint" || last.Fingerprint != "abc123" {
		t.Errorf("the record just written did not survive: %+v", last)
	}
}

func TestRotationLeavesTheLedgerAloneWhenALineIsMalformed(t *testing.T) {
	dir := newRepo(t)
	l := openLedger(t, dir)
	seed(t, l, 450, git(t, dir, "rev-parse", "HEAD"), nil)
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("not json at all\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	l.Append(Record{Fingerprint: "abc123", Step: "lint", Cmd: "true"})
	if n := len(ledgerLines(t, l)); n != 452 {
		t.Errorf("got %d lines, want 452 — a trim that cannot read the file does not trim", n)
	}
}

func TestRotationDefersToALockALiveTrimCouldHold(t *testing.T) {
	dir := newRepo(t)
	l := openLedger(t, dir)
	seed(t, l, 450, git(t, dir, "rev-parse", "HEAD"), nil)
	if err := os.Mkdir(l.Path+".lock", 0o755); err != nil {
		t.Fatal(err)
	}

	l.Append(Record{Fingerprint: "abc123", Step: "lint", Cmd: "true"})
	if n := len(ledgerLines(t, l)); n != 451 {
		t.Errorf("got %d lines, want 451 — the append still happens, the trim does not", n)
	}
	if _, err := os.Stat(l.Path + ".lock"); err != nil {
		t.Errorf("the lock was taken from a trim that could still be running: %v", err)
	}
}

// A trim killed mid-rewrite leaves the directory behind, and rotation would then
// be off forever, silently.
func TestRotationBreaksALockLeftBehindByAKilledTrim(t *testing.T) {
	dir := newRepo(t)
	l := openLedger(t, dir)
	seed(t, l, 450, git(t, dir, "rev-parse", "HEAD"), nil)
	lock := l.Path + ".lock"
	if err := os.Mkdir(lock, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}

	l.Append(Record{Fingerprint: "abc123", Step: "lint", Cmd: "true"})
	if n := len(ledgerLines(t, l)); n != LedgerKeep {
		t.Errorf("got %d lines, want %d", n, LedgerKeep)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Errorf("the stale lock survived: %v", err)
	}
}

// They always classify unknown-head anyway, so they are the cheapest to lose.
func TestRotationDropsDeadHeadsFirst(t *testing.T) {
	dir := newRepo(t)
	l := openLedger(t, dir)
	head := git(t, dir, "rev-parse", "HEAD")
	seed(t, l, 450, bogusHead, nil)
	seed(t, l, 2, head, nil)

	l.Append(Record{Fingerprint: "abc123", Step: "lint", Cmd: "true", Head: head})

	recs, err := l.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3 (two live seeds plus the new one)", len(recs))
	}
	for _, r := range recs {
		if r.Head == bogusHead {
			t.Errorf("a record with an unresolvable head survived: %+v", r)
		}
	}
}

func TestSkillFromRunDir(t *testing.T) {
	for in, want := range map[string]string{
		"/x/.mkit/review-20260819T111347Z-RPfCbj": "review",
		"/x/.mkit/commit-20260819T111347Z-aaaaaa": "commit",
		"/x/own-run": "own",
		"/x/plain":   "plain",
	} {
		if got := SkillFromRunDir(in); got != want {
			t.Errorf("SkillFromRunDir(%q) = %q, want %q", in, got, want)
		}
	}
}
