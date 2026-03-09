package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the dashboard.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Dismiss key.Binding
	Refresh key.Binding
	Quit    key.Binding
	Help    key.Binding
}

// DefaultKeyMap is the default key bindings for the dashboard.
var DefaultKeyMap = KeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "switch to pane")),
	Dismiss: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dismiss dead")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// ShortHelp returns a short list of key bindings for the help footer.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Dismiss, k.Refresh, k.Quit, k.Help}
}

// FullHelp returns a full list of key bindings for the extended help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Enter, k.Dismiss},
		{k.Refresh, k.Quit, k.Help},
	}
}
