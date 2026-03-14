package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/ItzDabbzz/GoMCTools/internal/config"
	page "github.com/ItzDabbzz/GoMCTools/internal/pages"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
)

func main() {
	// Initialize UI styles for adaptive colors
	ui.Init()

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

	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	// Persist config on clean exit.
	if err := config.Save(*state.Config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
	}
}
