package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The ported cases from tests/bats/facts.bats, which was `facts.sh`'s spec.

// factsField splits on whitespace too, not just newlines: some lines pack several
// pairs (`staged=1 unstaged=0 untracked=0 conflicted=0`).
func factsField(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		for _, tok := range strings.Fields(line) {
			if v, ok := strings.CutPrefix(tok, key+"="); ok {
				return v
			}
		}
		// A value may legitimately contain spaces on a line that carries only
		// one pair (`unstaged_stat=1 file changed, 1 insertion(+)`).
		if v, ok := strings.CutPrefix(line, key+"="); ok && !strings.Contains(line, "= ") {
			return v
		}
	}
	return ""
}

// factsRepo is a throwaway repo with the user-scoped dir pointed inside it, so no
// run can read or write a developer's real state.
func factsRepo(t *testing.T) string {
	t.Helper()
	repo, _ := gateRepo(t)
	if err := os.RemoveAll(filepath.Join(repo, ".mkit")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MKIT_HOME", filepath.Join(repo, ".mkit-home"))
	return repo
}

func TestFactsCleanTree(t *testing.T) {
	factsRepo(t)
	res := run(t, "facts", "commit", "--no-run")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	for key, want := range map[string]string{"clean": "yes", "branch": "main", "detached": "no"} {
		if got := factsField(res.stdout, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestFactsOpensARunDirectoryUnlessNoRun(t *testing.T) {
	factsRepo(t)
	res := run(t, "facts", "commit")
	dir := factsField(res.stdout, "run")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("run=%q is not a directory: %v", dir, err)
	}
	if got := run(t, "facts", "commit", "--no-run").stdout; strings.Contains(got, "run=") {
		t.Errorf("--no-run printed a run= line:\n%s", got)
	}
}

func TestFactsDistinguishesStagedUnstagedUntracked(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, "staged.txt", "staged\n")
	gateGit(t, repo, "add", "staged.txt")
	put(t, repo, "a.txt", "unstaged\n")
	put(t, repo, "untracked.txt", "x\n")

	res := run(t, "facts", "commit", "--no-run")
	for key, want := range map[string]string{"clean": "no", "staged": "1", "unstaged": "1", "untracked": "1"} {
		if got := factsField(res.stdout, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// A bare `git diff --shortstat` reports nothing when the work is fully staged,
// which reads exactly like a clean tree — the single most expensive misread in
// this bundle, and the reason both stats are always printed.
func TestFactsAFullyStagedTreeIsNotClean(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, "a.txt", "more\n")
	gateGit(t, repo, "add", "a.txt")
	res := run(t, "facts", "commit", "--no-run")
	if got := factsField(res.stdout, "clean"); got != "no" {
		t.Errorf("clean = %q", got)
	}
	if !strings.Contains(res.stdout, "staged_stat=") {
		t.Errorf("no staged_stat:\n%s", res.stdout)
	}
}

// `git diff` never lists an untracked file, so a dirty tree containing new files
// reported `untracked=6` beside a file list naming none of them.
func TestFactsUntrackedFilesGetTheirOwnBlock(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, "new-file.txt", "x\n")
	res := run(t, "facts", "commit", "--no-run")
	if got := factsField(res.stdout, "untracked_files"); got != "1" {
		t.Errorf("untracked_files = %q", got)
	}
	if !strings.Contains(res.stdout, "untracked_file_list:\nnew-file.txt") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
	if strings.Contains(res.stdout, "unstaged_file_list:\nnew-file.txt") {
		t.Errorf("an untracked file was folded into the diff list:\n%s", res.stdout)
	}
}

// It used to fall through silently and still exit 0, so `finish`/`pr` got a fact
// set with no commits_ahead_of_base and no way to tell that from a base with
// nothing on it.
func TestFactsAnUnresolvableBaseSaysSoOnStdoutAndFails(t *testing.T) {
	factsRepo(t)
	res := run(t, "facts", "commit", "--no-run", "--base", "does-not-exist")
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.stdout, "base_state=unresolvable") {
		t.Errorf("stdout:\n%s", res.stdout)
	}
}

func TestFactsBaseWithCommitsAhead(t *testing.T) {
	repo := factsRepo(t)
	gateGit(t, repo, "checkout", "-q", "-b", "feature")
	put(t, repo, "a.txt", "more\n")
	gateGit(t, repo, "add", "a.txt")
	gateGit(t, repo, "commit", "-q", "-m", "feature commit")

	res := run(t, "facts", "commit", "--no-run", "--base", "main")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if got := factsField(res.stdout, "base_state"); got != "ok" {
		t.Errorf("base_state = %q", got)
	}
	if got := factsField(res.stdout, "commits_ahead_of_base"); got != "1" {
		t.Errorf("commits_ahead_of_base = %q", got)
	}
	if !strings.Contains(res.stdout, "feature commit") {
		t.Errorf("no commit log:\n%s", res.stdout)
	}
	if got := factsField(res.stdout, "ff_from_base"); got != "yes" {
		t.Errorf("ff_from_base = %q", got)
	}
}

func TestFactsUsage(t *testing.T) {
	factsRepo(t)
	if res := run(t, "facts", "not a skill", "--no-run"); res.code != 2 {
		t.Errorf("a skill name with a space: exit = %d, want 2", res.code)
	}
	if res := run(t, "facts"); res.code != 2 {
		t.Errorf("no skill argument: exit = %d, want 2", res.code)
	}
}

func TestFactsFailsOutsideAGitRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if res := run(t, "facts", "commit", "--no-run"); res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
}

func TestFactsDetachedHead(t *testing.T) {
	repo := factsRepo(t)
	gateGit(t, repo, "checkout", "-q", "--detach", "HEAD")
	if got := factsField(run(t, "facts", "commit", "--no-run").stdout, "detached"); got != "yes" {
		t.Errorf("detached = %q", got)
	}
}

// `pr=none` used to mean four different things, and only one justifies opening one.
func TestFactsNoGHReportsPRGHMissing(t *testing.T) {
	factsRepo(t)
	noGH(t)
	res := run(t, "facts", "pr", "--no-run", "--gh")
	if res.code != 0 {
		t.Fatalf("exit = %d", res.code)
	}
	if got := factsField(res.stdout, "pr"); got != "gh-missing" {
		t.Errorf("pr = %q", got)
	}
}

// --- where a write may land ------------------------------------------------
//
// Three boundaries can refuse a write and none announces itself — the OS sandbox,
// the auto-mode classifier, the worktree-isolation guard. These keys turn a
// mid-run "Operation not permitted" into a starting fact.

func TestFactsTMPNamesTheEphemeralRoot(t *testing.T) {
	factsRepo(t)
	want := os.Getenv("TMPDIR")
	if want == "" {
		want = "/tmp"
	}
	if got := factsField(run(t, "facts", "commit", "--no-run").stdout, "tmp"); got != want {
		t.Errorf("tmp = %q, want %q", got, want)
	}
}

func TestFactsRunIgnored(t *testing.T) {
	repo := factsRepo(t)
	res := run(t, "facts", "commit", "--no-run")
	if got := factsField(res.stdout, "run_ignored"); got != "no" {
		t.Fatalf("run_ignored = %q", got)
	}
	for _, want := range []string{"notes:", "run_ignored=no", "Do not stage while", "info/exclude"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, res.stdout)
		}
	}

	// Opening a run directory is what makes it ignored, and the tree stays clean.
	res = run(t, "facts", "commit")
	if got := factsField(res.stdout, "run_ignored"); got != "yes" {
		t.Errorf("run_ignored = %q after opening a run dir", got)
	}
	if strings.Contains(res.stdout, "Do not stage while") {
		t.Errorf("the remedy survived the fix:\n%s", res.stdout)
	}
	if out := gateGit(t, repo, "status", "--porcelain"); out != "" {
		t.Errorf("the run directory dirtied the tree:\n%s", out)
	}
}

