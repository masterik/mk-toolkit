package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

// Origin says where a proposed command came from.
type Origin string

const (
	// Discovered — inferred from a manifest this run; cannot go stale.
	Discovered Origin = "discovered"
	// Pinned — read from `[gate.commands]` in the repo config.
	Pinned Origin = "pinned"
	// Documented — a `check:` target the repo's own Makefile or justfile
	// declares. Used as the chain only when nothing was discovered; otherwise
	// reported beside it, because a repo that declares its check has said
	// something inference cannot.
	Documented Origin = "documented"
)

// Proposal is one step of the pre-integration sequence. A proposal, never a
// decision: which command to trust stays the skill's call.
type Proposal struct {
	Step   string
	Cmd    string
	Origin Origin
	// Cache is what the ledger knows about Cmd. Zero when caching is off.
	Cache Lookup
}

// Detection is everything a skill needs to know before running a gate.
type Detection struct {
	Ecosystems []string
	// PM is the Node package manager, from the lockfile. Empty otherwise.
	PM    string
	Steps []Proposal
	// Documented is the repo's own `check:` target when one exists and a chain
	// was discovered anyway. Empty otherwise — when nothing was discovered it
	// *is* the chain and appears in Steps.
	Documented string
	// Scripts are package.json's script names, so a skill can see what was
	// skipped. ScriptsState says whether the list is trustworthy: `scripts=none`
	// with `unreadable` means undetected, not absent.
	Scripts      []string
	ScriptsState string
	Workspaces   bool
	// Docs are `<file>:<line>:<text>` hits from the repo's own documentation
	// naming a check command. Candidates — "the canonical check" is a judgement
	// the skill makes, not a grep result.
	Docs []string

	// Fingerprint identifies the content a gate command would read now. Empty
	// when CacheCause says why there is nothing to classify against.
	Fingerprint string
	// CacheCause is one of off, empty, no-fingerprint. Empty when the ledger
	// answered.
	CacheCause string
}

// DetectOptions are `gate detect`'s knobs.
type DetectOptions struct {
	// NoCache drops the ledger annotation entirely.
	NoCache bool
	// Now is the clock the age bound is measured against. Zero means time.Now.
	Now time.Time
}

// scriptsState values.
const (
	scriptsOK         = "ok"
	scriptsUnreadable = "unreadable"
	scriptsNA         = "n-a"
)

// docsPat is what a repo's own documentation looks like when it names its check.
var docsPat = regexp.MustCompile(`(^|[^a-z])(make|just|npm|pnpm|yarn|bun|deno|cargo|go|dotnet|uv|poetry|task) (run )?(check|lint|test|build|verify|ci)`)

// docsFiles are the documents a repo states its own conventions in.
var docsFiles = []string{"CLAUDE.md", "AGENTS.md", "README.md", "CONTRIBUTING.md", "docs/CONTRIBUTING.md"}

// DocsMaxPerFile bounds the hits taken from any one document.
const DocsMaxPerFile = 4

