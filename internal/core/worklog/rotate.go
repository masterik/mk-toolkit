package worklog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Keep is how many records survive a rotation — the gate ledger's LEDGER_KEEP, and
// the same number on purpose: two append-only logs in the same directory that
// disagree about their own size are two things to reason about instead of one.
const Keep = 200

// lockStale is how long a lock directory must be untouched before it is broken —
// the same 60-minute liveness heuristic `run-open.sh --prune` uses.
const lockStale = 60 * time.Minute

// rotate keeps the newest Keep records, dropping records whose head no longer
// resolves first.
//
// Three properties carried over from the gate ledger's ledger_trim, each learned
// there rather than reasoned about here:
//
//   - **Amortized.** Rotation runs only past Keep*2, so the rewrite happens once
//     every Keep appends rather than on every one.
//   - **Dead heads go first.** A record whose head no longer resolves describes a
//     tree nothing can be compared against, so it is the cheapest record to lose.
//     Resolved in one batched call, not one per record.
//   - **A rotation that cannot read the file cleanly does not rotate.** One
//     unparseable line and the safe move is to leave all of them: a rewrite that
//     guesses which records are real is how a log loses the valid ones.
//
// The lock excludes rotation against rotation and nothing else — Append takes none,
// so a record written between the read and the rename can be lost. That is the same
// deliberate trade the ledger makes: one record, after which a reader simply sees
// one fewer, is worth less than the never-blocks property of the append.
func (l *Log) rotate() error {
	path := l.Path()
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) <= Keep*2 {
		return nil
	}

	lock := path + ".lock"
	if err := os.Mkdir(lock, 0o755); err != nil {
		// A rotation killed mid-rewrite leaves the directory behind, and rotation
		// would then be off forever, silently. Break a lock nothing could still
		// be holding.
		fi, statErr := os.Stat(lock)
		if statErr != nil || time.Since(fi.ModTime()) < lockStale {
			return nil
		}
		_ = os.RemoveAll(lock)
		if err := os.Mkdir(lock, 0o755); err != nil {
			return nil
		}
	}
	defer func() { _ = os.RemoveAll(lock) }()

	recs := make([]Record, 0, len(lines))
	raw := make([]string, 0, len(lines))
	heads := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec Record
		if json.Unmarshal([]byte(line), &rec) != nil {
			// Cannot read it cleanly — do not rotate.
			return nil
		}
		recs = append(recs, rec)
		raw = append(raw, line)
		if rec.Head != "" && !seen[rec.Head] {
			seen[rec.Head] = true
			heads = append(heads, rec.Head)
		}
	}

	alive := l.repo.AliveCommits(heads)
	kept := make([]string, 0, len(recs))
	for i, rec := range recs {
		// A record with no head at all is dropped with the dead ones: it cannot be
		// matched against any tree either. Two things write one — an unborn branch,
		// where nothing downstream has a commit to compare with, and an `--append
		// --branch <name>` naming a branch that does not resolve (a typo, or one
		// already deleted). Their gists are readable while they sit in the log, and
		// a step may well have used one — what they cannot do is be matched against
		// a tree, which is the only question this sweep is allowed to ask.
		if !alive[rec.Head] {
			continue
		}
		kept = append(kept, raw[i])
	}
	if len(kept) > Keep {
		kept = kept[len(kept)-Keep:]
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(strings.Join(kept, "\n") + "\n"); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
