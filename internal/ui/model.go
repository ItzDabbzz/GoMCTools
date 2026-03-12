package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type Model struct {
	pages      []Page
	activePage int
	width      int
	height     int
	zone       *zone.Manager
	zonePrefix string
	state      *SharedState
	pageZones  map[int]string
	help       help.Model
}

const (
	MinWidth  = 60
	MinHeight = 27
)

func NewModel(state *SharedState, pages []Page) Model {
	if state == nil {
		state = &SharedState{}
	}
	if len(pages) == 0 {
		pages = []Page{}
	}
	z := zone.New()
	return Model{pages: pages, zone: z, zonePrefix: z.NewPrefix(), state: state, pageZones: map[int]string{}, help: help.New()}
}

func (m Model) Init() tea.Cmd {
	if len(m.pages) == 0 {
		return nil
	}

	// Check if auto-load is enabled and we're on the home page (index 0)
	// If so, trigger auto-load by returning a command to load the pack
	if m.activePage == 0 && m.state != nil && m.state.Config != nil {
		if m.state.Config.AutoLoadPreviousState && m.state.Config.Selector.LastPath != "" {
			abs, err := filepath.Abs(m.state.Config.Selector.LastPath)
			if err == nil {
				if info, err := os.Stat(abs); err == nil && info.IsDir() {
					// Return a command to load the pack (this will trigger PackLoadedMsg)
					return LoadPackCmd(abs)
				}
			}
		}
	}

	return m.pages[m.activePage].Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.pages) == 0 {
		return m, nil
	}

	resizeCmd := tea.Cmd(nil)

	// Provide the root zone manager to the active page when supported.
	if zpage, ok := m.pages[m.activePage].(ZoneAware); ok && m.zone != nil {
		prefix, ok := m.pageZones[m.activePage]
		if !ok {
			prefix = m.zone.NewPrefix()
			m.pageZones[m.activePage] = prefix
		}
		zpage.SetZone(m.zone, prefix)
	}

	allowGlobalNav := true
	if capturer, ok := m.pages[m.activePage].(KeyCapturer); ok {
		allowGlobalNav = !capturer.CaptureGlobalNav()
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.zone != nil {
			m.zone.AnyInBounds(m, msg)
			if msg.Type == tea.MouseLeft {
				for i := range m.pages {
					z := m.zone.Get(fmt.Sprintf("%stab-%d", m.zonePrefix, i))
					if z != nil && z.InBounds(msg) {
						m.activePage = i
						return m, m.resizeCmd()
					}
				}
			}
		}
	case zone.MsgZoneInBounds:
		for i := range m.pages {
			if m.zone != nil && m.zone.Get(fmt.Sprintf("%stab-%d", m.zonePrefix, i)) == msg.Zone {
				m.activePage = i
				return m, m.resizeCmd()
			}
		}
		updatedPage, cmd := m.pages[m.activePage].Update(msg)
		m.pages[m.activePage] = updatedPage
		return m, cmd
	case tea.KeyMsg:
		if allowGlobalNav {
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "right", "l", "n", "tab":
				m.activePage = clampInt(m.activePage+1, 0, len(m.pages)-1)
				return m, m.resizeCmd()
			case "left", "h", "p", "shift+tab":
				m.activePage = clampInt(m.activePage-1, 0, len(m.pages)-1)
				return m, m.resizeCmd()
			case "?":
				m.help.ShowAll = !m.help.ShowAll
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case PackLoadedMsg:
		if m.state != nil {
			if msg.Err != nil {
				m.state.LastLoadError = msg.Err.Error()
				m.state.Pack = PackInfo{}
			} else {
				m.state.Pack = msg.Info
				m.state.LastLoadError = ""
				if idx := findPageIndexByTitle(m.pages, "Dashboard"); idx >= 0 {
					m.activePage = idx
					resizeCmd = m.resizeCmd()
				}
			}
		}
	}

	updatedPage, cmd := m.pages[m.activePage].Update(msg)
	m.pages[m.activePage] = updatedPage
	return m, tea.Batch(cmd, resizeCmd)
}

