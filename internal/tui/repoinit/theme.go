package repoinit

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// clackTheme mimics the connector-and-diamond look of Vercel's CLI wizards
// (@clack/prompts, behind `vercel` and `create-next-app`): a thin rule down
// the left instead of a box, a diamond before every title, and filled/hollow
// circles for selection. Colour is emphasis only — the diamond, the circles
// and the rule are all still legible without it.
func clackTheme() *huh.Theme {
	t := huh.ThemeBase()

	var (
		accent = lipgloss.AdaptiveColor{Light: "27", Dark: "45"}
		dim    = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
		fg     = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
		bad    = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	)

	rule := lipgloss.NewStyle().PaddingLeft(1).
		BorderStyle(lipgloss.Border{Left: "│"}).BorderLeft(true)
	t.Focused.Base = rule.BorderForeground(accent)
	t.Blurred.Base = rule.BorderForeground(dim)
	t.Focused.Card = t.Focused.Base
	t.Blurred.Card = t.Blurred.Base

	t.Focused.Title = lipgloss.NewStyle().Bold(true).Foreground(accent)
	t.Blurred.Title = lipgloss.NewStyle().Foreground(dim)
	t.Focused.NoteTitle = t.Focused.Title
	t.Focused.Description = lipgloss.NewStyle().Foreground(dim)
	t.Blurred.Description = t.Focused.Description
	t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(bad).SetString(" *")
	t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(bad).SetString(" *")

	// The cursor: a diamond, the same mark clack puts before an active prompt.
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(accent).SetString("◆ ")
	t.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	t.Focused.MultiSelectSelector = t.Focused.SelectSelector
	t.Focused.Option = lipgloss.NewStyle().Foreground(fg)
	t.Blurred.Option = lipgloss.NewStyle().Foreground(dim)

	// Filled vs hollow circle for ticked vs not — clack's multi-select mark.
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(accent).SetString("● ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(dim).SetString("○ ")
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(fg)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(dim)

	t.Focused.FocusedButton = lipgloss.NewStyle().Padding(0, 2).
		Foreground(lipgloss.Color("0")).Background(accent)
	t.Focused.BlurredButton = lipgloss.NewStyle().Padding(0, 2).Foreground(dim)

	t.Help.ShortKey = lipgloss.NewStyle().Foreground(dim)
	t.Help.ShortDesc = lipgloss.NewStyle().Foreground(dim)
	t.Help.ShortSeparator = lipgloss.NewStyle().Foreground(dim)
	t.Help.Ellipsis = t.Help.ShortDesc

	return t
}

// diamond marks a page or review header, the way clack marks the step of its
// wizard currently on screen.
const diamond = "◆ "
