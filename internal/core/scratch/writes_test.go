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
// disk, and why it is allowed to.
//
// This is the Go half of what `tests/bats/payload.bats` asserted over the shell:
// three write locations chosen by lifetime, and nowhere else. Neither rule has a
// behavioral seam — the boundaries that would enforce them cannot be created
// inside a test — so both are asserted by shape instead, against a list a human
// reviewed. A new write site fails this test until someone adds it here, which
// is the entire mechanism.
var reviewed = map[string]string{
	"internal/core/scratch/scratch.go":       "the scratch root itself, plus the one named exception: the common dir's info/exclude",
	"internal/core/scratch/userdir.go":       "~/.mkit (MKIT_HOME) — the writability probe, net-zero by construction",
	"internal/core/gate/ledger.go":           "<toplevel>/.mkit/gate.jsonl and its trim lock — inside the scratch root",
	"internal/core/gate/run.go":              "one gate-<step>.log per step, inside the run directory it was handed",
	"internal/core/findings/record.go":       "a review run's artefacts, inside the run directory it was handed",
	"internal/core/repoconfig/repoconfig.go": "<toplevel>/.mkit/config.toml — the one committed file in the scratch root",
	"internal/core/doctor/doctor.go":         "probes writability by creating and removing a temp file in each declared location",
	"internal/core/storage/apply.go":         "deletes only what Scan named, home-containment guarded",
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
}
