package repoinit

import "github.com/charmbracelet/lipgloss"

// The look follows Vercel's CLI wizards (@clack/prompts, behind `vercel` and
// `create-next-app`): a thin rule down the left instead of a box, a diamond on
// the prompt in hand, a hollow one on each answered prompt, and filled/hollow
// marks for selection. Colour is emphasis only — every mark, and every
// provenance, is still legible without it.
var (
	accent = lipgloss.AdaptiveColor{Light: "27", Dark: "45"}
	dim    = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	fg     = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	warn   = lipgloss.AdaptiveColor{Light: "130", Dark: "214"}
	bad    = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}

	sAccent = lipgloss.NewStyle().Foreground(accent)
	sTitle  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	sDim    = lipgloss.NewStyle().Foreground(dim)
	sFg     = lipgloss.NewStyle().Foreground(fg)
	sWarn   = lipgloss.NewStyle().Foreground(warn)
	sBad    = lipgloss.NewStyle().Foreground(bad)
)

// The marks.
const (
	markIntro    = "┌"
	markBar      = "│"
	markOutro    = "└"
	markActive   = "◆"
	markDone     = "◇"
	markError    = "▲"
	markAbort    = "■"
	markOn       = "●"
	markOff      = "○"
	markTicked   = "◼"
	markUnticked = "◻"
	markCursor   = "›"
)
