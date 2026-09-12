package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The ported cases from tests/bats/run-open.bats, which was `run-open.sh`'s spec.

func runOpen(t *testing.T, skill string) string {
	t.Helper()
	res := run(t, "run", "open", skill)
	if res.code != 0 {
		t.Fatalf("exit %d: %s%s", res.code, res.stdout, res.stderr)
	}
	return strings.TrimRight(res.stdout, "\n")
}

var runDirName = regexp.MustCompile(`^[a-z]+-\d{8}T\d{6}Z-[A-Za-z0-9]+$`)

func TestRunOpen(t *testing.T) {
	repo := factsRepo(t)
	dir := runOpen(t, "review")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("%q is not a directory: %v", dir, err)
	}
	if got := filepath.Dir(dir); got != filepath.Join(repo, ".mkit") {
		t.Errorf("parent = %q, want the scratch root", got)
	}
	if !runDirName.MatchString(filepath.Base(dir)) {
		t.Errorf("name = %q", filepath.Base(dir))
	}
	// Absolute, never a relative `.mkit/…`: the path is handed to subagents and
	// reused across shells.
	if !filepath.IsAbs(dir) {
		t.Errorf("%q is not absolute", dir)
	}
	// Nothing but the path on stdout, so it is safe in a command substitution.
	if strings.Count(run(t, "run", "open", "review").stdout, "\n") != 1 {
		t.Error("run open printed more than the path")
	}
}

// The timestamp is second-resolution, so two runs in one checkout can pick the
// same name; a plain MkdirAll would merge them and let each clobber the other's
// logs.
func TestRunOpenTwoRunsGetDistinctDirectories(t *testing.T) {
	factsRepo(t)
	a, b := runOpen(t, "review"), runOpen(t, "review")
	if a == b {
		t.Fatalf("both runs got %q", a)
	}
}

func TestRunOpenUsage(t *testing.T) {
	factsRepo(t)
	if res := run(t, "run", "open", "bad skill"); res.code != 2 {
		t.Errorf("a name that is not a bare slug: exit = %d, want 2", res.code)
	}
	if res := run(t, "run", "open"); res.code != 2 {
		t.Errorf("no arguments: exit = %d, want 2", res.code)
	}
	t.Chdir(t.TempDir())
	if res := run(t, "run", "open", "review"); res.code != 1 {
		t.Errorf("outside a repo: exit = %d, want 1", res.code)
	}
}

// The run root is inside the working tree, not the git dir: under a shared `.git`
// it resolved into the main checkout, where the worktree-isolation guard refuses
// every write (ADR 0002).
func TestRunOpenIsInsideTheWorkTree(t *testing.T) {
	repo := factsRepo(t)
	dir := runOpen(t, "review")
	if !strings.HasPrefix(dir, repo+string(os.PathSeparator)) {
		t.Errorf("%q is not under %q", dir, repo)
	}
	if strings.Contains(dir, "/.git/") {
		t.Errorf("%q is inside the git dir", dir)
	}
}

// A linked worktree gets its own, so no write of a worktree-isolated session
// targets the shared checkout.
func TestRunOpenALinkedWorktreeGetsItsOwn(t *testing.T) {
	repo := factsRepo(t)
	wt, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wt = filepath.Join(wt, "linked")
	gateGit(t, repo, "worktree", "add", "-q", "-b", "linked", wt, "main")
	t.Cleanup(func() { _ = os.RemoveAll(wt) })

	t.Chdir(wt)
	dir := runOpen(t, "review")
	if !strings.HasPrefix(dir, wt+string(os.PathSeparator)) {
		t.Errorf("%q is not inside the linked worktree %q", dir, wt)
	}
}

// Unignored, `git worktree remove` refuses, `git add -A` would commit run
// artefacts, and the gate fingerprint sees a directory that changes while the
// gate runs.
func TestRunOpenIgnoresTheScratchBeforeTheFirstWrite(t *testing.T) {
	repo := factsRepo(t)
	runOpen(t, "review")

	common := filepath.Join(repo, ".git", "info", "exclude")
	b, err := os.ReadFile(common)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".mkit/*", "!.mkit/config.toml"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("exclude file is missing %q:\n%s", want, b)
		}
	}
	if out := gateGit(t, repo, "status", "--porcelain"); out != "" {
		t.Errorf("the run directory dirtied the tree:\n%s", out)
	}

	// Written once, not appended on every run.
	runOpen(t, "commit")
	after, err := os.ReadFile(common)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(b) {
		t.Errorf("a second run appended again:\n%s", after)
	}
}

