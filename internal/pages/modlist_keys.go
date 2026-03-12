package pages

import "github.com/charmbracelet/bubbles/key"

// modlistKeyMap holds all keybindings used by the Modlist Generator page.
type modlistKeyMap struct {
	LayoutMerged   key.Binding
	LayoutSplit    key.Binding
	ToggleLinks    key.Binding
	ToggleSide     key.Binding
	ToggleSource   key.Binding
	ToggleVersions key.Binding
	ToggleFilename key.Binding
	Copy           key.Binding
	Export         key.Binding
}

// ShortHelp returns the most important bindings shown in the footer bar.
func (k modlistKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ToggleLinks, k.ToggleSide, k.ToggleSource, k.Copy}
}

// FullHelp returns all bindings grouped into columns for the help overlay.
func (k modlistKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.LayoutMerged, k.LayoutSplit, k.ToggleLinks, k.ToggleSide},
		{k.ToggleSource, k.ToggleVersions, k.ToggleFilename, k.Copy, k.Export},
	}
}

// defaultModlistKeyMap returns a modlistKeyMap initialised with sensible defaults.
func defaultModlistKeyMap() modlistKeyMap {
	return modlistKeyMap{
		LayoutMerged:   key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "merged layout")),
		LayoutSplit:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "split by side")),
		ToggleLinks:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle links")),
		ToggleSide:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "toggle side")),
		ToggleSource:   key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "toggle source")),
		ToggleVersions: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "toggle versions")),
		ToggleFilename: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "toggle filename")),
		Copy:           key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy markdown")),
		Export:         key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export file")),
	}
}
