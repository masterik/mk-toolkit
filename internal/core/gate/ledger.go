package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/scratch"
)

// The ledger's two bounds.
const (
	// LedgerKeep is how many records survive a rotation. Rotation triggers at
	// twice this, so the file oscillates between 200 and 400 rather than being
	// rewritten on every append.
	LedgerKeep = 200

	// MaxAge is how long a *pass* stays trustworthy. It is a judgement about
	// time rather than about content: the fingerprint proves the tracked content
	// is identical, never that the environment is — dependency installs, tool
	// versions and env vars are all invisible to it.
	MaxAge = 60 * time.Minute
)

// Record is one finished gate step. The field set is what `gate-run.sh` wrote,
// unchanged, so a ledger written by either implementation stays readable by both.
type Record struct {
	Kind        string `json:"kind"`
	TS          string `json:"ts"`
	Epoch       int64  `json:"epoch"`
	Fingerprint string `json:"fingerprint"`
	Step        string `json:"step"`
	Cmd         string `json:"cmd"`
	Exit        int    `json:"exit"`
	Secs        int    `json:"secs"`
	Head        string `json:"head"`
	Branch      string `json:"branch"`
	Skill       string `json:"skill"`
	Log         string `json:"log"`
}

// Class is what the ledger has to say about one proposed command.
type Class string

// The six classes, in the order Classify evaluates them.
const (
	// UnknownHead — the proof was recorded against a commit that no longer
	// resolves here (a rebased or pruned branch).
	UnknownHead Class = "unknown-head"
	// Drifted — a proof exists, over different content.
	Drifted Class = "drifted"
	// Failed — this tree was red on exactly this content. Checked *before* the
	// age bound: it no longer means "skip", it means "this is known broken",
	// which is worth saying however old it is.
	Failed Class = "failed"
	// Stale — a pass over this exact content, past the age bound.
	Stale Class = "stale"
	// Fresh — a pass over this exact content, inside the age bound.
	Fresh Class = "fresh"
	// None — no record for this command string at all.
	None Class = "none"
)

// Lookup is the ledger's verdict on one command. Nothing here skips anything:
// annotating a proposal is all the ledger ever does, and the skill that acts on
// it owns the trade-off — and must label a skipped step `cached`.
type Lookup struct {
	Cmd   string
	Class Class
	// Found is false when no record matched; Exit and Age are meaningless then.
	Found bool
	Exit  int
	Age   time.Duration
}

// Ledger is <toplevel>/.mkit/gate.jsonl: append-only, one record per finished
// step, rotated back to the newest LedgerKeep once it passes twice that.
type Ledger struct {
	Path string
	repo *gitrepo.Repo
}

// OpenLedger names the ledger of repo. It touches nothing on disk.
func OpenLedger(repo *gitrepo.Repo) *Ledger {
	return &Ledger{Path: filepath.Join(scratch.Dir(repo), "gate.jsonl"), repo: repo}
}

// Append records one finished step — never one per chain. A chain that stops at
// `test` leaves two records, and a later skill gets per-step answers
// (`lint=fresh test=failed build=none`) instead of an all-or-nothing verdict.
//
// Strictly a side effect: it may not change an exit code, an output, or where a
// chain stops, and a failure is silent. Losing a cache entry is nothing; failing
// a gate over its own bookkeeping is unacceptable.
//
// A record with no fingerprint is not written: there would be nothing to classify
// it against, and a record that can only ever read `none` is noise in a file
// whose rotation then evicts real proofs.
func (l *Ledger) Append(rec Record) {
	if l == nil || l.Path == "" || rec.Fingerprint == "" {
		return
	}
	rec.Kind = "gate"
	if rec.Epoch == 0 {
		now := time.Now().UTC()
		// `epoch` beside the ISO `ts`: age arithmetic must not go through a
		// date parse, which is exactly the portability trap this layer avoids.
		rec.TS = now.Format("2006-01-02T15:04:05Z")
		rec.Epoch = now.Unix()
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	// Before the mkdir, in case a gate run — not `run open` — is what creates
	// `.mkit/` in this repo. An unignored scratch root feeds the fingerprint a
	// directory that changes while the gate runs, which is a run invalidating
	// its own cache entry. Best effort, like everything on this path.
	scratch.EnsureIgnored(l.repo)
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, werr := f.Write(append(line, '\n'))
	cerr := f.Close()
	if werr != nil || cerr != nil {
		return
	}
	l.trim()
}

// Records decodes the whole ledger, oldest first. A missing file is not an error
// — it is an empty ledger.
func (l *Ledger) Records() ([]Record, error) {
	lines, err := l.lines()
	if err != nil {
		return nil, err
	}
	recs := make([]Record, 0, len(lines))
	for _, line := range lines {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("ledger %s: %w", l.Path, err)
		}
		recs = append(recs, r)
	}
	return recs, nil
}

