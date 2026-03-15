package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"forgez/poc-go-charm/internal/claude"
	"forgez/poc-go-charm/internal/git"
)

// ---------------------------------------------------------------------------
// Menu phase
// ---------------------------------------------------------------------------

func viewMenu(m Model) string {
	title := BoldStyle.Render("  How would you like to commit?")

	var items []string
	for i, item := range MenuItems {
		indicator := "\u25CB"
		style := MenuNormal
		descStyle := MenuDim
		if i == m.Cursor {
			indicator = "\u25CF"
			style = MenuSelected
			descStyle = lipgloss.NewStyle().Foreground(ColorCyan).Italic(true)
		}
		line := fmt.Sprintf(
			"  %s %s %s %s",
			style.Render(indicator),
			item.Icon,
			style.Render(item.Label),
			descStyle.Render("("+item.Desc+")"),
		)
		items = append(items, line)
	}
	menu := strings.Join(items, "\n")

	helpView := "  " + m.Help.View(MenuKeys)

	return lipgloss.JoinVertical(lipgloss.Left, title, "", menu, "", helpView)
}

func updateMenu(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, MenuKeys.Quit):
			return m, tea.Quit
		case key.Matches(msg, MenuKeys.Up):
			if m.Cursor > 0 {
				m.Cursor--
			}
		case key.Matches(msg, MenuKeys.Down):
			if m.Cursor < len(MenuItems)-1 {
				m.Cursor++
			}
		case key.Matches(msg, MenuKeys.Select):
			m.Choice = MenuChoice(m.Cursor)
			if m.Choice == ChoiceCustomNote {
				m.Phase = PhaseTextInput
				m.TextInput.Focus()
				return m, textinput.Blink
			}
			m.Phase = PhaseRunning
			return m, tea.Batch(m.Spinner.Tick, runClaude(m))
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Text input phase
// ---------------------------------------------------------------------------

func viewTextInput(m Model) string {
	title := BoldStyle.Render("  Enter commit note:")
	input := "  " + m.TextInput.View()
	helpView := "  " + m.Help.View(InputKeys)

	return lipgloss.JoinVertical(lipgloss.Left, title, "", input, "", helpView)
}

func updateTextInput(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, InputKeys.Quit):
			return m, tea.Quit
		case key.Matches(msg, InputKeys.Back):
			m.Phase = PhaseMenu
			return m, nil
		case key.Matches(msg, InputKeys.Submit):
			m.Phase = PhaseRunning
			return m, tea.Batch(m.Spinner.Tick, runClaude(m))
		}
	}
	var cmd tea.Cmd
	m.TextInput, cmd = m.TextInput.Update(msg)
	return m, cmd
}

// ---------------------------------------------------------------------------
// Running phase
// ---------------------------------------------------------------------------

func viewRunning(m Model, bw int) string {
	spinLine := fmt.Sprintf("  %s Running Claude...", m.Spinner.View())

	if len(m.Lines) == 0 {
		return spinLine
	}

	box := renderOutputBox(m, bw)
	return lipgloss.JoinVertical(lipgloss.Left, spinLine, "", box)
}

func updateRunning(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case claude.LineMsg:
		m.Lines = append(m.Lines, string(msg))
		return m, nil
	case claude.DoneMsg:
		m.ExitCode = msg.ExitCode
		m.Phase = PhaseSummary
		if m.ExitCode == 0 {
			m.Oneline = git.Oneline()
			m.DiffStat = git.DiffStat()
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if key.Matches(msg, MenuKeys.Quit) {
			return m, tea.Quit
		}
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Summary phase
// ---------------------------------------------------------------------------

func viewSummary(m Model, bw int) string {
	var summary string

	if m.ExitCode == 0 {
		commitLine := BoldStyle.Render(m.Oneline)
		statLine := DimStyle.Render(m.DiffStat)
		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(ColorGreen).Bold(true).Render("\u2705 Committed!"),
			"",
			commitLine,
			statLine,
		)
		summary = SummaryBoxOK.Width(bw - borderPadding).Render(content)
	} else {
		content := lipgloss.NewStyle().Foreground(ColorRed).Bold(true).Render(
			fmt.Sprintf("\u274C Claude exited with code %d", m.ExitCode),
		)
		summary = SummaryBoxFail.Width(bw - borderPadding).Render(content)
	}

	var outputView string
	if len(m.Lines) > 0 {
		outputView = renderOutputBox(m, bw)
	}

	helpView := "  " + m.Help.View(SummaryKeys)

	var parts []string
	if outputView != "" {
		parts = append(parts, outputView, "")
	}
	parts = append(parts, summary, "", helpView)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func updateSummary(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, SummaryKeys.Exit) {
			return m, tea.Quit
		}
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// runClaude prepares and launches the Claude subprocess command.
func runClaude(m Model) tea.Cmd {
	if m.Choice == ChoiceCommitAll {
		if err := git.StageAll(); err != nil {
			return func() tea.Msg {
				return claude.DoneMsg{ExitCode: 1}
			}
		}
	}

	var customNote string
	if m.Choice == ChoiceCustomNote && m.TextInput.Value() != "" {
		customNote = m.TextInput.Value()
	}

	return claude.Run(*m.program, customNote)
}
