package branchscan

import (
	"strings"
	"testing"
)

// ProtectedSet is the one producer of the branches cleanup must never delete.
// Its hardest rule has no behavioral seam anywhere else: the default branch is in
// the set whether or not the pinned keep list names it, because a keep list that
// omits it is a mistake, not an instruction — and being wrong here deletes a
// branch.
func TestProtectedSet(t *testing.T) {
	for _, tc := range []struct {
		name    string
		def     string
		develop string
		keep    []string
		want    string
	}{
		{"nothing pinned", "main", "none", nil, "main"},
		{"develop joins the default", "main", "develop", nil, "main,develop"},
		{"a pinned name is added, not substituted", "main", "none", []string{"staging"}, "main,staging"},
		{"the default survives a keep list that omits it", "main", "none", []string{"staging", "release"},
			"main,staging,release"},
		// The whole point, stated twice: a list naming only some other branch
		// must not be readable as "delete main".
		{"the default survives a keep list naming another branch only", "main", "develop", []string{"staging"},
			"main,develop,staging"},
		{"the default listed again is not repeated", "main", "none", []string{"main", "staging"}, "main,staging"},
		{"develop listed again is not repeated", "main", "develop", []string{"develop"}, "main,develop"},
		{"a duplicate pin is not repeated", "main", "none", []string{"staging", "staging"}, "main,staging"},
		{"blank entries pin nothing", "main", "none", []string{"", "  ", "staging"}, "main,staging"},
		{"surrounding space is not part of a branch name", "main", "none", []string{" staging "}, "main,staging"},
		// `none` is Develop's sentinel for "this repo has no develop-like
		// branch", not a branch name to protect.
		{"the develop sentinel is not a branch", "main", "none", nil, "main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(ProtectedSet(tc.def, tc.develop, tc.keep), ",")
			if got != tc.want {
				t.Errorf("ProtectedSet(%q, %q, %v) = %q, want %q", tc.def, tc.develop, tc.keep, got, tc.want)
			}
		})
	}
}

// The default branch is first, always — `cleanup` prints the list as "keeping
// <protected list>", and a stable order is what makes two runs comparable.
func TestProtectedSetPutsTheDefaultFirst(t *testing.T) {
	got := ProtectedSet("main", "develop", []string{"staging", "main"})
	if got[0] != "main" {
		t.Errorf("first = %q, want main: %v", got[0], got)
	}
}

// ProtectedSet never returns nil: `protected=` is joined into output and encoded
// as JSON, where a nil list would print as `null` rather than an empty one.
func TestProtectedSetIsNeverNil(t *testing.T) {
	if got := ProtectedSet("", "none", nil); got == nil {
		t.Error("got nil")
	}
}
