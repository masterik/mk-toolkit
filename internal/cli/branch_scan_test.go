package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The ported cases from tests/bats/branch-scan.bats, which was `branch-scan.sh`'s
// spec. `branches:` and `worktrees:` are tab-separated rows under their own
// header, so the helpers below read them the way `cleanup` does.

func rows(out, header string) [][]string {
	var got [][]string
	in := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == header+":":
			in = true
			continue
		case strings.HasSuffix(line, ":") && !strings.Contains(line, "\t"):
			in = false
		}
		if in && line != "" {
			got = append(got, strings.Split(line, "\t"))
		}
	}
	return got
}

// col reads column n (1-based, as the bats helper did) of the row named name.
func col(t *testing.T, out, header, name string, n int) string {
	t.Helper()
	for _, r := range rows(out, header) {
		if r[0] == name && len(r) >= n {
			return r[n-1]
		}
	}
	return ""
}

// scanRepo is a throwaway repo on `main` with one commit.
func scanRepo(t *testing.T) string {
	t.Helper()
	repo, _ := gateRepo(t)
	return repo
}

// diverged makes a branch with one commit of its own, not merged back — the
// shape every "unmerged" classification needs, since a branch identical to
// main's HEAD is trivially its own ancestor and would misreport as `merged`.
func diverged(t *testing.T, repo, name string) {
	t.Helper()
	gateGit(t, repo, "checkout", "-q", "-b", name)
	put(t, repo, "a.txt", "diverged\n")
	gateGit(t, repo, "add", "a.txt")
	gateGit(t, repo, "commit", "-q", "-m", name+" work")
	gateGit(t, repo, "checkout", "-q", "main")
}

