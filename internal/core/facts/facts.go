// Package facts gathers every read-only fact an mkit skill needs to start, in one
// call, and opens the run directory while it is here.
//
// Every fact is mechanical, several are easy to get subtly wrong (an unresolved
// reference path, a bare `git diff --shortstat` that reads as a clean tree when
// the work is fully staged, the worktree classification at the end), and none of
// them is a judgement.
//
// It reports. It never acts: no staging, no merging, no `wt` invocation, and the
// only thing it writes is the run directory it opens.
//
// Layering: returns data, never prints, never assumes a terminal.
package facts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/pluginroot"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
	"github.com/masterik/mk-toolkit/internal/core/scratch"
)

// Options are `mkit facts`' knobs.
type Options struct {
	Skill string
	// Base is a branch to compare against; an unresolvable one is fatal.
	Base string
	// Range is an explicit git range, which replaces the working-tree scopes.
	Range     string
	GH        bool
	NoRun     bool
	StatusMax int
	FilesMax  int
}

// Scope is one diff scope: a stat line, a file list, and how many the list was
// capped from.
type Scope struct {
	// Stat is `--shortstat`, or "none".
	Stat  string
	Files int
	List  []string
}

// Facts is everything a skill starts from. Every field is a fact about this
// checkout at this moment; nothing here decides anything.
type Facts struct {
	Run    string
	Plugin string
	Refs   string
	// PluginCause explains an empty Plugin/Refs.
	PluginCause string

	Toplevel   string
	GitDir     string
	CommonDir  string
	Primary    string
	Linked     bool
	Worktrees  int
	TMP        string
	RunIgnored bool

	// UserDir is where user-scoped state goes; UserDirOK is whether it actually
	// takes a write. Probed once, here, because the probe is what `mkit doctor`
	// would otherwise duplicate.
	UserDir   string
	UserDirOK bool

	Config      string
	ConfigState repoconfig.State
	// GitBin is absolute, resolved and single-token.
	GitBin string

	WorktreeOrigin string
	CleanupPath    string
	// WTListsThis is yes, no or unknown.
	WTListsThis string
	WTConfig    string
	WTBin       string

	Branch        string
	Detached      bool
	Upstream      string
	Pushed        bool
	Remote        string
	DefaultBranch string
	Base          string
	Ahead         int
	Behind        int
	HasUpstream   bool

	Clean       bool
	Staged      int
	Unstaged    int
	Untracked   int
	Conflicted  int
	Status      []string
	StatusTotal int

	// Scopes are the diff scopes, in emission order: range, or
	// unstaged/staged/untracked, plus base when --base resolved.
	Scopes []NamedScope

	BaseState          string
	RangeState         string
	CommitsAheadOfBase int
	Commits            []string
	FFFromBase         string

	CodeOwners string
	// PR is gh-missing | gh-unauthenticated | no-remote | none | <url>, or empty
	// when --gh was not asked for.
	PR      string
	PRState string
	PRDraft string

	// Notes carry every cause that needs a sentence. A value with spaces never
	// goes on a key=value line: several of those lines pack more than one pair,
	// so a reader splitting on whitespace would mis-parse one.
	Notes []string
}

// NamedScope is a Scope under the label its keys are printed with.
type NamedScope struct {
	Label string
	Scope Scope
}

// ErrUnresolvableBase is returned when --base names nothing. The caller still
// prints `base_state=unresolvable` first: an unresolvable base used to fall
// through silently and still exit 0, so `finish`/`pr` got a fact set with no
// commits_ahead_of_base and no way to tell that from a base with nothing on it.
type ErrUnresolvableBase struct{ Base string }

func (e *ErrUnresolvableBase) Error() string {
	return "--base does not resolve to a commit: " + e.Base
}

// ErrUnresolvableRange is --base's sibling, and was the asymmetry: --base was
// made fatal because a silent fall-through gave `finish`/`pr` a fact set they
// could not distinguish from a real answer, while --range kept exactly that
// behaviour — a typo'd range printed `range_stat=none range_files=0` and exited
// 0, which reads precisely like a range with nothing in it.
type ErrUnresolvableRange struct{ Range string }

