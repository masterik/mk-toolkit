package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `[cleanup] keep` through the commands that still read it (`init`, `repo
// profile`) until #53. The protection itself is `branch-status.sh`'s, tested in
// tests/branch-status.bats.

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
	repo, _ := gateRepo(t)
	t.Setenv("MKIT_HOME", filepath.Join(repo, ".mkit-home"))
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	return repo
}
