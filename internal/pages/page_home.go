package pages

// page_home.go
import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
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
	// ContentSizeMsg carries the exact inner dimensions already stripped of
	// the tab bar, footer, window border, and padding — use these instead of
	// the raw tea.WindowSizeMsg so centering is always pixel-perfect.
	case ui.ContentSizeMsg:
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

	// Ensure we have reasonable dimensions for display
	displayWidth := h.width
	if displayWidth < 40 {
		displayWidth = 80 // Default reasonable width
	}
	displayHeight := h.height
	if displayHeight < 10 {
		displayHeight = 24
	}

	// Use a fixed reasonable width for centering elements
	// This should match the widest line in the logo
	logoWidth := 52 // Width of the GoMCTools logo

	logoStyled := lipgloss.NewStyle().
		Foreground(ui.HighlightColor).
		Bold(true).
		Width(logoWidth).
		Align(lipgloss.Center).
		Render(logo)

	subtitleStyled := lipgloss.NewStyle().
		Foreground(ui.HighlightColor).
		Italic(true).
		Width(logoWidth).
		Align(lipgloss.Center).
		Render(homeSubtitle)

	hintStyled := lipgloss.NewStyle().
		Foreground(ui.HighlightColor).
		Width(logoWidth).
		Align(lipgloss.Center).
		Render("Press enter or tab to get started")

	content := lipgloss.JoinVertical(lipgloss.Center, logoStyled, "", subtitleStyled, "", hintStyled)

	// Use Width + Align to center the content within available width
	if displayWidth > 0 {
		centeredContent := lipgloss.NewStyle().
			Width(displayWidth).
			Align(lipgloss.Center).
			Render(content)
		if displayHeight > 0 {
			return lipgloss.Place(displayWidth, displayHeight, lipgloss.Center, lipgloss.Center, centeredContent)
		}
		return centeredContent
	}
	return content
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
