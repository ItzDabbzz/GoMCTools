package pages

import "charm.land/bubbles/v2/key"

// modlistKeyMap holds all keybindings used by the Modlist Generator page.
type modlistKeyMap struct {
	// Layout
	LayoutMerged key.Binding
	LayoutSplit  key.Binding

	// Output format
	FormatBullet key.Binding
	FormatTable  key.Binding
	FormatBBCode key.Binding

	// Sort
	CycleSort     key.Binding
	ToggleSortDir key.Binding

	// Metadata toggles
	ToggleLinks    key.Binding
	ToggleSide     key.Binding
	ToggleSource   key.Binding
	ToggleVersions key.Binding
	ToggleFilename key.Binding

	// View
	ToggleRaw         key.Binding
	ToggleProjectMeta key.Binding

	// Actions
	Copy   key.Binding
	Export key.Binding
}

// ShortHelp returns the most important bindings shown in the footer bar.
func (k modlistKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ToggleLinks, k.ToggleSide, k.CycleSort, k.Copy}
}

// FullHelp returns all bindings grouped into columns for the help overlay.
func (k modlistKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.LayoutMerged, k.LayoutSplit, k.FormatBullet, k.FormatTable, k.FormatBBCode},
		{k.ToggleLinks, k.ToggleSide, k.ToggleSource, k.ToggleVersions, k.ToggleFilename},
		{k.CycleSort, k.ToggleSortDir, k.ToggleRaw, k.ToggleProjectMeta},
		{k.Copy, k.Export},
	}
}

// defaultModlistKeyMap returns a modlistKeyMap initialised with sensible defaults.
func defaultModlistKeyMap() modlistKeyMap {
	return modlistKeyMap{
		LayoutMerged:      key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "merged layout")),
		LayoutSplit:       key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "split by side")),
		FormatBullet:      key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "bullet list")),
		FormatTable:       key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "GFM table")),
		FormatBBCode:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "BBCode")),
		CycleSort:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle sort field")),
		ToggleSortDir:     key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "toggle sort dir")),
		ToggleLinks:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle links")),
		ToggleSide:        key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "toggle side")),
		ToggleSource:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "toggle source")),
		ToggleVersions:    key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "toggle versions")),
		ToggleFilename:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "toggle filename")),
		ToggleRaw:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "raw/rendered")),
		ToggleProjectMeta: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pack info header")),
		Copy:              key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
		Export:            key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export file")),
	}
}
