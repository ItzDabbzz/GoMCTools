package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Page is the interface that every top-level TUI page must satisfy.
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

// ShortHelpProvider provides compact keybindings for the footer bar.
// Pages implement this to contribute bindings merged with global defaults.
type ShortHelpProvider interface {
	ShortHelp() []key.Binding
}

// NavigateMsg can be emitted by a page to request the root model switch to a
// specific tab index. Use this instead of hard-coding page switches inside
// individual page Update methods.
type NavigateMsg struct{ Page int }

// ToggleHelpMsg can be emitted by a page that captures global nav (e.g. the
// selector) to request the root model toggle the help overlay, since those
// pages consume the ? key themselves before the root sees it.
type ToggleHelpMsg struct{}

// ContentSizeMsg is sent to pages alongside tea.WindowSizeMsg and carries the
// actual inner dimensions of the content area they render into — after the
// model has subtracted the tab bar, footer, window border, and padding.
// Pages that contain viewports or need exact height budgets should listen for
// this instead of trying to reverse-engineer frame overhead from raw terminal size.
type ContentSizeMsg struct {
	Width  int
	Height int
}