// `config_state=absent` is a normal state and never a note: config is an input,
// never a permission (ADR 0001 decision 3).
func TestFactsConfigStates(t *testing.T) {
	repo := factsRepo(t)
	res := run(t, "facts", "commit", "--no-run")
	if got := factsField(res.stdout, "config"); got != filepath.Join(repo, ".mkit", "config.toml") {
		t.Errorf("config = %q", got)
	}
	if got := factsField(res.stdout, "config_state"); got != "absent" {
		t.Errorf("config_state = %q", got)
	}
	if strings.Contains(res.stdout, "config_state=shadowed") {
		t.Error("absent must be silent")
	}

	put(t, repo, ".mkit/config.toml", "version = 1\n")
	if got := factsField(run(t, "facts", "commit", "--no-run").stdout, "config_state"); got != "untracked" {
		t.Errorf("config_state = %q, want untracked", got)
	}
	gateGit(t, repo, "add", "-f", ".mkit/config.toml")
	gateGit(t, repo, "commit", "-q", "-m", "mkit config")
	if got := factsField(run(t, "facts", "commit", "--no-run").stdout, "config_state"); got != "tracked" {
		t.Errorf("config_state = %q, want tracked", got)
	}
}

// A repo carrying the legacy directory-only rule: `mkit init` would write a file
// that never reaches a fresh clone, so it is named at the first call.
func TestFactsConfigShadowed(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, ".gitignore", ".mkit/\n")
	res := run(t, "facts", "commit", "--no-run")
	if got := factsField(res.stdout, "config_state"); got != "shadowed" {
		t.Fatalf("config_state = %q", got)
	}
	for _, want := range []string{"notes:", "config_state=shadowed", "!.mkit/config.toml", ".gitignore"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, res.stdout)
		}
	}
}

