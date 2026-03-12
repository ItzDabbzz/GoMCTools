package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Page is the interface that every top-level TUI page must satisfy.
// It mirrors the tea.Model contract but returns a Page instead of tea.Model
// so the root Model can store all pages in a single typed slice.
type Page interface {
	Title() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Page, tea.Cmd)
	View() string
}

// ZoneAware pages receive the root bubblezone manager and a unique prefix
// so they can mark clickable regions that align with the final rendered doc.
type ZoneAware interface {
	SetZone(z *zone.Manager, prefix string)
}

// KeyCapturer pages can request to handle navigation keys themselves.
// When true, the root model will not process global tab navigation keys.
type KeyCapturer interface {
	CaptureGlobalNav() bool
}

// ShortHelpProvider provides compact keybindings for the footer area.
// Returning a slice of key.Binding allows each page to expose its relevant
// short key hints that will be merged with global bindings in the footer.
type ShortHelpProvider interface {
	ShortHelp() []key.Binding
}
