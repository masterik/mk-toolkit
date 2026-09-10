package cli

import "github.com/spf13/cobra"

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Inspect how this repository is configured",
	}
	cmd.AddCommand(newRepoProfileCmd())
	return cmd
}
