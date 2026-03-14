/*
Package ui implements a tabbed multi-page TUI application using Bubble Tea.
It provides the root Model that manages tab switching via keyboard/mouse,
renders a consistent framed layout (tab bar, content window, footer),
handles window resizing with precise inner content dimensions passed to pages,
and overlays contextual help. Pages implement the Page interface and can
opt-in to advanced features like zone-based mouse handling, key capture,
and rich help integration.
*/
package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// Model is the root application state, managing multiple Page instances
// within a tabbed layout. It tracks dimensions, zones for mouse interaction,
// shared app state, and built-in help.
type Model struct {
	pages         []Page
	activePage    int
	width         int
	height        int
	contentWidth  int // inner width available to pages (inside window frame)
	contentHeight int // inner height available to pages (inside window frame)
	zone          *zone.Manager
	zonePrefix    string
	state         *SharedState
	pageZones     map[int]string
	help          help.Model
	keys          GlobalKeyMap
}

const (
	// MinWidth is the minimum usable terminal width.
	MinWidth = 150
	// MinHeight is the minimum usable terminal height.
	MinHeight = 26
)

// NewModel initializes a Model ready for tea.NewProgram.
// It sets up the zone manager, default keys, and help model.
// If state is nil, a new default SharedState is created.
func NewModel(state *SharedState, pages []Page) Model {
	if state == nil {
		state = &SharedState{}
	}
	if len(pages) == 0 {
		pages = []Page{}
	}
	z := zone.New()
	return Model{
		pages:      pages,
		zone:       z,
		zonePrefix: z.NewPrefix(),
		state:      state,
		pageZones:  map[int]string{},
		help:       help.New(),
		keys:       DefaultKeys,
	}
}

// Init runs the active page's Init command.
func (m Model) Init() tea.Cmd {
	if len(m.pages) == 0 {
		return nil
	}
	return m.pages[m.activePage].Init()
}

// Update processes all incoming tea.Msg.
// It handles global navigation (tab switching, help toggle, quit) unless the
// active page captures input. It also manages mouse zone interactions for tabs
// and delegates specific messages to the active page.
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
			mouse := msg.Mouse()
			if mouse.Button == tea.MouseLeft {
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
	case NavigateMsg:
		m.activePage = clampInt(msg.Page, 0, len(m.pages)-1)
		return m, m.resizeCmd()
	case ToggleHelpMsg:
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case tea.KeyMsg:
		if allowGlobalNav {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.NextTab):
				m.activePage = clampInt(m.activePage+1, 0, len(m.pages)-1)
				return m, m.resizeCmd()
			case key.Matches(msg, m.keys.PrevTab):
				m.activePage = clampInt(m.activePage-1, 0, len(m.pages)-1)
				return m, m.resizeCmd()
			case key.Matches(msg, m.keys.Help):
				m.help.ShowAll = !m.help.ShowAll
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
		m.contentWidth, m.contentHeight = m.computeContentSize()
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

// View renders the full TUI frame: tab bar, active page (or help overlay), and footer.
func (m Model) View() tea.View {
	if len(m.pages) == 0 {
		v := tea.NewView("No pages available")
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	if m.width > 0 && m.height > 0 && (m.width < MinWidth || m.height < MinHeight) {
		warning := fmt.Sprintf("Terminal too small\nW: %d (min %d)  H: %d (min %d)", m.width, MinWidth, m.height, MinHeight)
		box := warningBoxStyle.Render(warning)

		footer := m.help.ShortHelpView(m.pageShortHelp())
		footerHeight := lipgloss.Height(footer)
		contentHeight := m.height - footerHeight
		if contentHeight < 3 {
			contentHeight = 3
		}
		placed := lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, box)
		v := tea.NewView(placed + "\n" + footer)
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	titles := pageTitles(m.pages)
	innerWidth := m.width - docStyle.GetHorizontalFrameSize()
	if innerWidth < 0 {
		innerWidth = 0
	}

	contentWidth := innerWidth - windowStyle.GetHorizontalFrameSize()
	if contentWidth < 0 {
		contentWidth = 0
	}

	rowLines := strings.Split(renderTabs(m.zone, m.zonePrefix, titles, m.activePage), "\n")
	lastIdx := len(rowLines) - 1
	for i, line := range rowLines {
		if i != lastIdx {
			rowLines[i] = line
			continue
		}
		fillWidth := contentWidth - lipgloss.Width(line) - 1
		if fillWidth < 0 {
			fillWidth = 0
		}
		rowLines[i] = line + borderFillStyle.Render(strings.Repeat("─", fillWidth)+"┐")
	}
	row := strings.Join(rowLines, "\n")

	// Footer: when help overlay is open show a minimal close hint only.
	var footer string
	if m.help.ShowAll {
		closeKey := key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "close help"))
		footer = m.help.ShortHelpView([]key.Binding{closeKey})
	} else {
		footer = m.help.ShortHelpView(m.pageShortHelp())
	}

	contentWidth = innerWidth - windowStyle.GetHorizontalFrameSize()
	if contentWidth < 0 {
		contentWidth = 0
	}

	availableHeight := m.height - docStyle.GetVerticalFrameSize()
	tabHeight := lipgloss.Height(row)
	footerHeight := lipgloss.Height(footer)

	frameH := windowStyle.GetVerticalFrameSize()
	minContentHeight := frameH + 1
	contentHeight := minContentHeight + 6
	if m.height > 0 {
		contentHeight = availableHeight - tabHeight - footerHeight - 1
		if contentHeight < minContentHeight {
			contentHeight = minContentHeight
		}
	}

	// Content area: help overlay or active page.
	var content string
	if m.help.ShowAll {
		groups := buildHelpGroups(m.pages)
		helpText := m.help.FullHelpView(groups)
		content = windowStyle.
			Width(contentWidth).
			Height(contentHeight).
			MaxHeight(contentHeight).
			Render(helpText)
	} else {
		content = windowStyle.
			Width(contentWidth).
			Height(contentHeight).
			MaxHeight(contentHeight).
			Render(m.pages[m.activePage].View())
	}

	bottom := ""

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
		rendered = m.zone.Scan(rendered)
	}
	v := tea.NewView(rendered)
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// pageShortHelp merges the active page's short bindings with global defaults.
func (m Model) pageShortHelp() []key.Binding {
	combined := []key.Binding{}
	if provider, ok := m.pages[m.activePage].(ShortHelpProvider); ok {
		combined = append(combined, provider.ShortHelp()...)
	}
	combined = append(combined, m.keys.ShortHelp()...)
	return combined
}

