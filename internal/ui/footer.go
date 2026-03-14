package ui

import "charm.land/bubbles/v2/key"

// GlobalKeyMap defines the root-level navigation and application bindings.
// It implements help.KeyMap so it can be passed directly to help.Model.View().
type GlobalKeyMap struct {
	NextTab key.Binding
	PrevTab key.Binding
	Help    key.Binding
	Quit    key.Binding
}

// ShortHelp returns the most important global bindings shown in the footer.
func (k GlobalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns all global bindings grouped into columns for the overlay.
func (k GlobalKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextTab, k.PrevTab},
		{k.Help, k.Quit},
	}
}

// DefaultKeys is the application-wide key map used by the root model.
var DefaultKeys = GlobalKeyMap{
	NextTab: key.NewBinding(
		key.WithKeys("tab", "ctrl+right", "l"),
		key.WithHelp("tab/→", "next tab"),
	),
	PrevTab: key.NewBinding(
		key.WithKeys("shift+tab", "ctrl+left", "h"),
		key.WithHelp("shift+tab/←", "prev tab"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("ctrl+c/q", "quit"),
	),
}
