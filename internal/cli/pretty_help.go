package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/masterik/mk-toolkit/internal/tui/ui"
)

// installPrettyHelp swaps cobra's help for a styled one on a terminal. Anywhere
// else — a pipe, an agent, --json, --no-tui — the stock text is printed, byte for
// byte, so a skill that reads `--help` sees what it always did.
//
// Help short-circuits before PersistentPreRun, so Options is not resolved yet;
// the same three inputs are read here directly.
func installPrettyHelp(root *cobra.Command) {
	stock := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if !helpIsPretty(cmd) {
			stock(cmd, args)
			return
		}
		prettyHelp(cmd.OutOrStdout(), cmd)
	})
}

func helpIsPretty(cmd *cobra.Command) bool {
	f, ok := cmd.OutOrStdout().(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return false
	}
	for _, name := range []string{"json", "no-tui"} {
		if fl := cmd.Flag(name); fl != nil && fl.Value.String() == "true" {
			return false
		}
	}
	return true
}

func prettyHelp(out io.Writer, cmd *cobra.Command) {
	s := ui.New(out)

	_, _ = fmt.Fprintln(out, s.Title(cmd.CommandPath(), ""))
	if desc := strings.TrimSpace(cmd.Long); desc != "" {
		_, _ = fmt.Fprintf(out, "\n%s\n", desc)
	} else if cmd.Short != "" {
		_, _ = fmt.Fprintf(out, "\n%s\n", cmd.Short)
	}

	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section("usage"))
	if cmd.Runnable() {
		_, _ = fmt.Fprintf(out, "  %s\n", s.Bold(cmd.UseLine()))
	}
	if cmd.HasAvailableSubCommands() {
		_, _ = fmt.Fprintf(out, "  %s\n", s.Bold(cmd.CommandPath()+" [command]"))
	}
	if cmd.Example != "" {
		_, _ = fmt.Fprintf(out, "\n%s\n%s\n", s.Section("examples"), cmd.Example)
	}

	if cmd.HasAvailableSubCommands() {
		nameW := 0
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() || c.Name() == "help" {
				if w := lipgloss.Width(c.Name()); w > nameW {
					nameW = w
				}
			}
		}
		row := func(c *cobra.Command) {
			pad := strings.Repeat(" ", nameW-lipgloss.Width(c.Name()))
			_, _ = fmt.Fprintf(out, "  %s%s  %s\n", s.Accent(c.Name()), pad, s.Dim(c.Short))
		}
		// Groups first, in the order they were declared; whatever has none after.
		seen := map[*cobra.Command]bool{}
		for _, g := range cmd.Groups() {
			var members []*cobra.Command
			for _, c := range cmd.Commands() {
				if c.GroupID == g.ID && (c.IsAvailableCommand() || c.Name() == "help") {
					members = append(members, c)
				}
			}
			if len(members) == 0 {
				continue
			}
			_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(strings.TrimSuffix(g.Title, ":")))
			for _, c := range members {
				seen[c] = true
				row(c)
			}
		}
		var rest []*cobra.Command
		for _, c := range cmd.Commands() {
			if !seen[c] && (c.IsAvailableCommand() || c.Name() == "help") {
				rest = append(rest, c)
			}
		}
		if len(rest) > 0 {
			title := "commands"
			if len(seen) > 0 {
				title = "more commands"
			}
			_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(title))
			for _, c := range rest {
				row(c)
			}
		}
	}

	prettyFlags(out, s, "flags", cmd.LocalFlags())
	prettyFlags(out, s, "global flags", cmd.InheritedFlags())

	if cmd.HasAvailableSubCommands() {
		_, _ = fmt.Fprintf(out, "\n%s\n", s.Dim(fmt.Sprintf("Use \"%s [command] --help\" for more about a command.", cmd.CommandPath())))
	}
}

func prettyFlags(out io.Writer, s *ui.S, title string, fs *pflag.FlagSet) {
	type entry struct{ left, usage string }
	var entries []entry
	leftW := 0
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		left := "    --" + f.Name
		if f.Shorthand != "" {
			left = "-" + f.Shorthand + ", --" + f.Name
		}
		varname, usage := pflag.UnquoteUsage(f)
		if varname != "" {
			left += " " + varname
		}
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && f.DefValue != "[]" {
			usage += " " + s.Dim("(default "+f.DefValue+")")
		}
		entries = append(entries, entry{left, usage})
		if w := lipgloss.Width(left); w > leftW {
			leftW = w
		}
	})
	if len(entries) == 0 {
		return
	}
	_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(title))
	for _, e := range entries {
		pad := strings.Repeat(" ", leftW-lipgloss.Width(e.left))
		_, _ = fmt.Fprintf(out, "  %s%s  %s\n", s.Accent(e.left), pad, e.usage)
	}
}
