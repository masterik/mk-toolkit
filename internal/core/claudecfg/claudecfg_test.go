package claudecfg

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// git runs against a throwaway repo under the test's temp dir, always with -C,
// so no command can land in the checkout the tests run from.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func realpath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRoots(t *testing.T) {
	tmp := realpath(t, t.TempDir())
	main := filepath.Join(tmp, "app")
	if err := os.MkdirAll(filepath.Join(main, "pkg", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, main, "init", "-q")
	git(t, main, "commit", "-q", "--allow-empty", "-m", "x")
	wt := filepath.Join(tmp, "app-wt")
	git(t, main, "worktree", "add", "-q", "-b", "feat", wt)
	plain := filepath.Join(tmp, "notes")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(tmp, "removed")

	got := Roots([]string{
		filepath.Join(main, "pkg", "sub"), main, wt, plain, gone, "",
	})
	want := []string{main, wt, plain}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Roots = %v, want %v (subdir folds into its checkout, a linked worktree stays its own, a non-repo keeps its path, a removed path is skipped)", got, want)
	}
}

func TestLoadTagsFilesAndExtractsKeys(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "claude")
	root := filepath.Join(tmp, "app")
	write(t, filepath.Join(home, "settings.json"), `{
	  "sandbox": {"excludedCommands": ["docker *"],
	    "network": {"allowedDomains": ["github.com", "*.golang.org"]},
	    "filesystem": {"allowWrite": ["~/.mkit"]}},
	  "permissions": {"allow": ["Bash(go test *)"], "additionalDirectories": ["/data"]},
	  "autoMode": {"allow": ["$defaults", "x"], "soft_deny": ["y"]},
	  "env": {"A": "1", "N": 2}
	}`)
	write(t, filepath.Join(home, "CLAUDE.md"), "rules")
	write(t, filepath.Join(root, ".claude", "settings.json"), `{"permissions":{"deny":["Read(.env)"]}}`)
	write(t, filepath.Join(root, ".claude", "settings.local.json"), `{"sandbox":{"filesystem":{"allowWrite":["build"]}}}`)

	res := Load(home, "/home/u", []string{root})
	if !res.ClaudeMD {
		t.Error("CLAUDE.md exists but ClaudeMD is false")
	}
	u := res.User
	if u == nil || u.Scope != ScopeUser || u.Path != filepath.Join(home, "settings.json") {
		t.Fatalf("user file = %+v", u)
	}
	if !reflect.DeepEqual(u.AllowedDomains, []string{"github.com", "*.golang.org"}) ||
		!reflect.DeepEqual(u.AllowWrite, []string{"~/.mkit"}) ||
		!reflect.DeepEqual(u.ExcludedCommands, []string{"docker *"}) ||
		!reflect.DeepEqual(u.AdditionalDirs, []string{"/data"}) ||
		!reflect.DeepEqual(u.AutoModeSoftDeny, []string{"y"}) ||
		!reflect.DeepEqual(u.Env, []string{"A", "N"}) {
		t.Errorf("extracted = %+v", u)
	}
	if len(res.Projects) != 1 || len(res.Projects[0].Files) != 2 {
		t.Fatalf("projects = %+v", res.Projects)
	}
	p, l := res.Projects[0].Files[0], res.Projects[0].Files[1]
	if p.Scope != ScopeProject || !reflect.DeepEqual(p.Deny, []string{"Read(.env)"}) ||
		l.Scope != ScopeLocal || !reflect.DeepEqual(l.AllowWrite, []string{"build"}) {
		t.Errorf("project files = %+v / %+v", p, l)
	}
	if len(res.Unreadable) != 0 {
		t.Errorf("unreadable = %v", res.Unreadable)
	}
}

func TestAbsentIsNoEntryButUnparseableIsUnreadable(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "claude")
	root := filepath.Join(tmp, "app")
	write(t, filepath.Join(root, ".claude", "settings.json"), `{not json`)

	res := Load(home, "/home/u", []string{root})
	if res.User != nil || res.ClaudeMD {
		t.Errorf("absent user settings must be no entry: %+v", res.User)
	}
	bad := filepath.Join(root, ".claude", "settings.json")
	if !reflect.DeepEqual(res.Unreadable, []string{bad}) {
		t.Errorf("unreadable = %v, want [%s]", res.Unreadable, bad)
	}
	if len(res.Projects[0].Files) != 0 {
		t.Errorf("an unparseable file must not become an empty config: %+v", res.Projects[0].Files)
	}
}

