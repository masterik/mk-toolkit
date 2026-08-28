package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApply_DeletesExactlySelectedSet(t *testing.T) {
	root := buildFixtureHome(t, []fixtureFile{
		{rel: "paste-cache/old.txt", age: testOldAge, size: 20},
		{rel: "paste-cache/fresh.txt", age: testFreshAge, size: 30},
	}, nil)
	t.Setenv("CLAUDE_HOME", root)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "missing"))

	report, err := Scan(Options{Days: testDays, Provider: "claude"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	result, err := Apply(report, SelectAll(report))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Deleted != 1 || result.Bytes != 20 {
		t.Fatalf("result = %+v, want 1 deleted / 20 bytes", result)
	}
	if _, err := os.Stat(filepath.Join(root, "paste-cache", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old.txt should be gone, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "paste-cache", "fresh.txt")); err != nil {
		t.Fatalf("fresh.txt should survive: %v", err)
	}
}

func TestApply_SkipsStaleDirThatFreshenedSinceScan(t *testing.T) {
	root := buildFixtureHome(t, []fixtureFile{
		{rel: "file-history/sess-a/data.bin", age: testOldAge, size: 30},
	}, nil)
	t.Setenv("CLAUDE_HOME", root)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "missing"))

	report, err := Scan(Options{Days: testDays, Provider: "claude"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	cat := findCategory(t, findProvider(t, report, "claude"), "file-history/<session>/")
	if len(cat.Entries) != 1 {
		t.Fatalf("expected sess-a to be scanned as stale, got %v", cat.Entries)
	}

	// Simulate new activity in the directory between scan and apply.
	writeFile(t, filepath.Join(root, "file-history", "sess-a", "new.bin"), 5)

	result, err := Apply(report, SelectAll(report))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("expected nothing deleted, got %+v", result)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Path != cat.Entries[0].Path {
		t.Fatalf("expected sess-a skipped, got %+v", result.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, "file-history", "sess-a")); err != nil {
		t.Fatalf("sess-a should survive: %v", err)
	}
}

func TestApply_SkipsStaleDirWhenFreshnessRecheckErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits don't restrict access")
	}
	root := buildFixtureHome(t, []fixtureFile{
		{rel: "file-history/sess-a/sub/data.bin", age: testOldAge, size: 30},
	}, nil)
	t.Setenv("CLAUDE_HOME", root)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "missing"))

	report, err := Scan(Options{Days: testDays, Provider: "claude"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	cat := findCategory(t, findProvider(t, report, "claude"), "file-history/<session>/")
	if len(cat.Entries) != 1 {
		t.Fatalf("expected sess-a to be scanned as stale, got %v", cat.Entries)
	}

	// Block the re-check's walk after Scan already ran, so Apply cannot
	// confirm sess-a is still free of a fresh file.
	sub := filepath.Join(root, "file-history", "sess-a", "sub")
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	result, err := Apply(report, SelectAll(report))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("expected nothing deleted, got %+v", result)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Path != cat.Entries[0].Path {
		t.Fatalf("expected sess-a skipped, got %+v", result.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, "file-history", "sess-a")); err != nil {
		t.Fatalf("sess-a should survive an unconfirmed re-check: %v", err)
	}
}

func TestApply_NeverRemovesTheCategoryBase(t *testing.T) {
	root := buildFixtureHome(t, []fixtureFile{
		{rel: "projects/session1/transcript.jsonl", age: testOldAge, size: 10},
	}, nil)
	t.Setenv("CLAUDE_HOME", root)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "missing"))

	report, err := Scan(Options{Days: testDays, Provider: "claude"})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if _, err := Apply(report, SelectAll(report)); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "projects")); err != nil {
		t.Fatalf("projects/ base must survive even when emptied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "session1")); !os.IsNotExist(err) {
		t.Fatalf("session1 should have been swept as an empty dir, stat err = %v", err)
	}
}

func TestApply_SymlinkInsideHomeNeitherFollowedNorDeletedThrough(t *testing.T) {
	root := buildFixtureHome(t, nil, []fixtureDir{{rel: "paste-cache"}})
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "target.txt")
	writeFile(t, outsideFile, 100)

	link := filepath.Join(root, "paste-cache", "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Hand-construct a report as if Scan had (incorrectly) named the
	// symlink, to prove Apply's own guard — not just Scan's exclusion —
	// refuses to delete through it.
	report := &Report{
		Days: testDays,
		Providers: []ProviderReport{{
			Name: "claude",
			Home: root,
			Categories: []CategoryReport{{
				Label: "paste-cache/*",
				Base:  filepath.Join(root, "paste-cache"),
				Kind:  KindFiles,
				Entries: []Entry{
					{Path: link, Bytes: 100},
				},
			}},
		}},
	}

	result, err := Apply(report, SelectAll(report))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Deleted != 0 {
		t.Fatalf("expected nothing deleted, got %+v", result)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected the symlink to be reported skipped, got %+v", result)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("symlink itself must survive: %v", err)
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("file outside home must survive: %v", err)
	}
}

func TestApply_RefusesDangerousHomeBeforeAnyIO(t *testing.T) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no $HOME to test against")
	}

	for _, home := range []string{"/", userHome, ""} {
		home := home
		t.Run(home, func(t *testing.T) {
			victim := filepath.Join(t.TempDir(), "victim.txt")
			writeFile(t, victim, 1)

			report := &Report{
				Days: testDays,
				Providers: []ProviderReport{{
					Name: "claude",
					Home: home,
					Categories: []CategoryReport{{
						Label:   "paste-cache/*",
						Kind:    KindFiles,
						Entries: []Entry{{Path: victim, Bytes: 1}},
					}},
				}},
			}

			if _, err := Apply(report, SelectAll(report)); err == nil {
				t.Fatalf("expected Apply to refuse home %q", home)
			}
			if _, err := os.Stat(victim); err != nil {
				t.Fatalf("nothing should have been deleted before the guard fired: %v", err)
			}
		})
	}
}