func (m Model) View() string {
	if len(m.pages) == 0 {
		return "No pages available"
	}

	if m.width > 0 && m.height > 0 && (m.width < MinWidth || m.height < MinHeight) {
		warning := fmt.Sprintf("Warning: Terminal too small\nX: %d (Required: %d)\nY: %d (Required: %d)", m.width, MinWidth, m.height, MinHeight)
		box := warningBoxStyle.Render(warning)

		footer := renderFooter(defaultKeyBindings())
		if provider, ok := m.pages[m.activePage].(ShortHelpProvider); ok {
			per := RenderShortHelp(provider.ShortHelp())
			if per != "" {
				footer = lipgloss.JoinHorizontal(lipgloss.Bottom, per, footer)
			}
		}
		footerHeight := lipgloss.Height(footer)
		contentHeight := m.height - footerHeight
		if contentHeight < 3 {
			contentHeight = 3
		}

		placed := lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, box)
		return placed + "\n" + footer
	}

	titles := pageTitles(m.pages)
	innerWidth := m.width - docStyle.GetHorizontalFrameSize()
	if innerWidth < 0 {
		innerWidth = 0
	}

	rowLines := strings.Split(renderTabs(m.zone, m.zonePrefix, titles, m.activePage), "\n")
	lastIdx := len(rowLines) - 1
	for i, line := range rowLines {
		// Only extend the bottom tab line to keep a single horizontal connector.
		if i != lastIdx {
			rowLines[i] = line
			continue
		}

		endChar := "┐"
		fillWidth := innerWidth - lipgloss.Width(line) - 1
		if fillWidth < 0 {
			fillWidth = 0
		}
		rowLines[i] = line + borderFillStyle.Render(strings.Repeat("─", fillWidth)+endChar)
	}
	row := strings.Join(rowLines, "\n")
	// Build the initial footer with global default bindings
	// Build combined bindings: page short-help (if available) + global defaults
	combined := []key.Binding{}
	if provider, ok := m.pages[m.activePage].(ShortHelpProvider); ok {
		combined = append(combined, provider.ShortHelp()...)
	}
	combined = append(combined, DefaultHelpBindings()...)
	m.help.Width = innerWidth
	footer := m.help.ShortHelpView(combined)

	// If the full help overlay is requested, render it in the footer area
	if m.help.ShowAll {
		groups := [][]key.Binding{}
		for _, p := range m.pages {
			if hf, ok := p.(interface{ FullHelp() [][]key.Binding }); ok {
				cols := hf.FullHelp()
				for _, c := range cols {
					groups = append(groups, c)
				}
				continue
			}
			if sh, ok := p.(ShortHelpProvider); ok {
				groups = append(groups, sh.ShortHelp())
			}
		}
		m.help.Width = innerWidth
		footer = m.help.FullHelpView(groups)
	}

	contentWidth := innerWidth - windowStyle.GetHorizontalFrameSize()
	if contentWidth < 0 {
		contentWidth = 0
	}

	availableHeight := m.height - docStyle.GetVerticalFrameSize()
	tabHeight := lipgloss.Height(row)
	footerHeight := lipgloss.Height(footer)

	frameH := windowStyle.GetVerticalFrameSize()
	minContentHeight := frameH + 1        // at least one line of content inside frame
	contentHeight := minContentHeight + 6 // fallback if we have no window size yet
	if m.height > 0 {
		contentHeight = availableHeight - tabHeight - footerHeight - 1 // extra line between row/content/footer
		if contentHeight < minContentHeight {
			contentHeight = minContentHeight
		}
	}

	var content string
	if m.help.ShowAll {
		// Build columns/groups for full help using each page's FullHelp when available,
		// otherwise fall back to the page's ShortHelp.
		groups := [][]key.Binding{}
		for _, p := range m.pages {
			if hf, ok := p.(interface{ FullHelp() [][]key.Binding }); ok {
				cols := hf.FullHelp()
				for _, c := range cols {
					groups = append(groups, c)
				}
				continue
			}
			if sh, ok := p.(ShortHelpProvider); ok {
				groups = append(groups, sh.ShortHelp())
			}
		}

		helpContent := m.help.FullHelpView(groups)
		content = windowStyle.
			Width(contentWidth).
			Height(contentHeight).
			MaxHeight(contentHeight).
			Render(helpContent)
	} else {
		content = windowStyle.
			Width(contentWidth).
			Height(contentHeight).
			MaxHeight(contentHeight).
			Render(m.pages[m.activePage].View())
	}

	bottom := ""
	if innerWidth >= 2 {
		bottom = borderFillStyle.Render("└" + strings.Repeat("─", innerWidth-2) + "┘")
	}

	doc := strings.Builder{}
	doc.WriteString(row)

	doc.WriteString("\n")
	doc.WriteString(content)
	if bottom != "" {
		doc.WriteString("\n")
		doc.WriteString(bottom)
	}
	doc.WriteString("\n")
	doc.WriteString(footer)

	container := docStyle.Width(m.width)
	if m.height > 0 {
		container = container.MaxHeight(m.height)
	}
	rendered := container.Render(doc.String())
	if m.zone != nil {
		return m.zone.Scan(rendered)
	}

	return rendered
}

func pageTitles(pages []Page) []string {
	titles := make([]string, 0, len(pages))
	for _, p := range pages {
		titles = append(titles, p.Title())
	}
	return titles
}

func findPageIndexByTitle(pages []Page, title string) int {
	for i, p := range pages {
		if strings.EqualFold(p.Title(), title) {
			return i
		}
	}
	return -1
}

func (m Model) resizeCmd() tea.Cmd {
	if m.width == 0 || m.height == 0 {
		return nil
	}
	w, h := m.width, m.height
	return func() tea.Msg { return tea.WindowSizeMsg{Width: w, Height: h} }
}

func (m *Model) SetActivePage(index int) {
	m.activePage = clampInt(index, 0, len(m.pages)-1)
}

func (m Model) GetActivePage() int {
	return m.activePage
}
