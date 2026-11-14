// Package doctor reports what this machine and this repo will let mkit do.
//
// It reports and fixes nothing — every finding carries a remedy the user runs.
// That split is the point: the surfaces it replaced (`install.sh --status` and the
// SessionStart hook, both deleted in 0.15.0) could act, and acting is what made
// them two implementations of one invariant.
//
// **It cannot restore everything they did.** Doctor does not run unprompted at
// session start, and cannot report that `mkit` itself is absent — a missing binary
// cannot report on itself. Both were the hook's job and both are accepted losses
// (docs/backlog.md, "Accepted gaps").
//
// Every degradation sentence has one producer in the binary: the Go package that
// owns the surface — `userdir` for the user-scoped directory, `pluginroot` for a
// missing payload. Doctor relays, never re-words. The plugin cannot call the
// binary (ADR 0006), so its `facts.sh` prints the user-dir sentence itself; a test
// in `userdir` holds that copy to `userdir.Remedy()` word for word.
//
// The binary is optional (ADR 0006): the skills run without it, so nothing here
// is a precondition for them. Doctor checks what they compose (git, gh, bash,
// optional tools), the plugin's state, and what older releases left behind.
//
// Layering: returns data, never prints, never assumes a terminal.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/masterik/mk-toolkit/internal/buildinfo"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/pluginroot"
	"github.com/masterik/mk-toolkit/internal/core/userdir"
)

// Status is a check's verdict.
type Status string

const (
	// OK — verified working.
	OK Status = "ok"
	// Warn — degraded, but nothing is broken right now.
	Warn Status = "warn"
	// Fail — something the user asked for will not work.
	Fail Status = "fail"
	// Unknown — could not be determined, and the cause is worth saying.
	Unknown Status = "unknown"
)