// Classify answers, for each command, what the ledger knows about it.
//
// The lookup key is the **exact command string**, newest record wins. A step
// *name* is not identity — `bun run test` and `bun run test --coverage` are
// different checks, so only the command can be the key. (Which is also why
// `gate run`'s two call forms must normalize to the same string: the flagship
// `review` → `finish` hit is keyed on it.)
//
// Head resolution is batched — one `cat-file` for every distinct head — because
// paying a fork per command would make the cheap part of a gate its own cost.
func (l *Ledger) Classify(cmds []string, fingerprint string, now time.Time) ([]Lookup, error) {
	recs, err := l.Records()
	if err != nil {
		return nil, err
	}

	// Newest record per command string.
	newest := map[string]*Record{}
	for i := range recs {
		r := &recs[i]
		if r.Kind != "" && r.Kind != "gate" {
			continue
		}
		newest[r.Cmd] = r
	}

	heads := map[string]bool{}
	for _, c := range cmds {
		if r := newest[c]; r != nil && r.Head != "" {
			heads[r.Head] = true
		}
	}
	alive := l.aliveCommits(heads)

	out := make([]Lookup, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, classify(newest[c], c, fingerprint, now, alive))
	}
	return out, nil
}

func classify(r *Record, cmd, fingerprint string, now time.Time, alive map[string]bool) Lookup {
	lk := Lookup{Cmd: cmd, Class: None}
	// A record carrying no fingerprint cannot be compared against one.
	if r == nil || r.Fingerprint == "" {
		return lk
	}
	lk.Found = true
	lk.Exit = r.Exit
	lk.Age = now.Sub(time.Unix(r.Epoch, 0))

	switch {
	case r.Head != "" && !alive[r.Head]:
		lk.Class = UnknownHead
	case r.Fingerprint != fingerprint:
		lk.Class = Drifted
	case r.Exit != 0:
		lk.Class = Failed
	case lk.Age < 0 || lk.Age > MaxAge:
		// A record stamped in the future is not evidence of anything; it reads
		// as stale rather than as an unusually fresh pass.
		lk.Class = Stale
	default:
		lk.Class = Fresh
	}
	return lk
}

// aliveCommits reports which of heads still resolve to a commit here.
func (l *Ledger) aliveCommits(heads map[string]bool) map[string]bool {
	alive := map[string]bool{}
	if len(heads) == 0 || l.repo == nil {
		return alive
	}
	in := make([]string, 0, len(heads))
	var stdin strings.Builder
	for h := range heads {
		in = append(in, h)
		stdin.WriteString(h + "^{commit}\n")
	}
	out, err := gitStdin(l.repo.Toplevel, stdin.String(), "cat-file", "--batch-check")
	if err != nil {
		return alive
	}
	// One output line per input line, in order: `<sha> commit <size>` for a
	// commit, `<input> missing` for anything that does not peel to one.
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	for i, line := range lines {
		if i >= len(in) {
			break
		}
		if f := strings.Fields(line); len(f) >= 2 && f[1] == "commit" {
			alive[in[i]] = true
		}
	}
	return alive
}

