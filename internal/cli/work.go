package cli

import "github.com/spf13/cobra"

func newWorkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Read and write this branch's worklog",
		Long: "The per-branch record of what each workflow step concluded, at\n" +
			"`<toplevel>/.mkit/work/<branch>.jsonl`.\n\n" +
			"A step appends one record when it finishes; a later step reads them to be cheaper\n" +
			"and better informed — never to decide whether it is allowed to run. See the\n" +
			"payload's _shared/references/workflow-contract.md.",
	}
	cmd.AddCommand(newWorkShowCmd(), newWorkAppendCmd())
	return cmd
}
