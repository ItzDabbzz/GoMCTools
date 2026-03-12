package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

func defaultKeyBindings() []struct{ keys, desc string } {
	return []struct{ keys, desc string }{
		{"tab", "Next Tab"},
		{"shift+tab", "Previous Tab"},
		{"?", "Toggle Help"},
		{"ctrl+c / q", "Quit"},
	}
}

// DefaultHelpBindings converts the internal default keybinding descriptions
// into Bubbles `key.Binding` instances so the Charm `help.Model` can render
// them in the short or full help view.
func DefaultHelpBindings() []key.Binding {
	defaults := defaultKeyBindings()
	out := make([]key.Binding, 0, len(defaults))
	for _, d := range defaults {
		// Handle special case for "ctrl+c / q" which represents two keys
		keys := []string{d.keys}
		if d.keys == "ctrl+c / q" {
			keys = []string{"ctrl+c", "q"}
		}
		out = append(out, key.NewBinding(key.WithKeys(keys...), key.WithHelp(d.keys, d.desc)))
	}
	return out
}

// RenderShortHelp is a convenience that renders a compact help line using
// Charm's `help.Model` so pages can embed short help in-place.
func RenderShortHelp(bindings []key.Binding) string {
	h := help.New()
	return h.ShortHelpView(bindings)
}

// renderFooter renders the default key bindings in the footer format.
// This is a wrapper around RenderShortHelp for the default bindings.
func renderFooter(bindings []struct{ keys, desc string }) string {
	h := help.New()
	var keyBindings []key.Binding
	for _, b := range bindings {
		keys := []string{b.keys}
		if b.keys == "ctrl+c / q" {
			keys = []string{"ctrl+c", "q"}
		}
		keyBindings = append(keyBindings, key.NewBinding(key.WithKeys(keys...), key.WithHelp(b.keys, b.desc)))
	}
	return h.ShortHelpView(keyBindings)
}
