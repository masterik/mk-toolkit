package worklog

import "time"

// Schema is the version stamped into every record written. It exists so a later
// reader can tell a record it understands from one it does not; nothing branches on
// it today, and that is the point — a field added after the first records are in the
// wild cannot be relied on.
const Schema = 1

// Record is one finished step. One JSON object per line.
type Record struct {
	// Step is which of the seven steps (or cleanup) produced this record.
	Step string `json:"step"`
	// TS is when it finished, RFC3339 UTC. Set by the binary, never by the caller:
	// a caller-supplied timestamp is a caller-supplied ordering.
	TS string `json:"ts"`
	// Fingerprint is the content the step ran over — the same staging- and
	// commit-invariant hash the gate ledger keys on. "" when unavailable, and
	// never omitted: a reader must be able to tell "no fingerprint" from "field
	// I do not know about".
	Fingerprint string `json:"fingerprint"`
	// Head is the commit HEAD resolved to, so rotation can drop dead records first.
	Head string `json:"head"`
	// Artifact points at whatever the step produced — an issue URL, a path, a run
	// directory, a commit range. Optional, and **best effort by nature**: the log
	// keeps 200 records while `run-open.sh --prune` keeps the newest five run
	// directories per skill, so a recorded run directory outlives its own contents.
	// A dangling pointer here is expected, not a lookup failure — the gist is what a
	// later step reads. Prefer a durable target (a sha, a range, a URL) where the
	// step has one.
	Artifact string `json:"artifact,omitempty"`
	// Gist is the one line a later step reads instead of re-deriving intent.
	// Required: a record without one is a record nothing can consume.
	Gist string `json:"gist"`
	// Assumptions is contract rule 3 made machine-readable — what the step derived
	// because it could not find it. Empty array, never null.
	Assumptions []string `json:"assumptions"`
	// Schema is Schema.
	Schema int `json:"schema"`
}

// Steps is the closed vocabulary. The seven workflow steps plus cleanup, which is
// repo-wide gardening outside the line but still worth a record.
var Steps = []string{"brainstorm", "spec", "implement", "commit", "review", "pr", "finish", "cleanup"}

// ValidStep reports whether s is in Steps. An unknown step is rejected rather than
// written: a typo'd step name is a record every later reader silently skips.
func ValidStep(s string) bool {
	for _, v := range Steps {
		if v == s {
			return true
		}
	}
	return false
}

func now() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }
