package storage

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

const (
	testOldAge   = 20 * 24 * time.Hour
	testFreshAge = 1 * 24 * time.Hour
	testDays     = 7
)

func entryPaths(entries []Entry) []string {
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.Path
	}
	sort.Strings(paths)
	return paths
}

func findCategory(t *testing.T, pr ProviderReport, label string) CategoryReport {
	t.Helper()
	for _, c := range pr.Categories {
		if c.Label == label {
			return c
		}
	}
	t.Fatalf("category %q not found in provider %s (have: %v)", label, pr.Name, pr.Categories)
	return CategoryReport{}
}

func findProvider(t *testing.T, report *Report, name string) ProviderReport {
	t.Helper()
	for _, p := range report.Providers {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("provider %q not found in report (have: %v)", name, report.Providers)
	return ProviderReport{}
}

func buildClaudeFixture(t *testing.T) string {
	t.Helper()
	root := buildFixtureHome(t,
		[]fixtureFile{
			{rel: "projects/session1/transcript.jsonl", age: testOldAge, size: 100},
			{rel: "projects/session1/other.log", age: testOldAge, size: 50},
			{rel: "projects/session2/fresh.jsonl", age: testFreshAge, size: 10},

			{rel: "file-history/sess-a/data.bin", age: testOldAge, size: 30},
			{rel: "file-history/sess-b/data.bin", age: testFreshAge, size: 30},
			{rel: "file-history/.dotdir/data.bin", age: testOldAge, size: 30},

			{rel: "session-env/sess-x/env.json", age: testOldAge, size: 5},

			{rel: "paste-cache/old.txt", age: testOldAge, size: 20},
			{rel: "paste-cache/fresh.txt", age: testFreshAge, size: 20},

			{rel: "shell-snapshots/old.sh", age: testOldAge, size: 8},
			{rel: "telemetry/old.json", age: testOldAge, size: 9},
		},
		[]fixtureDir{
			{rel: "file-history/sess-empty"},
		},
	)

	// A symlink to a file: excluded from the files scan even though its
	// target (backdated) would otherwise qualify.
	target := filepath.Join(root, "paste-cache", "old.txt")
	if err := os.Symlink(target, filepath.Join(root, "paste-cache", "symlink-to-file")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// A symlink to a directory outside home: never descended, never
	// counted as a stale-dir candidate.
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "anything"), 5)
	if err := os.Symlink(outside, filepath.Join(root, "file-history", "symlink-out")); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}

	return root
}

func buildCodexFixture(t *testing.T) string {
	t.Helper()
	return buildFixtureHome(t,
		[]fixtureFile{
			{rel: "sessions/s1/transcript.jsonl", age: testOldAge, size: 40},
			{rel: "shell_snapshots/old.sh", age: testOldAge, size: 6},
		},
		nil,
	)
}

func scanFixtures(t *testing.T, provider string) *Report {
	t.Helper()
	t.Setenv("CLAUDE_HOME", buildClaudeFixture(t))
	t.Setenv("CODEX_HOME", buildCodexFixture(t))

	report, err := Scan(Options{Days: testDays, Provider: provider})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return report
}

func TestScan_ProjectsTranscripts(t *testing.T) {
	report := scanFixtures(t, "claude")
	pr := findProvider(t, report, "claude")
	cat := findCategory(t, pr, "projects/**/*.jsonl (transcripts)")

	got := entryPaths(cat.Entries)
	want := []string{filepath.Join(pr.Home, "projects/session1/transcript.jsonl")}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("projects entries = %v, want %v", got, want)
	}
	if cat.Bytes != 100 {
		t.Fatalf("projects bytes = %d, want 100", cat.Bytes)
	}
}

func TestScan_StaleDirs(t *testing.T) {
	report := scanFixtures(t, "claude")
	pr := findProvider(t, report, "claude")
	cat := findCategory(t, pr, "file-history/<session>/")

	got := entryPaths(cat.Entries)
	want := []string{
		filepath.Join(pr.Home, "file-history/sess-a"),
		filepath.Join(pr.Home, "file-history/sess-empty"),
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("stale dirs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stale dirs = %v, want %v", got, want)
		}
	}
	// sess-b (has a fresh file) and .dotdir (dot-directory) must never
	// appear; symlink-out (points outside home) must never appear either.
	for _, e := range cat.Entries {
		if filepath.Base(e.Path) == "sess-b" || filepath.Base(e.Path) == ".dotdir" || filepath.Base(e.Path) == "symlink-out" {
			t.Fatalf("stale dirs unexpectedly includes %s", e.Path)
		}
	}
	if cat.Bytes != 30 { // sess-a's one 30-byte file; sess-empty contributes 0
		t.Fatalf("stale dirs bytes = %d, want 30", cat.Bytes)
	}
}

