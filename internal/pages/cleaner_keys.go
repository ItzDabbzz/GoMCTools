package pages

import "github.com/charmbracelet/bubbles/key"

// cleanerKeyMap holds all keybindings used by the Pack Cleaner page.
type cleanerKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Toggle key.Binding
	Clean  key.Binding
	New    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Save   key.Binding
	Debug  key.Binding
}

// ShortHelp returns the most important bindings shown in the footer bar.
func (k cleanerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Toggle, k.Clean, k.New}
}

// FullHelp returns all bindings grouped into columns for the help overlay.
func (k cleanerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Toggle, k.Clean},
		{k.New, k.Edit, k.Delete, k.Save},
	}
}

// defaultCleanerKeyMap returns a cleanerKeyMap initialised with sensible defaults.
func defaultCleanerKeyMap() cleanerKeyMap {
	return cleanerKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "select previous"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "select next"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle preset"),
		),
		Clean: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "run cleaner"),
		),
		// "+" avoids conflicting with the global "n" next-tab binding.
		New: key.NewBinding(
			key.WithKeys("+", "ctrl+n"),
			key.WithHelp("+", "new preset"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit preset"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete preset"),
		),
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save presets"),
		),
		Debug: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "debug"),
		),
	}
}
