package pages

// page_modlist.go

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/modpack"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
	zone "github.com/lrstanley/bubblezone/v2"
)

// ─── Output format ────────────────────────────────────────────────────────────

// modlistFormat controls the syntax used when generating the output string.
type modlistFormat int

const (
	modlistFormatBullet modlistFormat = iota // markdown bullet list (default)
	modlistFormatTable                       // GFM pipe table
	modlistFormatBBCode                      // forum BBCode
)

// ─── Sort options ─────────────────────────────────────────────────────────────

// modlistSort controls which field mods are sorted by within each section.
type modlistSort int

const (
	modlistSortName   modlistSort = iota // alphabetical by mod name (default)
	modlistSortSource                    // by source (Modrinth / CurseForge / unknown)
	modlistSortSide                      // by distribution side
)

// modlistMode controls whether mods are listed in a single section or split
// by their distribution side (client / server / both).
type modlistMode int

const (
	modlistMerged    modlistMode = iota // all mods in one section
	modlistSeparated                    // separate sections by side
)

// ─── Page model ───────────────────────────────────────────────────────────────

// modlistPage renders a configurable mod list for the loaded pack.
type modlistPage struct {
	state        *modpack.SharedState
	zone         *zone.Manager
	prefix       string
	lastPackPath string

	keys modlistKeyMap

	// Layout / grouping
	mode modlistMode

	// Output format
	outputFormat modlistFormat

	// Sort
	sortField modlistSort
	sortAsc   bool

	// Metadata column toggles
	attachLinks     bool
	includeSide     bool
	includeSource   bool
	includeVersions bool
	includeFilename bool

	// Header / view options
	showProjectMeta bool // prepend pack author / version / description
	rawPreview      bool // show raw output source instead of glamour-rendered

	viewport   viewport.Model
	pageWidth  int
	pageHeight int
	// contentW/contentH are the exact inner dimensions delivered by
	// ContentSizeMsg — use these instead of subtracting frame sizes manually.
	contentW  int
	contentH  int
	settingsW int
	previewW  int
	status    string
	// markdown holds the current raw generated output (markdown or BBCode).
	markdown  string
	lastWrap  int
	renderer  *glamour.TermRenderer
	rendererW int
	dirty     bool

	// Cached raw output and its settings hash to avoid unnecessary re-renders.
	cachedMarkdown     string
	cachedSettingsHash uint64
}

// ─── Zone wiring ──────────────────────────────────────────────────────────────

// SetZone wires the page to the root bubblezone manager.
func (m *modlistPage) SetZone(z *zone.Manager, prefix string) {
	m.zone = z
	m.prefix = prefix
}

// ─── Constructor ──────────────────────────────────────────────────────────────

// NewModlistPage constructs a new Modlist Generator page backed by state.
func NewModlistPage(state *modpack.SharedState) ui.Page {
	vp := viewport.New(viewport.WithWidth(0), viewport.WithHeight(0))
	vp.MouseWheelDelta = 2
	vp.MouseWheelEnabled = true
	// Ensure viewport style doesn't add borders/padding that could offset zones
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(ui.HighlightColor)

	mode := modlistMerged
	if state.Config.Modlist.Mode == 1 {
		mode = modlistSeparated
	}
	if state.Config.Modlist.Mode == 1 {
		mode = modlistSeparated
	}

	outputFormat := modlistFormatBullet
	switch state.Config.Modlist.OutputFormat {
	case 1:
		outputFormat = modlistFormatTable
	case 2:
		outputFormat = modlistFormatBBCode
	}

	sortField := modlistSortName
	switch state.Config.Modlist.SortField {
	case 1:
		sortField = modlistSortSource
	case 2:
		sortField = modlistSortSide
	}

	return &modlistPage{
		state:           state,
		mode:            mode,
		outputFormat:    outputFormat,
		sortField:       sortField,
		sortAsc:         state.Config.Modlist.SortAsc,
		attachLinks:     state.Config.Modlist.AttachLinks,
		includeSide:     state.Config.Modlist.IncludeSide,
		includeSource:   state.Config.Modlist.IncludeSource,
		includeVersions: state.Config.Modlist.IncludeVersions,
		includeFilename: state.Config.Modlist.IncludeFilename,
		showProjectMeta: state.Config.Modlist.ShowProjectMeta,
		rawPreview:      state.Config.Modlist.RawPreview,
		viewport:        vp,
		status:          "Load a pack in Selector to generate a mod list.",
		dirty:           true,
		keys:            defaultModlistKeyMap(),
	}
}

