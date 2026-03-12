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

type homePage struct {
	width  int
	height int
}

func NewHomePage() ui.Page {
	return homePage{}
}

func (h homePage) Title() string {
	return "Home"
}

func (h homePage) Init() tea.Cmd {
	return nil
}

func (h homePage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height
	}
	return h, nil
}

func (h homePage) View() string {
	logo := strings.Join(homeLogo, "\n")
	return lipgloss.NewStyle().Align(lipgloss.Center).Render(logo)
}

func (h homePage) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter/space", "continue")),
	}
}

func (h homePage) FullHelp() [][]key.Binding {
	return [][]key.Binding{h.ShortHelp()}
}
