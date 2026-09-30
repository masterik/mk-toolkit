// Package ui is the one place mkit's human-facing terminal output is styled.
//
// Palette and glyphs follow the tools this is meant to sit beside — gh's quiet
// tables, cargo's status icons, charm's own rounded panels — and match the
// `mkit init` wizard (internal/tui/repoinit/theme.go), so a command's report and
// the wizard read as one program.
//
// Layering: this only turns data into strings. Whether to use it at all is the
// caller's call (`cli.Options.Pretty`): a pipe, `--json` and `--no-tui` never
// reach it, which is what keeps the `key=value` text the skills parse
// byte-for-byte unchanged.
package ui

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

var (
	accent = lipgloss.AdaptiveColor{Light: "27", Dark: "45"}
	dim    = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	good   = lipgloss.AdaptiveColor{Light: "28", Dark: "78"}
	warn   = lipgloss.AdaptiveColor{Light: "130", Dark: "214"}
	bad    = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	violet = lipgloss.AdaptiveColor{Light: "91", Dark: "141"}
)

// maxWidth keeps a report readable on an ultrawide terminal.
const maxWidth = 100

// S renders styled text for one output writer.
type S struct {
	r     *lipgloss.Renderer
	width int
}

// New returns a styler whose colour depth is detected from w, so NO_COLOR,
// TERM=dumb and a non-terminal writer all degrade to plain text.
func New(w io.Writer) *S {
	s := &S{r: lipgloss.NewRenderer(w), width: 80}
	if f, ok := w.(*os.File); ok {
		if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
			s.width = cols
		}
	}
	if s.width > maxWidth {
		s.width = maxWidth
	}
	return s
}

// Force returns a styler with a fixed width and full colour, for tests.
func Force(width int) *S {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI256)
	r.SetHasDarkBackground(true)
	return &S{r: r, width: width}
}

// Width is the usable line width.
func (s *S) Width() int { return s.width }

func (s *S) fg(c lipgloss.TerminalColor) lipgloss.Style { return s.r.NewStyle().Foreground(c) }

// Text styles.
func (s *S) Accent(t string) string { return s.fg(accent).Render(t) }
func (s *S) Bold(t string) string   { return s.r.NewStyle().Bold(true).Render(t) }
func (s *S) Dim(t string) string    { return s.fg(dim).Render(t) }
func (s *S) Good(t string) string   { return s.fg(good).Render(t) }
func (s *S) Warn(t string) string   { return s.fg(warn).Render(t) }
func (s *S) Bad(t string) string    { return s.fg(bad).Render(t) }
func (s *S) Violet(t string) string { return s.fg(violet).Render(t) }

// Title is a command's heading: a diamond, the name, and a dim subtitle.
func (s *S) Title(name, sub string) string {
	out := s.fg(accent).Bold(true).Render("◆ " + name)
	if sub != "" {
		out += "  " + s.Dim(sub)
	}
	return out
}

// Section is a heading followed by a dim rule to the edge, in the style of
// `── quality gate ─────────`.
func (s *S) Section(name string) string {
	head := "── " + name + " "
	fill := s.width - lipgloss.Width(head)
	if fill < 3 {
		fill = 3
	}
	return s.fg(accent).Bold(true).Render("── "+name+" ") + s.Dim(strings.Repeat("─", fill))
}

// Icon is a status glyph: ✔ ▲ ✖ ? — the same four the doctor already had as
// words, drawn from cargo/gh so they read at a glance.
func (s *S) Icon(kind string) string {
	switch kind {
	case "ok":
		return s.Good("✔")
	case "warn":
		return s.Warn("▲")
	case "fail":
		return s.Bad("✖")
	}
	return s.Dim("?")
}

// Tag is a value's provenance in a small coloured bracket: pinned (the repo
// said so), discovered (mkit found it), unavailable (neither).
func (s *S) Tag(kind string) string {
	switch kind {
	case "pinned":
		return s.fg(violet).Render("● pinned")
	case "discovered":
		return s.Dim("○ discovered")
	case "unavailable":
		return s.Warn("△ unavailable")
	}
	return s.Dim(kind)
}

// Pill is a filled label, for a run's mode (DRY-RUN, APPLY).
func (s *S) Pill(text string, c string) string {
	col := accent
	switch c {
	case "good":
		col = good
	case "warn":
		col = warn
	case "bad":
		col = bad
	}
	return s.r.NewStyle().Bold(true).Padding(0, 1).
		Foreground(lipgloss.Color("0")).Background(col).Render(text)
}

// Bar draws frac (0..1) as a filled/empty run of width cells.
func (s *S) Bar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	n := int(frac*float64(width) + 0.5)
	if frac > 0 && n == 0 {
		n = 1
	}
	return s.fg(accent).Render(strings.Repeat("█", n)) + s.Dim(strings.Repeat("░", width-n))
}

// KV is one aligned `label  value` row; labelW pads the dim label.
func (s *S) KV(label, value string, labelW int) string {
	pad := labelW - lipgloss.Width(label)
	if pad < 0 {
		pad = 0
	}
	return s.Dim(label) + strings.Repeat(" ", pad) + "  " + value
}

// Panel wraps lines in a rounded dim border.
func (s *S) Panel(lines ...string) string {
	return s.r.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(dim).
		Padding(0, 1).Render(strings.Join(lines, "\n"))
}

// Table renders rows under a bold header with only a rule beneath it — gh's
// table, not a grid. cell may recolour a cell by (row, col); row -1 is the header.
func (s *S) Table(header []string, rows [][]string, cell func(row, col int, text string) lipgloss.Style) string {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(s.fg(dim)).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderRow(false).BorderHeader(true).
		Headers(header...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			base := s.r.NewStyle().PaddingRight(2)
			if row == table.HeaderRow {
				return base.Bold(true).Foreground(dim)
			}
			if cell != nil {
				return cell(row, col, rows[row][col]).PaddingRight(2)
			}
			return base
		})
	return t.Render()
}

// Style exposes a fresh style from this renderer, for a table cell callback.
func (s *S) Style() lipgloss.Style { return s.r.NewStyle() }

// Colour styles for a cell callback.
func (s *S) GoodStyle() lipgloss.Style   { return s.fg(good) }
func (s *S) WarnStyle() lipgloss.Style   { return s.fg(warn) }
func (s *S) BadStyle() lipgloss.Style    { return s.fg(bad) }
func (s *S) DimStyle() lipgloss.Style    { return s.fg(dim) }
func (s *S) AccentStyle() lipgloss.Style { return s.fg(accent) }
func (s *S) VioletStyle() lipgloss.Style { return s.fg(violet) }
