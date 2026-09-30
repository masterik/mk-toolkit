package cli

import "github.com/spf13/cobra"

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Prune stale local provider caches (transcripts, caches, scratch state)",
	}
	cmd.AddCommand(newCachePruneCmd())
	return cmd
}