func TestFactsUserDir(t *testing.T) {
	factsRepo(t)
	home := os.Getenv("MKIT_HOME")
	res := run(t, "facts", "commit", "--no-run")
	if got := factsField(res.stdout, "user_dir"); got != home {
		t.Errorf("user_dir = %q, want %q", got, home)
	}
	if got := factsField(res.stdout, "user_dir_writable"); got != "yes" {
		t.Errorf("user_dir_writable = %q", got)
	}
	// Reporting on it does not create it: creating user-scoped state as a side
	// effect of reporting would make the report the thing that changed the answer.
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("the probe left the directory behind: %v", err)
	}
}

func TestFactsAnUnwritableUserDirIsANamedFactWithAWorkingRemedy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores mode bits")
	}
	factsRepo(t)
	home := os.Getenv("MKIT_HOME")
	if err := os.MkdirAll(home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	res := run(t, "facts", "commit", "--no-run")
	if res.code != 0 {
		t.Fatalf("exit = %d", res.code)
	}
	if got := factsField(res.stdout, "user_dir_writable"); got != "no" {
		t.Errorf("user_dir_writable = %q", got)
	}
	if !strings.Contains(res.stdout, "permissions.additionalDirectories") {
		t.Errorf("no remedy:\n%s", res.stdout)
	}
	// Never the unreachable one: a protected path cannot be allowlisted.
	if strings.Contains(res.stdout, ".claude/mkit") {
		t.Errorf("the remedy names a path no allowlist entry reaches:\n%s", res.stdout)
	}
}

func TestFactsGitBinIsAbsolute(t *testing.T) {
	factsRepo(t)
	bin := factsField(run(t, "facts", "commit", "--no-run").stdout, "git_bin")
	if !strings.HasPrefix(bin, "/") {
		t.Fatalf("git_bin = %q, want an absolute path", bin)
	}
	if len(strings.Fields(bin)) != 1 {
		t.Errorf("git_bin = %q, want one token", bin)
	}
	if fi, err := os.Stat(bin); err != nil || fi.Mode()&0o111 == 0 {
		t.Errorf("git_bin is not executable: %v", err)
	}
}

// The binary reporting its own presence is not a fact. Every skill hard-requires
// it after M5, and a skill that needs a subcommand asks for that subcommand.
func TestFactsDoesNotReportItsOwnPresence(t *testing.T) {
	factsRepo(t)
	out := run(t, "facts", "commit", "--no-run").stdout
	for _, gone := range []string{"mkit_bin=", "mkit="} {
		if strings.Contains(out, gone) {
			t.Errorf("%s survived the port:\n%s", gone, out)
		}
	}
}

