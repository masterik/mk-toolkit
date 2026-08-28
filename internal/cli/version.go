package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/masterik/mk-toolkit/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the mkit version",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := FromContext(cmd)
			if opts.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
					"version": buildinfo.Version,
					"commit":  buildinfo.Commit,
					"date":    buildinfo.Date,
				})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "mkit %s (%s, %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
			return err
		},
	}
}
