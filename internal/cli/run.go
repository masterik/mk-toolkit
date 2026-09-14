package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/scratch"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Open and prune mkit's per-run scratch directories",
	}
	cmd.AddCommand(newRunOpenCmd(), newRunPruneCmd())
	return cmd
}

func newRunOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <skill>",
		Short: "Open a fresh run directory and print its absolute path",
		Long: "Open a fresh mkit run directory under <toplevel>/.mkit/ and print its absolute\n" +
			"path — nothing else, so it is safe in a command substitution.\n\n" +
			"  mkit run open review  ->  /repo/.mkit/review-20260819T111347Z-RPfCbj\n\n" +
			"The directory is unique (two runs in one second cannot merge and clobber each\n" +
			"other's logs), absolute (the path is handed to subagents and reused across\n" +
			"shells), inside the work tree (a linked worktree gets its own), and mkit's\n" +
			"scratch is excluded before the first write.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := gitrepo.Open("")
			if err != nil {
				return err
			}
			dir, err := scratch.RunDir(repo, args[0])
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			if FromContext(cmd).JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{"run": dir})
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), dir)
			return nil
		},
	}
}

func newRunPruneCmd() *cobra.Command {
	var keep int
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove all but the newest run directories, per skill",
		Long: "Remove all but the newest --keep run directories per skill.\n\n" +
			"One pass per skill, so a busy `review` never evicts the only `pr` run; a\n" +
			"directory touched in the last 60 minutes is skipped as live, since age-ranked\n" +
			"eviction alone cannot see a run still being written. Only `<skill>-*`\n" +
			"directories are in range, which is what keeps gate.jsonl out of it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := gitrepo.Open("")
			if err != nil {
				return err
			}
			res, err := scratch.Prune(repo, keep)
			if err != nil {
				return &ExitError{Code: 2, Msg: err.Error()}
			}
			out := cmd.OutOrStdout()
			if FromContext(cmd).JSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]int{
					"removed": res.Removed, "kept": res.Kept, "live": res.Live,
				})
			}
			_, _ = fmt.Fprintf(out, "pruned %d run dir(s), kept %d", res.Removed, res.Kept)
			if res.Live > 0 {
				_, _ = fmt.Fprintf(out, ", skipped %d still active (<60m)", res.Live)
			}
			_, _ = fmt.Fprintln(out)
			return nil
		},
	}
	cmd.Flags().IntVar(&keep, "keep", 5, "run directories to keep per skill")
	return cmd
}
