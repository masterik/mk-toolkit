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
// Every degradation sentence has exactly one producer. Until M5 the shell was it
// and this fetched from `lib/common.sh`; since the payload's last script went,
// the producer is the Go package that owns the surface — `scratch` for the
// user-scoped directory and for an unignored scratch root, `repoconfig` for a
// shadowed config.
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
	"github.com/masterik/mk-toolkit/internal/core/scratch"
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
	r.writableSet(toplevel)
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
	{"bash", "prerequisites", Fail, "mkit gate run", ""},
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
			r.add(Check{Group: t.group, Name: t.name, Status: OK, Detail: path})
			continue
		}
		r.add(Check{Group: t.group, Name: t.name, Status: t.status,
			Detail: "not on PATH — needed for " + t.what, Remedy: t.remedy})
	}
}

// userDirCheck is a constant because a check's name is its identity: a report
// whose rows rename themselves by branch cannot be diffed or matched against.
const userDirCheck = "user state dir"

// userDir reports both halves from `internal/core/scratch`: the probe (net-zero
// by construction) and the sentence (which must name creating the directory *and*
// granting it). One producer for each — until M5 both were fetched from the
// payload's lib/common.sh, and re-wording the sentence is how the original
// one-grant mistake survived three files.
func (r *Report) userDir() {
	dir := scratch.UserDir()
	if dir == "" {
		r.add(Check{Group: "sandbox", Name: userDirCheck, Status: Warn,
			Detail: "no user-scoped directory: $HOME is unset and MKIT_HOME is not set",
			Remedy: scratch.UserDirRemedy()})
		return
	}
	if scratch.UserDirWritable() {
		r.add(Check{Group: "sandbox", Name: userDirCheck, Status: OK, Detail: dir + " is writable"})
		return
	}
	// Warn, not Fail: the binary writes nothing there today — its two files went
	// with the hook — and the one file in it, the sandbox-audit skill's ledger, is
	// optional: that skill prints the ledger instead. A later user-scoped write
	// would fail.
	r.add(Check{Group: "sandbox", Name: userDirCheck, Status: Warn,
		Detail: dir + " is not writable; the sandbox-audit ledger cannot be kept, and a later user-scoped write would fail",
		Remedy: scratch.UserDirRemedy()})
}

func (r *Report) repo(repo *gitrepo.Repo) {
	r.add(Check{Group: "repo", Name: "work tree", Status: OK, Detail: repo.Toplevel})

	// The remedy names this file rather than a fixed `.git/info/exclude`: under a
	// linked worktree the exclude lives in the main checkout.
	common, _ := repo.CommonDir()

	// scratch.Ignored, not a single check-ignore: it probes the ledger *and* a
	// run directory, because an unrelated `*.jsonl` rule hides the first while
	// leaving the second untracked. facts reads the same producer.
	if scratch.Ignored(repo) {
		r.add(Check{Group: "repo", Name: "scratch ignored", Status: OK,
			Detail: ".mkit/ scratch is excluded"})
	} else {
		r.add(Check{Group: "repo", Name: "scratch ignored", Status: Fail,
			Detail: ".mkit/ is not ignored here — `git worktree remove` will refuse, " +
				"`git add -A` would commit run artefacts, and the gate fingerprint " +
				"sees a directory that changes while the gate runs",
			Remedy: scratch.IgnoredRemedy(common)})
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
			Remedy: repoconfig.ShadowedRemedy(st)})
	default:
		// Absent is a normal state, not a finding: config is an input, never a
		// permission (ADR 0001 decision 3). Reported so the path is visible.
		r.add(Check{Group: "repo", Name: "config", Status: OK,
			Detail: "none — every command runs without one; `mkit init` writes " + st.Path})
	}

	r.configValues(repo)
}

// configValues reports what the config file says that mkit could not honour: a
// key it does not know, an enumerated value outside its set, a document that will
// not parse, a version from the future.
//
// Warn, never Fail, and the exit status stays 0: config is an input, never a
// permission (ADR 0001 decision 3) — nothing here stops a command, it only means
// a pin the reader believed in never took effect. The sentences come from
// `repoconfig`, which is their one producer; `mkit repo profile` prints the same
// words from the same place.
func (r *Report) configValues(repo *gitrepo.Repo) {
	cfg, present, err := repoconfig.Load(repo.Toplevel)
	if err != nil {
		// A file that is there and cannot be read is exactly what a human-run
		// report exists to name. Staying silent here left `doctor` reporting
		// nothing at all about the one config state a user cannot see for
		// themselves from the file's contents.
		pb := repoconfig.UnreadableProblem(repoconfig.Path(repo.Toplevel), err)
		r.add(Check{Group: "repo", Name: "config values", Status: Warn,
			Detail: pb.Detail, Remedy: configRemedy(pb)})
		return
	}
	if !present {
		return
	}
	if len(cfg.Problems) == 0 {
		r.add(Check{Group: "repo", Name: "config values", Status: OK,
			Detail: "every key and value in " + repoconfig.Path(repo.Toplevel) + " is understood"})
		return
	}
	for _, pb := range cfg.Problems {
		r.add(Check{Group: "repo", Name: "config values", Status: Warn,
			Detail: pb.Detail, Remedy: configRemedy(pb)})
	}
}

// configRemedy names an edit that works. A newer-version file gets none on
// purpose: there is nothing to fix in it — upgrading is the reader's move, not an
// edit to a colleague's committed file — so it says so rather than offering one.
func configRemedy(pb repoconfig.Problem) string {
	switch pb.Kind {
	case repoconfig.ProblemUnknownKey:
		return "remove or correct `" + pb.Key + "` in " + pb.Path
	case repoconfig.ProblemInvalidValue:
		// Not every invalid value has an enumeration behind it: `commit.subject_max`
		// is a number with a rule, and rendering its allowed set produces "to one
		// of , or remove it" — a remedy naming no value, which is the one thing a
		// degradation sentence may not be.
		if allowed := repoconfig.Allowed(pb.Key); len(allowed) > 0 {
			return "set `" + pb.Key + "` in " + pb.Path + " to one of " +
				strings.Join(allowed, ", ") + ", or remove it"
		}
		if rule := repoconfig.Rule(pb.Key); rule != "" {
			return "set `" + pb.Key + "` in " + pb.Path + " to " + rule + ", or remove it"
		}
		return "correct `" + pb.Key + "` in " + pb.Path + ", or remove it"
	case repoconfig.ProblemUnreadable:
		return "make " + pb.Path + " readable, or delete it — discovery answers " +
			"everything it pinned"
	case repoconfig.ProblemUnparsable:
		return "fix the TOML syntax in " + pb.Path + ", or delete the file — " +
			"discovery answers everything it pinned"
	default:
		return ""
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
	// doctor reports and fixes nothing, and that includes leaving the filesystem
	// as it found it: on a fresh clone `<toplevel>/.mkit` does not exist yet, and
	// a probe that creates it has made the repo dirty to answer a question about
	// it. Only directories this call created are removed, and only if still empty
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
