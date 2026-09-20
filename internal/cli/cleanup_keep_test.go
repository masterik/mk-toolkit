package cli

import (
	"encoding/json"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #20's "done when", end to end through the interface `cleanup` calls: a
// repo pinning `keep = ["staging"]` classifies `staging` as protected, with the
// default branch protected whether or not it appears in the list.

// pinKeep writes a config pinning the given keep names into an existing repo.
func pinKeep(t *testing.T, repo string, names ...string) {
	t.Helper()
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = `"` + n + `"`
	}
	dir := filepath.Join(repo, ".mkit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "version = 1\n\n[cleanup]\nkeep = [" + strings.Join(quoted, ", ") + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAPinnedKeepBranchIsProtected(t *testing.T) {
	repo := scanRepo(t)
	// Diverged, so it would otherwise classify as `unpushed` — a branch identical
	// to main's HEAD is trivially its own ancestor and would read as `merged`,
	// which would prove nothing about the keep list.
	diverged(t, repo, "staging")
	pinKeep(t, repo, "staging")

	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "branches", "staging", 2); got != "protected" {
		t.Errorf("staging class = %q, want protected\n%s", got, out)
	}
	if !strings.Contains(out, "protected=main,staging\n") {
		t.Errorf("protected= does not carry both branches:\n%s", out)
	}
	if !strings.Contains(out, "keep=staging\n") {
		t.Errorf("keep= does not report what the config pinned:\n%s", out)
	}
}

// The rule that cannot be bought back once it is wrong: a keep list naming only
// some other branch is not an instruction to delete the default one.
func TestTheDefaultBranchIsProtectedThoughTheKeepListOmitsIt(t *testing.T) {
	repo := scanRepo(t)
	diverged(t, repo, "staging")
	pinKeep(t, repo, "staging")

	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "branches", "main", 2); got != "protected" {
		t.Errorf("main class = %q, want protected\n%s", got, out)
	}
}

// A pinned name with no local branch is reported, never refused: a keep list
// travels with the repo, and a long-lived branch nobody checked out in this
// clone is the ordinary state.
func TestAPinnedNameWithNoLocalBranchIsReportedNotRefused(t *testing.T) {
	repo := scanRepo(t)
	pinKeep(t, repo, "no-such-branch")

	res := scan(t, "--default", "main", "--no-fetch", "--no-gh")
	if res.code != 0 {
		t.Fatalf("exit = %d, want 0: %s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "keep_unknown=no-such-branch\n") {
		t.Errorf("keep_unknown= does not name the pin:\n%s", res.stdout)
	}
	// It is not smuggled into the protected set either — `protected=` names
	// branches, and cleanup prints it as the list it is keeping.
	if !strings.Contains(res.stdout, "protected=main\n") {
		t.Errorf("protected= gained a branch that does not exist:\n%s", res.stdout)
	}
}

func TestScanReportsNoKeepWhenNothingIsPinned(t *testing.T) {
	scanRepo(t)
	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	for _, want := range []string{"keep=none\n", "keep_unknown=none\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestScanJSONCarriesKeepAndProtected(t *testing.T) {
	repo := scanRepo(t)
	diverged(t, repo, "staging")
	pinKeep(t, repo, "staging", "no-such-branch")

	res := scan(t, "--default", "main", "--no-fetch", "--no-gh", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	var got struct {
		Protected   []string `json:"protected"`
		Keep        []string `json:"keep"`
		KeepUnknown []string `json:"keep_unknown"`
		Branches    []struct{ Name, Class string }
	}
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, res.stdout)
	}
	if strings.Join(got.Protected, ",") != "main,staging" {
		t.Errorf("protected = %v", got.Protected)
	}
	if strings.Join(got.Keep, ",") != "staging,no-such-branch" {
		t.Errorf("keep = %v", got.Keep)
	}
	if strings.Join(got.KeepUnknown, ",") != "no-such-branch" {
		t.Errorf("keep_unknown = %v", got.KeepUnknown)
	}
	for _, b := range got.Branches {
		if b.Name == "staging" && b.Class != "protected" {
			t.Errorf("staging class = %q", b.Class)
		}
	}
}

// A keep branch is a branch never to delete, not a new proof that other work has
// landed. Treating "merged into staging" as merged would turn a key whose whole
// purpose is to delete less into one that deletes more.
func TestAKeepBranchIsNotAMergeTarget(t *testing.T) {
	repo := scanRepo(t)
	diverged(t, repo, "staging")
	// feature is an ancestor of staging, and of nothing else.
	gateGit(t, repo, "checkout", "-q", "staging")
	gateGit(t, repo, "checkout", "-q", "-b", "feature")
	put(t, repo, "b.txt", "feature\n")
	gateGit(t, repo, "add", "b.txt")
	gateGit(t, repo, "commit", "-q", "-m", "feature work")
	gateGit(t, repo, "checkout", "-q", "staging")
	gateGit(t, repo, "merge", "-q", "--ff-only", "feature")
	gateGit(t, repo, "checkout", "-q", "main")
	pinKeep(t, repo, "staging")

	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "branches", "feature", 2); got == "merged" {
		t.Errorf("feature classified as merged on the strength of a keep branch:\n%s", out)
	}
	if got := col(t, out, "branches", "feature", 4); got != "-" {
		t.Errorf("merged_into = %q, want none\n%s", got, out)
	}
}

