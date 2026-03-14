package pages

//page_dashboard.go
import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
)

type dashboardPage struct {
	state  *ui.SharedState
	width  int
	height int
	ready  bool
}

func NewDashboardPage(state *ui.SharedState) ui.Page {
	return &dashboardPage{state: state}
}

func (d *dashboardPage) Title() string { return "Dashboard" }
func (d *dashboardPage) Init() tea.Cmd { return nil }

func (d *dashboardPage) Update(msg tea.Msg) (ui.Page, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ContentSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		d.ready = true
	case ui.PackLoadedMsg:
		// No action needed, will re-render on next View() call
	}
	return d, nil
}

func (d *dashboardPage) View() string {
	if !d.ready {
		return dimStyle.Render("Initializing…")
	}
	return d.buildContent()
}

// buildContent produces the full dashboard string sized to d.width.
func (d *dashboardPage) buildContent() string {
	if d.state == nil || d.state.Pack.InstancePath == "" {
		msg := dimStyle.Render("Load a pack from the Selector tab to see details.")
		if d.state != nil && d.state.LastLoadError != "" {
			msg = errorStyle.Render("✗  " + d.state.LastLoadError)
		}
		return msg
	}

	pack := d.state.Pack

	w := d.width
	if w < 40 {
		w = 80
	}

	cardWidth := (w - 2) / 2
	if cardWidth < 24 {
		cardWidth = 24
	}

	header := d.renderHeader(pack, w)
	details := d.detailsCard(pack, cardWidth)
	mods := d.modsCard(pack, cardWidth)

	cards := lipgloss.JoinVertical(lipgloss.Center, details, mods)
	cards = lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(cards)

	path := "Path: " + pack.InstancePath
	pathLine := dashLabelStyle.Width(w).Align(lipgloss.Center).Render(path)

	return header + "\n\n" + cards + "\n\n" + pathLine
}

// ─── Header ───────────────────────────────────────────────────────────────────

func (d *dashboardPage) renderHeader(pack ui.PackInfo, width int) string {
	name := pack.InstanceName
	if name == "" {
		name = "Unknown Pack"
	}

	var parts []string

	mainLine := name
	if pack.PackVersion != "" {
		mainLine += "  v" + pack.PackVersion
	}
	mainLine += "  " + string(d.sourceBadge(pack.SourceType))
	parts = append(parts, dashHeaderStyle.Width(width).Align(lipgloss.Center).Render(mainLine))

	if pack.PackAuthor != "" {
		parts = append(parts, dashSubtitleStyle.Width(width).Align(lipgloss.Center).Render("by "+pack.PackAuthor))
	}
	if pack.WebsiteURL != "" {
		parts = append(parts, dimStyle.Width(width).Align(lipgloss.Center).Render(pack.WebsiteURL))
	}

	return strings.Join(parts, "\n")
}

// ─── Details card ─────────────────────────────────────────────────────────────

func (d *dashboardPage) detailsCard(pack ui.PackInfo, cardWidth int) string {
	rows := []string{
		dashCardTitleStyle.Render("Pack Details"),
		"",
		kv("Minecraft", valueOr(pack.MinecraftVersion, "unknown")),
		kv("Loader", formatLoader(pack.LoaderUID, pack.LoaderVersion)),
	}
	if pack.PackVersion != "" {
		rows = append(rows, kv("Version", pack.PackVersion))
	}
	if pack.PackAuthor != "" {
		rows = append(rows, kv("Author", pack.PackAuthor))
	}
	if pack.PackDescription != "" {
		rows = append(rows, kv("Desc", pack.PackDescription))
	}

	content := strings.Join(rows, "\n")
	return dashCardStyle.Width(cardWidth).Height(9).Render(content)
}

// ─── Mods card ────────────────────────────────────────────────────────────────

func (d *dashboardPage) modsCard(pack ui.PackInfo, cardWidth int) string {
	c := pack.Counts
	rows := []string{
		dashCardTitleStyle.Render("Mods"),
		"",
		kv("Total", fmt.Sprintf("%d", c.Total)),
		kv("Modrinth", fmt.Sprintf("%d", c.Modrinth)),
		kv("CurseForge", fmt.Sprintf("%d", c.Curseforge)),
	}
	if c.Unknown > 0 {
		rows = append(rows, kv("Unknown", fmt.Sprintf("%d", c.Unknown)))
	}

	hasBar := c.Total > 0
	if hasBar {
		rows = append(rows, "")
		rows = append(rows, d.sourceBar(c, cardWidth))
		rows = append(rows,
			modrinthBarStyle.Render("█")+" Modrinth  "+
				curseBarStyle.Render("█")+" CurseForge  "+
				unknownBarStyle.Render("█")+" Other",
		)
	}

	content := strings.Join(rows, "\n")
	return dashCardStyle.Width(cardWidth).Height(9).Render(content)
}

// ─── Source bar ───────────────────────────────────────────────────────────────

func (d *dashboardPage) sourceBar(c ui.ModCounts, cardWidth int) string {
	if c.Total == 0 {
		return ""
	}
	barWidth := cardWidth - 4
	if barWidth < 4 {
		barWidth = 4
	}
	mW := (c.Modrinth * barWidth) / c.Total
	cfW := (c.Curseforge * barWidth) / c.Total
	ukW := barWidth - mW - cfW
	return modrinthBarStyle.Render(strings.Repeat("█", mW)) +
		curseBarStyle.Render(strings.Repeat("█", cfW)) +
		unknownBarStyle.Render(strings.Repeat("█", ukW))
}

// ─── Source badge ─────────────────────────────────────────────────────────────

func (d *dashboardPage) sourceBadge(src ui.PackSourceType) string {
	switch src {
	case ui.PackSourceCurseForge:
		return curseBadgeStyle.Render("CurseForge")
	case ui.PackSourcePrism:
		return prismBadgeStyle.Render("Prism")
	default:
		return dimStyle.Render("Unknown")
	}
}

// ─── kv ───────────────────────────────────────────────────────────────────────

func kv(label, val string) string {
	return dashLabelStyle.Render(fmt.Sprintf("%-11s", label)) + valueStyle.Render(val)
}

func (d *dashboardPage) ShortHelp() []key.Binding  { return nil }
func (d *dashboardPage) FullHelp() [][]key.Binding { return nil }

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	dashHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.HighlightColor)

	dashVersionStyle = lipgloss.NewStyle().
				Foreground(ui.HighlightColor)

	dashSubtitleStyle = lipgloss.NewStyle().
				Foreground(ui.HighlightColor).
				Italic(true)

	dashCardTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.HighlightColor)

	dashCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	dashLabelStyle = lipgloss.NewStyle().
			Foreground(ui.HighlightColor)

	labelStyle = lipgloss.NewStyle().
			Width(10).
			Foreground(ui.HighlightColor)

	valueStyle = lipgloss.NewStyle().
			Foreground(ui.HighlightColor)

	dimStyle = lipgloss.NewStyle().
			Foreground(ui.HighlightColor).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5f56"))

	modrinthBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5f4b8b"))

	curseBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f16939"))

	unknownBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	curseBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#f16939")).
			Background(lipgloss.Color("#f16939")).
			Padding(0, 1)

	prismBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#5f4b8b")).
			Padding(0, 1)
)
