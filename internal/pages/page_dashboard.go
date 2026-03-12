package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"itzdabbzz.me/gomctools/internal/ui"
)

// dashboardPage displays a summary of the currently loaded modpack.
type dashboardPage struct {
	state  *ui.SharedState
	width  int
	height int
}

func NewDashboardPage(state *ui.SharedState) ui.Page {
	return dashboardPage{state: state}
}

func (d dashboardPage) Title() string { return "Dashboard" }
func (d dashboardPage) Init() tea.Cmd { return nil }

func (d dashboardPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
	}
	return d, nil
}

func (d dashboardPage) View() string {
	if d.state == nil || d.state.Pack.InstancePath == "" {
		if d.state != nil && d.state.LastLoadError != "" {
			return errorStyle.Render("✗  " + d.state.LastLoadError)
		}
		return dimStyle.Render("Load a Prism instance from the Selector tab to see pack details.")
	}

	pack := d.state.Pack

	label := func(s string) string {
		return labelStyle.Render(s)
	}
	value := func(s string) string {
		return valueStyle.Render(s)
	}
	row := func(l, v string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, label(l), value(v))
	}

	minecraft := pack.MinecraftVersion
	if minecraft == "" {
		minecraft = "unknown"
	}
	loader := "unknown"
	if pack.LoaderUID != "" {
		loader = pack.LoaderUID
		if pack.LoaderVersion != "" {
			loader = fmt.Sprintf("%s  %s", loader, pack.LoaderVersion)
		}
	}
	modLine := fmt.Sprintf("%d   (Modrinth %d · CurseForge %d · Unknown %d)",
		pack.Counts.Total, pack.Counts.Modrinth, pack.Counts.Curseforge, pack.Counts.Unknown)

	rows := []string{
		dashHeaderStyle.Render(pack.InstanceName),
		"",
		row("Instance ", pack.InstancePath),
		row("Minecraft", minecraft),
		row("Loader   ", loader),
		row("Mods     ", modLine),
	}

	return strings.Join(rows, "\n")
}

// dashboardPage has no page-specific bindings — it is read-only.
func (d dashboardPage) ShortHelp() []key.Binding  { return nil }
func (d dashboardPage) FullHelp() [][]key.Binding { return nil }

// --- styles ---

var (
	dashHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.HighlightColor).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Width(10).
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"})

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#111111", Dark: "#dddddd"})

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"}).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5f56"))
)
