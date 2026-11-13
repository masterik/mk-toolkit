package cli

import "github.com/spf13/cobra"

func newGateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "The quality gate: what to run",
		// Runnable only so cobra validates the argument: a parent without a Run
		// prints its help and exits 0 for any argument, and `mkit gate run` (gone
		// with the ledger) must fail as the unknown command it now is, not look
		// like it succeeded.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newGateDetectCmd())
	return cmd
}
