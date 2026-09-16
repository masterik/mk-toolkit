package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/branchscan"
	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/repoconfig"
)

func newBranchScanCmd() *cobra.Command {
	var def string
	var noFetch, noGH bool

	cmd := &cobra.Command{
		Use:   "scan --default <branch>",
		Short: "Classify every local branch and worktree for a repo-wide cleanup",
		Long: "Classify every local branch and worktree: which are merged (locally, or via a PR\n" +
			"git's own merge-base cannot see because of a squash merge), which still have an\n" +
			"open PR, which were never pushed, and which worktree each one owns.\n\n" +
			"Reports candidates. It never deletes a branch, removes a worktree, or touches a\n" +
			"remote — `git fetch --prune` is the one mutation, and it only ever updates this\n" +
			"repo's own remote-tracking refs.\n\n" +
			"`--default` is the branch `mkit facts` already resolved; this command does\n" +
			"not re-derive it, so there is exactly one place that logic lives.\n\n" +
			"`protected=` is the default branch, a local develop-like branch, and every\n" +
			"name pinned in the repo config's `[cleanup] keep`. The default branch is in it\n" +
			"whether or not the keep list names it — a list that omits it is a mistake, not\n" +
			"an instruction. A pinned name with no local branch is reported as\n" +
			"`keep_unknown=`, not an error: a keep list travels with the repo.\n\n" +
			"Columns, in the order they are printed:\n" +
			"  branch       the local branch name\n" +
			"  class        protected | current | merged | merged-pr | open-pr | closed-pr |\n" +
			"               gone | unpushed | tracking — first match wins, in that order\n" +
			"  upstream     the tracking ref, `none` when the branch was never pushed, or\n" +
			"               `gone` when the remote branch was deleted\n" +
			"  merged_into  the branch git's own merge-base says it is contained in\n" +
			"  pr           `#<n>:<state>`, or a cause: gh-missing | gh-unauthenticated |\n" +
			"               no-remote | none\n" +
			"  origin       which worktree owns it, `none` when no worktree has it checked out\n" +
			"  clean        yes | no | missing | error — `error` is a worktree whose status\n" +
			"               failed, never collapsed into yes",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if def == "" {
				return usageErr("usage: mkit branch scan --default <branch> [--no-fetch] [--no-gh]")
			}
			repo, err := gitrepo.Open("")
			if err != nil {
				return err
			}
			// Config is an input, never a permission: a config this binary could
			// not fully honour still scans, with whatever did parse. Load's only
			// error is a file it could not read at all.
			cfg, _, err := repoconfig.Load(repo.Toplevel)
			if err != nil {
				return err
			}
			s, err := branchscan.Run(repo, branchscan.Options{
				Default: def, Keep: cfg.Cleanup.Keep, NoFetch: noFetch, NoGH: noGH,
			})
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			if FromContext(cmd).JSON {
				return writeBranchScanJSON(cmd.OutOrStdout(), s)
			}
			renderBranchScan(cmd.OutOrStdout(), s)
			return nil
		},
	}

	cmd.Flags().StringVar(&def, "default", "", "the default branch, already resolved")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "skip `git fetch --prune`; upstream=gone then reflects a stale local view")
	cmd.Flags().BoolVar(&noGH, "no-gh", false, "skip the GitHub lookup even when gh is present and authenticated")
	return cmd
}

func renderBranchScan(out io.Writer, s *branchscan.Scan) {
	_, _ = fmt.Fprintf(out, "default=%s\n", s.Default)
	_, _ = fmt.Fprintf(out, "develop=%s\n", s.Develop)
	_, _ = fmt.Fprintf(out, "protected=%s\n", strings.Join(s.Protected, ","))
	// Both reported, because they answer different questions: `keep=` is what the
	// config pinned, `keep_unknown=` is which of those names this checkout has no
	// branch for. An empty `keep=` with a non-empty `keep_unknown=` is impossible;
	// a non-empty `keep=` with nothing in `protected=` beyond the default is not.
	_, _ = fmt.Fprintf(out, "keep=%s\n", orNone(strings.Join(s.Keep, ",")))
	_, _ = fmt.Fprintf(out, "keep_unknown=%s\n", orNone(strings.Join(s.KeepUnknown, ",")))
	_, _ = fmt.Fprintf(out, "remote=%s\n", orNone(s.Remote))
	_, _ = fmt.Fprintf(out, "fetch=%s\n", s.Fetch)
	_, _ = fmt.Fprintf(out, "gh=%s\n", s.GH)
	// Reported, not inferred from a short table: a worktree list git could not
	// read is not a repo with fewer worktrees, and `cleanup` plans from these.
	_, _ = fmt.Fprintf(out, "worktrees_state=%s\n", s.WorktreesState)

	_, _ = fmt.Fprintln(out, "branches:")
	for _, b := range s.Branches {
		_, _ = fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n",
			b.Name, b.Class, b.Upstream, dashIfEmpty(strings.Join(b.MergedInto, ",")), dashIfEmpty(b.PR))
	}
	_, _ = fmt.Fprintln(out, "worktrees:")
	for _, w := range s.Worktrees {
		_, _ = fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", w.Branch, w.Path, w.Origin, w.Clean)
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

type branchScanJSON struct {
	Default        string             `json:"default"`
	Develop        string             `json:"develop"`
	Protected      []string           `json:"protected"`
	Keep           []string           `json:"keep"`
	KeepUnknown    []string           `json:"keep_unknown"`
	Remote         string             `json:"remote"`
	Fetch          string             `json:"fetch"`
	GH             string             `json:"gh"`
	Branches       []branchJSON       `json:"branches"`
	Worktrees      []worktreeScanJSON `json:"worktrees"`
	WorktreesState string             `json:"worktrees_state"`
}

type branchJSON struct {
	Name       string   `json:"name"`
	Class      string   `json:"class"`
	Upstream   string   `json:"upstream"`
	MergedInto []string `json:"merged_into"`
	PR         string   `json:"pr,omitempty"`
}

type worktreeScanJSON struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Origin string `json:"origin"`
	Clean  string `json:"clean"`
}

func writeBranchScanJSON(out io.Writer, s *branchscan.Scan) error {
	j := branchScanJSON{
		Default: s.Default, Develop: s.Develop, Protected: s.Protected,
		Keep: nonNil(s.Keep), KeepUnknown: nonNil(s.KeepUnknown),
		Remote: s.Remote, Fetch: s.Fetch, GH: s.GH, WorktreesState: s.WorktreesState,
		Branches:  make([]branchJSON, 0, len(s.Branches)),
		Worktrees: make([]worktreeScanJSON, 0, len(s.Worktrees)),
	}
	for _, b := range s.Branches {
		j.Branches = append(j.Branches, branchJSON{
			Name: b.Name, Class: string(b.Class), Upstream: b.Upstream,
			MergedInto: nonNil(b.MergedInto), PR: b.PR,
		})
	}
	for _, w := range s.Worktrees {
		j.Worktrees = append(j.Worktrees, worktreeScanJSON{
			Branch: w.Branch, Path: w.Path, Origin: w.Origin, Clean: w.Clean,
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(j)
}
