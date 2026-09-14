package worklog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
)

// newRepo builds a throwaway work tree inside the test's own temp directory, so
// nothing here can reach a developer's real state.
func newRepo(t *testing.T) *gitrepo.Repo {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main", ".")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestAppendThenShowRoundTrips(t *testing.T) {
	log := Open(newRepo(t), "")

	if err := log.Append(Record{Step: "commit", Gist: "split into three", Artifact: "abc..def"}); err != nil {
		t.Fatal(err)
	}
	if err := log.Append(Record{Step: "review", Gist: "two findings", Assumptions: []string{"goal from branch name"}}); err != nil {
		t.Fatal(err)
	}

	recs, err := log.Show(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	// Oldest first: the last element is the most recent thing that happened.
	if recs[0].Step != "commit" || recs[1].Step != "review" {
		t.Fatalf("order: %s then %s", recs[0].Step, recs[1].Step)
	}
	if recs[0].Artifact != "abc..def" || recs[1].Assumptions[0] != "goal from branch name" {
		t.Errorf("fields did not survive: %+v %+v", recs[0], recs[1])
	}
	if recs[0].TS == "" || recs[0].Schema != Schema || recs[0].Head == "" {
		t.Errorf("binary-set fields missing: %+v", recs[0])
	}
}

// Contract rule 4 in its most literal form: being first on a branch is the normal
// case for an entry-capable step, so an unwritten log is zero records, not an error
// the caller has to handle before it can carry on.
func TestShowOnMissingFileIsEmptyAndNotAnError(t *testing.T) {
	recs, err := Open(newRepo(t), "").Show(Query{})
	if err != nil {
		t.Fatalf("missing log reported an error: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("got %d records from a missing log", len(recs))
	}
}

func TestShowFiltersAndLimits(t *testing.T) {
	log := Open(newRepo(t), "")
	for _, s := range []string{"spec", "commit", "review", "commit"} {
		if err := log.Append(Record{Step: s, Gist: s}); err != nil {
			t.Fatal(err)
		}
	}

	recs, _ := log.Show(Query{Steps: []string{"commit"}})
	if len(recs) != 2 {
		t.Fatalf("--step commit gave %d records, want 2", len(recs))
	}
	// The limit is applied after the filter, and keeps the newest.
	recs, _ = log.Show(Query{Limit: 1})
	if len(recs) != 1 || recs[0].Step != "commit" {
		t.Fatalf("--limit 1 gave %+v, want the newest record", recs)
	}
}

func TestAppendRejectsUnknownStepAndEmptyGist(t *testing.T) {
	log := Open(newRepo(t), "")
	if err := log.Append(Record{Step: "reviw", Gist: "typo'd step"}); err == nil {
		t.Error("an unknown step was written; a typo'd step is a record every reader skips")
	}
	if err := log.Append(Record{Step: "review", Gist: "  "}); err == nil {
		t.Error("a record with no gist was written")
	}
	if _, err := os.Stat(log.Path()); err == nil {
		t.Error("a rejected record still created the log file")
	}
}

// The mapping show and append both go through. `/` is the hazard that would turn a
// log into a directory; the rest is about staying injective, so two branches can
// never land in one file.
func TestFileNameEscapesEveryHazard(t *testing.T) {
	// Every name ends with a digest of the exact branch, so the readable part is
	// asserted as a prefix and the digest as the thing that makes it unique.
	for _, tc := range []struct{ branch, readable string }{
		{"main", "main-"},
		{"feature/a/b", "feature%2Fa%2Fb-"},
		{"fix up", "fix%20up-"},
		{".hidden", "%2Ehidden-"},
		{"a.b-c_d", "a.b-c_d-"},
	} {
		got := FileName(tc.branch)
		if !strings.HasPrefix(got, tc.readable) || !strings.HasSuffix(got, ".jsonl") {
			t.Errorf("FileName(%q) = %q, want %q<digest>.jsonl", tc.branch, got, tc.readable)
		}
	}
	if FileName("a/b") == FileName("a-b") {
		t.Error("two branches map to one file")
	}
	if strings.ContainsRune(FileName("feature/x"), '/') {
		t.Error("a slash survived, so the log would be a directory")
	}
	// macOS is case-insensitive, so these would be one file without the digest.
	if strings.EqualFold(FileName("JIRA-123"), FileName("jira-123")) {
		t.Error("two branches differing only by case map to one file on a case-insensitive filesystem")
	}
	if FileName("JIRA-123") == FileName("Jira-123") {
		t.Error("two uppercase variants map to one file")
	}
	if !strings.HasPrefix(FileName("JIRA-123"), "JIRA-123-") {
		t.Errorf("the readable name did not survive: %s", FileName("JIRA-123"))
	}
	// The digest is unconditional precisely so it cannot be spelled by another
	// branch: a suffix added only to uppercase names is still ordinary branch text.
	spelled := strings.TrimSuffix(FileName("FOO"), ".jsonl") // a legal branch name
	if strings.EqualFold(FileName(spelled), FileName("FOO")) {
		t.Errorf("a branch named %q collides with FOO's log", spelled)
	}
	// A detached log lives in a namespace no branch name can reach: `~` is
	// forbidden in a ref, and escaping makes a literal one distinct anyway.
	// `~` is forbidden in a ref name, and a caller passing one literally escapes
	// it — so nothing a branch can be called reaches a detached log's file.
	if !strings.HasPrefix(FileName(detachedPrefix+"abc"), "detached%7Eabc-") {
		t.Errorf("detached log file = %s", FileName(detachedPrefix+"abc"))
	}
	if FileName("detached-abc") == FileName(detachedPrefix+"abc") {
		t.Error("a branch could collide with a detached log")
	}
}

// A record written against another branch must carry that branch's tip, not
// whatever happens to be checked out.
func TestAppendAgainstAnotherBranchRecordsThatBranchesHead(t *testing.T) {
	repo := newRepo(t)
	git(t, repo.Toplevel, "branch", "other")
	git(t, repo.Toplevel, "commit", "-q", "--allow-empty", "-m", "second")
	mainHead := git(t, repo.Toplevel, "rev-parse", "HEAD")
	otherHead := git(t, repo.Toplevel, "rev-parse", "other")

	log := Open(repo, "other")
	if err := log.Append(Record{Step: "spec", Gist: "written from main"}); err != nil {
		t.Fatal(err)
	}
	recs, _ := log.Show(Query{})
	if recs[0].Head != otherHead {
		t.Errorf("head = %s, want other's tip %s (current HEAD is %s)", recs[0].Head, otherHead, mainHead)
	}
}

func TestPathIsPerWorkTree(t *testing.T) {
	repo := newRepo(t)
	log := Open(repo, "main")
	want := filepath.Join(repo.Toplevel, ".mkit", "work", FileName("main"))
	if log.Path() != want {
		t.Errorf("Path() = %q, want %q", log.Path(), want)
	}
}

// A detached HEAD has no branch, and a session there must still record somewhere
// rather than nowhere.
func TestDetachedHeadGetsItsOwnLog(t *testing.T) {
	repo := newRepo(t)
	git(t, repo.Toplevel, "checkout", "-q", "--detach")
	log := Open(repo, "")
	if !strings.HasPrefix(log.Branch(), detachedPrefix) {
		t.Fatalf("detached HEAD named its log %q", log.Branch())
	}
	if err := log.Append(Record{Step: "review", Gist: "detached"}); err != nil {
		t.Fatal(err)
	}
}

// One bad line must not cost a reader the records around it.
func TestShowSkipsAMalformedLine(t *testing.T) {
	log := Open(newRepo(t), "")
	if err := log.Append(Record{Step: "spec", Gist: "first"}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(log.Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("{not json\n")
	_ = f.Close()
	if err := log.Append(Record{Step: "review", Gist: "third"}); err != nil {
		t.Fatal(err)
	}

	recs, err := log.Show(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records around a malformed line, want 2", len(recs))
	}
}

// The property, not the rule text: `run-open.sh` writes `.mkit/*` into the common
// dir's info/exclude before the first write, so the worklog is already covered and
// this milestone must not add a second rule.
func TestWorklogIsIgnoredByTheExistingRule(t *testing.T) {
	repo := newRepo(t)
	exclude := filepath.Join(repo.Toplevel, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exclude, []byte(".mkit/*\n!.mkit/config.toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	log := Open(repo, "main")
	if err := log.Append(Record{Step: "commit", Gist: "x"}); err != nil {
		t.Fatal(err)
	}
	rel, _ := filepath.Rel(repo.Toplevel, log.Path())
	if ignored, _ := repo.Ignored(rel); !ignored {
		t.Errorf("%s is not ignored — run artefacts would be committed", rel)
	}
}

// git's own limit on a ref name is far above a filename's, and the failure is not a
// long name — it is ENAMETOOLONG from open(2), so the branch cannot record at all.
func TestFileNameStaysWithinNameMax(t *testing.T) {
	long := strings.Repeat("a", 300)
	if got := len(FileName(long)); got > nameMax {
		t.Errorf("a 300-char branch produced a %d-byte filename, over the %d limit", got, nameMax)
	}
	// Escapes triple the byte cost; the cut must still land under the limit, and
	// must never split one `%XX` into a fragment.
	slashes := strings.Repeat("a/", 200)
	name := FileName(slashes)
	if len(name) > nameMax {
		t.Errorf("escaped branch produced a %d-byte filename, over the %d limit", len(name), nameMax)
	}
	if i := strings.LastIndex(name, "%"); i >= 0 && !strings.HasPrefix(name[i:], "%2F") {
		t.Errorf("a %%XX escape was split by the truncation: %s", name)
	}

	// Truncation costs readability, never identity: two branches sharing a prefix
	// past the cut are still two files, because the digest is over the whole name.
	if FileName(long) == FileName(long+"b") {
		t.Error("two branches differing only past the truncation point share a file")
	}
	if !strings.HasPrefix(FileName(long), "aaaa") {
		t.Errorf("the readable part did not survive the cut: %s", FileName(long))
	}
}

// `mkit work append` can be the first thing ever to write under `.mkit/` — no skill
// has to have opened a run directory first. Unignored, that scratch makes
// `git worktree remove` refuse and puts the worklog in reach of `git add -A`.
func TestAppendEstablishesTheIgnoreRule(t *testing.T) {
	payload, err := filepath.Abs(filepath.Join("..", "..", "..", "plugin"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(payload, "scripts", "lib", "common.sh")); err != nil {
		t.Skipf("payload not in reach: %v", err)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", payload)

	repo := newRepo(t)
	log := Open(repo, "")
	if err := log.Append(Record{Step: "spec", Gist: "the plan"}); err != nil {
		t.Fatal(err)
	}

	rel, err := filepath.Rel(repo.Toplevel, log.Path())
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "check-ignore", "-q", rel)
	cmd.Dir = repo.Toplevel
	if err := cmd.Run(); err != nil {
		t.Errorf("the worklog it just created is not ignored: %s", rel)
	}
}

// ...and an append must still succeed where the rule cannot be established, which is
// the normal case in a worktree-isolated session: the write lands in the main
// checkout's `.git/info/exclude`, out of reach. Contract rule 4 — a recorded fact is
// an input, never a permission — so this must never become an error.
func TestAppendSucceedsWithNoPayloadInReach(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("MKIT_PLUGIN_ROOT", "")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "no-such-config"))

	log := Open(newRepo(t), "")
	if err := log.Append(Record{Step: "spec", Gist: "the plan"}); err != nil {
		t.Fatalf("append refused over an ignore rule it could not write: %v", err)
	}
	recs, err := log.Show(Query{})
	if err != nil || len(recs) != 1 {
		t.Fatalf("the record did not land: %d records, err %v", len(recs), err)
	}
}