// buildHelpGroups collects keybinding columns from every page that implements
// FullHelp, followed by the global bindings as the final column.
func buildHelpGroups(pages []Page) [][]key.Binding {
	groups := [][]key.Binding{}
	for _, p := range pages {
		if hf, ok := p.(interface{ FullHelp() [][]key.Binding }); ok {
			for _, col := range hf.FullHelp() {
				if len(col) > 0 {
					groups = append(groups, col)
				}
			}
			continue
		}
		if sh, ok := p.(ShortHelpProvider); ok {
			groups = append(groups, sh.ShortHelp())
		}
	}
	return groups
}

// pageTitles returns the Title() string for every registered page.
func pageTitles(pages []Page) []string {
	titles := make([]string, 0, len(pages))
	for _, p := range pages {
		titles = append(titles, p.Title())
	}
	return titles
}

// findPageIndexByTitle returns the index of the first page whose Title()
// matches title (case-insensitive), or -1 if not found.
func findPageIndexByTitle(pages []Page, title string) int {
	for i, p := range pages {
		if strings.EqualFold(p.Title(), title) {
			return i
		}
	}
	return -1
}

// computeContentSize calculates the exact inner dimensions available to a page
// after all frame elements are subtracted. Pages should size their viewports
// to these dimensions via ContentSizeMsg rather than guessing at frame overhead.
func (m Model) computeContentSize() (width, height int) {
	if m.width == 0 || m.height == 0 {
		return 0, 0
	}
	const (
		tabBarLines = 3 // top border + content row + bottom border/connector
		footerLines = 1 // single line of short-help
		bottomRule  = 1 // the "└───┘" row drawn below the window box
	)

	// Get frame sizes - with defensive minimums
	docHFrame := docStyle.GetHorizontalFrameSize()
	winHFrame := windowStyle.GetHorizontalFrameSize()
	docVFrame := docStyle.GetVerticalFrameSize()
	winVFrame := windowStyle.GetVerticalFrameSize()

	// Calculate with minimum floor of 20 chars width and 8 lines height
	// This ensures the UI is usable even on small or unusual terminal sizes
	width = m.width - docHFrame - winHFrame
	if width < 20 {
		width = 20
	}

	height = m.height - docVFrame - winVFrame - tabBarLines - footerLines - bottomRule
	if height < 8 {
		height = 8
	}

	return width, height
}

// resizeCmd emits both a raw WindowSizeMsg (for bubbles components that expect
// it) and a ContentSizeMsg with the exact inner dimensions pages should use
// for viewport sizing.
func (m Model) resizeCmd() tea.Cmd {
	if m.width == 0 || m.height == 0 {
		return nil
	}
	w, h := m.width, m.height
	cw, ch := m.contentWidth, m.contentHeight
	return tea.Batch(
		func() tea.Msg { return tea.WindowSizeMsg{Width: w, Height: h} },
		func() tea.Msg { return ContentSizeMsg{Width: cw, Height: ch} },
	)
}

// SetActivePage changes the active page to the given index, clamped to valid bounds.
func (m *Model) SetActivePage(index int) {
	m.activePage = clampInt(index, 0, len(m.pages)-1)
}

// GetActivePage returns the index of the currently displayed page.
func (m Model) GetActivePage() int {
	return m.activePage
}