// The run directory lives *inside* the working directory, so without the
// exclusion the scratch mkit just created is reported back as the user's own
// change — and `run_ignored=no` is exactly the session that hits it.
func TestFactsAnUnignoredScratchIsNotTheUsersWork(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, ".mkit/review-x/step.log", "step output\n")
	put(t, repo, ".mkit/gate.jsonl", `{"step":"test"}`+"\n")
	if out := gateGit(t, repo, "status", "--porcelain"); out == "" {
		t.Fatal("the fixture is not dirty, so this proves nothing")
	}

	res := run(t, "facts", "commit", "--no-run")
	factsOnly, _, _ := strings.Cut(res.stdout, "\nnotes:")
	if got := factsField(factsOnly, "clean"); got != "yes" {
		t.Errorf("clean = %q — a tree holding nothing but mkit's scratch is clean", got)
	}
	if strings.Contains(factsOnly, "gate.jsonl") || strings.Contains(factsOnly, "step.log") {
		t.Errorf("mkit's own scratch was reported as the user's work:\n%s", factsOnly)
	}
}

func TestFactsJSON(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, "a.txt", "edited\n")
	res := run(t, "facts", "--json", "commit", "--no-run")
	var got struct {
		Toplevel string `json:"toplevel"`
		Clean    bool   `json:"clean"`
		Unstaged int    `json:"unstaged"`
		GitBin   string `json:"git_bin"`
		Scopes   map[string]struct {
			Stat  string   `json:"stat"`
			Files int      `json:"files"`
			List  []string `json:"list"`
		} `json:"scopes"`
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, res.stdout)
	}
	if got.Toplevel != repo || got.Clean || got.Unstaged != 1 {
		t.Errorf("%+v", got)
	}
	if s, ok := got.Scopes["unstaged"]; !ok || s.Files != 1 || s.List[0] != "a.txt" {
		t.Errorf("scopes = %+v", got.Scopes)
	}
	if _, ok := got.Scopes["staged"]; !ok {
		t.Error("both stats, always — the staged scope must be present")
	}
}

// --range gets --base's treatment. A range that does not resolve used to print
// `range_stat=none range_files=0` and exit 0, which is byte-identical to a range
// with nothing in it — so a skill reviewing "the last three commits" against a
// typo'd ref reported a clean review of nothing.
func TestFactsAnUnresolvableRangeIsFatal(t *testing.T) {
	factsRepo(t)
	res := run(t, "facts", "review", "--no-run", "--range", "nonexistentA..nonexistentB")

	if res.code != 1 {
		t.Errorf("exit = %d, want 1 — an unresolvable range is fatal, like an unresolvable base", res.code)
	}
	if got := factsField(res.stdout, "range_state"); got != "unresolvable" {
		t.Errorf("range_state = %q, want unresolvable", got)
	}
	// The fact is on stdout *and* the command fails: a skill that only reads the
	// exit code and one that only parses the facts must both get the answer.
	if strings.Contains(res.stdout, "range_stat=none") {
		t.Error("a range that does not resolve reported an empty range anyway")
	}
}

func TestFactsAResolvableRangeIsNotFatal(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, "b.txt", "second commit\n")
	gateGit(t, repo, "add", "-A")
	gateGit(t, repo, "commit", "-q", "-m", "second")

	res := run(t, "facts", "review", "--no-run", "--range", "HEAD~1..HEAD")
	if res.code != 0 {
		t.Fatalf("exit = %d: %s%s", res.code, res.stdout, res.stderr)
	}
	if got := factsField(res.stdout, "range_state"); got != "ok" {
		t.Errorf("range_state = %q, want ok", got)
	}
	if got := factsField(res.stdout, "range_files"); got != "1" {
		t.Errorf("range_files = %q, want 1", got)
	}
}

// A `git status` that fails must not read as a clean tree. The shell died here
// under `set -euo pipefail`; discarding the error made "cannot tell" and
// "nothing to do" the same output, and `clean=yes` is what a skill skips work on.
func TestFactsAFailingStatusIsNotACleanTree(t *testing.T) {
	repo := factsRepo(t)
	put(t, repo, "dirty.txt", "uncommitted\n")

	// A corrupt index is the cheap reproduction of "git cannot answer".
	if err := os.WriteFile(filepath.Join(repo, ".git", "index"),
		[]byte("not an index at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := run(t, "facts", "commit", "--no-run")
	if res.code == 0 {
		t.Errorf("exit = 0 with an unreadable index; facts reported:\n%s", res.stdout)
	}
	if got := factsField(res.stdout, "clean"); got == "yes" {
		t.Error("clean=yes from a git status that failed — the tree was dirty")
	}
}
