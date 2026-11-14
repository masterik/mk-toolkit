package userdir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The plugin's facts.sh cannot call the binary (ADR 0006: the binary is optional),
// so it carries its own copy of Remedy's sentence. Two producers of one sentence
// stay one wording only if something compares them: this runs facts.sh and wants
// Remedy() verbatim in its `notes:` entry.
func TestFactsShNotesCarryRemedyVerbatim(t *testing.T) {
	facts, err := filepath.Abs("../../../plugin/skills/_shared/scripts/facts.sh")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	// A regular file as an ancestor: nothing can be created under it, sandbox or not.
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_CEILING_DIRECTORIES=" + tmp}
	git := exec.Command("git", "init", "-q", "-b", "main", repo)
	git.Env = append(os.Environ(), env...)
	if out, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	for _, tc := range []struct{ name, mkitHome, home string }{
		{"plain path", filepath.Join(blocker, "state"), ""},
		{"path the shell would split", filepath.Join(blocker, "my state's"), ""},
		{"no HOME, no MKIT_HOME", "", "-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MKIT_HOME", tc.mkitHome)
			if tc.home == "-" {
				t.Setenv("HOME", "")
			}
			cmd := exec.Command("bash", facts, "commit")
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), env...)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("facts.sh: %v\n%s", err, out)
			}
			want := "\n- user_dir_writable=no — " + Remedy() + ". "
			if !strings.Contains(string(out), want) {
				t.Errorf("facts.sh notes lack Remedy() verbatim\nwant substring: %q\ngot:\n%s", want, out)
			}
		})
	}
}
