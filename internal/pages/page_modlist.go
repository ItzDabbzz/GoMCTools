package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/glow/v2/utils"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"itzdabbzz.me/gomctools/internal/ui"
)

type modlistMode int

const (
	modlistMerged modlistMode = iota
	modlistSeparated
)

type modlistKeyMap struct {
	LayoutMerged   key.Binding
	LayoutSplit    key.Binding
	ToggleLinks    key.Binding
	ToggleSide     key.Binding
	ToggleSource   key.Binding
	ToggleVersions key.Binding
	ToggleFilename key.Binding
	Copy           key.Binding
	Export         key.Binding
	Help           key.Binding
}

func (k modlistKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ToggleLinks, k.ToggleSide, k.ToggleSource, k.Copy}
}

func (k modlistKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.LayoutMerged, k.LayoutSplit, k.ToggleLinks, k.ToggleSide},
		{k.ToggleSource, k.ToggleVersions, k.ToggleFilename, k.Copy},
		{k.Export, k.Help},
	}
}

func defaultModlistKeyMap() modlistKeyMap {
	return modlistKeyMap{
		LayoutMerged:   key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "merged layout")),
		LayoutSplit:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "split by side")),
		ToggleLinks:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle links")),
		ToggleSide:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "toggle side")),
		ToggleSource:   key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "toggle source")),
		ToggleVersions: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "toggle versions")),
		ToggleFilename: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "toggle filename")),
		Copy:           key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy markdown")),
		Export:         key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export file")),
		Help:           key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	}
}

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
}

// SetZone wires the page to the root bubblezone manager so click detection
// happens against the final rendered document instead of an inner layout.
func (m *modlistPage) SetZone(z *zone.Manager, prefix string) {
	m.zone = z
	m.prefix = prefix
}

func NewModlistPage(state *ui.SharedState) ui.Page {
	vp := viewport.New(0, 0)
	vp.MouseWheelDelta = 2
	vp.MouseWheelEnabled = true

	return &modlistPage{
		state:           state,
		mode:            modlistMerged,
		attachLinks:     true,
		includeSide:     true,
		includeSource:   true,
		includeVersions: false,
		includeFilename: false,
		viewport:        vp,
		status:          "Load a pack in Selector to generate a mod list.",
		dirty:           true,
		keys:            defaultModlistKeyMap(),
	}
}

func (m *modlistPage) Title() string {
	return "Modlist Generator"
}

func (m *modlistPage) Init() tea.Cmd {
	return nil
}

