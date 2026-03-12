package pages

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"itzdabbzz.me/gomctools/internal/ui"
)

type settingsPage struct {
	state            *ui.SharedState
	telemetryEnabled bool
	autoLoad         bool
	width            int
	height           int
}

func NewSettingsPage(state *ui.SharedState) ui.Page {
	return &settingsPage{
		state:            state,
		telemetryEnabled: state.Config.TelemetryEnabled,
		autoLoad:         state.Config.AutoLoadPreviousState,
	}
}

func (s *settingsPage) Title() string {
	return "Settings"
}

func (s *settingsPage) Init() tea.Cmd {
	return nil
}

func (s *settingsPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			s.telemetryEnabled = !s.telemetryEnabled
		case "a":
			s.autoLoad = !s.autoLoad
		case "r":
			// Reset to defaults
			s.telemetryEnabled = true
			s.autoLoad = true
			// Reset modlist settings in config
			s.state.Config.Modlist.Mode = 0
			s.state.Config.Modlist.AttachLinks = true
			s.state.Config.Modlist.IncludeSide = true
			s.state.Config.Modlist.IncludeSource = true
			s.state.Config.Modlist.IncludeVersions = false
			s.state.Config.Modlist.IncludeFilename = false
		}
		// Update config immediately
		s.state.Config.TelemetryEnabled = s.telemetryEnabled
		s.state.Config.AutoLoadPreviousState = s.autoLoad
	}
	return s, nil
}

func (s *settingsPage) View() string {
	telemetry := "off"
	if s.telemetryEnabled {
		telemetry = "on"
	}
	autoLoadStr := "off"
	if s.autoLoad {
		autoLoadStr = "on"
	}
	return fmt.Sprintf(
		"Settings\n\n"+
			"  [t] Telemetry: %s\n"+
			"  [a] Auto-load previous state: %s\n"+
			"  [r] Reset all settings to defaults\n\n"+
			"Settings are saved automatically on exit.",
		telemetry, autoLoadStr,
	)
}

func (s *settingsPage) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle telemetry")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle auto-load")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset defaults")),
	}
}

func (s *settingsPage) FullHelp() [][]key.Binding {
	return [][]key.Binding{s.ShortHelp()}
}
