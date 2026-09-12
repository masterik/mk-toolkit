package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/worklog"
)

func newWorkShowCmd() *cobra.Command {
	var branch string
	var steps []string
	var limit int

	cmd := &cobra.Command{
		Use:   "show",
		Short: "What has run on this branch, and what each step concluded",
		Long: "Records oldest first, so the last line is the most recent thing that happened.\n\n" +
			"A branch nothing has run on is zero records and exit 0, not an error — every step\n" +
			"is entry-capable, and being first is the normal case, not a problem to report.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := FromContext(cmd)
			repo, err := gitrepo.Open("")
			if err != nil {
				return err
			}
			log := worklog.Open(repo, branch)
			recs, err := log.Show(worklog.Query{Steps: steps, Limit: limit})
			if err != nil {
				return err
			}
			if opts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(struct {
					Branch  string           `json:"branch"`
					Path    string           `json:"path"`
					Records []worklog.Record `json:"records"`
				}{log.Branch(), log.Path(), recs})
			}
			renderWorklog(cmd.OutOrStdout(), log, recs)
			return nil
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "read another branch's log (default: the current branch)")
	cmd.Flags().StringArrayVar(&steps, "step", nil, "keep only these steps (repeatable)")
	cmd.Flags().IntVar(&limit, "limit", 0, "keep only the newest N records")
	return cmd
}

func renderWorklog(out io.Writer, log *worklog.Log, recs []worklog.Record) {
	_, _ = fmt.Fprintf(out, "%s — %d record(s)\n", log.Branch(), len(recs))
	for _, r := range recs {
		fp := r.Fingerprint
		if fp == "" {
			fp = "-"
		}
		_, _ = fmt.Fprintf(out, "  %s  %-10s fp:%-16s %s\n", r.TS, r.Step, fp, r.Gist)
		if r.Artifact != "" {
			_, _ = fmt.Fprintf(out, "  %*s  %s\n", len(r.TS), "", r.Artifact)
		}
		if len(r.Assumptions) > 0 {
			_, _ = fmt.Fprintf(out, "  %*s  assumed: %s\n", len(r.TS), "", strings.Join(r.Assumptions, "; "))
		}
	}
}
