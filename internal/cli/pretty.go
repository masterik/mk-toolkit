package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/masterik/mk-toolkit/internal/buildinfo"
	"github.com/masterik/mk-toolkit/internal/core/doctor"
	"github.com/masterik/mk-toolkit/internal/tui/ui"
)

// The pretty renderers below are the human forms of the commands that have one.
// They are reached only when Options.Pretty is set — a terminal, no --json, no
// --no-tui — so the plain renderers beside them stay the machine contract.

// wrapIndent wraps text to width and indents every line after the first.
func wrapIndent(text string, width, indent int) string {
	if width < 20 {
		width = 20
	}
	lines := strings.Split(lipgloss.NewStyle().Width(width).Render(text), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
		if i > 0 {
			lines[i] = strings.Repeat(" ", indent) + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func prettyVersion(out io.Writer) {
	s := ui.New(out)
	_, _ = fmt.Fprintf(out, "%s %s\n", s.Title("mkit", buildinfo.Version),
		s.Dim(fmt.Sprintf("%s · %s", buildinfo.Commit, buildinfo.Date)))
}

func prettyDoctor(out io.Writer, r *doctor.Report) {
	s := ui.New(out)
	_, _ = fmt.Fprintln(out, s.Title("mkit doctor", buildinfo.Version))

	nameW := 0
	for _, c := range r.Checks {
		if w := lipgloss.Width(c.Name); w > nameW {
			nameW = w
		}
	}
	if nameW > 24 {
		nameW = 24
	}
	indent := 5 + nameW + 2

	var order []string
	byGroup := map[string][]doctor.Check{}
	for _, c := range r.Checks {
		if _, seen := byGroup[c.Group]; !seen {
			order = append(order, c.Group)
		}
		byGroup[c.Group] = append(byGroup[c.Group], c)
	}
	for _, g := range order {
		_, _ = fmt.Fprintf(out, "\n%s\n", s.Section(g))
		for _, c := range byGroup[g] {
			name := c.Name
			pad := nameW - lipgloss.Width(name)
			if pad < 0 {
				pad = 0
			}
			detail := wrapIndent(c.Detail, s.Width()-indent, indent)
			_, _ = fmt.Fprintf(out, "  %s  %s%s  %s\n", s.Icon(string(c.Status)), name, strings.Repeat(" ", pad), detail)
			if c.Remedy != "" {
				rem := wrapIndent(c.Remedy, s.Width()-indent-2, indent+2)
				_, _ = fmt.Fprintf(out, "%s%s %s\n", strings.Repeat(" ", indent), s.Accent("→"), s.Dim(rem))
			}
		}
	}

	n := r.Counts()
	part := func(count int, icon, word string) string {
		text := fmt.Sprintf("%d %s", count, word)
		if count == 0 {
			return s.Dim(text)
		}
		return icon + " " + text
	}
	_, _ = fmt.Fprintf(out, "\n%s  %s  %s  %s\n",
		part(n[doctor.OK], s.Icon("ok"), "ok"),
		part(n[doctor.Warn], s.Icon("warn"), "warn"),
		part(n[doctor.Fail], s.Icon("fail"), "fail"),
		part(n[doctor.Unknown], s.Icon("?"), "unknown"))
}
