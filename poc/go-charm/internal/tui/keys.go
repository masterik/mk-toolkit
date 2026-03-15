package tui

import "github.com/charmbracelet/bubbles/key"

// MenuKeyMap defines key bindings for the menu phase.
type MenuKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Quit   key.Binding
}

// ShortHelp returns the short help key bindings.
func (k MenuKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Quit}
}

// FullHelp returns the full help key bindings.
func (k MenuKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down}, {k.Select, k.Quit}}
}

// MenuKeys is the default key map for the menu phase.
var MenuKeys = MenuKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("\u2191/k", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("\u2193/j", "down")),
	Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// InputKeyMap defines key bindings for the text input phase.
type InputKeyMap struct {
	Submit key.Binding
	Back   key.Binding
	Quit   key.Binding
}

// ShortHelp returns the short help key bindings.
func (k InputKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Back, k.Quit}
}

// FullHelp returns the full help key bindings.
func (k InputKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Submit, k.Back, k.Quit}}
}

// InputKeys is the default key map for the text input phase.
var InputKeys = InputKeyMap{
	Submit: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
}

// SummaryKeyMap defines key bindings for the summary phase.
type SummaryKeyMap struct {
	Exit key.Binding
}

// ShortHelp returns the short help key bindings.
func (k SummaryKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Exit}
}

// FullHelp returns the full help key bindings.
func (k SummaryKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Exit}}
}

// SummaryKeys is the default key map for the summary phase.
var SummaryKeys = SummaryKeyMap{
	Exit: key.NewBinding(key.WithKeys("q", "enter", "ctrl+c"), key.WithHelp("q/enter", "exit")),
}
