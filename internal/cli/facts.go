package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/facts"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/scratch"
)

func newFactsCmd() *cobra.Command {
	var opt facts.Options

	cmd := &cobra.Command{
		Use:   "facts <skill>",
		Short: "Every starting fact a skill needs, in one call — and the run directory",
		Long: "Gather every read-only fact an mkit skill needs to start, in one call, and open\n" +
			"the run directory while we are here.\n\n" +
			"  mkit facts commit\n" +
			"  mkit facts finish --base main\n" +
			"  mkit facts pr --base main --gh\n\n" +
			"It reports. It never acts: no staging, no merging, no `wt` invocation, and the\n" +
			"only thing it writes is the run directory it opens.\n\n" +
			"Output is `key=value` lines, then optional `block:` sections. A cause needing a\n" +
			"sentence goes in the trailing `notes:` block, never on a `key=value` line — some\n" +
			"of those lines pack more than one pair.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opt.Skill = args[0]
			if strings.HasPrefix(opt.Skill, "-") {
				return usageErr("first argument must be the skill name, got: %s", opt.Skill)
			}
			repo, err := gitrepo.Open("")
			if err != nil {
				return err
			}
			f, gerr := facts.Gather(repo, opt)
			if f == nil {
				// 2 is "you typed it wrong" and belongs to the slug and the
				// revision flags, which are checked before anything is done. A
				// run directory that could not be created is an operational
				// failure and exits 1, or the caller goes looking at its argv.
				code := 1
				var slug *scratch.SlugError
				var rev *facts.RevError
				if errors.As(gerr, &slug) || errors.As(gerr, &rev) {
					code = 2
				}
				return &ExitError{Code: code, Msg: gerr.Error()}
			}

			out := cmd.OutOrStdout()
			if FromContext(cmd).JSON {
				if err := writeFactsJSON(out, f); err != nil {
					return err
				}
			} else {
				renderFacts(out, f, opt)
			}

			// An unresolvable --base says so on stdout *and then fails*: it used
			// to fall through silently and still exit 0, so `finish`/`pr` got a
			// fact set with no commits_ahead_of_base and no way to tell that from
			// a base with nothing on it.
			var bad *facts.ErrUnresolvableBase
			if errors.As(gerr, &bad) {
				return &ExitError{Code: 1, Msg: bad.Error()}
			}
			// --range gets the same treatment, for the same reason: a range that
			// resolves to nothing and a range that does not resolve at all are
			// the same output otherwise.
			var badRange *facts.ErrUnresolvableRange
			if errors.As(gerr, &badRange) {
				return &ExitError{Code: 1, Msg: badRange.Error()}
			}
			return gerr
		},
	}

	cmd.Flags().StringVar(&opt.Base, "base", "", "compare against this branch; an unresolvable one is fatal")
	cmd.Flags().StringVar(&opt.Range, "range", "",
		"an explicit git range, instead of the working-tree scopes; an unresolvable one is fatal")
	cmd.Flags().BoolVar(&opt.GH, "gh", false, "ask GitHub whether this branch already has a PR")
	cmd.Flags().BoolVar(&opt.NoRun, "no-run", false, "do not open a run directory")
	cmd.Flags().IntVar(&opt.StatusMax, "status-max", 60, "maximum status entries to print")
	cmd.Flags().IntVar(&opt.FilesMax, "files-max", 200, "maximum file names to print per scope")
	return cmd
}

