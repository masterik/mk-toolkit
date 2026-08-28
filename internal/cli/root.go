// Package cli builds the cobra root command and command tree.
package cli

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type optionsKey struct{}

// Options is the front-end contract every command reads instead of checking
// flags or the terminal itself — resolved once, in the root's PersistentPreRun.
type Options struct {
	JSON        bool
	Interactive bool
	Yes         bool
}

// FromContext returns the Options resolved for the running command.
func FromContext(cmd *cobra.Command) Options {
	opts, _ := cmd.Context().Value(optionsKey{}).(Options)
	return opts
}

// NewRoot builds the mkit root command.
func NewRoot() *cobra.Command {
	var jsonFlag, noTUI, yes bool

	root := &cobra.Command{
		Use:           "mkit",
		Short:         "mkit — coding-workflow toolkit",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// --json implies non-interactive; so does a piped stdout.
			interactive := !noTUI && !jsonFlag && term.IsTerminal(int(os.Stdout.Fd()))
			opts := Options{JSON: jsonFlag, Interactive: interactive, Yes: yes}
			cmd.SetContext(context.WithValue(cmd.Context(), optionsKey{}, opts))
		},
	}

	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output JSON instead of human-readable text")
	root.PersistentFlags().BoolVar(&noTUI, "no-tui", false, "disable the interactive TUI even on a terminal")
	root.PersistentFlags().BoolVar(&yes, "yes", false, "assume yes to any confirmation")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newStorageCmd())

	return root
}
