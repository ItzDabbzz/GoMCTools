package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"itzdabbzz.me/gomctools/internal/ui"
)

// modlistMode controls whether mods are listed in a single section or split
// by their distribution side (client / server / both).
type modlistMode int

const (
	modlistMerged    modlistMode = iota // all mods in one section
	modlistSeparated                    // separate sections by side
)

// modlistPage renders a configurable markdown mod list for the loaded pack.
type modlistPage struct {
	state        *ui.SharedState
	zone         *zone.Manager
	prefix       string
	lastPackPath string

	keys modlistKeyMap

	mode            modlistMode
	attachLinks     bool
	includeSide     bool
	includeSource   bool
	includeVersions bool
	includeFilename bool

	viewport   viewport.Model
	pageWidth  int
	pageHeight int
	contentW   int
	settingsW  int
	previewW   int
	status     string
	markdown   string
	lastWrap   int
	renderer   *glamour.TermRenderer
	rendererW  int
	dirty      bool

	// Cached markdown and its settings hash to avoid unnecessary re-renders.
	cachedMarkdown     string
	cachedSettingsHash uint64
}

// SetZone wires the page to the root bubblezone manager.
func (m *modlistPage) SetZone(z *zone.Manager, prefix string) {
	m.zone = z
	m.prefix = prefix
}

// NewModlistPage constructs a new Modlist Generator page backed by state.
func NewModlistPage(state *ui.SharedState) ui.Page {
	vp := viewport.New(0, 0)
	vp.MouseWheelDelta = 2
	vp.MouseWheelEnabled = true

	mode := modlistMerged
	if state.Config.Modlist.Mode == 1 {
		mode = modlistSeparated
	}

	return &modlistPage{
		state:           state,
		mode:            mode,
		attachLinks:     state.Config.Modlist.AttachLinks,
		includeSide:     state.Config.Modlist.IncludeSide,
		includeSource:   state.Config.Modlist.IncludeSource,
		includeVersions: state.Config.Modlist.IncludeVersions,
		includeFilename: state.Config.Modlist.IncludeFilename,
		viewport:        vp,
		status:          "Load a pack in Selector to generate a mod list.",
		dirty:           true,
		keys:            defaultModlistKeyMap(),
	}
}

func (m *modlistPage) Title() string { return "Modlist Generator" }
func (m *modlistPage) Init() tea.Cmd { return nil }