// A worktree holding only a run directory must still come out without --force.
func TestRunOpenAWorktreeHoldingOnlyARunDirIsRemovable(t *testing.T) {
	repo := factsRepo(t)
	wt, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wt = filepath.Join(wt, "linked")
	gateGit(t, repo, "worktree", "add", "-q", "-b", "linked", wt, "main")
	t.Chdir(wt)
	runOpen(t, "finish")
	t.Chdir(repo)
	gateGit(t, repo, "worktree", "remove", wt)
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("the worktree survived: %v", err)
	}
}

// --- prune -----------------------------------------------------------------

// stale makes a run directory that prune is allowed to consider: age-ranked
// eviction alone cannot see a run still being written, so anything touched in
// the last hour is skipped as live.
func stale(t *testing.T, repo, name string) string {
	t.Helper()
	dir := filepath.Join(repo, ".mkit", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunPruneUsage(t *testing.T) {
	factsRepo(t)
	// `0` is a perfectly good number and would evict every run directory —
	// including the live one belonging to the caller doing the pruning.
	if res := run(t, "run", "prune", "--keep", "0"); res.code != 2 {
		t.Errorf("--keep 0: exit = %d, want 2", res.code)
	}
	if res := run(t, "run", "prune", "--keep", "notanumber"); res.code != 2 {
		t.Errorf("a non-numeric count: exit = %d, want 2", res.code)
	}
}

func TestRunPruneWithNothingToPrune(t *testing.T) {
	factsRepo(t)
	res := run(t, "run", "prune")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "pruned 0") {
		t.Errorf("stdout = %q", res.stdout)
	}
}

// One pass per skill, so a busy `review` never evicts the only `pr` run.
func TestRunPruneKeepsTheNewestPerSkill(t *testing.T) {
	repo := factsRepo(t)
	var review []string
	for _, ts := range []string{"20260101T000001Z", "20260101T000002Z", "20260101T000003Z"} {
		review = append(review, stale(t, repo, "review-"+ts+"-aaaaaa"))
	}
	pr := stale(t, repo, "pr-20250101T000001Z-aaaaaa")

	res := run(t, "run", "prune", "--keep", "2")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if _, err := os.Stat(review[0]); !os.IsNotExist(err) {
		t.Errorf("the oldest review run survived: %v", err)
	}
	for _, d := range review[1:] {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("a kept review run was removed: %v", err)
		}
	}
	// Older than every review run, and still kept: the passes are per skill.
	if _, err := os.Stat(pr); err != nil {
		t.Errorf("the only pr run was evicted by a busy review: %v", err)
	}
}

func TestRunPruneSkipsALiveDirectory(t *testing.T) {
	repo := factsRepo(t)
	stale(t, repo, "review-20260101T000001Z-aaaaaa")
	stale(t, repo, "review-20260101T000002Z-aaaaaa")
	// Beyond --keep, but touched just now: a long review holding an older
	// directory would otherwise be removed underneath itself.
	live := filepath.Join(repo, ".mkit", "review-20260101T000000Z-aaaaaa")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}

	res := run(t, "run", "prune", "--keep", "1")
	if !strings.Contains(res.stdout, "still active") {
		t.Errorf("stdout = %q", res.stdout)
	}
	if _, err := os.Stat(live); err != nil {
		t.Errorf("a live run was removed: %v", err)
	}
}

// Only `<skill>-*` directories are in range, which is what keeps gate.jsonl out
// of it.
func TestRunPruneTouchesNothingButRunDirectories(t *testing.T) {
	repo := factsRepo(t)
	for i := 0; i < 7; i++ {
		stale(t, repo, "cleanup-2026010"+string(rune('1'+i))+"T000001Z-aaaaaa")
	}
	ledger := filepath.Join(repo, ".mkit", "gate.jsonl")
	if err := os.WriteFile(ledger, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(repo, ".mkit", "config.toml")
	if err := os.WriteFile(config, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := stale(t, repo, "not-a-run-dir")

	res := run(t, "run", "prune", "--keep", "1")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "pruned 6") {
		t.Errorf("stdout = %q — cleanup-* is in range like every other skill", res.stdout)
	}
	for _, p := range []string{ledger, config, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed: %v", p, err)
		}
	}
}
