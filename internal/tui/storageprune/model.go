// Package storageprune is the Bubble Tea front end over
// internal/core/storage for "mkit storage prune --apply" on a terminal.
// Update holds no prune logic — it only toggles selection state; the
// confirmed selection goes back to storage.Apply.
package storageprune

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/masterik/mk-toolkit/internal/core/storage"
)

type row struct {
	providerName string
	category     storage.CategoryReport
	selected     bool
}

type model struct {
	rows      []row
	cursor    int
	confirmed bool
	aborted   bool
}

func newModel(report *storage.Report) model {
	var rows []row
	for _, p := range report.Providers {
		for _, c := range p.Categories {
			if len(c.Entries) == 0 {
				continue
			}
			rows = append(rows, row{providerName: p.Name, category: c, selected: true})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].category.Bytes > rows[j].category.Bytes })
	return model{rows: rows}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case " ":
		if len(m.rows) > 0 {
			m.rows[m.cursor].selected = !m.rows[m.cursor].selected
		}
	case "a":
		for i := range m.rows {
			m.rows[i].selected = true
		}
	case "n":
		for i := range m.rows {
			m.rows[i].selected = false
		}
	case "enter":
		m.confirmed = true
		return m, tea.Quit
	case "q", "esc", "ctrl+c":
		m.aborted = true
		return m, tea.Quit
	}
	return m, nil
}

var (
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m model) View() string {
	var b strings.Builder
	b.WriteString(helpStyle.Render("space toggle · a/n select all/none · enter delete · q abort"))
	b.WriteString("\n\n")

	var total int64
	for i, r := range m.rows {
		mark := "[ ]"
		style := dimStyle
		if r.selected {
			mark = "[x]"
			style = selectedStyle
			total += r.category.Bytes
		}

		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
		}

		kind := "files"
		if r.category.Kind == storage.KindStaleDirs {
			kind = "dirs"
		}

		line := fmt.Sprintf("%s %s %-8s %-38s %4d %s, %s",
			cursor, mark, r.providerName, r.category.Label, len(r.category.Entries), kind, storage.HumanBytes(r.category.Bytes))
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("selected: %s\n", storage.HumanBytes(total)))
	return b.String()
}

// Run shows the size-sorted tick-list over report and blocks until the
// user confirms or aborts. ok is false on abort — the caller must delete
// nothing in that case.
func Run(report *storage.Report) (sel *storage.Selection, ok bool, err error) {
	m := newModel(report)
	if len(m.rows) == 0 {
		return storage.NewSelection(), false, nil
	}

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, false, err
	}

	fm := final.(model)
	if fm.aborted || !fm.confirmed {
		return storage.NewSelection(), false, nil
	}

	result := storage.NewSelection()
	for _, r := range fm.rows {
		if !r.selected {
			continue
		}
		for _, e := range r.category.Entries {
			result.Add(e.Path)
		}
	}
	return result, true, nil
}
