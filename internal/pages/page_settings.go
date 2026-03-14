package pages

// page_settings.go
import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/modpack"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
)

type settingsPage struct {
	state       *modpack.SharedState
	autoLoad    bool
	width       int
	height      int
	showConfirm bool
}

func NewSettingsPage(state *modpack.SharedState) ui.Page {
	return &settingsPage{
		state:    state,
		autoLoad: state.Config.AutoLoadPreviousState,
	}
}

func (s *settingsPage) Title() string { return "Settings" }
func (s *settingsPage) Init() tea.Cmd { return nil }

// CaptureGlobalNav blocks tab navigation when the confirmation modal is visible.
func (s *settingsPage) CaptureGlobalNav() bool {
	return s.showConfirm
}

func (s *settingsPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch msg := msg.(type) {
	// ContentSizeMsg carries exact inner dimensions; raw WindowSizeMsg
	// includes the frame overhead that the root model already accounts for.
	case ui.ContentSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case tea.KeyMsg:
		if s.showConfirm {
			switch msg.String() {
			case "y", "Y", "enter":
				s.resetToDefaults()
				s.showConfirm = false
			case "n", "N", "esc":
				s.showConfirm = false
			}
			return s, nil
		}

		switch {
		case key.Matches(msg, settingsKeys.ToggleAutoLoad):
			s.autoLoad = !s.autoLoad
			s.state.Config.AutoLoadPreviousState = s.autoLoad
		case key.Matches(msg, settingsKeys.ResetDefaults):
			s.showConfirm = true
		}
	}
	return s, nil
}

func (s *settingsPage) resetToDefaults() {
	s.autoLoad = true
	s.state.Config.AutoLoadPreviousState = true
	s.state.Config.Modlist.Mode = 0
	s.state.Config.Modlist.AttachLinks = true
	s.state.Config.Modlist.IncludeSide = true
	s.state.Config.Modlist.IncludeSource = true
	s.state.Config.Modlist.IncludeVersions = false
	s.state.Config.Modlist.IncludeFilename = false
	s.state.Config.Modlist.OutputFormat = 0
	s.state.Config.Modlist.SortField = 0
	s.state.Config.Modlist.SortAsc = true
	s.state.Config.Modlist.ShowProjectMeta = false
	s.state.Config.Modlist.RawPreview = false
}

func (s *settingsPage) View() string {
	if s.showConfirm {
		modal := ui.WarningBoxStyle.Render(
			"Are you sure you want to reset all settings?\n\n" +
				"[ y ] Yes, reset  [ n ] Cancel",
		)
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, modal)
	}

	title := sectionTitleStyle.Render("Settings")

	toggle := func(on bool) string {
		if on {
			return settingsOnStyle.Render("on ")
		}
		return settingsOffStyle.Render("off")
	}

	row := func(b key.Binding, on bool, desc string) string {
		keyHint := settingsKeyStyle.UnsetWidth().Render("[ " + b.Help().Key + " ]")
		status := toggle(on)

		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			keyHint, "  ",
			status, "  ",
			settingsDescStyle.Render(desc),
		)
	}

	autoRow := row(settingsKeys.ToggleAutoLoad, s.autoLoad, "Auto-load previous pack on startup")

	resetKey := "[ " + settingsKeys.ResetDefaults.Help().Key + " ]"
	resetHint := lipgloss.JoinHorizontal(lipgloss.Top,
		settingsKeyStyle.UnsetWidth().Render(resetKey),
		"  ",
		resetWarning.Render("Reset all settings to defaults"),
	)

	note := dimStyle.Render("Settings are saved automatically on exit.")

	lines := []string{title, "", autoRow, "", resetHint, "", note}
	content := strings.Join(lines, "\n")

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, content)
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
				Underline(true).
				Foreground(lipgloss.Color("#888888")).
				Bold(true).
				Width(3)

	settingsDescStyle = lipgloss.NewStyle().
				Foreground(ui.HighlightColor)

	settingsOnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5af78e")).
			Bold(true)

	settingsOffStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Red).
				Bold(true)

	resetWarning = lipgloss.NewStyle().
			Foreground(lipgloss.BrightRed).
			Background(lipgloss.Black).
			Bold(true)
)