func (e *ErrUnresolvableRange) Error() string {
	return "--range does not resolve: " + e.Range
}

// checkRev refuses a caller-supplied revision that git would read as an option.
//
// Not cosmetic: `--base=--output=PATH` reaches `git rev-parse` as an option, and
// git creates PATH before failing on the rest — an arbitrary file write through
// a flag that only ever names a commit. Rejected before any git call sees it,
// and every call that takes one of these also passes it after --end-of-options.
func checkRev(flag, v string) error {
	if strings.HasPrefix(v, "-") {
		return &RevError{Flag: flag, Value: v}
	}
	return nil
}

// RevError is a rejected revision — a usage mistake, exit 2, like a bad slug.
type RevError struct {
	Flag, Value string
}

func (e *RevError) Error() string {
	return fmt.Sprintf("%s may not begin with '-': git reads %q as an option "+
		"rather than a revision", e.Flag, e.Value)
}

// Gather collects the facts. repo is the work tree; the caller resolved it.
func Gather(repo *gitrepo.Repo, opt Options) (*Facts, error) {
	if err := scratch.CheckSlug(opt.Skill); err != nil {
		return nil, err
	}
	if err := checkRev("--base", opt.Base); err != nil {
		return nil, err
	}
	if err := checkRev("--range", opt.Range); err != nil {
		return nil, err
	}
	if opt.StatusMax <= 0 {
		opt.StatusMax = 60
	}
	if opt.FilesMax <= 0 {
		opt.FilesMax = 200
	}

	f := &Facts{Toplevel: repo.Toplevel, TMP: tmpDir(), Base: opt.Base}

	// The reference paths, which retire "resolve ${CLAUDE_PLUGIN_ROOT} before you
	// put a path in a brief" from the skills. The last thing that needs the
	// payload located at all.
	if root, err := pluginroot.Find(repo.Toplevel); err == nil {
		f.Plugin = root.Dir
		f.Refs = filepath.Join(root.Dir, "skills", "_shared", "references")
	} else {
		f.PluginCause = pluginroot.Remedy()
		f.Notes = append(f.Notes, "plugin=none — the skills and their shared references\n"+
			"  are unreachable, so any step that reads one cannot run. Remedy: "+
			f.PluginCause+".")
	}

	if !opt.NoRun {
		dir, err := scratch.RunDir(repo, opt.Skill)
		if err != nil {
			return nil, err
		}
		f.Run = dir
	}

	f.GitDir = git(repo, "rev-parse", "--absolute-git-dir")
	f.CommonDir, _ = repo.CommonDir()
	f.Primary = primaryWorktree(repo)
	f.Linked = f.GitDir != f.CommonDir
	for _, line := range lines(git(repo, "worktree", "list", "--porcelain")) {
		if strings.HasPrefix(line, "worktree ") {
			f.Worktrees++
		}
	}

	// `.mkit/` unignored is not cosmetic: `git worktree remove` refuses,
	// `git add -A` would commit run artefacts, and the gate fingerprint sees a
	// directory that changes while the gate runs. A worktree-isolated session
	// cannot reach the exclude file, so the answer is a fact.
	f.RunIgnored = scratch.Ignored(repo)
	if !f.RunIgnored {
		f.Notes = append(f.Notes, fmt.Sprintf(
			"run_ignored=no — .mkit/ is not ignored here, so a staging step would sweep run\n"+
				"  artefacts into a commit and `git worktree remove` would refuse. Do not stage while\n"+
				"  this says no. Remedy: %s.", scratch.IgnoredRemedy(f.CommonDir)))
	}

	st := repoconfig.Stat(repo)
	f.Config, f.ConfigState = st.Path, st.State
	// The state worth a sentence is `shadowed`: a repo set up before the config
	// existed carries a directory-only `.mkit/` rule, under which the file can be
	// written and then silently never travels to a fresh clone — the one property
	// it exists for. `absent` is a normal state and never a note: config is an
	// input, never a permission (ADR 0001 decision 3).
	if st.State == repoconfig.StateShadowed {
		f.Notes = append(f.Notes, "config_state=shadowed — .mkit/config.toml is ignored in this "+
			"checkout, so `mkit init`\n  would write a file that never reaches a fresh clone. Remedy: "+
			repoconfig.ShadowedRemedy(st)+".")
	}

	f.setUserDirNote()
	f.GitBin = gitBin()
	f.setWorktreeOrigin(repo)
	f.setBranch(repo)
	if err := f.setWorkingTree(repo, opt); err != nil {
		return f, err
	}
	f.CodeOwners = codeowners(repo)
	if opt.GH {
		f.setPR(repo)
	}
	return f, nil
}