func TestScan_SimpleFileCategories(t *testing.T) {
	report := scanFixtures(t, "claude")
	pr := findProvider(t, report, "claude")

	cases := []struct {
		label string
		want  string
		bytes int64
	}{
		{"paste-cache/*", "paste-cache/old.txt", 20},
		{"shell-snapshots/*", "shell-snapshots/old.sh", 8},
		{"telemetry/*", "telemetry/old.json", 9},
	}
	for _, tc := range cases {
		cat := findCategory(t, pr, tc.label)
		got := entryPaths(cat.Entries)
		want := []string{filepath.Join(pr.Home, tc.want)}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("%s entries = %v, want %v", tc.label, got, want)
		}
		if cat.Bytes != tc.bytes {
			t.Fatalf("%s bytes = %d, want %d", tc.label, cat.Bytes, tc.bytes)
		}
	}

	// paste-cache's symlink-to-file and fresh.txt must never appear.
	cat := findCategory(t, pr, "paste-cache/*")
	for _, e := range cat.Entries {
		base := filepath.Base(e.Path)
		if base == "symlink-to-file" || base == "fresh.txt" {
			t.Fatalf("paste-cache unexpectedly includes %s", e.Path)
		}
	}
}

func TestScan_Codex(t *testing.T) {
	report := scanFixtures(t, "codex")
	if len(report.Providers) != 1 {
		t.Fatalf("expected only codex in report, got %v", report.Providers)
	}
	pr := findProvider(t, report, "codex")

	transcripts := findCategory(t, pr, "sessions/**/*.jsonl (transcripts)")
	if got := entryPaths(transcripts.Entries); len(got) != 1 || got[0] != filepath.Join(pr.Home, "sessions/s1/transcript.jsonl") {
		t.Fatalf("codex transcripts = %v", got)
	}

	snapshots := findCategory(t, pr, "shell_snapshots/*")
	if got := entryPaths(snapshots.Entries); len(got) != 1 || got[0] != filepath.Join(pr.Home, "shell_snapshots/old.sh") {
		t.Fatalf("codex shell_snapshots = %v", got)
	}
}

func TestScan_MissingBaseIsOmittedNotZeroed(t *testing.T) {
	t.Setenv("CLAUDE_HOME", t.TempDir()) // exists, but no category subdirs
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "does-not-exist"))

	report, err := Scan(Options{Days: testDays, Provider: "all"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	pr := findProvider(t, report, "claude")
	if len(pr.Categories) != 0 {
		t.Fatalf("expected no categories for an empty claude home, got %v", pr.Categories)
	}
	codex := findProvider(t, report, "codex")
	if len(codex.Categories) != 0 {
		t.Fatalf("expected no categories for a missing codex home, got %v", codex.Categories)
	}
}

func TestScan_UnreadableDirectoryIsReportedNotSwallowed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits don't restrict access")
	}
	root := buildFixtureHome(t, nil, []fixtureDir{{rel: "paste-cache"}})
	blocked := filepath.Join(root, "paste-cache", "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(blocked, "f.txt"), 1)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	t.Setenv("CLAUDE_HOME", root)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "missing"))

	report, err := Scan(Options{Days: testDays, Provider: "claude"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	cat := findCategory(t, findProvider(t, report, "claude"), "paste-cache/*")
	if len(cat.Errors) == 0 {
		t.Fatalf("expected an error for the unreadable directory, got none")
	}
}

// TestScanFiles_BoundaryDeviation1 pins the documented cutoff rule: delete
// when mtime is strictly before cutoff. A file exactly at cutoff is fresh
// (kept); a file one nanosecond older is stale (pruned).
func TestScanFiles_BoundaryDeviation1(t *testing.T) {
	dir := t.TempDir()
	cutoff := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	atCutoff := filepath.Join(dir, "at-cutoff.txt")
	beforeCutoff := filepath.Join(dir, "before-cutoff.txt")
	writeFile(t, atCutoff, 1)
	writeFile(t, beforeCutoff, 1)
	chtimesAt(t, atCutoff, cutoff)
	chtimesAt(t, beforeCutoff, cutoff.Add(-time.Nanosecond))

	entries, errs := scanFiles(dir, "", cutoff)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(entries) != 1 || entries[0].Path != beforeCutoff {
		t.Fatalf("scanFiles at boundary = %v, want only %s pruned", entries, beforeCutoff)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0.0B"},
		{512, "512.0B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1.0MB"},
		{1024 * 1024 * 1024, "1.0GB"},
		{1024 * 1024 * 1024 * 1024, "1.0TB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1024.0TB"}, // capped at TB
	}
	for _, tc := range cases {
		if got := HumanBytes(tc.n); got != tc.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
