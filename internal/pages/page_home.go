package pages

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"itzdabbzz.me/gomctools/internal/ui"
)

var homeLogo = []string{
	".d88b        8b   d8 .d88b 88888             8      ",
	"8P www .d8b. 8YbmdP8 8P      8   .d8b. .d8b. 8 d88b ",
	"8b  d8 8' .8 8  \"  8 8b      8   8' .8 8' .8 8 `Yb. ",
	"`Y88P' `Y8P' 8     8 `Y88P   8   `Y8P' `Y8P' 8 Y88P ",
}

var homeSubtitle = "Prism Launcher modpack tools"

type homePage struct {
	width  int
	height int
}

func NewHomePage() ui.Page {
	return homePage{}
}

func (h homePage) Title() string { return "Home" }
func (h homePage) Init() tea.Cmd { return nil }

func (h homePage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, homeKeys.Continue):
			return h, func() tea.Msg { return ui.NavigateMsg{Page: 1} }
		}
	}
	return h, nil
}

func (h homePage) View() string {
	logo := strings.Join(homeLogo, "\n")

	logoStyled := lipgloss.NewStyle().
		Foreground(ui.HighlightColor).
		Bold(true).
		Render(logo)

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"}).
		Italic(true).
		Render(homeSubtitle)

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"}).
		Render("Press enter or tab to get started")

	// windowStyle in model.go already has Align(Center) — just return the
	// content directly. Setting Width(h.width) here overshoots the frame
	// (which has its own border/padding) and breaks centering.
	return lipgloss.JoinVertical(lipgloss.Center, logoStyled, "", subtitle, "", hint)
}

// homeKeyMap holds the bindings specific to the home page.
type homeKeyMap struct {
	Continue key.Binding
}

func (k homeKeyMap) ShortHelp() []key.Binding  { return []key.Binding{k.Continue} }
func (k homeKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{{k.Continue}} }

var homeKeys = homeKeyMap{
	Continue: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter", "open selector"),
	),
}

func (h homePage) ShortHelp() []key.Binding  { return homeKeys.ShortHelp() }
func (h homePage) FullHelp() [][]key.Binding { return homeKeys.FullHelp() }
