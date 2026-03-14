package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/ItzDabbzz/GoMCTools/internal/config"
	"github.com/ItzDabbzz/GoMCTools/internal/logger"
	"github.com/ItzDabbzz/GoMCTools/internal/modpack"
	page "github.com/ItzDabbzz/GoMCTools/internal/pages"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
)

func main() {
	logger.InitLogger()
	defer logger.CloseLogger()

	logger.Info("Application Starting..")
	// Initialize UI styles for adaptive colors
	ui.Init()

	// Load config from disk.
	logger.Info("Loading Config From Disk")
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Warning: could not load config", "error", err)
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
	}

	state := modpack.NewSharedState()
	state.Config = &cfg
	logger.Info("State Loaded")

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
	logger.Info("Application Loaded")
	if _, err := program.Run(); err != nil {
		logger.Error("Error running program", "error", err)
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	// Persist config on clean exit.
	if err := config.Save(*state.Config); err != nil {
		logger.Error("Error saving config", "error", err)
		fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
	}
}