func renderFacts(out io.Writer, f *facts.Facts, opt facts.Options) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(out, format, a...) }

	if f.Run != "" {
		p("run=%s\n", f.Run)
	}
	p("plugin=%s\nrefs=%s\n", orNone(f.Plugin), orNone(f.Refs))
	p("toplevel=%s\ngit_dir=%s\ncommon_dir=%s\nprimary=%s\n",
		f.Toplevel, f.GitDir, f.CommonDir, f.Primary)
	p("linked=%s\nworktrees=%d\n", yesNo(f.Linked), f.Worktrees)
	p("tmp=%s\n", f.TMP)
	p("run_ignored=%s\n", yesNo(f.RunIgnored))
	p("config=%s\nconfig_state=%s\n", f.Config, f.ConfigState)
	p("user_dir=%s\nuser_dir_writable=%s\n", orNone(f.UserDir), yesNo(f.UserDirOK))
	p("git_bin=%s\n", f.GitBin)
	p("worktree_origin=%s\ncleanup_path=%s\nwt_lists_this=%s\nwt_config=%s\n",
		f.WorktreeOrigin, f.CleanupPath, f.WTListsThis, f.WTConfig)
	p("wt_bin=%s\n", f.WTBin)

	p("branch=%s\ndetached=%s\n", f.Branch, yesNo(f.Detached))
	p("upstream=%s\npushed=%s\n", f.Upstream, yesNo(f.Pushed))
	p("remote=%s\n", orNone(f.Remote))
	p("default_branch=%s\n", f.DefaultBranch)
	if f.Base != "" {
		p("base=%s\n", f.Base)
	}
	if f.HasUpstream {
		p("ahead=%d behind=%d\n", f.Ahead, f.Behind)
	}

	p("clean=%s\n", yesNo(f.Clean))
	p("staged=%d unstaged=%d untracked=%d conflicted=%d\n",
		f.Staged, f.Unstaged, f.Untracked, f.Conflicted)
	if f.StatusTotal > 0 {
		p("status:\n")
		for _, l := range f.Status {
			p("%s\n", l)
		}
		if n := f.StatusTotal - len(f.Status); n > 0 {
			p("... %d more entries not shown (status_max=%d)\n", n, opt.StatusMax)
		}
	}

	for _, ns := range f.Scopes {
		renderScope(out, ns, opt.FilesMax)
	}

	if f.RangeState != "" {
		p("range_state=%s\n", f.RangeState)
	}
	if f.BaseState != "" {
		p("base_state=%s\n", f.BaseState)
	}
	if f.BaseState == "ok" {
		p("commits_ahead_of_base=%d\n", f.CommitsAheadOfBase)
		if len(f.Commits) > 0 {
			p("commits:\n")
			for _, c := range f.Commits {
				p("%s\n", c)
			}
		}
		p("ff_from_base=%s\n", f.FFFromBase)
	}

	p("codeowners=%s\n", f.CodeOwners)
	if f.PR != "" {
		if f.PRState != "" {
			p("pr=%s pr_state=%s pr_draft=%s\n", f.PR, f.PRState, f.PRDraft)
		} else {
			p("pr=%s\n", f.PR)
		}
	}

	if len(f.Notes) > 0 {
		p("notes:\n")
		for _, n := range f.Notes {
			p("  %s\n", n)
		}
	}
}

// renderScope prints one diff scope. `untracked` has no stat line — `git diff`
// never lists an untracked file, so there is no diff to summarize.
func renderScope(out io.Writer, ns facts.NamedScope, filesMax int) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(out, format, a...) }
	if ns.Scope.Stat != "" {
		p("%s_stat=%s\n", ns.Label, ns.Scope.Stat)
	}
	p("%s_files=%d\n", ns.Label, ns.Scope.Files)
	if ns.Scope.Files == 0 {
		return
	}
	p("%s_file_list:\n", ns.Label)
	for _, l := range ns.Scope.List {
		p("%s\n", l)
	}
	if n := ns.Scope.Files - len(ns.Scope.List); n > 0 {
		p("... %d more files not shown (files_max=%d)\n", n, filesMax)
	}
}

