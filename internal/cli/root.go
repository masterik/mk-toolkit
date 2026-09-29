// Package cli builds the cobra root command and command tree.
package cli

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Help groups: what a command is about, not who calls it — most are used by
// both a person and a skill. Presentation only; no command moves.
const (
	groupSetup   = "setup"
	groupState   = "state"
	groupReview  = "review"
	groupCleanup = "cleanup"
)

type optionsKey struct{}

// Options is the front-end contract every command reads instead of checking
// flags or the terminal itself — resolved once, in the root's PersistentPreRun.
type Options struct {
	JSON        bool
	Interactive bool
	// Pretty is styled, human-facing output: stdout is a terminal, and neither
	// --json nor --no-tui asked for the machine forms. Distinct from Interactive,
	// which is a TUI that reads keys — a report is pretty without prompting. A
	// pipe is never Pretty, which is what keeps the `key=value` text the skills
	// parse byte-for-byte unchanged.
	Pretty bool
	Yes    bool
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
			pretty := !noTUI && !jsonFlag && term.IsTerminal(int(os.Stdout.Fd()))
			opts := Options{JSON: jsonFlag, Interactive: interactive, Pretty: pretty, Yes: yes}
			cmd.SetContext(context.WithValue(cmd.Context(), optionsKey{}, opts))
		},
	}

	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output JSON instead of human-readable text")
	root.PersistentFlags().BoolVar(&noTUI, "no-tui", false, "disable the interactive TUI and styled output, even on a terminal")
	root.PersistentFlags().BoolVar(&yes, "yes", false, "assume yes to any confirmation")

	root.AddGroup(
		&cobra.Group{ID: groupSetup, Title: "Setup & health:"},
		&cobra.Group{ID: groupState, Title: "Repo & branch state:"},
		&cobra.Group{ID: groupReview, Title: "Quality gate & review:"},
		&cobra.Group{ID: groupCleanup, Title: "Housekeeping & audit:"},
	)
	root.SetHelpCommandGroupID(groupSetup)
	root.SetCompletionCommandGroupID(groupSetup)

	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			root.AddCommand(c)
		}
	}
	add(groupSetup, newInitCmd(), newDoctorCmd(), newVersionCmd())
	add(groupState, newFactsCmd(), newRepoCmd(), newBranchCmd(), newWorklogCmd())
	add(groupReview, newGateCmd(), newFindingsCmd())
	add(groupCleanup, newScratchCmd(), newCacheCmd(), newAuditCmd())

	installPrettyHelp(root)

	return root
}