// fakeGH puts a `gh` on PATH that answers `auth status` and returns prsJSON from
// `pr list`.
func fakeGH(t *testing.T, prsJSON string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/usr/bin/env bash\ncase \"$1 $2\" in\n" +
		"\"auth status\") exit 0 ;;\n" +
		"\"pr list\") cat <<'JSON'\n" + prsJSON + "\nJSON\n;;\nesac\n"
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// noGH puts a PATH on that holds everything the scan legitimately calls except
// `gh`. Excluding whole PATH directories would take out more than intended —
// macOS keeps tools the scan needs beside the ones it is simulating away.
func noGH(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, tool := range []string{"git", "bash", "sh"} {
		real, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		if err := os.Symlink(real, filepath.Join(dir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func scan(t *testing.T, args ...string) result {
	t.Helper()
	return run(t, append([]string{"branch", "scan"}, args...)...)
}

func TestScanRejectsADefaultThatDoesNotExist(t *testing.T) {
	scanRepo(t)
	if res := scan(t, "--default", "no-such-branch", "--no-fetch", "--no-gh"); res.code != 2 {
		t.Errorf("exit = %d, want 2", res.code)
	}
}

func TestScanRequiresADefault(t *testing.T) {
	scanRepo(t)
	if res := scan(t, "--no-fetch", "--no-gh"); res.code != 2 {
		t.Errorf("exit = %d, want 2", res.code)
	}
}

func TestScanFailsOutsideAGitRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if res := scan(t, "--default", "main", "--no-fetch", "--no-gh"); res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
}

func TestScanProtectedBranches(t *testing.T) {
	repo := scanRepo(t)
	res := scan(t, "--default", "main", "--no-fetch", "--no-gh")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if got := field(res.stdout, "default"); got != "main" {
		t.Errorf("default = %q", got)
	}
	if got := field(res.stdout, "develop"); got != "none" {
		t.Errorf("develop = %q", got)
	}
	if got := col(t, res.stdout, "branches", "main", 2); got != "protected" {
		t.Errorf("main class = %q", got)
	}

	// A develop-like local branch is protected alongside the default.
	gateGit(t, repo, "branch", "develop")
	res = scan(t, "--default", "main", "--no-fetch", "--no-gh")
	if got := field(res.stdout, "develop"); got != "develop" {
		t.Errorf("develop = %q", got)
	}
	if got := field(res.stdout, "protected"); got != "main,develop" {
		t.Errorf("protected = %q", got)
	}
	if got := col(t, res.stdout, "branches", "develop", 2); got != "protected" {
		t.Errorf("develop class = %q", got)
	}
}

func TestScanRecognizesDevWhenDevelopIsAbsent(t *testing.T) {
	repo := scanRepo(t)
	gateGit(t, repo, "branch", "dev")
	if got := field(scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout, "develop"); got != "dev" {
		t.Errorf("develop = %q", got)
	}
}

// Only a *local* branch counts: a develop that exists only as origin/develop is
// not one of this repo's branches to keep.
func TestScanARemoteDevelopIsNotKept(t *testing.T) {
	repo := scanRepo(t)
	gateGit(t, repo, "update-ref", "refs/remotes/origin/develop", "HEAD")
	if got := field(scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout, "develop"); got != "none" {
		t.Errorf("develop = %q", got)
	}
}

func TestScanMergedBranch(t *testing.T) {
	repo := scanRepo(t)
	diverged(t, repo, "feature")
	gateGit(t, repo, "merge", "-q", "--no-ff", "feature")
	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "branches", "feature", 2); got != "merged" {
		t.Errorf("class = %q", got)
	}
	if got := col(t, out, "branches", "feature", 4); got != "main" {
		t.Errorf("merged_into = %q", got)
	}
}

// feature == main's HEAD, so it is trivially an ancestor of main — but it must
// still report `current`, since you cannot delete the branch you are on.
func TestScanTheCurrentBranchIsNeverMerged(t *testing.T) {
	repo := scanRepo(t)
	gateGit(t, repo, "checkout", "-q", "-b", "feature")
	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "branches", "feature", 2); got != "current" {
		t.Errorf("class = %q", got)
	}
}

func TestScanUpstreamStates(t *testing.T) {
	t.Run("unpushed", func(t *testing.T) {
		repo := scanRepo(t)
		diverged(t, repo, "feature")
		out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
		if got := col(t, out, "branches", "feature", 2); got != "unpushed" {
			t.Errorf("class = %q", got)
		}
		if got := col(t, out, "branches", "feature", 3); got != "none" {
			t.Errorf("upstream = %q", got)
		}
	})
	t.Run("gone", func(t *testing.T) {
		repo := scanRepo(t)
		diverged(t, repo, "feature")
		gateGit(t, repo, "remote", "add", "origin", "https://example.invalid/repo.git")
		gateGit(t, repo, "update-ref", "refs/remotes/origin/feature", "feature")
		gateGit(t, repo, "branch", "--set-upstream-to=refs/remotes/origin/feature", "feature", "-q")
		gateGit(t, repo, "update-ref", "-d", "refs/remotes/origin/feature")
		out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
		if got := col(t, out, "branches", "feature", 2); got != "gone" {
			t.Errorf("class = %q", got)
		}
		if got := col(t, out, "branches", "feature", 3); got != "gone" {
			t.Errorf("upstream = %q", got)
		}
	})
	t.Run("tracking", func(t *testing.T) {
		repo := scanRepo(t)
		diverged(t, repo, "feature")
		gateGit(t, repo, "remote", "add", "origin", "https://example.invalid/repo.git")
		gateGit(t, repo, "update-ref", "refs/remotes/origin/feature", "feature")
		gateGit(t, repo, "branch", "--set-upstream-to=refs/remotes/origin/feature", "feature", "-q")
		out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
		if got := col(t, out, "branches", "feature", 2); got != "tracking" {
			t.Errorf("class = %q", got)
		}
		if got := col(t, out, "branches", "feature", 3); got != "origin/feature" {
			t.Errorf("upstream = %q", got)
		}
	})
}

// One value per distinct cause: only some of these mean "there was nothing to
// look up". Every one of them still emits one row per branch and per worktree —
// the classifier's contract is met without the PR column.
func TestScanGHStates(t *testing.T) {
	t.Run("no-remote", func(t *testing.T) {
		scanRepo(t)
		res := scan(t, "--default", "main", "--no-fetch")
		if got := field(res.stdout, "remote"); got != "none" {
			t.Errorf("remote = %q", got)
		}
		if got := field(res.stdout, "gh"); got != "no-remote" {
			t.Errorf("gh = %q", got)
		}
	})
	t.Run("gh-missing", func(t *testing.T) {
		repo := scanRepo(t)
		gateGit(t, repo, "remote", "add", "origin", "https://example.invalid/repo.git")
		noGH(t)
		res := scan(t, "--default", "main", "--no-fetch")
		if res.code != 0 {
			t.Fatalf("exit = %d", res.code)
		}
		if got := field(res.stdout, "gh"); got != "gh-missing" {
			t.Errorf("gh = %q", got)
		}
		if got := col(t, res.stdout, "branches", "main", 2); got != "protected" {
			t.Errorf("a degraded PR lookup cost a branch row: %q", got)
		}
		if got := col(t, res.stdout, "worktrees", "main", 3); got != "primary" {
			t.Errorf("a degraded PR lookup cost a worktree row: %q", got)
		}
	})
	t.Run("skipped", func(t *testing.T) {
		repo := scanRepo(t)
		gateGit(t, repo, "remote", "add", "origin", "https://example.invalid/repo.git")
		fakeGH(t, "[]")
		if got := field(scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout, "gh"); got != "skipped" {
			t.Errorf("gh = %q", got)
		}
	})
}

func TestScanPRClasses(t *testing.T) {
	cases := []struct {
		name, prs, class, pr string
		goneUpstream         bool
	}{
		{
			name:  "merged PR beats gone",
			prs:   `[{"headRefName":"feature","number":42,"state":"MERGED"}]`,
			class: "merged-pr", pr: "merged#42", goneUpstream: true,
		},
		{
			name:  "open PR",
			prs:   `[{"headRefName":"feature","number":7,"state":"OPEN"}]`,
			class: "open-pr", pr: "open#7",
		},
		{
			name:  "a standalone closed PR",
			prs:   `[{"headRefName":"feature","number":5,"state":"CLOSED"}]`,
			class: "closed-pr", pr: "closed#5",
		},
		{
			// History a cleanup decision should see, not just whichever the API
			// returned first.
			name:  "the newest PR wins",
			prs:   `[{"headRefName":"feature","number":3,"state":"CLOSED"},{"headRefName":"feature","number":9,"state":"OPEN"}]`,
			class: "open-pr", pr: "open#9",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := scanRepo(t)
			diverged(t, repo, "feature")
			gateGit(t, repo, "remote", "add", "origin", "https://example.invalid/repo.git")
			if c.goneUpstream {
				gateGit(t, repo, "update-ref", "refs/remotes/origin/feature", "feature")
				gateGit(t, repo, "branch", "--set-upstream-to=refs/remotes/origin/feature", "feature", "-q")
				gateGit(t, repo, "update-ref", "-d", "refs/remotes/origin/feature")
			}
			fakeGH(t, c.prs)
			res := scan(t, "--default", "main", "--no-fetch")
			if got := field(res.stdout, "gh"); got != "ok" {
				t.Fatalf("gh = %q", got)
			}
			if got := col(t, res.stdout, "branches", "feature", 2); got != c.class {
				t.Errorf("class = %q, want %q", got, c.class)
			}
			if got := col(t, res.stdout, "branches", "feature", 5); got != c.pr {
				t.Errorf("pr = %q, want %q", got, c.pr)
			}
		})
	}
}

// Guards against a reused or coincidentally-matching branch name: a `merged`
// match is trusted only when its head commit is this branch's own tip, or an
// ancestor of it.
func TestScanAMergedPRWithAMismatchedHeadOidIsNotTrusted(t *testing.T) {
	repo := scanRepo(t)
	diverged(t, repo, "feature")
	badOID := gateGit(t, repo, "rev-parse", "main")
	gateGit(t, repo, "remote", "add", "origin", "https://example.invalid/repo.git")
	gateGit(t, repo, "update-ref", "refs/remotes/origin/feature", "feature")
	gateGit(t, repo, "branch", "--set-upstream-to=refs/remotes/origin/feature", "feature", "-q")
	gateGit(t, repo, "update-ref", "-d", "refs/remotes/origin/feature")
	fakeGH(t, `[{"headRefName":"feature","number":42,"state":"MERGED","headRefOid":"`+badOID+`"}]`)

	res := scan(t, "--default", "main", "--no-fetch")
	if got := field(res.stdout, "gh"); got != "ok" {
		t.Fatalf("gh = %q", got)
	}
	if got := col(t, res.stdout, "branches", "feature", 2); got != "gone" {
		t.Errorf("class = %q, want gone — the PR's head is not this branch's", got)
	}
}

func TestScanAMergedPRWithTheBranchesOwnTipIsTrusted(t *testing.T) {
	repo := scanRepo(t)
	diverged(t, repo, "feature")
	gateGit(t, repo, "remote", "add", "origin", "https://example.invalid/repo.git")
	tip := gateGit(t, repo, "rev-parse", "feature")
	fakeGH(t, `[{"headRefName":"feature","number":42,"state":"MERGED","headRefOid":"`+tip+`"}]`)
	if got := col(t, scan(t, "--default", "main", "--no-fetch").stdout, "branches", "feature", 2); got != "merged-pr" {
		t.Errorf("class = %q", got)
	}
}

// A branch name may legally contain a comma, so the protected set is never
// tested by splitting a joined string.
func TestScanAProtectedBranchNameContainingAComma(t *testing.T) {
	repo := scanRepo(t)
	gateGit(t, repo, "branch", "release,2026")
	diverged(t, repo, "feature")
	gateGit(t, repo, "checkout", "-q", "release,2026")
	gateGit(t, repo, "merge", "-q", "--no-ff", "feature")
	gateGit(t, repo, "checkout", "-q", "main")

	res := scan(t, "--default", "release,2026", "--no-fetch", "--no-gh")
	if got := field(res.stdout, "default"); got != "release,2026" {
		t.Errorf("default = %q", got)
	}
	if got := col(t, res.stdout, "branches", "feature", 2); got != "merged" {
		t.Errorf("class = %q", got)
	}
	if got := col(t, res.stdout, "branches", "feature", 4); got != "release,2026" {
		t.Errorf("merged_into = %q", got)
	}
}

func TestScanWorktrees(t *testing.T) {
	repo := scanRepo(t)
	wt := filepath.Join(t.TempDir(), "wt-feature")
	gateGit(t, repo, "worktree", "add", "-q", "-b", "feature", wt, "main")
	t.Cleanup(func() { _ = os.RemoveAll(wt) })

	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "worktrees", "main", 3); got != "primary" {
		t.Errorf("primary origin = %q", got)
	}
	if got := col(t, out, "worktrees", "feature", 3); got != "linked" {
		t.Errorf("linked origin = %q", got)
	}
	if got := col(t, out, "worktrees", "feature", 4); got != "yes" {
		t.Errorf("clean = %q", got)
	}

	put(t, wt, "dirty.txt", "dirty\n")
	out = scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "worktrees", "feature", 4); got != "no" {
		t.Errorf("clean = %q, want no", got)
	}

	// mkit's own scratch never makes a worktree dirty — cleanup demotes a
	// `merged` branch to ask on that key, then offers `--force` with its
	// "discards uncommitted work" sentence for a log mkit wrote.
	if err := os.Remove(filepath.Join(wt, "dirty.txt")); err != nil {
		t.Fatal(err)
	}
	put(t, wt, ".mkit/review-1/gate-lint.log", "noise\n")
	out = scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "worktrees", "feature", 4); got != "yes" {
		t.Errorf("clean = %q, want yes — the scratch is excluded", got)
	}
}