// `mkit init --keep` writes the key, and a keep list alone is enough to pin.
func TestInitPinsAKeepList(t *testing.T) {
	repo := brokenConfigFreeRepo(t)
	res := run(t, "init", "--yes", "--keep", "staging", "--keep", "release", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	var got struct{ Written bool }
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, res.stdout)
	}
	if !got.Written {
		t.Fatalf("nothing written:\n%s", res.stdout)
	}
	b, err := os.ReadFile(filepath.Join(repo, ".mkit", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[cleanup]", "staging", "release"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the written config never mentions %q:\n%s", want, b)
		}
	}
}

// `repo profile` reports the value tagged, the way commit_scopes and reviewers
// already are — discovered when nothing is pinned, pinned when something is.
func TestRepoProfileTagsCleanupKeep(t *testing.T) {
	repo := brokenConfigFreeRepo(t)

	var discovered struct {
		Keep struct {
			Values []string
			Source string
		} `json:"cleanup_keep"`
	}
	res := run(t, "repo", "profile", "--json")
	if err := json.Unmarshal([]byte(res.stdout), &discovered); err != nil {
		t.Fatalf("%v\n%s", err, res.stdout)
	}
	if discovered.Keep.Source != "discovered" || strings.Join(discovered.Keep.Values, ",") != "main" {
		t.Errorf("cleanup_keep = %+v, want the hardcoded set, discovered", discovered.Keep)
	}

	pinKeep(t, repo, "staging")
	var pinned struct {
		Keep struct {
			Values []string
			Source string
		} `json:"cleanup_keep"`
	}
	res = run(t, "repo", "profile", "--json")
	if err := json.Unmarshal([]byte(res.stdout), &pinned); err != nil {
		t.Fatalf("%v\n%s", err, res.stdout)
	}
	if pinned.Keep.Source != "pinned" || strings.Join(pinned.Keep.Values, ",") != "staging" {
		t.Errorf("cleanup_keep = %+v, want the pinned list", pinned.Keep)
	}
	if out := run(t, "repo", "profile").stdout; !strings.Contains(out, "cleanup keep: staging [pinned]") {
		t.Errorf("`repo profile` does not report the kept branches:\n%s", out)
	}
}

// brokenConfigFreeRepo is a throwaway repo on `main` with every environment
// lookup pointed inside it — never the developer's home.
func brokenConfigFreeRepo(t *testing.T) string {
	t.Helper()
	repo := scanRepo(t)
	t.Setenv("MKIT_HOME", filepath.Join(repo, ".mkit-home"))
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	return repo
}

// Config is an input, never a permission — and that rule does not stop at a file
// mkit could not parse. This command did not open the config at all before
// `[cleanup] keep`, so returning Load's read error would let an unreadable
// config.toml abort the scan and take `cleanup`'s whole classifier with it, over
// a file the scan does not need. It scans with nothing pinned instead.
func TestAnUnreadableConfigDoesNotAbortTheScan(t *testing.T) {
	dir := scanRepo(t)
	path := filepath.Join(dir, repoconfig.RelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mode 000 is the cheapest unreadable-but-present file; a directory at the
	// same path is the other shape and reaches the same branch.
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	res := scan(t, "--default", "main", "--no-fetch", "--no-gh")
	if res.code != 0 {
		t.Fatalf("exit %d on an unreadable config; the scan needs no config to classify: %s%s",
			res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "protected=main") {
		t.Errorf("default branch not protected on the degraded path:\n%s", res.stdout)
	}
	// Carrying on is only half of it: an unreadable config must not look like a
	// repo that pinned nothing, or `cleanup` deletes on an empty keep list it
	// believes is real.
	if !strings.Contains(res.stdout, "config_problems=1") || !strings.Contains(res.stdout, "notes:") {
		t.Errorf("an unreadable config was swallowed silently:\n%s", res.stdout)
	}
}

// The failure #19 exists to prevent, on the one path that deletes things: a
// typo'd table drops the keep pin, and `keep=none` then reads exactly like a
// repo that pinned nothing.
func TestADroppedKeepPinIsReportedNotSilentlyEmpty(t *testing.T) {
	dir := scanRepo(t)
	writeConfig(t, dir, "version = 1\n\n[cleanp]\nkeep = [\"staging\"]\n")

	res := scan(t, "--default", "main", "--no-fetch", "--no-gh")
	if res.code != 0 {
		t.Fatalf("exit %d: config is an input, never a permission: %s%s", res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "keep=none") {
		t.Errorf("the dropped pin should leave keep empty:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "config_problems=1") {
		t.Fatalf("a dropped keep pin is indistinguishable from none pinned:\n%s", res.stdout)
	}
	notes, ok := cutAfter(res.stdout, "notes:")
	if !ok || !strings.Contains(notes, "cleanp") {
		t.Errorf("the note does not name the key that was dropped:\n%s", res.stdout)
	}
}

func cutAfter(s, sep string) (string, bool) {
	_, rest, ok := strings.Cut(s, sep)
	return rest, ok
}
