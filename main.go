package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	page "itzdabbzz.me/gomctools/internal/pages"
	"itzdabbzz.me/gomctools/internal/ui"
)

func main() {
	state := ui.NewSharedState()
	pages := []ui.Page{
		page.NewHomePage(),
		page.NewSelectorPage(state),
		page.NewDashboardPage(state),
		page.NewModlistPage(state),
		page.NewCleanerPage(state),
		page.NewSettingsPage(),
	}

	program := tea.NewProgram(ui.NewModel(state, pages), tea.WithMouseAllMotion())
	if _, err := program.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