func TestScanAWorktreeUnderClaudeWorktreesIsClaudeCode(t *testing.T) {
	repo := scanRepo(t)
	wt := filepath.Join(t.TempDir(), ".claude", "worktrees", "feature")
	if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
		t.Fatal(err)
	}
	gateGit(t, repo, "worktree", "add", "-q", "-b", "feature", wt, "main")
	t.Cleanup(func() { _ = os.RemoveAll(wt) })
	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "worktrees", "feature", 3); got != "claude-code" {
		t.Errorf("origin = %q", got)
	}
}

// An unreadable worktree is not a proven-clean one.
func TestScanABrokenWorktreeReportsError(t *testing.T) {
	repo := scanRepo(t)
	wt := filepath.Join(t.TempDir(), "wt-broken")
	gateGit(t, repo, "worktree", "add", "-q", "-b", "feature", wt, "main")
	t.Cleanup(func() { _ = os.RemoveAll(wt) })
	// The linked worktree's gitdir-pointer file.
	if err := os.Remove(filepath.Join(wt, ".git")); err != nil {
		t.Fatal(err)
	}
	out := scan(t, "--default", "main", "--no-fetch", "--no-gh").stdout
	if got := col(t, out, "worktrees", "feature", 4); got != "error" {
		t.Errorf("clean = %q, want error", got)
	}
}

func TestScanJSON(t *testing.T) {
	repo := scanRepo(t)
	diverged(t, repo, "feature")
	res := scan(t, "--json", "--default", "main", "--no-fetch", "--no-gh")
	var got struct {
		Default   string   `json:"default"`
		Protected []string `json:"protected"`
		GH        string   `json:"gh"`
		Branches  []struct {
			Name     string `json:"name"`
			Class    string `json:"class"`
			Upstream string `json:"upstream"`
		} `json:"branches"`
		Worktrees []struct {
			Origin string `json:"origin"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, res.stdout)
	}
	if got.Default != "main" || got.GH != "skipped" || len(got.Protected) != 1 {
		t.Errorf("%+v", got)
	}
	if len(got.Branches) != 2 {
		t.Fatalf("branches = %+v", got.Branches)
	}
	if len(got.Worktrees) != 1 || got.Worktrees[0].Origin != "primary" {
		t.Errorf("worktrees = %+v", got.Worktrees)
	}
}
