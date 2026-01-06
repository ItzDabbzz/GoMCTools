package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// DefaultHelpBindings converts the internal default keybinding descriptions
// into Bubbles `key.Binding` instances so the Charm `help.Model` can render
// them in the short or full help view.
func DefaultHelpBindings() []key.Binding {
	defaults := defaultKeyBindings()
	out := make([]key.Binding, 0, len(defaults))
	for _, d := range defaults {
		out = append(out, key.NewBinding(key.WithHelp(d.keys, d.desc)))
	}
	return out
}

// RenderShortHelp is a convenience that renders a compact help line using
// Charm's `help.Model` so pages can embed short help in-place.
func RenderShortHelp(bindings []key.Binding) string {
	h := help.New()
	return h.ShortHelpView(bindings)
}
