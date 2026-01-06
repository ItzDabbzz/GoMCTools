package pages

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"itzdabbzz.me/gomctools/internal/ui"
)

type settingsPage struct {
	telemetryEnabled bool
}

func NewSettingsPage() ui.Page {
	return settingsPage{telemetryEnabled: true}
}

func (s settingsPage) Title() string {
	return "Settings"
}

func (s settingsPage) Init() tea.Cmd {
	return nil
}

func (s settingsPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}

	switch keyMsg.String() {
	case "t":
		s.telemetryEnabled = !s.telemetryEnabled
	}
	return s, nil
}

func (s settingsPage) View() string {
	telemetry := "off"
	if s.telemetryEnabled {
		telemetry = "on"
	}
	return fmt.Sprintf("Telemetry: %s (press 't' to toggle)\nPlanned: settings reset, persistence, and additional preferences.", telemetry)
}
