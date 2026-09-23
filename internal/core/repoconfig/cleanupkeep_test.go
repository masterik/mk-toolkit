package repoconfig

import (
	"strings"
	"testing"
)

// The file is committed and hand-edited, so what Write renders has to be what
// Load reads back — the round trip is the whole contract for a key nobody
// marshals.
func TestCleanupKeepRoundTrips(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, &Config{Cleanup: Cleanup{Keep: []string{"staging", "release/2024"}}}); err != nil {
		t.Fatal(err)
	}
	got, present, err := Load(dir)
	if err != nil || !present {
		t.Fatalf("Load: %v present=%v", err, present)
	}
	if len(got.Problems) != 0 {
		t.Errorf("a file mkit wrote is a file mkit reads cleanly: %+v", got.Problems)
	}
	if strings.Join(got.Cleanup.Keep, ",") != "staging,release/2024" {
		t.Errorf("keep = %v", got.Cleanup.Keep)
	}
}

// The rendered file carries the rule the key is easiest to misread as: a keep
// list is added to what cleanup protects, it does not replace it.
func TestRenderedKeepSaysTheDefaultIsKeptRegardless(t *testing.T) {
	out := render(&Config{Version: Version, Cleanup: Cleanup{Keep: []string{"staging"}}})
	for _, want := range []string{"[cleanup]", "keep = ", "staging", "default branch is kept whether or not"} {
		if !strings.Contains(out, want) {
			t.Errorf("the rendered config never says %q:\n%s", want, out)
		}
	}
}

// A keep list is the only thing pinned in a repo that needs nothing else. If it
// did not count, `mkit init --keep staging` would report `nothing-to-pin` and
// write no file at all.
func TestAKeepListAloneIsNotAZeroConfig(t *testing.T) {
	if (&Config{Cleanup: Cleanup{Keep: []string{"staging"}}}).IsZero() {
		t.Error("a config pinning only a keep list reads as empty")
	}
	if !(&Config{}).IsZero() {
		t.Error("the zero config does not read as empty")
	}
}

// `[cleanup] keep` has no enumeration, so it fits #19's validation machinery by
// having nothing in it: any string is a legal branch name to pin. A name with no
// local branch is reported by `mkit branch scan` as `keep_unknown=`, never
// rejected on read — config is an input, and the list travels with the repo.
func TestCleanupKeepHasNoEnumerationAndIsNeverRejected(t *testing.T) {
	if got := Allowed("cleanup.keep"); got != nil {
		t.Errorf("Allowed(\"cleanup.keep\") = %v, want nil", got)
	}
	dir := t.TempDir()
	writeFile(t, dir, RelPath, "version = 1\n\n[cleanup]\nkeep = [\"no-such-branch\", \"staging\"]\n")
	got, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Problems) != 0 {
		t.Errorf("a pinned name is never a problem on read: %+v", got.Problems)
	}
	if strings.Join(got.Cleanup.Keep, ",") != "no-such-branch,staging" {
		t.Errorf("keep = %v", got.Cleanup.Keep)
	}
}

// A misspelled table is still caught by the strict decode #19 landed — the new
// key routes through that machinery rather than around it.
func TestAMisspelledCleanupTableIsAnUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, RelPath, "version = 1\n\n[cleanupp]\nkeep = [\"staging\"]\n")
	got, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Problem("cleanupp") == nil {
		t.Errorf("problems = %+v", got.Problems)
	}
	if len(got.Cleanup.Keep) != 0 {
		t.Errorf("keep = %v, want nothing pinned", got.Cleanup.Keep)
	}
}