// trim keeps the newest LedgerKeep records, dropping records whose head no longer
// resolves first — those always classify unknown-head anyway, so they are the
// cheapest to lose.
//
// The lock excludes **trim against trim** and nothing else: Append takes no lock,
// so a record appended between the head scan and the rename below can be lost.
// That is a deliberate trade — one cache record, after which the lookup reads
// `none` and the step runs — because locking the append would cost it the
// never-blocks, never-fails property the whole side effect rests on.
func (l *Ledger) trim() {
	lines, err := l.lines()
	if err != nil || len(lines) <= LedgerKeep*2 {
		return
	}

	lock := l.Path + ".lock"
	if !acquireLock(lock) {
		return
	}
	defer func() { _ = os.RemoveAll(lock) }()

	// A trim that cannot read the file cleanly does not trim. One malformed line
	// used to make the shell's `jq` stop there, after which `paste` paired the
	// heads it had printed with the WRONG records and the rewrite kept a handful
	// of the oldest and dropped every valid record after them.
	recHeads := make([]string, len(lines))
	heads := map[string]bool{}
	for i, line := range lines {
		var r struct {
			Head string `json:"head"`
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return
		}
		recHeads[i] = r.Head
		if r.Head != "" {
			heads[r.Head] = true
		}
	}
	alive := l.aliveCommits(heads)

	kept := make([]string, 0, len(lines))
	for i, line := range lines {
		// An empty head is "there was no HEAD to record" — an unborn repo, where
		// every record carries one — not "a head that no longer exists". Treating
		// the two alike evicted every proof an unborn repo had ever written,
		// including the record appended a moment earlier, as soon as the ledger
		// rotated. Classification already handles an unresolvable head; rotation
		// has no reason to be stricter.
		if recHeads[i] == "" || alive[recHeads[i]] {
			kept = append(kept, line)
		}
	}
	if len(kept) > LedgerKeep {
		kept = kept[len(kept)-LedgerKeep:]
	}

	tmp, err := os.CreateTemp(filepath.Dir(l.Path), "gate.jsonl.")
	if err != nil {
		return
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	for _, line := range kept {
		if _, err := tmp.WriteString(line + "\n"); err != nil {
			_ = tmp.Close()
			return
		}
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmp.Name(), l.Path)
}

// acquireLock takes the trim lock, breaking one nothing could still be holding.
// A trim killed mid-rewrite leaves the directory behind, and rotation would then
// be off forever, silently — the same 60-minute liveness heuristic `run prune`
// uses for a run directory.
func acquireLock(lock string) bool {
	if err := os.Mkdir(lock, 0o755); err == nil {
		return true
	}
	fi, err := os.Stat(lock)
	if err != nil || time.Since(fi.ModTime()) <= MaxAge {
		return false
	}
	if err := os.RemoveAll(lock); err != nil {
		return false
	}
	return os.Mkdir(lock, 0o755) == nil
}

// lines returns the ledger's raw lines. A missing file is an empty ledger, not an
// error; trim rewrites these verbatim, so nothing here re-serializes a record.
func (l *Ledger) lines() ([]string, error) {
	b, err := os.ReadFile(l.Path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b = []byte(strings.TrimSuffix(string(b), "\n"))
	if len(b) == 0 {
		return nil, nil
	}
	return strings.Split(string(b), "\n"), nil
}

// SkillFromRunDir derives `review` from `review-20260819T111347Z-RPfCbj`.
// Diagnostic only: no classification reads it. It is recorded so that "is
// anything ever *consuming* this ledger?" is answerable from the file alone.
func SkillFromRunDir(runDir string) string {
	base := filepath.Base(runDir)
	if i := strings.IndexByte(base, '-'); i >= 0 {
		return base[:i]
	}
	return base
}
