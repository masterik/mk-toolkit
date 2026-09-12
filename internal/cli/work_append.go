package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/core/gitrepo"
	"github.com/masterik/mk-toolkit/internal/core/worklog"
)

func newWorkAppendCmd() *cobra.Command {
	var branch, step, gist, artifact string
	var assume []string

	cmd := &cobra.Command{
		Use:   "append",
		Short: "Record one finished step",
		Long: "One record per finished step, appended after the step's report is produced.\n\n" +
			"A write that fails is an error here — this is a command someone invoked. It is the\n" +
			"*skill* that treats the append as best-effort: contract rule 4 says a recorded fact\n" +
			"is an input, never a permission, so a log that could not be written costs the next\n" +
			"step a derivation and costs this one nothing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := FromContext(cmd)
			repo, err := gitrepo.Open("")
			if err != nil {
				// Exit 1, like `repo profile`: no work tree is the environment,
				// not a mistake at the command line, which is what 2 means.
				return err
			}
			if !worklog.ValidStep(step) {
				return usageErr("unknown --step %q (one of: %s)", step, strings.Join(worklog.Steps, ", "))
			}
			if gist == "" {
				return usageErr("--gist is required: a record with no gist is one no later step can read")
			}

			fp, cause := worklog.Fingerprint(repo.Toplevel)
			log := worklog.Open(repo, branch)
			rec := worklog.Record{
				Step:        step,
				Fingerprint: fp,
				Artifact:    artifact,
				Gist:        gist,
				Assumptions: assume,
			}
			if err := log.Append(rec); err != nil {
				return err
			}

			if opts.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(struct {
					Branch string `json:"branch"`
					Path   string `json:"path"`
					Step   string `json:"step"`
					// Cause is why there is no fingerprint, "" when there is one.
					// A degradation stays a reported fact rather than a silent
					// empty field.
					Cause string `json:"cause,omitempty"`
				}{log.Branch(), log.Path(), step, cause})
			}
			if cause != "" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "note:", cause)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "recorded %s on %s\n", step, log.Branch())
			return nil
		},
	}

	cmd.Flags().StringVar(&step, "step", "", "which step finished (required)")
	cmd.Flags().StringVar(&gist, "gist", "", "one line: what this step concluded (required)")
	cmd.Flags().StringVar(&artifact, "artifact", "", "what it produced — an issue URL, a path, a run directory, a commit range")
	cmd.Flags().StringArrayVar(&assume, "assume", nil, "something the step derived because it could not find it (repeatable)")
	cmd.Flags().StringVar(&branch, "branch", "", "record against another branch (default: the current branch)")
	return cmd
}
