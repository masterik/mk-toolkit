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
	var pathOnly bool
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
			// --path is the whole answer: one line, no decoding. `finish` needs
			// this path to retire a log whose branch is gone, and asking for it
			// through --json meant piping to jq — the one dependency M5 removed
			// from the payload, which ships no executable code and no jq.
			if pathOnly {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), log.Path())
				return nil
			}
			recs, err := log.Show(worklog.Query{Steps: steps, Limit: limit})
			if err != nil {
				return err
			}
			// The tree as it is right now, so a reader can tell which records still
			// describe it. Without it the fingerprint on a record is a value with
			// nothing to compare against, and "is this gist still true?" is not a
			// question the log can answer.
			fp, cause := worklog.Fingerprint(repo.Toplevel)
			if opts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(struct {
					Branch string `json:"branch"`
					Path   string `json:"path"`
					// Fingerprint is the *current* tree's, not any record's.
					Fingerprint string           `json:"fingerprint"`
					Cause       string           `json:"cause,omitempty"`
					Records     []worklog.Record `json:"records"`
				}{log.Branch(), log.Path(), fp, cause, recs})
			}
			renderWorklog(cmd.OutOrStdout(), log, fp, recs)
			return nil
		},
	}

	cmd.Flags().BoolVar(&pathOnly, "path", false, "print this branch's worklog path and nothing else")
	cmd.Flags().StringVar(&branch, "branch", "", "read another branch's log (default: the current branch)")
	cmd.Flags().StringArrayVar(&steps, "step", nil, "keep only these steps (repeatable)")
	cmd.Flags().IntVar(&limit, "limit", 0, "keep only the newest N records")
	return cmd
}

func renderWorklog(out io.Writer, log *worklog.Log, current string, recs []worklog.Record) {
	// `current` keeps its own value throughout: it is the comparison, and a display
	// placeholder written over it turns "we could not fingerprint the tree" into a
	// fingerprint that matches no record — which is how every record came out
	// `(stale)`. Only the header substitutes.
	header := current
	if header == "" {
		header = "-"
	}
	_, _ = fmt.Fprintf(out, "%s — %d record(s), tree now fp:%s\n", log.Branch(), len(recs), header)
	for _, r := range recs {
		fp := r.Fingerprint
		switch {
		case fp == "":
			fp = "-"
		case current == "":
			// Nothing to compare against: a record is not stale merely because
			// this run could not fingerprint the tree.
			fp += " (unknown)"
		case fp == current:
			fp += " (current)"
		default:
			fp += " (stale)"
		}
		_, _ = fmt.Fprintf(out, "  %s  %-10s fp:%-26s %s\n", r.TS, r.Step, fp, r.Gist)
		if r.Artifact != "" {
			_, _ = fmt.Fprintf(out, "  %*s  %s\n", len(r.TS), "", r.Artifact)
		}
		if len(r.Assumptions) > 0 {
			_, _ = fmt.Fprintf(out, "  %*s  assumed: %s\n", len(r.TS), "", strings.Join(r.Assumptions, "; "))
		}
	}
}
