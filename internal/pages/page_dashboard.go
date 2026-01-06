package pages

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"itzdabbzz.me/gomctools/internal/ui"
)

type dashboardPage struct {
	state *ui.SharedState
}

func NewDashboardPage(state *ui.SharedState) ui.Page {
	return dashboardPage{state: state}
}

func (d dashboardPage) Title() string {
	return "Dashboard"
}

func (d dashboardPage) Init() tea.Cmd {
	return nil
}

func (d dashboardPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	return d, nil
}

func (d dashboardPage) View() string {
	if d.state == nil || d.state.Pack.InstancePath == "" {
		if d.state != nil && d.state.LastLoadError != "" {
			return fmt.Sprintf("No pack loaded. Last error: %s", d.state.LastLoadError)
		}
		return "Load a Prism instance from the Selector tab to see pack details."
	}

	pack := d.state.Pack
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Instance: %s\n", pack.InstanceName))
	builder.WriteString(fmt.Sprintf("Path: %s\n", pack.InstancePath))
	minecraft := pack.MinecraftVersion
	if minecraft == "" {
		minecraft = "unknown"
	}
	builder.WriteString(fmt.Sprintf("Minecraft: %s\n", minecraft))
	loader := "unknown"
	if pack.LoaderUID != "" {
		loader = pack.LoaderUID
		if pack.LoaderVersion != "" {
			loader = fmt.Sprintf("%s %s", loader, pack.LoaderVersion)
		}
	}
	builder.WriteString(fmt.Sprintf("Loader: %s\n", loader))
	builder.WriteString(fmt.Sprintf("Mods: %d (Modrinth %d | CurseForge %d | Unknown %d)\n",
		pack.Counts.Total, pack.Counts.Modrinth, pack.Counts.Curseforge, pack.Counts.Unknown))

	return builder.String()

}
