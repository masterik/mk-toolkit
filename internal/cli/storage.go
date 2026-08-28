package cli

import "github.com/spf13/cobra"

func newStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Manage local provider storage (transcripts, caches, scratch state)",
	}
	cmd.AddCommand(newStoragePruneCmd())
	return cmd
}
