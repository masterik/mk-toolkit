package worklog

import (
	"github.com/masterik/mk-toolkit/internal/core/gate"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/scratch"
)

// Fingerprint is the content hash a record is keyed on — the staging- and
// commit-invariant `path → blob` mapping a quality-gate command would read.
//
// `gate.Fingerprint`, never reimplemented here. That hash is the one thing making
// a `pr` → `finish` cache hit possible at all, and its properties (invariant under
// staging and committing, hashed through git so a clean filter cannot desync it,
// `--no-renames` rather than porcelain) are the kind a second implementation
// reproduces almost correctly. This used to reach it through the payload's
// `mkit_tree_fingerprint` over `lib/common.sh`, which was always documented as
// temporary: M5 ported it, the shell layer is gone, and this is the call site
// that switched over.
//
// An unavailable fingerprint is "" and a cause, never an error the caller has to
// handle: a record without one is still a gist a later step can read — it just
// cannot tell whether the gist still describes the tree in front of it.
func Fingerprint(toplevel string) (fp string, cause string) {
	repo, err := gitrepo.Open(toplevel)
	if err != nil {
		return "", "not a work tree, so no fingerprint"
	}
	out, err := gate.Fingerprint(repo)
	if err != nil || out == "" {
		// Unspecific on purpose: an unresolvable HEAD and a path git refused to
		// hash both land here, and naming one would be a guess reported as a
		// diagnosis.
		return "", "no fingerprint could be computed for this tree"
	}
	return out, ""
}

// ensureIgnored puts mkit's scratch rule in place before this package creates
// anything under `.mkit/`.
//
// The reasons are the ones `scratch.EnsureIgnored` exists for: an unignored
// `.mkit/` makes `git worktree remove` refuse, puts the worklog in reach of
// `git add -A`, and feeds the gate fingerprint a directory that changes while the
// gate runs. The worklog needs no rule of its own — `.mkit/*` already covers it —
// but `mkit work append` can be the **first** thing to write under `.mkit/` in a
// repo where no skill has ever opened a run directory.
//
// Delegated, never reimplemented: which file the rule lands in, which probe
// answers "is it ignored", and why that probe is two concrete paths rather than
// the directory are all decided in `scratch`, and a second implementation gets
// one of them subtly wrong.
//
// Best effort, and deliberately returns nothing. The write lands in the main
// checkout's `.git/info/exclude`, which a worktree-isolated session cannot reach,
// so failure is the normal case on some machines — and contract rule 4 says a
// recorded fact is an input, never a permission. An append that refused over an
// ignore rule would be the worklog gating a step. `mkit facts` already reports
// the state as `run_ignored=` for skills that need to know.
func ensureIgnored(toplevel string) {
	repo, err := gitrepo.Open(toplevel)
	if err != nil {
		return
	}
	_ = scratch.EnsureIgnored(repo)
}
