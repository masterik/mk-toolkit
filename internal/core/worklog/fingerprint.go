package worklog

import (
	"github.com/masterik/mk-toolkit/internal/core/pluginroot"
)

// Fingerprint is the content hash a record is keyed on — the staging- and
// commit-invariant `path → blob` mapping a quality-gate command would read.
//
// Delegated to the payload's `mkit_tree_fingerprint`, never reimplemented here.
// That hash is the one thing making a `pr` → `finish` cache hit possible at all,
// and its properties (invariant under staging and committing, batched hashing,
// `--no-renames` rather than porcelain) are the kind that a second implementation
// reproduces almost correctly. M5 ports it for real; this is the first call site
// that will switch over, and until then one producer is worth a subprocess.
//
// An unavailable fingerprint is "" and a cause, never an error the caller has to
// handle: a record without one is still a gist a later step can read — it just
// cannot tell whether the gist still describes the tree in front of it.
func Fingerprint(toplevel string) (fp string, cause string) {
	root, err := pluginroot.Find(toplevel)
	if err != nil {
		return "", "payload not found, so no fingerprint — " + pluginroot.Remedy()
	}
	out, err := root.CommonFuncIn(toplevel, "mkit_tree_fingerprint")
	if err != nil || out == "" {
		// Deliberately unspecific. `mkit_tree_fingerprint` exits 1 for an absent
		// hash tool, an unresolvable HEAD *and* a temp directory it cannot write,
		// and it prints nothing either way — so naming one of them here would be a
		// guess reported as a diagnosis.
		return "", "no fingerprint: mkit_tree_fingerprint did not produce one"
	}
	return out, ""
}
