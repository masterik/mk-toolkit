package cli

import "github.com/spf13/cobra"

func newGateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "The quality gate: what to run, and running it",
	}
	cmd.AddCommand(newGateDetectCmd())
	cmd.AddCommand(newGateRunCmd())
	return cmd
}
