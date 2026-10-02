package cli

import "github.com/spf13/cobra"

func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report what the sandbox and permission gate did in past sessions, against the current settings",
	}
	cmd.AddCommand(newAuditSandboxCmd(), newAuditSessionsCmd())
	return cmd
}
