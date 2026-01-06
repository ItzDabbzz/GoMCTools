package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type clearErrorMsg struct{}

func clearErrorAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}
