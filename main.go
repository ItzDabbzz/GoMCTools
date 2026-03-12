package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"itzdabbzz.me/gomctools/internal/config"
	page "itzdabbzz.me/gomctools/internal/pages"
	"itzdabbzz.me/gomctools/internal/ui"
)

func main() {
	// Load config from disk.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
	}

	state := ui.NewSharedState()
	state.Config = &cfg

	pages := []ui.Page{
		page.NewHomePage(),
		page.NewSelectorPage(state),
		page.NewDashboardPage(state),
		page.NewModlistPage(state),
		page.NewCleanerPage(state),
		page.NewSettingsPage(state),
	}

	model := ui.NewModel(state, pages)

	// When auto-load is configured, start on the Selector page so that its
	// Init() method fires and triggers the pack load automatically.
	if cfg.AutoLoadPreviousState && cfg.Selector.LastPath != "" {
		model.SetActivePage(1)
	}

	program := tea.NewProgram(model, tea.WithMouseAllMotion())
	if _, err := program.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	// Persist config on clean exit.
	if err := config.Save(*state.Config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
	}
}
