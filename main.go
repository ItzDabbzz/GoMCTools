package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"itzdabbzz.me/gomctools/internal/config"
	page "itzdabbzz.me/gomctools/internal/pages"
	"itzdabbzz.me/gomctools/internal/ui"
)

func main() {
	// Load config from disk
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

	// If auto-load is enabled and there's a last pack path, switch to Selector page
	// so that its Init() method gets called and triggers the auto-load
	if cfg.AutoLoadPreviousState && cfg.Selector.LastPath != "" {
		abs, err := filepath.Abs(cfg.Selector.LastPath)
		if err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				// Switch to Selector page (index 1)
				model.SetActivePage(1)
			}
		}
	}

	program := tea.NewProgram(model, tea.WithMouseAllMotion())
	if _, err := program.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	// Save config on exit
	if err := config.Save(*state.Config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
	}
}
