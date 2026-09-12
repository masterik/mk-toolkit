package cli

import "github.com/spf13/cobra"

func newBranchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Inspect this repo's local branches",
	}
	cmd.AddCommand(newBranchScanCmd())
	return cmd
}
