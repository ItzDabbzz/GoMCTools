package pages

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"itzdabbzz.me/gomctools/internal/ui"
)

type settingsPage struct {
	state    *ui.SharedState
	autoLoad bool
	width    int
	height   int
}

func NewSettingsPage(state *ui.SharedState) ui.Page {
	return &settingsPage{
		state:    state,
		autoLoad: state.Config.AutoLoadPreviousState,
	}
}

func (s *settingsPage) Title() string { return "Settings" }
func (s *settingsPage) Init() tea.Cmd { return nil }

func (s *settingsPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, settingsKeys.ToggleAutoLoad):
			s.autoLoad = !s.autoLoad
		case key.Matches(msg, settingsKeys.ResetDefaults):
			s.autoLoad = true
			s.state.Config.Modlist.Mode = 0
			s.state.Config.Modlist.AttachLinks = true
			s.state.Config.Modlist.IncludeSide = true
			s.state.Config.Modlist.IncludeSource = true
			s.state.Config.Modlist.IncludeVersions = false
			s.state.Config.Modlist.IncludeFilename = false
		}
		// Sync config after any key.
		s.state.Config.AutoLoadPreviousState = s.autoLoad
	}
	return s, nil
}

func (s *settingsPage) View() string {
	title := sectionTitleStyle.Render("Settings")

	toggle := func(on bool) string {
		if on {
			return settingsOnStyle.Render("on ")
		}
		return settingsOffStyle.Render("off")
	}

	row := func(b key.Binding, v string, desc string) string {
		keyHint := settingsKeyStyle.Render(b.Help().Key)
		status := toggle(v == "on")
		return lipgloss.JoinHorizontal(lipgloss.Top,
			keyHint, "  ", status, "  ", settingsDescStyle.Render(desc),
		)
	}

	autoRow := row(settingsKeys.ToggleAutoLoad, toggle(s.autoLoad), "Auto-load previous pack on startup")

	resetHint := lipgloss.JoinHorizontal(lipgloss.Top,
		settingsKeyStyle.Render(settingsKeys.ResetDefaults.Help().Key),
		"  ",
		settingsDescStyle.Render("Reset all settings to defaults"),
	)

	note := dimStyle.Render("Settings are saved automatically on exit.")

	lines := []string{title, "", autoRow, "", resetHint, "", note}
	return strings.Join(lines, "\n")
}

// settingsKeyMap holds all bindings for the Settings page.
type settingsKeyMap struct {
	ToggleAutoLoad key.Binding
	ResetDefaults  key.Binding
}

func (k settingsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ToggleAutoLoad, k.ResetDefaults}
}

func (k settingsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

var settingsKeys = settingsKeyMap{
	ToggleAutoLoad: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "toggle auto-load"),
	),
	ResetDefaults: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reset defaults"),
	),
}

func (s *settingsPage) ShortHelp() []key.Binding  { return settingsKeys.ShortHelp() }
func (s *settingsPage) FullHelp() [][]key.Binding { return settingsKeys.FullHelp() }

// --- styles ---

var (
	settingsKeyStyle = lipgloss.NewStyle().
				Foreground(ui.HighlightColor).
				Bold(true).
				Width(3)

	settingsDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

	settingsOnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5af78e")).
			Bold(true)

	settingsOffStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
)