// Detect reports the repo's quality-gate commands. It never runs them, and never
// decides which one the skill should trust.
//
// The mapping from lockfile to package manager and from script names to the
// sequence is a table, and reading it here keeps a 25-script package.json (~700
// tokens) out of context.
func Detect(repo *gitrepo.Repo, cfg *repoconfig.Config, opt DetectOptions) (*Detection, error) {
	root := repo.Toplevel
	d := &Detection{ScriptsState: scriptsNA}

	var full []string
	addEco := func(name string) { d.Ecosystems = append(d.Ecosystems, name) }

	// --- Node / Bun / Deno -------------------------------------------------
	if exists(root, "package.json") {
		addEco("node")
		d.PM = packageManager(root)
		run := d.PM + " run"

		// An unparseable package.json used to leave the list empty, so a real
		// node repo reported `scripts=none` — indistinguishable from a
		// package.json that genuinely declares no scripts. Say which happened.
		scripts, workspaces, err := readPackageJSON(filepath.Join(root, "package.json"))
		if err != nil {
			d.ScriptsState = scriptsUnreadable
		} else {
			d.ScriptsState = scriptsOK
			d.Scripts = scripts
			d.Workspaces = workspaces
		}
		has := func(s string) bool {
			for _, got := range d.Scripts {
				if got == s {
					return true
				}
			}
			return false
		}
		// lint -> typecheck -> test -> build, whichever exist, in that order.
		for _, s := range []string{"lint", "typecheck", "test", "build"} {
			if has(s) {
				full = append(full, run+" "+s)
			}
		}
	}
	if exists(root, "deno.json") || exists(root, "deno.jsonc") {
		addEco("deno")
		full = append(full, "deno lint", "deno test")
	}

	// --- Rust ---------------------------------------------------------------
	if exists(root, "Cargo.toml") {
		addEco("rust")
		full = append(full, "cargo clippy --all-targets -- -D warnings", "cargo test", "cargo build")
	}

	// --- Go -----------------------------------------------------------------
	if exists(root, "go.mod") {
		addEco("go")
		full = append(full, "go vet ./...", "go test ./...", "go build ./...")
	}

	// --- Python -------------------------------------------------------------
	if exists(root, "pyproject.toml") || exists(root, "setup.cfg") || exists(root, "tox.ini") {
		addEco("python")
		linter := "python -m flake8"
		if _, err := exec.LookPath("ruff"); err == nil {
			linter = "ruff check ."
		}
		full = append(full, linter, "pytest -q")
		if exists(root, "pyproject.toml") && fileMatches(filepath.Join(root, "pyproject.toml"), regexp.MustCompile(`\[tool\.mypy\]`)) {
			full = append(full, "mypy .")
		}
	}

	// --- .NET ---------------------------------------------------------------
	if glob(root, "*.sln") || glob(root, "*.csproj") {
		addEco("dotnet")
		full = append(full, "dotnet build --nologo", "dotnet test --nologo")
	}

	// --- Make / Just: a documented `check:` target ---------------------------
	checkTarget := regexp.MustCompile(`(?m)^check:`)
	for _, f := range []string{"Makefile", "makefile", "GNUmakefile"} {
		if !exists(root, f) {
			continue
		}
		addEco("make")
		if fileMatches(filepath.Join(root, f), checkTarget) {
			d.Documented = "make check"
		}
		break
	}
	for _, f := range []string{"justfile", "Justfile", ".justfile"} {
		if !exists(root, f) {
			continue
		}
		addEco("just")
		if fileMatches(filepath.Join(root, f), checkTarget) {
			d.Documented = "just check"
		}
		break
	}

	// A repo whose only signal is its own `check:` target still has a gate. The
	// documented target fills an empty chain rather than replacing a discovered
	// one: a `check:` that lints and nothing else would otherwise silently drop
	// this repo's tests and build, and a gate is never half-capable.
	origin := Discovered
	if len(full) == 0 && d.Documented != "" {
		full, origin = []string{d.Documented}, Documented
		d.Documented = ""
	}

	// A newline in a pinned command would split the `full:` block's one-line-per-
	// step contract, so `cmd=` would stop naming the command. Reported as a step
	// whose command is refused rather than printed wrongly.
	seen := map[string]int{}
	for i, cmd := range full {
		name := StepName(cmd, i)
		// A label is a log filename and a pin key, so it has to be unique. In a
		// polyglot repo `npm run test` and `go test ./...` both want `test`:
		// the first keeps it, the rest are suffixed.
		if n := seen[name]; n > 0 {
			seen[name] = n + 1
			name = fmt.Sprintf("%s-%d", name, n+1)
		} else {
			seen[name] = 1
		}
		d.Steps = append(d.Steps, Proposal{Step: name, Cmd: cmd, Origin: origin})
	}
	d.mergePinned(cfg)
	d.Docs = scanDocs(root)

	d.annotate(repo, opt)
	return d, nil
}

// mergePinned applies `[gate.commands]`: pinned wins per step name, and a pinned
// step discovery did not find is appended. After this the merge exists in exactly
// one place — `mkit repo profile` consumes the tagged result rather than redoing
// it.
func (d *Detection) mergePinned(cfg *repoconfig.Config) {
	if cfg == nil {
		return
	}
	names := make([]string, 0, len(cfg.Gate.Commands))
	for name := range cfg.Gate.Commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cmd := cfg.Gate.Commands[name]
		replaced := false
		for i := range d.Steps {
			if d.Steps[i].Step == name {
				d.Steps[i] = Proposal{Step: name, Cmd: cmd, Origin: Pinned}
				replaced = true
				break
			}
		}
		if !replaced {
			d.Steps = append(d.Steps, Proposal{Step: name, Cmd: cmd, Origin: Pinned})
		}
	}
}

