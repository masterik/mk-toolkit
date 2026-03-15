// Package tui implements the Bubble Tea model for the ForgeZ commit helper.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Phase represents the current state of the TUI state machine.
type Phase int

const (
	// PhaseMenu is the initial menu selection phase.
	PhaseMenu Phase = iota
	// PhaseTextInput is the custom note text input phase.
	PhaseTextInput
	// PhaseRunning is the Claude subprocess execution phase.
	PhaseRunning
	// PhaseSummary is the final commit summary phase.
	PhaseSummary
)

// MenuChoice represents the user's selected commit action.
type MenuChoice int

const (
	// ChoiceCommitAll stages everything then commits.
	ChoiceCommitAll MenuChoice = iota
	// ChoiceCommitStaged commits only staged files.
	ChoiceCommitStaged
	// ChoiceCustomNote prompts for a custom message then commits.
	ChoiceCustomNote
)

// MenuItem describes a single entry in the commit menu.
type MenuItem struct {
	Icon  string
	Label string
	Desc  string
}

// MenuItems is the list of available commit actions.
var MenuItems = []MenuItem{
	{"\U0001F680", "Commit all", "stage everything + commit"},
	{"\U0001F4E6", "Commit staged only", "spawn Claude as-is"},
	{"\U0001F4DD", "Custom note", "pass message to Claude"},
}

const (
	defaultWidth        = 80
	defaultHeight       = 24
	maxBoxWidth         = 76
	boxPadding          = 4
	borderPadding       = 2
	viewportInnerOffset = 4
	maxViewportHeight   = 14
	viewportBaseHeight  = 16
	textInputCharLimit  = 200
	textInputWidth      = 60
	viewportWidth       = 70
	viewportHeight      = 12
)

// Model is the top-level Bubble Tea model for the commit helper.
type Model struct {
	Phase     Phase
	Cursor    int
	Choice    MenuChoice
	Width     int
	Height    int
	TextInput textinput.Model
	Spinner   spinner.Model
	Viewport  viewport.Model
	Help      help.Model
	Lines     []string
	ExitCode  int
	Oneline   string
	DiffStat  string
	Branch    string
	Staged    int

	// program holds the tea.Program reference for sending messages from goroutines.
	// This is the documented Bubble Tea pattern for streaming subprocess output.
	// Using a pointer-to-pointer so the value can be set after tea.NewProgram copies the model.
	program **tea.Program
}

// NewModel creates a Model initialized with the given git context.
func NewModel(branch string, staged int) Model {
	ti := textinput.New()
	ti.Placeholder = "Describe your changes..."
	ti.CharLimit = textInputCharLimit
	ti.Width = textInputWidth
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorCyan)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	vp := viewport.New(viewportWidth, viewportHeight)
	vp.Style = lipgloss.NewStyle().Foreground(ColorDim)

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(ColorCyan)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(ColorDim)

	pp := new(*tea.Program)
	return Model{
		Phase:     PhaseMenu,
		Cursor:    1, // default: "Commit staged only"
		TextInput: ti,
		Spinner:   sp,
		Viewport:  vp,
		Help:      h,
		Branch:    branch,
		Staged:    staged,
		Width:     defaultWidth,
		Height:    defaultHeight,
		program:   pp,
	}
}

// SetProgram sets the tea.Program reference used for streaming subprocess output.
// This must be called after tea.NewProgram since the program copies the model.
func (m Model) SetProgram(p *tea.Program) {
	*m.program = p
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Spinner.Tick, tea.SetWindowTitle("fz commit"))
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window resize globally.
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.Width = wsm.Width
		m.Height = wsm.Height

		boxWidth := min(m.Width-boxPadding, maxBoxWidth)
		m.Viewport.Width = boxWidth - viewportInnerOffset
		m.Viewport.Height = min(m.Height-viewportBaseHeight, maxViewportHeight)
		m.Help.Width = m.Width

		return m, nil
	}

	switch m.Phase {
	case PhaseMenu:
		return updateMenu(m, msg)
	case PhaseTextInput:
		return updateTextInput(m, msg)
	case PhaseRunning:
		return updateRunning(m, msg)
	case PhaseSummary:
		return updateSummary(m, msg)
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	boxWidth := min(m.Width-boxPadding, maxBoxWidth)

	branchStyled := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render(m.Branch)
	stagedStyled := lipgloss.NewStyle().Foreground(ColorYellow).Render(fmt.Sprintf("%d staged", m.Staged))
	header := HeaderBox.Width(boxWidth - boxPadding).Render(
		fmt.Sprintf("\u26A1 ForgeZ \u2014 commit\n\U0001F4CD %s \u00B7 %s", branchStyled, stagedStyled),
	)

	var content string
	switch m.Phase {
	case PhaseMenu:
		content = viewMenu(m)
	case PhaseTextInput:
		content = viewTextInput(m)
	case PhaseRunning:
		content = viewRunning(m, boxWidth)
	case PhaseSummary:
		content = viewSummary(m, boxWidth)
	}

	return lipgloss.JoinVertical(lipgloss.Left, "", header, "", content, "")
}

// renderOutputBox renders the scrollable Claude output viewport inside a styled box.
func renderOutputBox(m Model, bw int) string {
	m.Viewport.SetContent(strings.Join(m.Lines, "\n"))
	m.Viewport.GotoBottom()
	outputContent := lipgloss.JoinVertical(lipgloss.Left,
		OutputTitle.Render("claude output"),
		m.Viewport.View(),
	)
	return OutputBox.Width(bw - borderPadding).Render(outputContent)
}
