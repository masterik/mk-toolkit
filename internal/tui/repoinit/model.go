// Package repoinit renders the interactive form `mkit init` opens on a TTY.
//
// It edits strings and nothing else. Which values are worth pinning, what the
// discovered defaults are, and how the result becomes a config file are all
// decided before this runs and after it returns — Update holds no init logic,
// per the layering invariant.
package repoinit

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Field is one editable line of the form.
type Field struct {
	// Key identifies the field to the caller; never shown.
	Key string
	// Label is the display name.
	Label string
	// Value is the current text, edited in place.
	Value string
	// Discovered is what mkit found on its own, shown as the thing the user is
	// choosing to override. Empty when discovery had no answer.
	Discovered string
	// Help is one line explaining what pinning this buys.
	Help string
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	selStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	editStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

type model struct {
	fields  []Field
	cursor  int
	editing bool
	buf     string
	saved   bool
	quit    bool
}

// Run opens the form and returns the edited fields, and whether the user chose to
// write. A false second return means abort — the caller writes nothing.
func Run(fields []Field) ([]Field, bool, error) {
	m := model{fields: fields}
	out, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, false, err
	}
	final := out.(model)
	return final.fields, final.saved, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.editing {
		return m.updateEditing(key)
	}
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.fields)-1 {
			m.cursor++
		}
	case "enter":
		m.editing = true
		m.buf = m.fields[m.cursor].Value
	case "d":
		// Clear a pin, so the field falls back to discovery. Distinct from
		// typing an empty string only in that it is one key.
		m.fields[m.cursor].Value = ""
	case "w":
		m.saved = true
		m.quit = true
		return m, tea.Quit
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateEditing(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEnter:
		m.fields[m.cursor].Value = strings.TrimSpace(m.buf)
		m.editing = false
	case tea.KeyEsc:
		m.editing = false
	case tea.KeyBackspace:
		if r := []rune(m.buf); len(r) > 0 {
			m.buf = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.buf += " "
	case tea.KeyRunes:
		m.buf += string(key.Runes)
	}
	return m, nil
}

func (m model) View() string {
	if m.quit {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("mkit init — pin what discovery cannot establish"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Leave a field empty to keep discovering it every run."))
	b.WriteString("\n\n")

	for i, f := range m.fields {
		cursor := "  "
		label := f.Label
		if i == m.cursor {
			cursor = "> "
			label = selStyle.Render(label)
		}
		value := f.Value
		switch {
		case m.editing && i == m.cursor:
			value = editStyle.Render(m.buf + "█")
		case value == "" && f.Discovered != "":
			value = dimStyle.Render(f.Discovered + "  (discovered)")
		case value == "":
			value = dimStyle.Render("—")
		}
		fmt.Fprintf(&b, "%s%-14s %s\n", cursor, label, value)
		if i == m.cursor && f.Help != "" {
			b.WriteString(dimStyle.Render("                 " + f.Help))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.editing {
		b.WriteString(dimStyle.Render("enter save field · esc cancel"))
	} else {
		b.WriteString(dimStyle.Render("↑/↓ move · enter edit · d clear · w write · q abort"))
	}
	b.WriteString("\n")
	return b.String()
}
