package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

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
}

const (
	MinWidth  = 60
	MinHeight = 15
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

// Init runs the active page's Init command.
// Auto-loading is handled entirely by selectorPage.Init() when the Selector
// page is the active page on startup (set by main.go).
func (m Model) Init() tea.Cmd {
	if len(m.pages) == 0 {
		return nil
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
		m.help.Width = msg.Width
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

	// Footer: when the full help overlay is open, show a minimal "close" hint
	// so the footer doesn't duplicate what is already visible in the content area.
	var footer string
	if m.help.ShowAll {
		closeKey := key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "close help"))
		footer = m.help.ShortHelpView([]key.Binding{closeKey})
	} else {
		combined := []key.Binding{}
		if provider, ok := m.pages[m.activePage].(ShortHelpProvider); ok {
			combined = append(combined, provider.ShortHelp()...)
		}
		combined = append(combined, DefaultHelpBindings()...)
		footer = m.help.ShortHelpView(combined)
	}

	contentWidth := innerWidth - windowStyle.GetHorizontalFrameSize()
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

	// Content: when help overlay is open render it here (single location);
	// otherwise render the active page as normal.
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

// pageTitles returns the Title() string for every registered page.
func pageTitles(pages []Page) []string {
	titles := make([]string, 0, len(pages))
	for _, p := range pages {
		titles = append(titles, p.Title())
	}
	return titles
}

// buildHelpGroups collects keybinding columns from every page.
// Pages that implement FullHelp() contribute their full columns; pages that
// only implement ShortHelpProvider contribute a single column of short keys.
func buildHelpGroups(pages []Page) [][]key.Binding {
	groups := [][]key.Binding{}
	for _, p := range pages {
		if hf, ok := p.(interface{ FullHelp() [][]key.Binding }); ok {
			for _, col := range hf.FullHelp() {
				groups = append(groups, col)
			}
			continue
		}
		if sh, ok := p.(ShortHelpProvider); ok {
			groups = append(groups, sh.ShortHelp())
		}
	}
	return groups
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
// after all frame elements (tab bar, window border+padding, footer, bottom rule)
// are subtracted. These are the dimensions pages should size their viewports to.
//
// Tab bar renders to exactly 3 lines (top border + content + bottom connection).
// Footer is always 1 line of short-help in normal mode.
// The bottom "└─┘" rule adds 1 more line outside the window frame.
func (m Model) computeContentSize() (width, height int) {
	if m.width == 0 || m.height == 0 {
		return 0, 0
	}
	const (
		tabBarLines = 3 // top border + content row + bottom border/connector
		footerLines = 1 // single line of short-help
		bottomRule  = 1 // the "└───┘" row drawn below the window box
	)
	innerWidth := m.width - docStyle.GetHorizontalFrameSize()
	width = innerWidth - windowStyle.GetHorizontalFrameSize()
	if width < 0 {
		width = 0
	}

	available := m.height - docStyle.GetVerticalFrameSize()
	pageHeight := available - tabBarLines - footerLines - bottomRule - windowStyle.GetVerticalFrameSize()
	if pageHeight < 1 {
		pageHeight = 1
	}
	return width, pageHeight
}

// resizeCmd emits both a raw WindowSizeMsg (for components like filepicker and
// textinput that expect it) and a ContentSizeMsg with the exact inner dimensions
// pages should use for viewport sizing.
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