// Check is one reported finding.
type Check struct {
	Group  string `json:"group"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	// Remedy is empty unless there is one that actually works. A check that
	// cannot name a working remedy says so in Detail rather than offering
	// configuration that changes nothing — the ADR 0002 rule.
	Remedy string `json:"remedy,omitempty"`
}

// Report is the whole diagnostic.
type Report struct {
	Checks []Check `json:"checks"`
}

// Counts summarises the report by status, for a one-line verdict.
func (r *Report) Counts() map[Status]int {
	c := map[Status]int{}
	for _, ch := range r.Checks {
		c[ch.Status]++
	}
	return c
}

// Worst returns the most severe status present.
func (r *Report) Worst() Status {
	c := r.Counts()
	switch {
	case c[Fail] > 0:
		return Fail
	case c[Warn] > 0:
		return Warn
	case c[Unknown] > 0:
		return Unknown
	}
	return OK
}

// Options selects what to check. A repo is optional: doctor is useful outside one,
// and simply omits the repo group.
type Options struct {
	Repo *gitrepo.Repo
}

// Run performs every check. It never returns an error: an undiagnosable machine is
// a report full of Unknown, which is more useful than a failed command.
func Run(opts Options) *Report {
	r := &Report{}
	toplevel := ""
	if opts.Repo != nil {
		toplevel = opts.Repo.Toplevel
	}

	root, rootErr := pluginroot.Find(toplevel)
	r.binary()
	r.payload(root, rootErr, toplevel)
	r.prerequisites()
	r.userDir()
	if opts.Repo != nil {
		r.repo(opts.Repo)
	} else {
		r.add(Check{Group: "repo", Name: "work tree", Status: Unknown,
			Detail: "not inside a git repository — the repo checks were skipped"})
	}
	r.orphans(opts.Repo)
	r.writableSet()
	r.allowlist(toplevel)
	return r
}

func (r *Report) add(c Check) { r.Checks = append(r.Checks, c) }

func (r *Report) binary() {
	r.add(Check{Group: "install", Name: "mkit binary", Status: OK,
		Detail: buildinfo.Version + " (" + buildinfo.Commit + ")"})
}

func (r *Report) payload(root *pluginroot.Root, err error, toplevel string) {
	if err != nil {
		r.add(Check{Group: "install", Name: "plugin payload", Status: Fail,
			Detail: "not found — the skills are unavailable",
			Remedy: pluginroot.Remedy()})
		// Enablement is an independent question — the harness's settings say
		// whether the plugin is switched on whether or not a checkout was found,
		// and "payload missing *and* not enabled" is a different report from
		// "payload missing". Skipping it here hid half the answer.
		r.enabled(toplevel)
		return
	}
	detail := root.Dir + " (" + root.Via + ")"
	if v := root.Version(); v != "" {
		detail = "payload " + v + " at " + detail
	}
	// The two channels version independently by design, so a mismatch is a
	// standing condition to report — never an error, and never something to
	// "fix" by pinning one to the other (ADR 0003).
	if v := root.Version(); v != "" && v != buildinfo.Version && buildinfo.Version != "dev" {
		r.add(Check{Group: "install", Name: "plugin payload", Status: Warn,
			Detail: detail + "; binary is " + buildinfo.Version +
				" — the binary is optional and ships over its own channel, so drift is expected, not broken",
			Remedy: "update whichever is behind: `brew upgrade mkit` for the binary, " +
				"`/plugin marketplace update masterik` for the payload"})
	} else {
		r.add(Check{Group: "install", Name: "plugin payload", Status: OK, Detail: detail})
	}

	if root.HasHooks() {
		r.add(Check{Group: "install", Name: "hook registration", Status: Fail,
			Detail: "the manifest declares a `hooks` key — the payload ships no hooks " +
				"since 0.15.0, and a Stop/SubagentStop hook is deliberately excluded",
			Remedy: "remove the `hooks` key from .claude-plugin/plugin.json"})
	} else {
		r.add(Check{Group: "install", Name: "hook registration", Status: OK,
			Detail: "no hooks declared, as intended since 0.15.0"})
	}

	r.enabled(toplevel)
}

// enabled reads the harness's own settings to say whether the plugin is switched
// on. Read-only: `~/.claude/settings.json` is sandbox-denied for writes, so this
// can never offer to fix it and says so.
func (r *Report) enabled(toplevel string) {
	found, where := false, ""
	for _, f := range settingsFiles(toplevel) {
		var s struct {
			EnabledPlugins map[string]bool `json:"enabledPlugins"`
		}
		if readJSON(f, &s) != nil {
			continue
		}
		for k, on := range s.EnabledPlugins {
			if on && strings.HasPrefix(k, pluginroot.PluginName+"@") {
				found, where = true, f
			}
		}
	}
	if found {
		r.add(Check{Group: "install", Name: "plugin enabled", Status: OK, Detail: where})
		return
	}
	r.add(Check{Group: "install", Name: "plugin enabled", Status: Warn,
		Detail: "no enabled `" + pluginroot.PluginName + "@…` entry in any readable settings file",
		Remedy: "enable it with `/plugin` — this is a human-run step: " +
			userSettingsFile() + " is sandbox-denied, so nothing here can write it"})
}

// tool is one prerequisite.
type tool struct {
	name   string
	group  string
	status Status // the status to report when it is missing
	what   string
	remedy string
}

// The prerequisite table. `mkit doctor` restoring this report is M7's value: since
// 0.15.0 nothing tells a user unprompted that a tool is missing. Before M5 it
// surfaced as a thinner `mkit facts` block or a degraded gate annotation; the
// binary has no half-capable mode, so today a missing tool surfaces only here.
var tools = []tool{
	{"git", "prerequisites", Fail, "every skill", ""},
	{"bash", "prerequisites", Fail, "the plugin's facts.sh/branch-status.sh and the gate commands", ""},
	{"gh", "prerequisites", Warn, "pr, finish, and cleanup's PR column", "brew install gh, then `gh auth login`"},
	{"rg", "optional", Warn, "faster searching; grep -E is used otherwise", "brew install ripgrep"},
	{"wt", "optional", Warn, "worktrunk-managed worktree teardown in finish/cleanup", "brew install worktrunk"},
	{"codex", "optional", Warn, "review's Codex reviewer", ""},
	{"coderabbit", "optional", Warn, "review's CodeRabbit reviewer", ""},
}

func (r *Report) prerequisites() {
	for _, t := range tools {
		path, err := exec.LookPath(t.name)
		if err == nil {
			if t.name == "bash" {
				r.add(bashCheck(path))
				continue
			}
			r.add(Check{Group: t.group, Name: t.name, Status: OK, Detail: path})
			continue
		}
		r.add(Check{Group: t.group, Name: t.name, Status: t.status,
			Detail: "not on PATH — needed for " + t.what, Remedy: t.remedy})
	}
	r.ghStack()
}

// bashCheck names the version: the plugin's scripts are written for bash 3.2
// (macOS /bin/bash) and up, so anything older fails them.
func bashCheck(path string) Check {
	out, err := exec.Command(path, "-c", `printf '%s' "$BASH_VERSION"`).Output()
	v := strings.TrimSpace(string(out))
	if err != nil || v == "" {
		return Check{Group: "prerequisites", Name: "bash", Status: Unknown,
			Detail: path + " — could not read its version"}
	}
	var major, minor int
	_, _ = fmt.Sscanf(v, "%d.%d", &major, &minor)
	if major < 3 || major == 3 && minor < 2 {
		return Check{Group: "prerequisites", Name: "bash", Status: Fail,
			Detail: "bash " + v + " at " + path + " — the plugin's scripts need 3.2 or later",
			Remedy: "brew install bash"}
	}
	return Check{Group: "prerequisites", Name: "bash", Status: OK, Detail: "bash " + v + " at " + path}
}

// hasExtension reports whether `gh extension list` output names repo exactly —
// a substring match would accept a fork such as github/gh-stack-fork.
func hasExtension(list, repo string) bool {
	for _, l := range strings.Split(list, "\n") {
		for _, f := range strings.Fields(l) {
			if f == repo {
				return true
			}
		}
	}
	return false
}

// ghStack reports the `gh stack` extension. It is a gh extension, not a PATH
// binary, so LookPath cannot see it; `gh extension list` is the question. Skipped
// when gh itself is missing — that row already says so.
func (r *Report) ghStack() {
	if _, err := exec.LookPath("gh"); err != nil {
		return
	}
	out, err := exec.Command("gh", "extension", "list").Output()
	if err == nil && hasExtension(string(out), "github/gh-stack") {
		r.add(Check{Group: "prerequisites", Name: "gh stack", Status: OK, Detail: "gh extension github/gh-stack"})
		return
	}
	r.add(Check{Group: "prerequisites", Name: "gh stack", Status: Warn,
		Detail: "gh extension not installed — needed for stacked PRs in pr and finish",
		Remedy: "gh extension install github/gh-stack"})
}

// userDirCheck is a constant because a check's name is its identity: a report
// whose rows rename themselves by branch cannot be diffed or matched against.
const userDirCheck = "user state dir"

// userDir reports both halves from `internal/core/userdir`: the probe (net-zero
// by construction) and the sentence (which must name creating the directory *and*
// granting it). One producer for each — until M5 both were fetched from the
// payload's lib/common.sh, and re-wording the sentence is how the original
// one-grant mistake survived three files.
func (r *Report) userDir() {
	dir := userdir.Dir()
	if dir == "" {
		r.add(Check{Group: "sandbox", Name: userDirCheck, Status: Warn,
			Detail: "no user-scoped directory: $HOME is unset and MKIT_HOME is not set",
			Remedy: userdir.Remedy()})
		return
	}
	if userdir.Writable() {
		r.add(Check{Group: "sandbox", Name: userDirCheck, Status: OK, Detail: dir + " is writable"})
		return
	}
	// Warn, not Fail: the binary writes nothing there today — its two files went
	// with the hook — and the one file in it, the sandbox-audit skill's ledger, is
	// optional: that skill prints the ledger instead. A later user-scoped write
	// would fail.
	r.add(Check{Group: "sandbox", Name: userDirCheck, Status: Warn,
		Detail: dir + " is not writable; the sandbox-audit ledger cannot be kept, and a later user-scoped write would fail",
		Remedy: userdir.Remedy()})
}

// repo names the work tree the leftover checks look in. Repo settings are prose
// under docs/agents/ now (ADR 0006), read by the skills; nothing here checks
// them, and an old `.mkit/config.toml` is a leftover (orphans.go).
func (r *Report) repo(repo *gitrepo.Repo) {
	r.add(Check{Group: "repo", Name: "work tree", Status: OK, Detail: repo.Toplevel})
}

// writableSet reports the one write location the skills use, by writing to it.
// Probed rather than inferred: the sandbox leaves the mode bits saying yes while
// denying the write, so `[ -w ]` and os.Stat both lie here. `<toplevel>/.mkit`
// is no longer one (#50): per-run files live under $TMPDIR.
func (r *Report) writableSet() {
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = os.TempDir()
	}
	r.probe("$TMPDIR", tmp, "per-run files, which die with the session",
		"nothing to grant — a denied $TMPDIR means the sandbox is configured unusually")
}

func (r *Report) probe(label, dir, what, cannotFix string) {
	// doctor reports and fixes nothing, and that includes leaving the filesystem
	// as it found it: a probe that creates the directory it asks about has
	// changed the answer to report it. Only directories this call created are removed, and only if still empty
	// — never one that was already there, and never one another process has since
	// written into.
	created := createdDirs(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.add(Check{Group: "sandbox", Name: label, Status: Fail,
			Detail: "cannot create " + dir + " (" + what + "): " + err.Error(),
			Remedy: cannotFix})
		return
	}
	defer func() {
		// Deepest first; os.Remove on a non-empty directory fails, which is the
		// guard we want rather than a check to race against.
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.Remove(created[i])
		}
	}()
	f, err := os.CreateTemp(dir, ".mkit-doctor-*")
	if err != nil {
		r.add(Check{Group: "sandbox", Name: label, Status: Fail,
			Detail: "not writable: " + dir + " (" + what + ")", Remedy: cannotFix})
		return
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	r.add(Check{Group: "sandbox", Name: label, Status: OK, Detail: dir + " — " + what})
}

// createdDirs lists the path components missing right now, outermost first — the
// set MkdirAll is about to create, and so the only set this probe may remove.
func createdDirs(dir string) []string {
	var missing []string
	for p := filepath.Clean(dir); ; {
		if _, err := os.Stat(p); err == nil {
			break
		}
		missing = append(missing, p)
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	// Reverse to outermost-first, so callers can undo from the deepest end.
	for i, j := 0, len(missing)-1; i < j; i, j = i+1, j-1 {
		missing[i], missing[j] = missing[j], missing[i]
	}
	return missing
}

// grant is a directory a composed tool needs in permissions.additionalDirectories.
// Mirrors the table in docs/prerequisites.md.
type grant struct {
	dir     string
	tool    string
	without string
}

var grants = []grant{
	{"~/.cache/gh", "gh", "`gh run view --log` fails"},
	{"~/.codex", "codex", "`could not create PATH aliases`, app-server client fails to initialize"},
	{"~/.coderabbit", "coderabbit", "its own state writes fail"},
}

// allowlist reports grants that are missing for tools the user actually has.
// Gated on the tool being installed, because a grant for a tool nobody runs is
// noise, and noise is what makes a diagnostic get ignored.
//
// **Both grant keys are read, and they are not equivalent.**
// `permissions.additionalDirectories` grants the sandbox write *and* makes the
// path a working directory, which is what also satisfies the auto-mode
// classifier's "no writes outside the working directories" rule.
// `sandbox.filesystem.allowWrite` grants only the sandbox and leaves that rule
// biting. Reporting the narrower one as a clean pass would hide the failure it
// still produces; reporting it as missing would be wrong. It gets its own state.
func (r *Report) allowlist(toplevel string) {
	full, sandboxOnly := map[string]bool{}, map[string]bool{}
	for _, f := range settingsFiles(toplevel) {
		var s struct {
			Permissions struct {
				AdditionalDirectories []string `json:"additionalDirectories"`
			} `json:"permissions"`
			Sandbox struct {
				Filesystem struct {
					AllowWrite []string `json:"allowWrite"`
				} `json:"filesystem"`
			} `json:"sandbox"`
		}
		if readJSON(f, &s) != nil {
			continue
		}
		for _, d := range s.Permissions.AdditionalDirectories {
			full[normalizeDir(d)] = true
		}
		for _, d := range s.Sandbox.Filesystem.AllowWrite {
			sandboxOnly[normalizeDir(d)] = true
		}
	}

	var missing, narrow []string
	for _, g := range grants {
		if _, err := exec.LookPath(g.tool); err != nil {
			continue
		}
		dir := normalizeDir(g.dir)
		switch {
		case full[dir]:
		case sandboxOnly[dir]:
			narrow = append(narrow, g.dir+" ("+g.tool+")")
		default:
			missing = append(missing, g.dir+" ("+g.tool+" — without it: "+g.without+")")
		}
	}
	sort.Strings(missing)
	sort.Strings(narrow)

	remedy := "add them to permissions.additionalDirectories in " + userSettingsFile() +
		" (human-run: that file is sandbox-denied)"

	switch {
	case len(missing) > 0:
		detail := "missing grants for installed tools: " + strings.Join(missing, "; ")
		if len(narrow) > 0 {
			detail += "; granted by sandbox.filesystem.allowWrite only: " + strings.Join(narrow, ", ")
		}
		r.add(Check{Group: "sandbox", Name: "allowlist", Status: Warn, Detail: detail, Remedy: remedy})
	case len(narrow) > 0:
		r.add(Check{Group: "sandbox", Name: "allowlist", Status: Warn,
			Detail: "granted by sandbox.filesystem.allowWrite only: " + strings.Join(narrow, ", ") +
				" — the sandbox write is allowed, but the auto-mode classifier's " +
				"out-of-working-directory rule still applies",
			Remedy: remedy})
	default:
		r.add(Check{Group: "sandbox", Name: "allowlist", Status: OK,
			Detail: "every installed composed tool has its directory in additionalDirectories"})
	}
}

// userSettingsFile is the file every remedy here must name — CLAUDE_CONFIG_DIR
// relocates it, and a remedy pointing at ~/.claude/settings.json when the active
// configuration lives elsewhere is a change the reader makes that does nothing.
func userSettingsFile() string {
	cfg := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "~/.claude/settings.json"
		}
		cfg = filepath.Join(home, ".claude")
	}
	return filepath.Join(cfg, "settings.json")
}

// settingsFiles lists every settings file that can answer these questions, user
// scope first.
//
// Project settings hang off the *work tree root*, not the working directory: run
// from a subdirectory, a cwd-only search finds nothing and reports a project that
// enables the plugin as one that does not.
func settingsFiles(toplevel string) []string {
	out := []string{userSettingsFile()}
	seen := map[string]bool{}
	for _, base := range []string{toplevel, cwd()} {
		if base == "" || seen[base] {
			continue
		}
		seen[base] = true
		out = append(out,
			filepath.Join(base, ".claude", "settings.json"),
			filepath.Join(base, ".claude", "settings.local.json"))
	}
	return out
}

func cwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func normalizeDir(d string) string {
	d = strings.TrimSuffix(d, "/")
	if home, err := os.UserHomeDir(); err == nil {
		if d == "~" {
			return home
		}
		if strings.HasPrefix(d, "~/") {
			return filepath.Join(home, d[2:])
		}
	}
	return d
}