// UserDir and its writability are reported for the skill; `mkit doctor` is the
// report for a human. Nothing writes to the directory today — it emptied when the
// SessionStart hook went — but an unwritable one is a starting fact rather than a
// later surprise.
func (f *Facts) setUserDirNote() {
	f.UserDir = scratch.UserDir()
	f.UserDirOK = scratch.UserDirWritable()
	if f.UserDirOK {
		return
	}
	f.Notes = append(f.Notes, "user_dir_writable=no — nothing needs that directory today, so "+
		"nothing is failing\n  yet; a later user-scoped write would. Remedy: "+
		scratch.UserDirRemedy()+".\n  Tell the user; do not retry the write.")
}

// gitBin is the git invocation a skill parses with: absolute, no spaces.
//
// Emitted as a fact because a skill must not assemble it from a guess, and for
// two measured reasons: a PreToolUse hook can rewrite `git status --short` into a
// wrapper that reshapes output for reading, so a summarized status reaches a
// skill looking exactly like the tree it is judging; and the worktree-isolation
// guard refuses any launcher it cannot read a git target through, while an
// absolute binary path is not rewritten at all.
func gitBin() string {
	p, err := exec.LookPath("git")
	if err != nil {
		return "git"
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// setWorktreeOrigin runs worktree.md's lookup table for the worktree this session
// is in.
//
// Tested, not assumed: `wt list` enumerates *every* git worktree in the repo, not
// only the ones worktrunk created — so its output cannot tell a worktrunk
// worktree from a hand-made one, and neither can the path (worktrunk's
// `worktree-path` template is fully configurable). What actually decides cleanup
// is narrower:
//
//	the harness's own worktree             -> hand back with ExitWorktree, never rm it
//	any other linked worktree, wt present  -> `wt merge` (honors the user's hooks)
//	any other linked worktree, no wt       -> plain `git worktree remove`
//	the primary checkout                   -> nothing to tear down
//
// `wt list` is worth one call inside a linked worktree — it proves wt can see
// this repo at all — and costs ~350 ms against git's ~20 ms, so it runs only there.
func (f *Facts) setWorktreeOrigin(repo *gitrepo.Repo) {
	f.WTConfig = "none"
	for _, c := range []string{
		filepath.Join(repo.Toplevel, ".config", "wt.toml"),
		filepath.Join(configHome(), "worktrunk", "config.toml"),
	} {
		if isFile(c) {
			f.WTConfig = c
			break
		}
	}
	// Advisory: `wt` is usually also a shell function, and the agent's own shell
	// may have it even when this does not. `wt_bin=none` is not proof the agent
	// cannot call wt.
	f.WTBin = wtBin()

	f.WorktreeOrigin, f.CleanupPath, f.WTListsThis = "primary", "none", "unknown"
	if !f.Linked {
		return
	}
	if strings.Contains(repo.Toplevel, "/.claude/worktrees/") {
		f.WorktreeOrigin, f.CleanupPath = "claude-code", "exit-worktree"
		return
	}
	f.WorktreeOrigin, f.CleanupPath = "linked", "git-worktree"
	items, err := wtItems(f.WTBin)
	if err != nil {
		return
	}
	for _, it := range items {
		if it.path() == repo.Toplevel {
			f.WTListsThis, f.CleanupPath = "yes", "wt"
			return
		}
	}
	f.WTListsThis = "no"
}

func (f *Facts) setBranch(repo *gitrepo.Repo) {
	if b := git(repo, "branch", "--show-current"); b != "" {
		f.Branch, f.Detached = b, false
	} else {
		f.Branch, f.Detached = git(repo, "rev-parse", "--short", "HEAD"), true
	}

	up, err := gitErr(repo, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && up != "" {
		f.Upstream, f.Pushed, f.HasUpstream = up, true, true
	} else {
		f.Upstream = "none"
	}
	f.Remote = firstLine(git(repo, "remote"))

	// Delegated, never re-derived: `mkit branch scan` is handed this answer and
	// `mkit repo profile` reports the branches it protects, so the resolution
	// lives in exactly one place.
	f.DefaultBranch = repo.DefaultBranch()

	if f.HasUpstream {
		counts := strings.Fields(git(repo, "rev-list", "--left-right", "--count", "HEAD..."+f.Upstream))
		if len(counts) == 2 {
			f.Ahead, _ = strconv.Atoi(counts[0])
			f.Behind, _ = strconv.Atoi(counts[1])
		}
	}
}

// excludeScratch keeps mkit's own scratch out of every enumeration of the user's
// work. The run directory lives *inside* the working directory, so without it the
// scratch mkit just created is reported back to the skill as the user's own
// change — and `run_ignored=no` is exactly the session that hits it.
//
// It excludes the scratch *contents* rather than the directory, because
// `.mkit/config.toml` is committed repo config — the user's own file. Excluding
// `.mkit` wholesale reported `clean=yes` on a tree whose config had been edited,
// and `commit` then had nothing to stage. Run directories are `.mkit/<skill>-*/`
// so `:(exclude,glob).mkit/*/**` covers every file inside one, and the ledger is
// named outright.
var excludeScratch = []string{".", ":(exclude,glob).mkit/*/**", ":(exclude).mkit/gate.jsonl"}

// excludeNoise additionally drops the generated files no diff scope wants.
var excludeNoise = []string{".", ":(exclude)*.lock", ":(exclude)*.snap"}

func (f *Facts) setWorkingTree(repo *gitrepo.Repo, opt Options) error {
	raw, err := gitErr(repo, append([]string{"status", "--porcelain", "--"}, excludeScratch...)...)
	if err != nil {
		return fmt.Errorf("git status failed, so no answer about the working tree "+
			"is available (an empty one would read as a clean tree): %w", err)
	}
	porcelain := lines(raw)
	f.Clean = len(porcelain) == 0
	f.StatusTotal = len(porcelain)
	for _, l := range porcelain {
		if len(l) < 2 {
			continue
		}
		switch {
		case strings.HasPrefix(l, "??"):
			f.Untracked++
		default:
			if l[0] != ' ' {
				f.Staged++
			}
			if l[1] != ' ' {
				f.Unstaged++
			}
		}
	}
	f.Conflicted = len(lines(git(repo, "diff", "--name-only", "--diff-filter=U")))
	f.Status = capList(porcelain, opt.StatusMax)

	switch {
	case opt.Range != "":
		// `--` so a range that collides with a path name is still read as a
		// range, and an unreadable one is reported rather than scoped.
		if run(repo, "rev-list", "--count", "--end-of-options", opt.Range, "--") != nil {
			f.RangeState = "unresolvable"
			return &ErrUnresolvableRange{Range: opt.Range}
		}
		f.RangeState = "ok"
		sc, err := scope(repo, nil, opt.Range, opt.FilesMax)
		if err != nil {
			return err
		}
		f.Scopes = append(f.Scopes, NamedScope{"range", sc})
	case !f.Clean:
		// Both stats, always. A bare `git diff --shortstat` reports nothing when
		// the work is fully staged, which reads exactly like a clean tree — the
		// single most expensive misread in this bundle.
		unstaged, err := scope(repo, nil, "", opt.FilesMax)
		if err != nil {
			return err
		}
		staged, err := scope(repo, []string{"--cached"}, "", opt.FilesMax)
		if err != nil {
			return err
		}
		untracked, err := untrackedScope(repo, opt.FilesMax)
		if err != nil {
			return err
		}
		f.Scopes = append(f.Scopes,
			NamedScope{"unstaged", unstaged},
			NamedScope{"staged", staged},
			NamedScope{"untracked", untracked})
	}

	if opt.Base == "" {
		return nil
	}
	if run(repo, "rev-parse", "--verify", "--quiet", "--end-of-options", opt.Base) != nil {
		f.BaseState = "unresolvable"
		return &ErrUnresolvableBase{Base: opt.Base}
	}
	f.BaseState = "ok"
	f.CommitsAheadOfBase, _ = strconv.Atoi(git(repo, "rev-list", "--count", opt.Base+"..HEAD"))
	if f.CommitsAheadOfBase > 0 {
		f.Commits = lines(git(repo, "log", "--oneline", opt.Base+"..HEAD"))
	}
	// Unconditionally: a resolved base has a diff whether or not any commit sits
	// between it and HEAD, and `pr` reads base_stat after `commit` has run — the
	// one moment the count can still be 0 while the work is real.
	sc, err := scope(repo, nil, opt.Base, opt.FilesMax)
	if err != nil {
		return err
	}
	f.Scopes = append(f.Scopes, NamedScope{"base", sc})
	f.FFFromBase = "no"
	if run(repo, "merge-base", "--is-ancestor", opt.Base, "HEAD") == nil {
		f.FFFromBase = "yes"
	}
	return nil
}

// scope returns an error rather than an empty Scope: `stat=none files=0` is the
// honest answer for a scope with nothing in it, so a git failure silently
// producing it is a wrong answer that reads exactly like a right one.
// opts are git's own flags for this scope (`--cached` for the staged one); rev
// is a caller-supplied revision, which is a different kind of argument and must
// never be passed where a flag is expected. Keeping them apart is what lets the
// revision go after --end-of-options: options first, then the marker, then the
// revision, so a rev beginning with `-` can never be read as a flag. checkRev
// refuses one anyway; this makes the call safe regardless.
func scope(repo *gitrepo.Repo, opts []string, rev string, max int) (Scope, error) {
	statArgs := append([]string{"diff", "--shortstat"}, opts...)
	listArgs := append([]string{"diff", "--name-only"}, opts...)
	if rev != "" {
		statArgs = append(statArgs, "--end-of-options", rev)
		listArgs = append(listArgs, "--end-of-options", rev)
	}
	// The same pathspec on both, or the stat counts a file the list drops and a
	// scope reports `files=0` beside a shortstat that says otherwise.
	stat, err := gitErr(repo, append(append(statArgs, "--"), excludeNoise...)...)
	if err != nil {
		return Scope{}, fmt.Errorf("git diff --shortstat %s: %w", rev, err)
	}
	stat = strings.TrimSpace(stat)
	if stat == "" {
		stat = "none"
	}
	raw, err := gitErr(repo, append(append(listArgs, "--"), excludeNoise...)...)
	if err != nil {
		return Scope{}, fmt.Errorf("git diff --name-only %s: %w", rev, err)
	}
	list := lines(raw)
	return Scope{Stat: stat, Files: len(list), List: capList(list, max)}, nil
}

// untrackedScope is its own block because `git diff` never lists an untracked
// file: a dirty tree containing new files reported `untracked=6` beside a file
// list naming none of them — a review or commit scope that silently omits every
// new implementation file.
func untrackedScope(repo *gitrepo.Repo, max int) (Scope, error) {
	args := append([]string{"ls-files", "--others", "--exclude-standard", "--"}, excludeNoise...)
	args = append(args, ":(exclude).mkit")
	raw, err := gitErr(repo, args...)
	if err != nil {
		return Scope{}, fmt.Errorf("git ls-files --others: %w", err)
	}
	list := lines(raw)
	return Scope{Files: len(list), List: capList(list, max)}, nil
}

// setPR names which of the four things `pr=none` used to mean actually happened;
// only the first of them justifies opening one.
func (f *Facts) setPR(repo *gitrepo.Repo) {
	if _, err := exec.LookPath("gh"); err != nil {
		f.PR = "gh-missing"
		return
	}
	out, err := output(repo, "gh", "pr", "view", "--json", "url,state,isDraft")
	if err == nil && len(out) > 0 {
		var v struct {
			URL     string `json:"url"`
			State   string `json:"state"`
			IsDraft bool   `json:"isDraft"`
		}
		if json.Unmarshal(out, &v) == nil && v.URL != "" {
			f.PR, f.PRState, f.PRDraft = v.URL, v.State, strconv.FormatBool(v.IsDraft)
			return
		}
	}
	switch {
	case run2(repo, "gh", "auth", "status") != nil:
		f.PR = "gh-unauthenticated"
	case f.Remote == "":
		f.PR = "no-remote"
	default:
		f.PR = "none"
	}
}

func codeowners(repo *gitrepo.Repo) string {
	for _, c := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		if p := filepath.Join(repo.Toplevel, c); isFile(p) {
			return p
		}
	}
	return "none"
}

// wtBin walks PATH for a real executable. `exec.LookPath` is not enough on its
// own conceptually: worktrunk's shell integration installs `wt` as a shell
// function (it has to, to cd the parent shell), so the agent's shell can have a
// `wt` this never sees.
func wtBin() string {
	p, err := exec.LookPath("wt")
	if err != nil {
		return "none"
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// wtItem is one entry of `wt list --format=json`, which emits schema 1 (a bare
// array) or schema 2 (an envelope with .items) depending on the user's config.
type wtItem struct {
	Path     string `json:"path"`
	Worktree struct {
		Path string `json:"path"`
	} `json:"worktree"`
}

func (i wtItem) path() string {
	if i.Path != "" {
		return i.Path
	}
	return i.Worktree.Path
}

func wtItems(bin string) ([]wtItem, error) {
	if bin == "none" {
		return nil, fmt.Errorf("no wt binary")
	}
	out, err := exec.Command(bin, "list", "--format=json").Output()
	if err != nil {
		return nil, err
	}
	var items []wtItem
	if json.Unmarshal(out, &items) == nil {
		return items, nil
	}
	var envelope struct {
		Items []wtItem `json:"items"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return nil, err
	}
	return envelope.Items, nil
}

// primaryWorktree is always listed first, regardless of which worktree the caller
// runs from.
func primaryWorktree(repo *gitrepo.Repo) string {
	for _, line := range lines(git(repo, "worktree", "list", "--porcelain")) {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			return p
		}
	}
	return repo.Toplevel
}

// tmpDir is the home for anything that dies with the command.
func tmpDir() string {
	if t := os.Getenv("TMPDIR"); t != "" {
		return t
	}
	return "/tmp"
}

func configHome() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return x
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config"
	}
	return filepath.Join(home, ".config")
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func capList(list []string, max int) []string {
	if len(list) <= max {
		return list
	}
	return list[:max]
}

func lines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func git(repo *gitrepo.Repo, args ...string) string {
	out, _ := gitErr(repo, args...)
	return out
}

func gitErr(repo *gitrepo.Repo, args ...string) (string, error) {
	out, err := output(repo, "git", args...)
	return strings.TrimRight(string(out), "\n"), err
}

func run(repo *gitrepo.Repo, args ...string) error { return run2(repo, "git", args...) }

func run2(repo *gitrepo.Repo, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = repo.Toplevel
	return cmd.Run()
}

func output(repo *gitrepo.Repo, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = repo.Toplevel
	return cmd.Output()
}