func TestUnreadableWhenDirectoryNotFile(t *testing.T) {
	// A read that fails for any reason but "not there" — a sandbox denial is
	// EPERM — is unreadable, not absent. A directory in the file's place fails
	// the same way without needing a permission change.
	tmp := t.TempDir()
	home := filepath.Join(tmp, "claude")
	if err := os.MkdirAll(filepath.Join(home, "settings.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	res := Load(home, "/home/u", nil)
	if len(res.Unreadable) != 1 {
		t.Errorf("unreadable = %v", res.Unreadable)
	}
}

func TestUnknownKeysAreNamed(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "claude")
	write(t, filepath.Join(home, "settings.json"), `{
	  "sandbox": {"excludedComands": [], "network": {"allowedDomain": ["a.com"]}},
	  "permissions": {"allow": "Bash(x)"},
	  "autoMode": {"soft_deny": ["ok"], "nope": 1}
	}`)
	got := Load(home, "/h", nil).User.UnknownKeys
	want := []string{
		"sandbox.excludedComands",
		"sandbox.network.allowedDomain",
		"permissions.allow (not a list of strings)",
		"autoMode.nope",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unknown = %v\nwant      %v", got, want)
	}
}

func fixture() *Result {
	return &Result{
		Home:     "/home/u/.claude",
		UserHome: "/home/u",
		User: &File{Path: "/home/u/.claude/settings.json", Scope: ScopeUser,
			AllowedDomains: []string{"github.com", "*.golang.org"},
			AllowWrite:     []string{"~/.mkit", "~/.claude/plugins/data", "~/.claude/plugins", "relative"},
			AdditionalDirs: []string{"/data"}},
		Projects: []Project{{Root: "/r/app", Files: []File{
			{Path: "/r/app/.claude/settings.local.json", Scope: ScopeLocal, AllowWrite: []string{"build"}, AllowedDomains: []string{"internal.test"}},
		}}},
	}
}

func TestJudgeHost(t *testing.T) {
	r := fixture()
	for _, tc := range []struct {
		target, want string
		roots        []string
	}{
		{"network-outbound github.com:443", "github.com", nil},
		{"network-outbound GitHub.com:443", "github.com", nil},
		{"network-outbound proxy.golang.org:443", "*.golang.org", nil},
		{"network-outbound golang.org:443", "", nil}, // the wildcard is for subdomains
		{"network-outbound evilgithub.com:443", "", nil},
		{"network-outbound internal.test:80", "internal.test", []string{"/r/app"}},
		{"network-outbound internal.test:80", "", []string{"/r/other"}}, // another project's file does not apply
	} {
		v := r.Judge(tc.target, tc.roots)
		got := ""
		if v.Covered != nil {
			got = v.Covered.Entry
		}
		if got != tc.want {
			t.Errorf("%s roots=%v: covered by %q, want %q", tc.target, tc.roots, got, tc.want)
		}
	}
}

func TestJudgePath(t *testing.T) {
	r := fixture()
	for _, tc := range []struct {
		target    string
		roots     []string
		entry     string
		protected bool
	}{
		{"file-write-create /home/u/.mkit/sandbox-audit.md", nil, "~/.mkit", false},
		{"file-write-create ~/.mkit/x", nil, "~/.mkit", false}, // the scan normalizes the home prefix to ~
		{"file-write-create /home/u/.mkitx/x", nil, "", false}, // containment, not string prefix
		{"file-write-create /data/out/f", nil, "/data", false},
		{"file-write-create /data", nil, "", false}, // additionalDirectories does not grant the directory's own creation
		{"file-write-create /r/app/build/o", []string{"/r/app"}, "build", false},
		{"file-write-create /r/app/build/o", nil, "", false},
		{"file-write-create /elsewhere/f", nil, "", false},
		// Protected: an entry reaching it exists (~/.claude/plugins) and is inert.
		{"file-write-create /home/u/.claude/plugins/data/x", nil, "~/.claude/plugins/data", false}, // the carve-out
		{"file-write-create /home/u/.claude/plugins/cache/x", nil, "", true},
		{"file-write-create /home/u/.claude/todos/x", nil, "", true},
		{"file-read-data /home/u/.mkit/x", nil, "", false}, // allowWrite does not govern a read
		{"file-write-create /home/u/.claude/settings.json", nil, "", true},
		{"file-write-create /r/app/.claude/settings.local.json", []string{"/r/app"}, "", true},
		{"file-write-create /r/app/.claude/hooks/pre.sh", []string{"/r/app"}, "", true},
		{"file-write-create /r/app/.claude/skills/x/SKILL.md", []string{"/r/app"}, "", true},
		// A name that merely resembles a protected one is not protected.
		{"file-write-create /r/app/.claude-notes/x", []string{"/r/app"}, "", false},
	} {
		v := r.Judge(tc.target, tc.roots)
		got := ""
		if v.Covered != nil {
			got = v.Covered.Entry
		}
		if got != tc.entry || v.Protected != tc.protected {
			t.Errorf("%s: covered=%q protected=%v, want %q / %v", tc.target, got, v.Protected, tc.entry, tc.protected)
		}
		if v.Protected && v.Covered != nil {
			t.Errorf("%s: protected and covered at once", tc.target)
		}
	}
	if !strings.Contains(fixture().User.AllowWrite[2], "plugins") {
		t.Fatal("fixture lost its inert entry")
	}
}

func TestJudgeIgnoresUnnamedTargets(t *testing.T) {
	r := fixture()
	if v := r.Judge("EPERM: operation not permitted", nil); v.Covered != nil || v.Protected {
		t.Errorf("a target that names neither host nor path judged: %+v", v)
	}
}

func TestNullIsMalformedNotEmpty(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "claude")
	write(t, filepath.Join(home, "settings.json"), `null`)
	res := Load(home, "/h", nil)
	if res.User != nil || len(res.Unreadable) != 1 {
		t.Errorf("top-level null: user=%+v unreadable=%v", res.User, res.Unreadable)
	}
	write(t, filepath.Join(home, "settings.json"), `{"sandbox":null,"env":null,"permissions":{"allow":null}}`)
	got := Load(home, "/h", nil).User.UnknownKeys
	want := []string{"sandbox (not an object)", "env (not an object)", "permissions.allow (not a list of strings)"}
	if len(got) != 3 {
		t.Errorf("unknown = %v, want %v", got, want)
	}
}

func TestEnvValuesAreNeverKept(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "claude")
	write(t, filepath.Join(home, "settings.json"), `{"env":{"TOKEN":"s3cret"}}`)
	f := Load(home, "/h", nil).User
	if !reflect.DeepEqual(f.Env, []string{"TOKEN"}) {
		t.Errorf("env = %v", f.Env)
	}
}