// ─── ui.Page interface ────────────────────────────────────────────────────────

func (m *modlistPage) Title() string { return "Modlist Generator" }
func (m *modlistPage) Init() tea.Cmd { return nil }

func (m *modlistPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	m.detectPackChange()
	var cmds []tea.Cmd

	switch typed := msg.(type) {
	case modpack.PackLoadedMsg:
		if typed.Err != nil {
			m.status = fmt.Sprintf("Load failed: %v", typed.Err)
			return m, nil
		}
		m.status = fmt.Sprintf("Loaded %d mods", len(typed.Info.Mods))
		m.dirty = true

	case ui.ContentSizeMsg:
		m.contentW = typed.Width
		m.contentH = typed.Height
		m.updateLayout()

	case tea.WindowSizeMsg:
		m.pageWidth = typed.Width
		m.pageHeight = typed.Height

	case tea.MouseReleaseMsg:
		if m.zone != nil {
			for _, id := range m.clickIDs() {
				fullID := m.prefix + id
				z := m.zone.Get(fullID)
				if z != nil && z.InBounds(typed) {
					m = m.handleClick(id)
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch {
		// Layout
		case key.Matches(typed, m.keys.LayoutMerged):
			m = m.setLayout(modlistMerged)
			m.saveToConfig()
		case key.Matches(typed, m.keys.LayoutSplit):
			m = m.setLayout(modlistSeparated)
			m.saveToConfig()

		// Output format
		case key.Matches(typed, m.keys.FormatBullet):
			m = m.setFormat(modlistFormatBullet)
			m.saveToConfig()
		case key.Matches(typed, m.keys.FormatTable):
			m = m.setFormat(modlistFormatTable)
			m.saveToConfig()
		case key.Matches(typed, m.keys.FormatBBCode):
			m = m.setFormat(modlistFormatBBCode)
			m.saveToConfig()

		// Sort
		case key.Matches(typed, m.keys.CycleSort):
			m.sortField = (m.sortField + 1) % 3
			m.dirty = true
			m.saveToConfig()
		case key.Matches(typed, m.keys.ToggleSortDir):
			m.sortAsc = !m.sortAsc
			m.dirty = true
			m.saveToConfig()

		// Metadata toggles
		case key.Matches(typed, m.keys.ToggleLinks):
			m.attachLinks = !m.attachLinks
			m.dirty = true
			m.saveToConfig()
		case key.Matches(typed, m.keys.ToggleSide):
			m.includeSide = !m.includeSide
			m.dirty = true
			m.saveToConfig()
		case key.Matches(typed, m.keys.ToggleSource):
			m.includeSource = !m.includeSource
			m.dirty = true
			m.saveToConfig()
		case key.Matches(typed, m.keys.ToggleVersions):
			m.includeVersions = !m.includeVersions
			m.dirty = true
			m.saveToConfig()
		case key.Matches(typed, m.keys.ToggleFilename):
			m.includeFilename = !m.includeFilename
			m.dirty = true
			m.saveToConfig()

		// View
		case key.Matches(typed, m.keys.ToggleRaw):
			m.rawPreview = !m.rawPreview
			m.dirty = true // forces viewport content refresh
			m.saveToConfig()
		case key.Matches(typed, m.keys.ToggleProjectMeta):
			m.showProjectMeta = !m.showProjectMeta
			m.dirty = true
			m.saveToConfig()

		// Actions
		case key.Matches(typed, m.keys.Copy):
			m.status = m.copyMarkdown()
		case key.Matches(typed, m.keys.Export):
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

// ─── Layout ───────────────────────────────────────────────────────────────────

// updateLayout recomputes all dimension fields from ContentSizeMsg dimensions.
// contentW and contentH must already be set before calling this.
func (m *modlistPage) updateLayout() {
	const gap = 2

	contentW := m.contentW
	if contentW < 64 {
		contentW = 64
	}

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

	if m.previewW > 2 {
		m.viewport.SetWidth(m.previewW - 2) // 1 left border + 1 right border
	}

	// ContentSizeMsg already accounts for all frame overhead (tab bar, footer,
	// window border, padding). Reserve 1 line for the status bar in View().
	avail := m.contentH - 1
	if avail < 8 {
		avail = 8
	}
	if avail > 2 {
		avail -= 2 // 1 top border + 1 bottom border
	}
	m.viewport.SetHeight(avail)

	wrap := preview - 2
	if wrap < 16 {
		wrap = 16
	}
	if wrap != m.lastWrap {
		m.lastWrap = wrap
		m.dirty = true
	}
}

// ─── Rebuild ─────────────────────────────────────────────────────────────────

// rebuild regenerates the output (if dirty) and refreshes the viewport content.
// It is a no-op when layout dimensions have not yet been received (previewW==0)
// so that we never set content on a zero-width viewport before ContentSizeMsg
// arrives on the first frame.
func (m *modlistPage) rebuild() {
	if m.previewW == 0 {
		// Layout not yet known — remain dirty so the next ContentSizeMsg triggers a real rebuild.
		m.dirty = true
		return
	}
	settingsHash := m.calculateSettingsHash()
	if m.dirty || settingsHash != m.cachedSettingsHash || m.cachedMarkdown == "" {
		m.markdown = m.generateOutput()
		m.cachedSettingsHash = settingsHash
		m.cachedMarkdown = m.markdown
	} else {
		m.markdown = m.cachedMarkdown
	}

	// Determine what goes into the viewport:
	//   • No pack loaded         → raw placeholder text
	//   • BBCode format          → always raw (no glamour renderer for BBCode)
	//   • rawPreview is on       → raw source text
	//   • otherwise              → glamour-rendered markdown
	//
	// For raw content we must hard-wrap at previewW before calling
	// viewport.SetContent.  Bubble Tea's viewport splits on \n but does NOT
	// itself word-wrap long lines; those lines then overflow the column width.
	// When View() later wraps them inside lipgloss.NewStyle().Width(previewW),
	// they expand to multiple visual rows, blowing up the layout height and
	// shifting the settings column out of alignment.
	noPack := m.state == nil || m.state.Pack.InstancePath == ""
	if noPack || m.rawPreview || m.outputFormat == modlistFormatBBCode {
		content := m.markdown
		if m.previewW > 4 {
			content = hardWrapLines(content, m.previewW-2)
		}
		m.viewport.SetContent(content)
	} else {
		m.viewport.SetContent(m.renderMarkdown(m.markdown))
	}
	m.viewport.SetYOffset(0)
	m.dirty = false
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *modlistPage) View() string {
	// Guard: ContentSizeMsg hasn't arrived yet — show a placeholder so the
	// initial frame doesn't render garbage from zero-valued dimensions.
	if m.settingsW == 0 || m.previewW == 0 {
		return "Modlist Generator — loading…"
	}

	// renderSettings already pads each column manually — no Width wrapper needed.
	settings := m.renderSettings()

	var preview string
	if m.state != nil && m.state.Pack.InstancePath != "" {
		preview = m.viewport.View()
	} else {
		preview = statusStyle.Render("Load a pack from the Selector tab to generate a mod list.")
	}

	// Clamp the preview column to exactly previewW so it can never overflow
	// into or past the settings column regardless of viewport internal state.
	preview = lipgloss.NewStyle().Width(m.previewW).MaxWidth(m.previewW).Render(preview)

	gap := strings.Repeat(" ", 2)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, settings, gap, preview)

	if m.status != "" {
		layout += "\n" + statusStyle.Render(m.status)
	}
	return layout
}

// ─── Settings panel ───────────────────────────────────────────────────────────

// renderSettings builds the left-hand settings column.
//
// IMPORTANT: we deliberately avoid lipgloss.NewStyle().Width(n).Render() on any
// string that contains zone-mark escape sequences.  Lipgloss's internal
// line-width calculator counts the raw bytes of those sequences as printable
// characters, so it under-pads or wraps zone-marked lines, shifting their
// visual position by one row relative to the coordinate that zone.Scan()
// recorded — causing the "click one row above" mis-hit.
//
// Instead we measure each line's visual width with lipgloss.Width() (which
// correctly strips all escape sequences) and pad manually with spaces.
func (m *modlistPage) renderSettings() string {
	sortDir := "↑"
	if !m.sortAsc {
		sortDir = "↓"
	}

	left := []string{
		sectionTitleStyle.Render("Layout"),
		m.markOption("layout-merged", fmt.Sprintf("%s Merged (1)", checkbox(m.mode == modlistMerged))),
		m.markOption("layout-split", fmt.Sprintf("%s Split by side (2)", checkbox(m.mode == modlistSeparated))),
		"",
		sectionTitleStyle.Render("Format"),
		m.markOption("format-bullet", fmt.Sprintf("%s Bullet (b)", checkbox(m.outputFormat == modlistFormatBullet))),
		m.markOption("format-table", fmt.Sprintf("%s Table (t)", checkbox(m.outputFormat == modlistFormatTable))),
		m.markOption("format-bbcode", fmt.Sprintf("%s BBCode (x)", checkbox(m.outputFormat == modlistFormatBBCode))),
		"",
		sectionTitleStyle.Render("Sort"),
		m.markOption("sort-name", fmt.Sprintf("%s By Name (s)", checkbox(m.sortField == modlistSortName))),
		m.markOption("sort-source", fmt.Sprintf("%s By Source (s)", checkbox(m.sortField == modlistSortSource))),
		m.markOption("sort-side", fmt.Sprintf("%s By Side (s)", checkbox(m.sortField == modlistSortSide))),
		m.markOption("sort-dir", fmt.Sprintf("%s Ascending (S) %s", checkbox(m.sortAsc), sortDir)),
	}

	right := []string{
		sectionTitleStyle.Render("Metadata"),
		m.markOption("meta-links", fmt.Sprintf("%s Links (a)", checkbox(m.attachLinks))),
		m.markOption("meta-side", fmt.Sprintf("%s Side tag (i)", checkbox(m.includeSide))),
		m.markOption("meta-source", fmt.Sprintf("%s Source (o)", checkbox(m.includeSource))),
		m.markOption("meta-version", fmt.Sprintf("%s Versions (v)", checkbox(m.includeVersions))),
		m.markOption("meta-filename", fmt.Sprintf("%s Filenames (f)", checkbox(m.includeFilename))),
		m.markOption("view-meta", fmt.Sprintf("%s Pack info (p)", checkbox(m.showProjectMeta))),
		"",
		sectionTitleStyle.Render("View"),
		m.markOption("view-raw", fmt.Sprintf("%s Raw source (r)", checkbox(m.rawPreview))),
		"",
		sectionTitleStyle.Render("Actions"),
		m.markOption("action-copy", "Copy (c)"),
		m.markOption("action-export", "Export (e)"),
	}

	// colGap is the space between the two settings columns.
	// colW is derived from settingsW so that 2*colW + colGap <= settingsW,
	// ensuring the settings output never exceeds settingsW and overflows into
	// the preview column when the outer layout is rendered.
	const colGap = 2
	colW := (m.settingsW - colGap) / 2
	if colW < 20 {
		colW = 20
	}

	// Pad each line to exactly colW using visual-width measurement.
	// This avoids passing zone-marked strings through a Width-constrained
	// lipgloss Render, which would miscount their widths and cause reflow.
	padLine := func(s string, w int) string {
		vw := lipgloss.Width(s)
		if vw >= w {
			return s
		}
		return s + strings.Repeat(" ", w-vw)
	}

	leftPadded := make([]string, len(left))
	for i, l := range left {
		leftPadded[i] = padLine(l, colW)
	}
	rightPadded := make([]string, len(right))
	for i, r := range right {
		rightPadded[i] = padLine(r, colW)
	}

	// Equalise row counts so the horizontal join is a clean rectangle.
	for len(leftPadded) < len(rightPadded) {
		leftPadded = append(leftPadded, strings.Repeat(" ", colW))
	}
	for len(rightPadded) < len(leftPadded) {
		rightPadded = append(rightPadded, "")
	}

	colGapStr := strings.Repeat(" ", colGap)
	var sb strings.Builder
	for i := range leftPadded {
		sb.WriteString(leftPadded[i])
		sb.WriteString(colGapStr)
		sb.WriteString(rightPadded[i])
		if i < len(leftPadded)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// ─── Zone / click helpers ─────────────────────────────────────────────────────

func (m *modlistPage) markOption(id, content string) string {
	if m.zone == nil {
		return content
	}
	return m.zone.Mark(m.prefix+id, content)
}

func (m *modlistPage) clickIDs() []string {
	return []string{
		"layout-merged", "layout-split",
		"format-bullet", "format-table", "format-bbcode",
		"sort-name", "sort-source", "sort-side", "sort-dir",
		"meta-links", "meta-side", "meta-source", "meta-version", "meta-filename",
		"view-meta", "view-raw",
		"action-copy", "action-export",
	}
}

func (m *modlistPage) handleClick(id string) *modlistPage {
	switch strings.TrimPrefix(id, m.prefix) {
	// Layout
	case "layout-merged":
		m = m.setLayout(modlistMerged)
		m.saveToConfig()
	case "layout-split":
		m = m.setLayout(modlistSeparated)
		m.saveToConfig()

	// Format
	case "format-bullet":
		m = m.setFormat(modlistFormatBullet)
		m.saveToConfig()
	case "format-table":
		m = m.setFormat(modlistFormatTable)
		m.saveToConfig()
	case "format-bbcode":
		m = m.setFormat(modlistFormatBBCode)
		m.saveToConfig()

	// Sort
	case "sort-name":
		m.sortField = modlistSortName
		m.dirty = true
		m.saveToConfig()
	case "sort-source":
		m.sortField = modlistSortSource
		m.dirty = true
		m.saveToConfig()
	case "sort-side":
		m.sortField = modlistSortSide
		m.dirty = true
		m.saveToConfig()
	case "sort-dir":
		m.sortAsc = !m.sortAsc
		m.dirty = true
		m.saveToConfig()

	// Metadata toggles
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

	// View
	case "view-meta":
		m.showProjectMeta = !m.showProjectMeta
		m.dirty = true
		m.saveToConfig()
	case "view-raw":
		m.rawPreview = !m.rawPreview
		m.dirty = true
		m.saveToConfig()

	// Actions
	case "action-copy":
		m.status = m.copyMarkdown()
	case "action-export":
		m.status = m.exportToFile()
	}
	return m
}

// ─── Mutation helpers ─────────────────────────────────────────────────────────

func (m *modlistPage) setLayout(mode modlistMode) *modlistPage {
	if m.mode != mode {
		m.mode = mode
		m.dirty = true
	}
	return m
}

func (m *modlistPage) setFormat(f modlistFormat) *modlistPage {
	if m.outputFormat != f {
		m.outputFormat = f
		m.dirty = true
		// Force glamour renderer recreate when switching format.
		m.renderer = nil
	}
	return m
}

// ─── Config sync ─────────────────────────────────────────────────────────────

// saveToConfig writes the current display settings back into the shared config.
func (m *modlistPage) saveToConfig() {
	if m.state == nil || m.state.Config == nil {
		return
	}
	modeInt := 0
	if m.mode == modlistSeparated {
		modeInt = 1
	}
	fmtInt := 0
	switch m.outputFormat {
	case modlistFormatTable:
		fmtInt = 1
	case modlistFormatBBCode:
		fmtInt = 2
	}
	sortInt := 0
	switch m.sortField {
	case modlistSortSource:
		sortInt = 1
	case modlistSortSide:
		sortInt = 2
	}

	m.state.Config.Modlist.Mode = modeInt
	m.state.Config.Modlist.OutputFormat = fmtInt
	m.state.Config.Modlist.SortField = sortInt
	m.state.Config.Modlist.SortAsc = m.sortAsc
	m.state.Config.Modlist.AttachLinks = m.attachLinks
	m.state.Config.Modlist.IncludeSide = m.includeSide
	m.state.Config.Modlist.IncludeSource = m.includeSource
	m.state.Config.Modlist.IncludeVersions = m.includeVersions
	m.state.Config.Modlist.IncludeFilename = m.includeFilename
	m.state.Config.Modlist.ShowProjectMeta = m.showProjectMeta
	m.state.Config.Modlist.RawPreview = m.rawPreview
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
		total = m.contentW
	}
	if total == 0 {
		return 52
	}
	w := total / 2
	if w < 48 {
		w = 48
	}
	if w > 72 {
		w = 72
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

// ─── Key help ─────────────────────────────────────────────────────────────────

func (m *modlistPage) ShortHelp() []key.Binding  { return m.keys.ShortHelp() }
func (m *modlistPage) FullHelp() [][]key.Binding { return m.keys.FullHelp() }

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ui.HighlightColor)
	statusStyle       = lipgloss.NewStyle().Foreground(ui.HighlightColor)
)
