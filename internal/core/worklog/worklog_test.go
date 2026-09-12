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
	for _, tc := range []struct{ branch, want string }{
		{"main", "main.jsonl"},
		{"feature/a/b", "feature%2Fa%2Fb.jsonl"},
		{"fix up", "fix%20up.jsonl"},
		{".hidden", "%2Ehidden.jsonl"},
		{"a.b-c_d", "a.b-c_d.jsonl"},
	} {
		if got := FileName(tc.branch); got != tc.want {
			t.Errorf("FileName(%q) = %q, want %q", tc.branch, got, tc.want)
		}
	}
	if FileName("a/b") == FileName("a-b") {
		t.Error("two branches map to one file")
	}
	if strings.ContainsRune(FileName("feature/x"), '/') {
		t.Error("a slash survived, so the log would be a directory")
	}
}

func TestPathIsPerWorkTree(t *testing.T) {
	repo := newRepo(t)
	log := Open(repo, "main")
	want := filepath.Join(repo.Toplevel, ".mkit", "work", "main.jsonl")
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
	if !strings.HasPrefix(log.Branch(), "detached-") {
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