func (m *modlistPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	m.detectPackChange()
	var cmds []tea.Cmd

	switch typed := msg.(type) {
	case ui.PackLoadedMsg:
		if typed.Err != nil {
			m.status = fmt.Sprintf("Load failed: %v", typed.Err)
			return m, nil
		}
		m.status = fmt.Sprintf("Loaded %d mods from %s", len(typed.Info.Mods), filepath.Base(typed.Info.InstancePath))
		m.dirty = true
	case zone.MsgZoneInBounds:
		if typed.Event.Type == tea.MouseLeft {
			if id := m.resolveZoneID(typed.Zone); id != "" {
				m = m.handleClick(id)
			}
		}
	case tea.WindowSizeMsg:
		if typed.Width != m.pageWidth || typed.Height != m.pageHeight {
			m.pageWidth = typed.Width
			m.pageHeight = typed.Height
			m.updateViewportSize()
			m.dirty = true
		}
	case tea.MouseMsg:
		if m.zone != nil && typed.Type == tea.MouseLeft {
			if id := m.resolveMouseZone(typed); id != "" {
				m = m.handleClick(id)
			}
		}
	case tea.KeyMsg:
		if key.Matches(typed, m.keys.LayoutMerged) {
			m = m.setLayout(modlistMerged)
		} else if key.Matches(typed, m.keys.LayoutSplit) {
			m = m.setLayout(modlistSeparated)
		} else if key.Matches(typed, m.keys.ToggleLinks) {
			m.attachLinks = !m.attachLinks
			m.dirty = true
		} else if key.Matches(typed, m.keys.ToggleSide) {
			m.includeSide = !m.includeSide
			m.dirty = true
		} else if key.Matches(typed, m.keys.ToggleSource) {
			m.includeSource = !m.includeSource
			m.dirty = true
		} else if key.Matches(typed, m.keys.ToggleVersions) {
			m.includeVersions = !m.includeVersions
			m.dirty = true
		} else if key.Matches(typed, m.keys.ToggleFilename) {
			m.includeFilename = !m.includeFilename
			m.dirty = true
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

func (m *modlistPage) rebuild() {
	m.markdown = m.generateMarkdown()
	wrapped := m.renderMarkdown(m.markdown)
	m.viewport.SetContent(wrapped)
	m.dirty = false
}

func (m *modlistPage) View() string {
	m.ensureLayout()

	settings := lipgloss.NewStyle().Width(m.settingsW).Render(m.renderSettings())
	preview := lipgloss.NewStyle().Width(m.previewW).Render(m.viewport.View())
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

func (m *modlistPage) updateViewportSize() {
	m.recomputeWidths()
	m.applyViewportSizes()
}

func (m *modlistPage) renderSettings() string {
	m.ensureLayout()

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

func (m *modlistPage) ensureLayout() {
	if m.contentW == 0 {
		m.recomputeWidths()
	}
	if m.settingsW == 0 || m.previewW == 0 {
		m.recomputeWidths()
	}
	if m.viewport.Width == 0 && m.previewW > 0 {
		m.viewport.Width = m.previewW
	}
}

func (m *modlistPage) recomputeWidths() {
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

	wrap := preview - 2
	if wrap < 16 {
		wrap = 16
	}
	if wrap != m.lastWrap {
		m.lastWrap = wrap
		m.dirty = true
	}
}

func (m *modlistPage) applyViewportSizes() {
	if m.previewW > 0 {
		m.viewport.Width = m.previewW
	}
	frame := ui.WindowStyle.GetVerticalFrameSize()
	avail := m.pageHeight - ui.DocStyle.GetVerticalFrameSize() - frame - 4 // tabs/footer breathing room
	if avail < 8 {
		avail = 8
	}
	m.viewport.Height = avail
}

func (m *modlistPage) markOption(id, content string) string {
	if m.zone == nil {
		return content
	}
	return m.zone.Mark(m.prefix+id, content)
}

func (m *modlistPage) clickIDs() []string {
	return []string{
		"layout-merged",
		"layout-split",
		"meta-links",
		"meta-side",
		"meta-source",
		"meta-version",
		"meta-filename",
		"action-copy",
		"action-export",
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

func (m *modlistPage) generateMarkdown() string {
	if m.state == nil || m.state.Pack.InstancePath == "" {
		return "# Modlist\n\nLoad a Prism pack from the Selector tab to generate a list."
	}

	pack := m.state.Pack
	name := pack.InstanceName
	if name == "" {
		name = "Modlist"
	}

	b := strings.Builder{}
	fmt.Fprintf(&b, "# %s\n\n", name)

	if pack.MinecraftVersion != "" || pack.LoaderUID != "" {
		b.WriteString("- Minecraft [" + valueOr(pack.MinecraftVersion, "unknown") + "]\n")
		b.WriteString("- " + formatLoader(pack.LoaderUID, pack.LoaderVersion) + "\n\n")
	}

	mods := append([]ui.IndexedMod(nil), pack.Mods...)
	if len(mods) == 0 {
		b.WriteString("_No mods found in this pack._")
		return b.String()
	}

	if m.mode == modlistSeparated {
		client, server, both := splitBySide(mods)
		writeSection(&b, "Client", client, m)
		writeSection(&b, "Server", server, m)
		if len(both) > 0 {
			writeSection(&b, "Dual/Unspecified", both, m)
		}
		return b.String()
	}

	writeSection(&b, "All Mods", mods, m)
	return b.String()
}

func writeSection(b *strings.Builder, title string, mods []ui.IndexedMod, page *modlistPage) {
	if len(mods) == 0 {
		return
	}

	sort.SliceStable(mods, func(i, j int) bool {
		return strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name)
	})

	fmt.Fprintf(b, "## %s (***%d***)\n", title, len(mods))
	for _, mod := range mods {
		b.WriteString(page.formatMod(mod))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func splitBySide(mods []ui.IndexedMod) (client, server, both []ui.IndexedMod) {
	for _, mod := range mods {
		side := strings.ToLower(mod.Side)
		switch {
		case strings.Contains(side, "client"):
			client = append(client, mod)
		case strings.Contains(side, "server"):
			server = append(server, mod)
		default:
			both = append(both, mod)
		}
	}
	return
}

func (m *modlistPage) formatMod(mod ui.IndexedMod) string {
	name := mod.Name
	if name == "" {
		name = mod.Filename
	}
	if name == "" {
		name = "Unnamed mod"
	}

	link := ""
	if m.attachLinks {
		switch mod.Source {
		case ui.SourceModrinth:
			if mod.ModrinthID != "" {
				link = fmt.Sprintf("https://modrinth.com/mod/%s", mod.ModrinthID)
			}
		case ui.SourceCurseforge:
			if mod.CurseProject != 0 {
				link = fmt.Sprintf("http://curseforge.com/projects/%d", mod.CurseProject)
			}
		}
		if link == "" {
			link = mod.DownloadURL
		}
		if link != "" {
			name = fmt.Sprintf("[%s](%s)", name, link)
		}
	}

	details := []string{}
	if m.includeSide && mod.Side != "" {
		details = append(details, fmt.Sprintf("[**Side**] `%s`", titleCase(strings.ToLower(mod.Side))))
	}
	if m.includeSource {
		source := string(mod.Source)
		if source == "" {
			source = "unknown"
		}
		details = append(details, fmt.Sprintf("[**Source**] `%s`", titleCase(strings.ToLower(source))))
	}
	if m.includeVersions && len(mod.GameVersions) > 0 {
		details = append(details, fmt.Sprintf("[**Versions**] `MC %s`", strings.Join(mod.GameVersions, ", ")))
	}
	if m.includeFilename && mod.Filename != "" {
		details = append(details, fmt.Sprintf("[**File**] `%s`", mod.Filename))
	}

	block := strings.Builder{}
	fmt.Fprintf(&block, "- **%s**\n", name)
	for _, d := range details {
		fmt.Fprintf(&block, "  - %s\n", d)
	}
	return block.String()
}

func valueOr(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

func formatLoader(uid, version string) string {
	name := loaderLabel(uid)
	if name == "" {
		name = valueOr(uid, "Unknown loader")
	}
	if version != "" {
		name += " [" + version + "]"
	}
	return name
}

func loaderLabel(uid string) string {
	switch strings.TrimSpace(uid) {
	case "net.neoforged":
		return "NeoForge"
	case "net.minecraftforge":
		return "Forge"
	case "net.fabricmc.fabric-loader":
		return "Fabric"
	case "net.fabricmc.intermediary":
		return "Fabric (Intermediary)"
	case "org.quiltmc.quilt-loader":
		return "Quilt"
	default:
		return strings.TrimSpace(uid)
	}
}

func (m *modlistPage) renderMarkdown(md string) string {
	wrap := m.lastWrap
	if wrap <= 0 {
		wrap = 80
	}

	if m.renderer == nil || m.rendererW != wrap {
		options := []glamour.TermRendererOption{
			utils.GlamourStyle(styles.AutoStyle, false),
			glamour.WithWordWrap(wrap),
		}
		r, err := glamour.NewTermRenderer(options...)
		if err != nil {
			return md
		}
		m.renderer = r
		m.rendererW = wrap
	}

	out, err := m.renderer.Render(md)
	if err != nil {
		return md
	}
	return out
}

func (m *modlistPage) exportToFile() string {
	if m.markdown == "" {
		return "Nothing to export"
	}

	var path string
	if m.state != nil && m.state.Pack.InstancePath != "" {
		path = filepath.Join(m.state.Pack.InstancePath, "modlist.md")
	} else {
		path = filepath.Join(".", "modlist.md")
	}

	if err := os.WriteFile(path, []byte(m.markdown), 0o644); err != nil {
		return fmt.Sprintf("Export failed: %v", err)
	}
	return fmt.Sprintf("Exported to %s", path)
}

func (m *modlistPage) copyMarkdown() string {
	if m.markdown == "" {
		return "Nothing to copy"
	}
	if err := clipboard.WriteAll(m.markdown); err != nil {
		return fmt.Sprintf("Copy failed: %v", err)
	}
	return "Markdown copied to clipboard"
}

func (m *modlistPage) handleClick(id string) *modlistPage {
	switch strings.TrimPrefix(id, m.prefix) {
	case "layout-merged":
		m = m.setLayout(modlistMerged)
	case "layout-split":
		m = m.setLayout(modlistSeparated)
	case "meta-links":
		m.attachLinks = !m.attachLinks
		m.dirty = true
	case "meta-side":
		m.includeSide = !m.includeSide
		m.dirty = true
	case "meta-source":
		m.includeSource = !m.includeSource
		m.dirty = true
	case "meta-version":
		m.includeVersions = !m.includeVersions
		m.dirty = true
	case "meta-filename":
		m.includeFilename = !m.includeFilename
		m.dirty = true
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

func checkbox(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func (m *modlistPage) ShortHelp() []key.Binding { return m.keys.ShortHelp() }

// FullHelp exposes grouped bindings for the global help menu
func (m *modlistPage) FullHelp() [][]key.Binding { return m.keys.FullHelp() }

var (
	settingsStyle     = lipgloss.NewStyle().Padding(0, 2, 0, 0).MarginRight(3)
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ui.HighlightColor)
	statusStyle       = lipgloss.NewStyle().Foreground(ui.HighlightColor)
)
