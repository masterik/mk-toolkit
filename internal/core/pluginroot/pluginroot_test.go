package pluginroot

import (
	"os"
	"path/filepath"
	"testing"
)

// payload builds a minimally valid payload: the manifest and a skills/ directory.
// Nothing else is load-bearing — the payload has shipped no executable code since
// M5, so anything this helper does not create must not be required to find it.
func payload(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills", "commit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "plugin.json"),
		[]byte(`{"name":"mkit"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The regression this file exists for. M5 deleted `scripts/lib/common.sh`, which
// isPayload still listed as a required marker, so every correct checkout was
// rejected: `mkit facts` printed `plugin=none`, `mkit doctor` reported the skills
// unavailable, and the search fell through to whatever stale installed copy still
// carried a script — inverting the work-tree-wins rule. The package had no test
// file at all, which is why it shipped.
func TestPayloadWithNoScriptsIsFound(t *testing.T) {
	dir := payload(t)
	if _, err := os.Stat(filepath.Join(dir, "scripts")); !os.IsNotExist(err) {
		t.Fatal("fixture must carry no scripts/ — that is the whole point")
	}
	if !isPayload(dir) {
		t.Fatal("a payload with a manifest and skills/ was rejected; " +
			"isPayload requires something the payload no longer ships")
	}
}

// Both markers are required, so neither alone is enough. Enumerated rather than
// asserted on the manifest only: a test pinning just the case that prompted the
// fix cannot catch what was left out.
func TestIsPayloadRequiresBothMarkers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		remove string
	}{
		{"no manifest", filepath.Join(".claude-plugin", "plugin.json")},
		{"no skills dir", "skills"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := payload(t)
			if err := os.RemoveAll(filepath.Join(dir, tc.remove)); err != nil {
				t.Fatal(err)
			}
			if isPayload(dir) {
				t.Errorf("accepted a directory with %s missing", tc.remove)
			}
		})
	}
}

// skills/ must be a directory. A regular file of that name is not a payload, and
// os.Stat alone would not have told them apart.
func TestSkillsMustBeADirectory(t *testing.T) {
	dir := payload(t)
	if err := os.RemoveAll(filepath.Join(dir, "skills")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isPayload(dir) {
		t.Error("accepted a regular file named skills")
	}
}

func TestIsPayloadRejectsEmptyAndMissing(t *testing.T) {
	if isPayload("") {
		t.Error("accepted the empty path")
	}
	if isPayload(filepath.Join(t.TempDir(), "nope")) {
		t.Error("accepted a path that does not exist")
	}
}

// An explicit override is trusted as given, but it still has to be a payload —
// and the M5 break was visible precisely here, where the user said where to look
// and was told `none` anyway.
func TestExplicitOverrideFindsAScriptlessPayload(t *testing.T) {
	dir := payload(t)
	t.Setenv("CLAUDE_PLUGIN_ROOT", dir)
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	root, err := Find("")
	if err != nil {
		t.Fatalf("Find with CLAUDE_PLUGIN_ROOT set: %v", err)
	}
	if root.Dir != dir {
		t.Errorf("Dir = %q, want %q", root.Dir, dir)
	}
}
