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
// (docs/backlog.md, "Staying in bash, permanently").
//
// Every remedy sentence with a shell counterpart is *fetched* from
// `lib/common.sh`, never re-worded here. One producer per sentence is a project
// rule until M5 ports facts.sh.
//
// Layering: returns data, never prints, never assumes a terminal.
package doctor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/masterik/mk-toolkit/internal/buildinfo"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/pluginroot"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
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
	r.payload(root, rootErr)
	r.prerequisites()
	r.userDir(root)
	if opts.Repo != nil {
		r.repo(opts.Repo)
	} else {
		r.add(Check{Group: "repo", Name: "work tree", Status: Unknown,
			Detail: "not inside a git repository — the repo checks were skipped"})
	}
	r.writableSet(toplevel)
	r.allowlist()
	return r
}

func (r *Report) add(c Check) { r.Checks = append(r.Checks, c) }

func (r *Report) binary() {
	r.add(Check{Group: "install", Name: "mkit binary", Status: OK,
		Detail: buildinfo.Version + " (" + buildinfo.Commit + ")"})
}

func (r *Report) payload(root *pluginroot.Root, err error) {
	if err != nil {
		r.add(Check{Group: "install", Name: "plugin payload", Status: Fail,
			Detail: "not found — the skills are unavailable, and so is every remedy " +
				"sentence the binary reads from lib/common.sh",
			Remedy: pluginroot.Remedy()})
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
				" — the two ship over separate channels and drift is expected, not broken",
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

	r.enabled()
}

// enabled reads the harness's own settings to say whether the plugin is switched
// on. Read-only: `~/.claude/settings.json` is sandbox-denied for writes, so this
// can never offer to fix it and says so.
func (r *Report) enabled() {
	found, where := false, ""
	for _, f := range settingsFiles() {
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
			"~/.claude/settings.json is sandbox-denied, so nothing here can write it"})
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
// 0.15.0 nothing tells a user unprompted that a tool is missing, and it surfaces
// only as a thinner facts.sh block or a gate_cache=no-hash annotation.
var tools = []tool{
	{"git", "prerequisites", Fail, "every skill", ""},
	{"bash", "prerequisites", Fail, "the whole payload", ""},
	{"gh", "prerequisites", Warn, "pr, finish, and cleanup's PR column", "brew install gh, then `gh auth login`"},
	{"jq", "prerequisites", Warn, "facts.sh's PR lookup, the gate ledger, branch-scan", "brew install jq"},
	{"node", "prerequisites", Warn, "review's findings.mjs", "brew install node"},
	{"shasum", "prerequisites", Warn, "the gate cache fingerprint (reports gate_cache=no-hash without it)", ""},
	{"rg", "optional", Warn, "faster searching; grep -E is used otherwise", "brew install ripgrep"},
	{"wt", "optional", Warn, "worktrunk-managed worktree teardown in finish/cleanup", "brew install worktrunk"},
	{"codex", "optional", Warn, "review's Codex reviewer", ""},
	{"coderabbit", "optional", Warn, "review's CodeRabbit reviewer", ""},
}

func (r *Report) prerequisites() {
	for _, t := range tools {
		path, err := exec.LookPath(t.name)
		if err == nil {
			r.add(Check{Group: t.group, Name: t.name, Status: OK, Detail: path})
			continue
		}
		r.add(Check{Group: t.group, Name: t.name, Status: t.status,
			Detail: "not on PATH — needed for " + t.what, Remedy: t.remedy})
	}
}

// userDir asks the payload rather than probing itself, for both halves: the probe
// (`mkit_user_dir_writable`, which is net-zero by construction) and the sentence
// (`mkit_user_dir_remedy`, which must name creating the directory *and* granting
// it). Re-implementing either here would be the second implementation the porting
// rules forbid, and re-wording the sentence is how the original one-grant mistake
// survived three files.
// userDirCheck is a constant because a check's name is its identity: a report
// whose rows rename themselves by branch cannot be diffed or matched against.
const userDirCheck = "user state dir"

func (r *Report) userDir(root *pluginroot.Root) {
	if root == nil {
		r.add(Check{Group: "sandbox", Name: userDirCheck, Status: Unknown,
			Detail: "cannot probe it: the writability check and its remedy both live in " +
				"the payload's lib/common.sh, which was not found"})
		return
	}
	dir, _ := root.CommonFunc("mkit_user_dir")
	if _, err := root.CommonFunc("mkit_user_dir_writable"); err == nil {
		r.add(Check{Group: "sandbox", Name: userDirCheck, Status: OK, Detail: dir + " is writable"})
		return
	}
	remedy, err := root.CommonFunc("mkit_user_dir_remedy")
	if err != nil {
		remedy = ""
	}
	// Warn, not Fail: the directory is empty today — its two files went with the
	// hook — so nothing is failing yet. A later user-scoped write would.
	r.add(Check{Group: "sandbox", Name: userDirCheck, Status: Warn,
		Detail: dir + " is not writable; nothing needs it today, a later user-scoped write would",
		Remedy: remedy})
}

func (r *Report) repo(repo *gitrepo.Repo) {
	r.add(Check{Group: "repo", Name: "work tree", Status: OK, Detail: repo.Toplevel})

	if ignored, _ := repo.Ignored(".mkit/gate.jsonl"); ignored {
		r.add(Check{Group: "repo", Name: "scratch ignored", Status: OK,
			Detail: ".mkit/ scratch is excluded"})
	} else {
		r.add(Check{Group: "repo", Name: "scratch ignored", Status: Fail,
			Detail: ".mkit/ is not ignored here — `git worktree remove` will refuse, " +
				"`git add -A` would commit run artefacts, and the gate fingerprint " +
				"sees a directory that changes while the gate runs",
			Remedy: "from the main checkout, add `.mkit/*` and `!.mkit/config.toml` to " +
				".git/info/exclude or .gitignore (run any mkit skill and run-open.sh does it)"})
	}

	st := repoconfig.Stat(repo)
	switch st.State {
	case repoconfig.StateTracked:
		r.add(Check{Group: "repo", Name: "config", Status: OK,
			Detail: st.Path + " (tracked — a fresh clone inherits it)"})
	case repoconfig.StateUntracked:
		r.add(Check{Group: "repo", Name: "config", Status: Warn,
			Detail: st.Path + " exists but is not committed, so no colleague inherits it",
			Remedy: "git add .mkit/config.toml && git commit"})
	case repoconfig.StateShadowed:
		r.add(Check{Group: "repo", Name: "config", Status: Fail,
			Detail: st.Path + " is ignored here, so `mkit init` would write a file that " +
				"never reaches a fresh clone — the one property it exists for",
			Remedy: repoconfig.ShadowedRemedy(st.IgnoreSource)})
	default:
		// Absent is a normal state, not a finding: config is an input, never a
		// permission (ADR 0001 decision 3). Reported so the path is visible.
		r.add(Check{Group: "repo", Name: "config", Status: OK,
			Detail: "none — every command runs without one; `mkit init` writes " + st.Path})
	}
}

// writableSet reports the three locations the project declares, by writing to
// them. Probed rather than inferred: the sandbox leaves the mode bits saying yes
// while denying the write, so `[ -w ]` and os.Stat both lie here.
func (r *Report) writableSet(toplevel string) {
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = os.TempDir()
	}
	r.probe("$TMPDIR", tmp, "anything that dies with the command",
		"nothing to grant — a denied $TMPDIR means the sandbox is configured unusually")

	if toplevel != "" {
		r.probe("<toplevel>/.mkit", filepath.Join(toplevel, ".mkit"),
			"run directories and the gate ledger",
			"nothing to grant — the working directory is writable by default; "+
				"a refusal here usually means the session is isolated in another worktree")
	}
}

func (r *Report) probe(label, dir, what, cannotFix string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.add(Check{Group: "sandbox", Name: label, Status: Fail,
			Detail: "cannot create " + dir + " (" + what + "): " + err.Error(),
			Remedy: cannotFix})
		return
	}
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
func (r *Report) allowlist() {
	full, sandboxOnly := map[string]bool{}, map[string]bool{}
	for _, f := range settingsFiles() {
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

	const remedy = "add them to permissions.additionalDirectories in ~/.claude/settings.json " +
		"(human-run: that file is sandbox-denied)"

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

func settingsFiles() []string {
	var out []string
	cfg := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfg == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg = filepath.Join(home, ".claude")
		}
	}
	if cfg != "" {
		out = append(out, filepath.Join(cfg, "settings.json"))
	}
	if wd, err := os.Getwd(); err == nil {
		out = append(out,
			filepath.Join(wd, ".claude", "settings.json"),
			filepath.Join(wd, ".claude", "settings.local.json"))
	}
	return out
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
