package scratch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// Every file in the binary that creates, writes, renames or removes something on
// disk, why it is allowed to, and how many such calls it is allowed to make.
//
// This is the Go half of what `tests/bats/payload.bats` asserted over the shell:
// three write locations chosen by lifetime, and nowhere else. Neither rule has a
// behavioral seam — the boundaries that would enforce them cannot be created
// inside a test — so both are asserted by shape instead, against a list a human
// reviewed.
//
// The count is what makes that a guarantee rather than a gesture. Keyed by file
// alone, the check only caught a write appearing in a *new* file, and a new
// write site added to a file already on the list — `scratch.go`, which already
// writes both the scratch root and the `info/exclude` exception — passed
// silently. Adding one now changes the count and fails here until a human
// updates it, which is the entire mechanism.
var reviewed = map[string]struct {
	sites  int
	reason string
}{
	"internal/core/scratch/scratch.go":       {5, "the scratch root and the one named exception: MkdirAll+OpenFile for the common dir's info/exclude, MkdirAll+MkdirTemp for a run directory, RemoveAll for prune"},
	"internal/core/scratch/userdir.go":       {4, "~/.mkit (MKIT_HOME): MkdirAll, then a CreateTemp probe and the two Removes that make it net-zero"},
	"internal/core/gate/ledger.go":           {9, "<toplevel>/.mkit/gate.jsonl: MkdirAll+OpenFile to append, CreateTemp+Remove+Rename to rotate, Mkdir/RemoveAll/Mkdir for the trim lock"},
	"internal/core/gate/run.go":              {1, "one gate-<step>.log per step, inside the run directory it was handed"},
	"internal/core/findings/record.go":       {1, "a review run's artefacts, inside the run directory it was handed"},
	"internal/core/repoconfig/repoconfig.go": {2, "<toplevel>/.mkit/config.toml — the one committed file in the scratch root"},
	"internal/core/doctor/doctor.go":         {4, "probes writability by creating and removing a temp file in each declared location"},
	"internal/core/storage/apply.go":         {3, "deletes only what Scan named, home-containment guarded"},
	"internal/core/worklog/worklog.go":       {2, "<toplevel>/.mkit/work/<branch>.jsonl — MkdirAll+OpenFile to append, inside the scratch root"},
	"internal/core/worklog/rotate.go":        {8, "rotates that same file: Mkdir/RemoveAll/Mkdir/RemoveAll for the lock, CreateTemp+Remove+Chmod+Rename to replace it"},
}

// The calls that put something on disk, or take it off.
var writeCalls = map[string]bool{
	"Create": true, "CreateTemp": true, "WriteFile": true, "OpenFile": true,
	"Mkdir": true, "MkdirAll": true, "MkdirTemp": true,
	"Rename": true, "Remove": true, "RemoveAll": true, "Symlink": true,
	"Chmod": true, "Chown": true, "Chtimes": true, "Truncate": true, "Link": true,
}

func TestWriteSitesAreOnTheReviewedAllowlist(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	found := map[string]int{}

	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "os" || !writeCalls[sel.Sel.Name] {
					return true
				}
				found[rel]++
				if _, ok := reviewed[rel]; !ok {
					t.Errorf("%s:%d: os.%s writes to disk from a file that is not on the reviewed allowlist "+
						"in internal/core/scratch/writes_test.go — three write locations, chosen by lifetime, and nowhere else",
						rel, fset.Position(call.Pos()).Line, sel.Sel.Name)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// The counts. A file on the list that grew a write site is the case keying by
	// name alone could not see.
	for rel, want := range reviewed {
		if got := found[rel]; got != want.sites {
			t.Errorf("%s has %d os.* write calls, the allowlist says %d (%s) — "+
				"a write site was added or removed; re-read the file and update "+
				"internal/core/scratch/writes_test.go deliberately",
				rel, got, want.sites, want.reason)
		}
	}
}