type factsJSON struct {
	Run            string               `json:"run,omitempty"`
	Plugin         string               `json:"plugin,omitempty"`
	Refs           string               `json:"refs,omitempty"`
	PluginCause    string               `json:"plugin_cause,omitempty"`
	Toplevel       string               `json:"toplevel"`
	GitDir         string               `json:"git_dir"`
	CommonDir      string               `json:"common_dir"`
	Primary        string               `json:"primary"`
	Linked         bool                 `json:"linked"`
	Worktrees      int                  `json:"worktrees"`
	TMP            string               `json:"tmp"`
	RunIgnored     bool                 `json:"run_ignored"`
	UserDir        string               `json:"user_dir"`
	UserDirOK      bool                 `json:"user_dir_writable"`
	Config         string               `json:"config"`
	ConfigState    string               `json:"config_state"`
	GitBin         string               `json:"git_bin"`
	WorktreeOrigin string               `json:"worktree_origin"`
	CleanupPath    string               `json:"cleanup_path"`
	WTListsThis    string               `json:"wt_lists_this"`
	WTConfig       string               `json:"wt_config"`
	WTBin          string               `json:"wt_bin"`
	Branch         string               `json:"branch"`
	Detached       bool                 `json:"detached"`
	Upstream       string               `json:"upstream"`
	Pushed         bool                 `json:"pushed"`
	Remote         string               `json:"remote"`
	DefaultBranch  string               `json:"default_branch"`
	Base           string               `json:"base,omitempty"`
	Ahead          int                  `json:"ahead"`
	Behind         int                  `json:"behind"`
	Clean          bool                 `json:"clean"`
	Staged         int                  `json:"staged"`
	Unstaged       int                  `json:"unstaged"`
	Untracked      int                  `json:"untracked"`
	Conflicted     int                  `json:"conflicted"`
	Status         []string             `json:"status"`
	StatusTotal    int                  `json:"status_total"`
	Scopes         map[string]scopeJSON `json:"scopes"`
	BaseState      string               `json:"base_state,omitempty"`
	RangeState     string               `json:"range_state,omitempty"`
	CommitsAhead   int                  `json:"commits_ahead_of_base"`
	Commits        []string             `json:"commits,omitempty"`
	FFFromBase     string               `json:"ff_from_base,omitempty"`
	CodeOwners     string               `json:"codeowners"`
	PR             string               `json:"pr,omitempty"`
	PRState        string               `json:"pr_state,omitempty"`
	PRDraft        string               `json:"pr_draft,omitempty"`
	Notes          []string             `json:"notes"`
}

type scopeJSON struct {
	Stat  string   `json:"stat,omitempty"`
	Files int      `json:"files"`
	List  []string `json:"list"`
}

func writeFactsJSON(out io.Writer, f *facts.Facts) error {
	j := factsJSON{
		Run: f.Run, Plugin: f.Plugin, Refs: f.Refs, PluginCause: f.PluginCause,
		Toplevel: f.Toplevel, GitDir: f.GitDir, CommonDir: f.CommonDir, Primary: f.Primary,
		Linked: f.Linked, Worktrees: f.Worktrees, TMP: f.TMP, RunIgnored: f.RunIgnored,
		UserDir: f.UserDir, UserDirOK: f.UserDirOK,
		Config: f.Config, ConfigState: string(f.ConfigState), GitBin: f.GitBin,
		WorktreeOrigin: f.WorktreeOrigin, CleanupPath: f.CleanupPath,
		WTListsThis: f.WTListsThis, WTConfig: f.WTConfig, WTBin: f.WTBin,
		Branch: f.Branch, Detached: f.Detached, Upstream: f.Upstream, Pushed: f.Pushed,
		Remote: f.Remote, DefaultBranch: f.DefaultBranch, Base: f.Base,
		Ahead: f.Ahead, Behind: f.Behind,
		Clean: f.Clean, Staged: f.Staged, Unstaged: f.Unstaged, Untracked: f.Untracked,
		Conflicted: f.Conflicted, Status: nonNil(f.Status), StatusTotal: f.StatusTotal,
		Scopes:    map[string]scopeJSON{},
		BaseState: f.BaseState, RangeState: f.RangeState, CommitsAhead: f.CommitsAheadOfBase,
		Commits: f.Commits, FFFromBase: f.FFFromBase,
		CodeOwners: f.CodeOwners, PR: f.PR, PRState: f.PRState, PRDraft: f.PRDraft,
		Notes: nonNil(f.Notes),
	}
	for _, ns := range f.Scopes {
		j.Scopes[ns.Label] = scopeJSON{Stat: ns.Scope.Stat, Files: ns.Scope.Files, List: nonNil(ns.Scope.List)}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(j)
}