func (m *modlistPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	m.detectPackChange()
	var cmds []tea.Cmd

	switch typed := msg.(type) {
	case ui.PackLoadedMsg:
		if typed.Err != nil {
			m.status = fmt.Sprintf("Load failed: %v", typed.Err)
			return m, nil
		}
		m.status = fmt.Sprintf("Loaded %d mods", len(typed.Info.Mods))
		m.dirty = true
	case zone.MsgZoneInBounds:
		if typed.Event.Type == tea.MouseLeft {
			if id := m.resolveZoneID(typed.Zone); id != "" {
				m = m.handleClick(id)
			}
		}
	case tea.WindowSizeMsg:
		m.pageWidth = typed.Width
		m.pageHeight = typed.Height
		m.updateLayout()
	case tea.MouseMsg:
		if m.zone != nil && typed.Type == tea.MouseLeft {
			if id := m.resolveMouseZone(typed); id != "" {
				m = m.handleClick(id)
			}
		}
	case tea.KeyMsg:
		if key.Matches(typed, m.keys.LayoutMerged) {
			m = m.setLayout(modlistMerged)
			m.saveToConfig()
		} else if key.Matches(typed, m.keys.LayoutSplit) {
			m = m.setLayout(modlistSeparated)
			m.saveToConfig()
		} else if key.Matches(typed, m.keys.ToggleLinks) {
			m.attachLinks = !m.attachLinks
			m.dirty = true
			m.saveToConfig()
		} else if key.Matches(typed, m.keys.ToggleSide) {
			m.includeSide = !m.includeSide
			m.dirty = true
			m.saveToConfig()
		} else if key.Matches(typed, m.keys.ToggleSource) {
			m.includeSource = !m.includeSource
			m.dirty = true
			m.saveToConfig()
		} else if key.Matches(typed, m.keys.ToggleVersions) {
			m.includeVersions = !m.includeVersions
			m.dirty = true
			m.saveToConfig()
		} else if key.Matches(typed, m.keys.ToggleFilename) {
			m.includeFilename = !m.includeFilename
			m.dirty = true
			m.saveToConfig()
		} else if key.Matches(typed, m.keys.Copy) {
			m.status = m.copyMarkdown()
		} else if key.Matches(typed, m.keys.Export) {
			m.status = m.exportToFile()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if m.dirty {
		m.rebuild()
	}

	return m, tea.Batch(cmds...)
}

// updateLayout recomputes all dimension fields from the current window size.
func (m *modlistPage) updateLayout() {
	const gap = 2

	contentW := m.pageWidth - ui.DocStyle.GetHorizontalFrameSize() - ui.WindowStyle.GetHorizontalFrameSize()
	if contentW < 64 {
		contentW = 64
	}
	m.contentW = contentW

	settings := m.estimatedSettingsWidth(contentW)
	if settings < 44 {
		settings = 44
	}
	maxSettings := contentW - gap - 32
	if settings > maxSettings {
		settings = maxSettings
	}
	preview := contentW - gap - settings
	if preview < 32 {
		preview = 32
		settings = contentW - gap - preview
	}
	m.settingsW = settings
	m.previewW = preview

	if m.previewW > 0 {
		m.viewport.Width = m.previewW
	}
	frame := ui.WindowStyle.GetVerticalFrameSize()
	avail := m.pageHeight - ui.DocStyle.GetVerticalFrameSize() - frame - 4
	if avail < 8 {
		avail = 8
	}
	m.viewport.Height = avail

	wrap := preview - 2
	if wrap < 16 {
		wrap = 16
	}
	if wrap != m.lastWrap {
		m.lastWrap = wrap
		m.dirty = true
	}
}

// rebuild regenerates the markdown (if dirty) and updates the viewport content.
func (m *modlistPage) rebuild() {
	settingsHash := m.calculateSettingsHash()
	if m.dirty || settingsHash != m.cachedSettingsHash || m.cachedMarkdown == "" {
		m.markdown = m.generateMarkdown()
		m.cachedSettingsHash = settingsHash
		m.cachedMarkdown = m.markdown
	} else {
		m.markdown = m.cachedMarkdown
	}

	if m.state == nil || m.state.Pack.InstancePath == "" {
		m.viewport.SetContent(m.markdown)
	} else {
		m.viewport.SetContent(m.renderMarkdown(m.markdown))
	}
	m.viewport.SetYOffset(0)
	m.dirty = false
}

func (m *modlistPage) View() string {
	if m.pageWidth == 0 || m.pageHeight == 0 {
		return "Modlist Generator - initializing..."
	}

	settings := lipgloss.NewStyle().Width(m.settingsW).Render(m.renderSettings())

	var preview string
	if m.state != nil && m.state.Pack.InstancePath != "" {
		preview = lipgloss.NewStyle().Width(m.previewW).Render(m.viewport.View())
	} else {
		preview = lipgloss.NewStyle().
			Width(m.previewW).
			Height(m.viewport.Height).
			Render(statusStyle.Render("Load a pack from the Selector tab to generate a mod list."))
	}

	gap := strings.Repeat(" ", 2)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, settings, gap, preview)
	if m.contentW > 0 {
		layout = lipgloss.NewStyle().Width(m.contentW).Render(layout)
	}
	if m.status != "" {
		layout += "\n" + statusStyle.Render(m.status)
	}
	return layout
}

// renderSettings builds the left-hand settings column.
func (m *modlistPage) renderSettings() string {
	left := []string{
		sectionTitleStyle.Render("Layout"),
		m.markOption("layout-merged", fmt.Sprintf("%s Merged (1)", checkbox(m.mode == modlistMerged))),
		m.markOption("layout-split", fmt.Sprintf("%s Split by side (2)", checkbox(m.mode == modlistSeparated))),
		"",
		sectionTitleStyle.Render("Metadata"),
		m.markOption("meta-links", fmt.Sprintf("%s Links (a)", checkbox(m.attachLinks))),
		m.markOption("meta-side", fmt.Sprintf("%s Side tag (i)", checkbox(m.includeSide))),
		m.markOption("meta-source", fmt.Sprintf("%s Source (o)", checkbox(m.includeSource))),
		m.markOption("meta-version", fmt.Sprintf("%s Game versions (v)", checkbox(m.includeVersions))),
		m.markOption("meta-filename", fmt.Sprintf("%s Filenames (f)", checkbox(m.includeFilename))),
	}

	right := []string{
		sectionTitleStyle.Render("Actions"),
		m.markOption("action-copy", "Copy (c)"),
		m.markOption("action-export", "Export modlist.md (e)"),
	}

	colW := m.settingsW / 2
	if colW < 22 {
		colW = 22
	}
	leftCol := settingsStyle.Width(colW + 4).Render(strings.Join(left, "\n"))
	rightCol := settingsStyle.Width(colW + 4).Render(strings.Join(right, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
}

// --- click / zone helpers ---

func (m *modlistPage) markOption(id, content string) string {
	if m.zone == nil {
		return content
	}
	return m.zone.Mark(m.prefix+id, content)
}

func (m *modlistPage) clickIDs() []string {
	return []string{
		"layout-merged", "layout-split",
		"meta-links", "meta-side", "meta-source", "meta-version", "meta-filename",
		"action-copy", "action-export",
	}
}

func (m *modlistPage) resolveZoneID(z *zone.ZoneInfo) string {
	if z == nil || m.zone == nil {
		return ""
	}
	for _, id := range m.clickIDs() {
		if stored := m.zone.Get(m.prefix + id); stored == z {
			return id
		}
	}
	return ""
}

func (m *modlistPage) resolveMouseZone(msg tea.MouseMsg) string {
	if m.zone == nil {
		return ""
	}
	for _, id := range m.clickIDs() {
		if stored := m.zone.Get(m.prefix + id); stored != nil && stored.InBounds(msg) {
			return id
		}
	}
	return ""
}

func (m *modlistPage) handleClick(id string) *modlistPage {
	switch strings.TrimPrefix(id, m.prefix) {
	case "layout-merged":
		m = m.setLayout(modlistMerged)
		m.saveToConfig()
	case "layout-split":
		m = m.setLayout(modlistSeparated)
		m.saveToConfig()
	case "meta-links":
		m.attachLinks = !m.attachLinks
		m.dirty = true
		m.saveToConfig()
	case "meta-side":
		m.includeSide = !m.includeSide
		m.dirty = true
		m.saveToConfig()
	case "meta-source":
		m.includeSource = !m.includeSource
		m.dirty = true
		m.saveToConfig()
	case "meta-version":
		m.includeVersions = !m.includeVersions
		m.dirty = true
		m.saveToConfig()
	case "meta-filename":
		m.includeFilename = !m.includeFilename
		m.dirty = true
		m.saveToConfig()
	case "action-copy":
		m.status = m.copyMarkdown()
	case "action-export":
		m.status = m.exportToFile()
	}
	return m
}

func (m *modlistPage) setLayout(mode modlistMode) *modlistPage {
	if m.mode != mode {
		m.mode = mode
		m.dirty = true
	}
	return m
}

// saveToConfig writes the current display settings back into the shared config.
func (m *modlistPage) saveToConfig() {
	if m.state == nil || m.state.Config == nil {
		return
	}
	modeInt := 0
	if m.mode == modlistSeparated {
		modeInt = 1
	}
	m.state.Config.Modlist.Mode = modeInt
	m.state.Config.Modlist.AttachLinks = m.attachLinks
	m.state.Config.Modlist.IncludeSide = m.includeSide
	m.state.Config.Modlist.IncludeSource = m.includeSource
	m.state.Config.Modlist.IncludeVersions = m.includeVersions
	m.state.Config.Modlist.IncludeFilename = m.includeFilename
}

// detectPackChange marks the page dirty whenever a different pack is loaded.
func (m *modlistPage) detectPackChange() {
	if m.state == nil {
		return
	}
	path := m.state.Pack.InstancePath
	if path != m.lastPackPath {
		m.lastPackPath = path
		m.dirty = true
	}
}

func (m *modlistPage) estimatedSettingsWidth(total int) int {
	if total == 0 {
		total = m.pageWidth
	}
	if total == 0 {
		return 48
	}
	w := total / 2
	if w < 44 {
		w = 44
	}
	if w > 64 {
		w = 64
	}
	return w
}

// checkbox returns "[x]" when on and "[ ]" when off.
func checkbox(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func (m *modlistPage) ShortHelp() []key.Binding  { return m.keys.ShortHelp() }
func (m *modlistPage) FullHelp() [][]key.Binding { return m.keys.FullHelp() }

// --- styles ---

var (
	settingsStyle     = lipgloss.NewStyle().Padding(0, 2, 0, 0).MarginRight(3)
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ui.HighlightColor)
	statusStyle       = lipgloss.NewStyle().Foreground(ui.HighlightColor)
)
