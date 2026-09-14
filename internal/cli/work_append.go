package cli

import (
	"encoding/json"
	"errors"
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
			log := worklog.Open(repo, branch)

			// The fingerprint is always of the tree in front of us, and `head()`
			// deliberately resolves the *named* branch's tip — so a cross-branch
			// append would pair one branch's head with another branch's content.
			// A later reader on that branch compares fingerprints to decide whether
			// a gist still describes the tree it is looking at; an accidental match
			// reads "this exact content was reviewed" over content nothing ran
			// against. No fingerprint is the honest answer, reported as a cause the
			// same way an unreachable payload is.
			// Keyed on the *flag*, not on the resolved name: with no --branch,
			// Open picks whatever this checkout is on, and on a detached HEAD that
			// is `detached~<sha>` while Repo.Branch() is "" — comparing the two
			// resolved names would call every detached append cross-branch and
			// throw away a fingerprint that was perfectly good.
			var fp, cause string
			if branch == "" || branch == repo.Branch() {
				fp, cause = worklog.Fingerprint(repo.Toplevel)
			} else {
				cause = "no fingerprint: --branch " + branch +
					" is not checked out, and this tree is not its content"
			}
			rec := worklog.Record{
				Step:        step,
				Fingerprint: fp,
				Artifact:    artifact,
				Gist:        gist,
				Assumptions: assume,
			}
			if err := log.Append(rec); err != nil {
				// Append owns both rules; this only re-codes them as exit 2.
				// Checking them here as well would be two copies of one rule —
				// the failure the layering exists to prevent — and would leave
				// core's sentinels exported with no caller at all.
				switch {
				case errors.Is(err, worklog.ErrBadStep):
					return usageErr("unknown --step %q (one of: %s)", step, strings.Join(worklog.Steps, ", "))
				case errors.Is(err, worklog.ErrNoGist):
					return usageErr("--gist is required: a record with no gist is one no later step can read")
				}
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
