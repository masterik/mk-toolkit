package worklog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seed writes n records straight to the file, bypassing Append, so a rotation test
// costs one write rather than n.
func seed(t *testing.T, log *Log, n int, head string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(log.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		line, err := json.Marshal(Record{
			Step: "review", TS: now(), Head: head,
			Gist: fmt.Sprintf("seeded %d", i), Assumptions: []string{}, Schema: Schema,
		})
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(log.Path(), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, log *Log) int {
	t.Helper()
	recs, err := log.Show(Query{})
	if err != nil {
		t.Fatal(err)
	}
	return len(recs)
}

// Amortized on purpose: the rewrite happens once every Keep appends, not on each one.
func TestRotationOnlyPastTwiceKeep(t *testing.T) {
	repo := newRepo(t)
	head := git(t, repo.Toplevel, "rev-parse", "HEAD")

	below := Open(repo, "below")
	seed(t, below, Keep*2-1, head)
	if err := below.Append(Record{Step: "commit", Head: head, Gist: "the one that reaches the boundary"}); err != nil {
		t.Fatal(err)
	}
	if got := count(t, below); got != Keep*2 {
		t.Errorf("rotated at the boundary: %d records, want %d", got, Keep*2)
	}

	above := Open(repo, "above")
	seed(t, above, Keep*2, head)
	if err := above.Append(Record{Step: "commit", Head: head, Gist: "the one that crosses it"}); err != nil {
		t.Fatal(err)
	}
	if got := count(t, above); got != Keep {
		t.Errorf("past the boundary: %d records, want %d", got, Keep)
	}
	// The newest survive, and the record that triggered the rotation is one of them.
	recs, _ := above.Show(Query{})
	if recs[len(recs)-1].Gist != "the one that crosses it" {
		t.Errorf("rotation dropped the newest record: %q", recs[len(recs)-1].Gist)
	}
}

// A record whose head no longer resolves describes a tree nothing can be compared
// against, so it is the cheapest one to lose — and it goes before any live record.
func TestRotationDropsDeadHeadsFirst(t *testing.T) {
	repo := newRepo(t)
	head := git(t, repo.Toplevel, "rev-parse", "HEAD")
	dead := strings.Repeat("0", 39) + "1"

	log := Open(repo, "mixed")
	seed(t, log, Keep*2, dead)
	// Ten live records, all older than nothing and newer than the dead ones.
	for i := 0; i < 10; i++ {
		if err := log.Append(Record{Step: "review", Head: head, Gist: fmt.Sprintf("live %d", i)}); err != nil {
			t.Fatal(err)
		}
	}

	recs, _ := log.Show(Query{})
	if len(recs) != 10 {
		t.Fatalf("got %d records, want only the 10 live ones", len(recs))
	}
	for _, r := range recs {
		if r.Head == dead {
			t.Fatalf("a dead-head record survived while live ones existed")
		}
	}
}

// The ledger's hardest-won rule: one unparseable line and the safe move is to leave
// all of them. A rewrite that guesses which records are real is how a log loses the
// valid ones.
func TestRotationRefusesOnAMalformedLine(t *testing.T) {
	repo := newRepo(t)
	head := git(t, repo.Toplevel, "rev-parse", "HEAD")
	log := Open(repo, "torn")
	seed(t, log, Keep*2, head)

	f, err := os.OpenFile(log.Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("{half a rec\n")
	_ = f.Close()

	if err := log.Append(Record{Step: "commit", Head: head, Gist: "after the tear"}); err != nil {
		t.Fatal(err)
	}
	// Show skips the bad line; the file itself must still hold everything.
	b, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(strings.Split(strings.TrimRight(string(b), "\n"), "\n")); got != Keep*2+2 {
		t.Errorf("the file was rotated despite an unreadable line: %d lines, want %d", got, Keep*2+2)
	}
}

// A rotation killed mid-rewrite leaves the lock behind, and rotation would then be
// off forever, silently. A fresh lock is respected; a stale one is broken.
func TestRotationLockIsRespectedThenBroken(t *testing.T) {
	repo := newRepo(t)
	head := git(t, repo.Toplevel, "rev-parse", "HEAD")
	log := Open(repo, "locked")
	seed(t, log, Keep*2, head)
	if err := os.Mkdir(log.Path()+".lock", 0o755); err != nil {
		t.Fatal(err)
	}

	if err := log.Append(Record{Step: "commit", Head: head, Gist: "held"}); err != nil {
		t.Fatal(err)
	}
	if got := count(t, log); got != Keep*2+1 {
		t.Errorf("rotated through a held lock: %d records", got)
	}

	stale := time.Now().Add(-2 * lockStale)
	if err := os.Chtimes(log.Path()+".lock", stale, stale); err != nil {
		t.Fatal(err)
	}
	if err := log.Append(Record{Step: "commit", Head: head, Gist: "after the break"}); err != nil {
		t.Fatal(err)
	}
	if got := count(t, log); got != Keep {
		t.Errorf("a stale lock was not broken: %d records, want %d", got, Keep)
	}
}
