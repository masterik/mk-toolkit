package tui

import "github.com/charmbracelet/lipgloss"

// Adaptive colors for light/dark terminal backgrounds.
var (
	ColorCyan   = lipgloss.AdaptiveColor{Light: "#0097A7", Dark: "#00BCD4"}
	ColorGreen  = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#66BB6A"}
	ColorRed    = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#EF5350"}
	ColorDim    = lipgloss.AdaptiveColor{Light: "#9E9E9E", Dark: "#616161"}
	ColorYellow = lipgloss.AdaptiveColor{Light: "#F57F17", Dark: "#FFD54F"}
	ColorText   = lipgloss.AdaptiveColor{Light: "#212121", Dark: "#FAFAFA"}
)

// Pre-built styles shared across the TUI.
var (
	HeaderBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorCyan).
			Bold(true).
			Padding(0, 1)

	MenuSelected = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	MenuNormal   = lipgloss.NewStyle().Foreground(ColorText)
	MenuDim      = lipgloss.NewStyle().Foreground(ColorDim)

	OutputBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDim).
			Padding(0, 1)

	OutputTitle = lipgloss.NewStyle().
			Foreground(ColorDim).
			Italic(true)

	SummaryBoxOK = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGreen).
			Padding(0, 1)

	SummaryBoxFail = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorRed).
			Padding(0, 1)

	BoldStyle = lipgloss.NewStyle().Bold(true)
	DimStyle  = lipgloss.NewStyle().Foreground(ColorDim)
)
