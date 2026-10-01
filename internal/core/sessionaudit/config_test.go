package sessionaudit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// settings writes a project's .claude/settings.local.json under root.
func settings(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.local.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// One project's local settings covering a target must not mark the bucket
// covered while another project still hits it: "covered" tells the skill to
// propose nothing.
func TestCoverageNeedsEveryProjectTheBucketOccurredIn(t *testing.T) {
	tmp := t.TempDir()
	a, b := filepath.Join(tmp, "a"), filepath.Join(tmp, "b")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	settings(t, a, `{"sandbox":{"network":{"allowedDomains":["only-a.test"]}}}`)

	block := "<sandbox_violations>\ndeny network-outbound only-a.test:443 (x)\n</sandbox_violations>"
	home := t.TempDir()
	// projects are named after the transcript directory; cwd gives each its root.
	writeTranscriptCwd(t, home, "-p-a/s.jsonl", a, block)
	rep := scan(t, home)
	rep.AttachConfig(filepath.Join(tmp, "claude-home"))
	if len(rep.BlockTargets) != 1 || rep.BlockTargets[0].CoveredBy == nil {
		t.Fatalf("project a alone: %+v", rep.BlockTargets)
	}
	if got := rep.BlockTargets[0].CoveredBy.Entry; got != "only-a.test" {
		t.Errorf("entry = %q", got)
	}

	writeTranscriptCwd(t, home, "-p-b/s.jsonl", b, block)
	rep = scan(t, home)
	rep.AttachConfig(filepath.Join(tmp, "claude-home"))
	if len(rep.BlockTargets) != 1 || rep.BlockTargets[0].CoveredBy != nil {
		t.Errorf("project b hits it uncovered, bucket must not be covered: %+v", rep.BlockTargets)
	}
}

// writeTranscriptCwd writes a transcript whose session ran in cwd and hit one
// sandbox block with the given result.
func writeTranscriptCwd(t *testing.T, home, rel, cwd, result string) {
	t.Helper()
	path := filepath.Join(home, "projects", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	enc := func(v any) string { raw, _ := json.Marshal(v); return string(raw) + "\n" }
	body := enc(map[string]any{"cwd": cwd, "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": "t1", "name": "Bash", "input": map[string]any{"command": "go mod download"}}}}}) +
		enc(map[string]any{"message": map[string]any{"content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": result, "is_error": true}}}})
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