// annotate asks the ledger what it already proved about each proposed command.
//
// Annotating a proposal is all this ever does. Nothing here skips anything: that
// trade-off — latency against a safety net — is the SKILL.md's, and a skipped
// step must be reported as `cached`, never as a pass.
func (d *Detection) annotate(repo *gitrepo.Repo, opt DetectOptions) {
	// One value per distinct cause, per the `pr=gh-missing` / `pr=none` lesson:
	// only some of these mean "there was nothing to use".
	if opt.NoCache {
		d.CacheCause = "off"
		return
	}
	fp, err := Fingerprint(repo)
	if err != nil || fp == "" {
		// No work tree, or git plumbing failed. Not a missing tool: the hash is
		// computed in process now, so there is nothing to install.
		d.CacheCause = "no-fingerprint"
		return
	}
	ledger := OpenLedger(repo)
	if fi, serr := os.Stat(ledger.Path); serr != nil || fi.Size() == 0 {
		d.CacheCause = "empty"
		return
	}
	d.Fingerprint = fp

	cmds := make([]string, 0, len(d.Steps))
	for _, s := range d.Steps {
		cmds = append(cmds, s.Cmd)
	}
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	lookups, err := ledger.Classify(cmds, fp, now)
	if err != nil {
		// A ledger that cannot be read cleanly is a ledger with nothing to say;
		// a confident wrong class is the one answer it may not give.
		d.Fingerprint, d.CacheCause = "", "empty"
		return
	}
	for i := range d.Steps {
		d.Steps[i].Cache = lookups[i]
	}
}

// StepName labels a discovered command. An explicit table, not a guess: the label
// is for a human reading the proposal and for pinning an override by name, and a
// command matching none of them keeps a positional name rather than a wrong one.
func StepName(cmd string, i int) string {
	for _, name := range []string{"typecheck", "build", "vet", "test", "lint", "fmt", "check"} {
		for _, tok := range strings.Fields(cmd) {
			if tok == name {
				return name
			}
		}
	}
	return "step" + strconv.Itoa(i+1)
}

// packageManager reads the lockfile. npm is the answer when there is none.
func packageManager(root string) string {
	switch {
	case exists(root, "bun.lock"), exists(root, "bun.lockb"):
		return "bun"
	case exists(root, "pnpm-lock.yaml"):
		return "pnpm"
	case exists(root, "yarn.lock"):
		return "yarn"
	default:
		return "npm"
	}
}

// readPackageJSON returns the script names, sorted, and whether workspaces are
// declared.
func readPackageJSON(path string) ([]string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var pkg struct {
		Scripts    map[string]json.RawMessage `json:"scripts"`
		Workspaces json.RawMessage            `json:"workspaces"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return nil, false, err
	}
	names := make([]string, 0, len(pkg.Scripts))
	for name := range pkg.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	ws := len(pkg.Workspaces) > 0 &&
		string(pkg.Workspaces) != "null" && string(pkg.Workspaces) != "false"
	return names, ws, nil
}

// scanDocs surfaces the check commands the repo's own documents name.
func scanDocs(root string) []string {
	var out []string
	for _, f := range docsFiles {
		path := filepath.Join(root, f)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		hits := 0
		for n, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
			if !docsPat.MatchString(line) {
				continue
			}
			out = append(out, fmt.Sprintf("%s:%d:%s", f, n+1, line))
			if hits++; hits >= DocsMaxPerFile {
				break
			}
		}
	}
	return out
}

func exists(root, name string) bool {
	fi, err := os.Stat(filepath.Join(root, name))
	return err == nil && fi.Mode().IsRegular()
}

func glob(root, pattern string) bool {
	m, err := filepath.Glob(filepath.Join(root, pattern))
	return err == nil && len(m) > 0
}

func fileMatches(path string, re *regexp.Regexp) bool {
	b, err := os.ReadFile(path)
	return err == nil && re.Match(b)
}
