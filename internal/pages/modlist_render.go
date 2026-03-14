package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/ItzDabbzz/GoMCTools/internal/ui"
	"github.com/atotto/clipboard"
)

// ─── Output dispatcher ────────────────────────────────────────────────────────

// generateOutput dispatches to the correct generator based on outputFormat.
func (m *modlistPage) generateOutput() string {
	if m.state == nil || m.state.Pack.InstancePath == "" {
		return "# Modlist\n\nLoad a pack from the Selector tab to generate a list."
	}
	switch m.outputFormat {
	case modlistFormatTable:
		return m.generateMarkdownTable()
	case modlistFormatBBCode:
		return m.generateBBCode()
	default:
		return m.generateMarkdownBullet()
	}
}

// ─── Shared header helpers ────────────────────────────────────────────────────

// writePackHeader writes the pack name, optional project metadata block,
// and the Minecraft / loader line into b.  Used by both markdown generators.
func (m *modlistPage) writePackHeader(b *strings.Builder, pack ui.PackInfo) {
	name := pack.InstanceName
	if name == "" {
		name = "Modlist"
	}
	fmt.Fprintf(b, "# %s\n\n", name)

	if m.showProjectMeta {
		// Use list items rather than a blockquote with hard-line-breaks (> ... \n).
		// Glamour does not reliably honour the two-space hard-break inside a
		// blockquote — consecutive > lines without a blank > separator merge into
		// a single paragraph, putting everything on one line.  A plain list
		// renders correctly in both raw and glamour-rendered modes.
		sourceLabel := string(pack.SourceType)
		if sourceLabel == "" {
			sourceLabel = "unknown"
		}
		fmt.Fprintf(b, "- **Source:** %s\n", sourceLabel)
		if pack.PackAuthor != "" {
			fmt.Fprintf(b, "- **Author:** %s\n", pack.PackAuthor)
		}
		if pack.PackVersion != "" {
			fmt.Fprintf(b, "- **Version:** %s\n", pack.PackVersion)
		}
		if pack.PackDescription != "" {
			fmt.Fprintf(b, "- **Description:** %s\n", pack.PackDescription)
		}
		if pack.WebsiteURL != "" {
			fmt.Fprintf(b, "- **Website:** %s\n", pack.WebsiteURL)
		}
		b.WriteString("\n")
	}

	if pack.MinecraftVersion != "" || pack.LoaderUID != "" {
		b.WriteString("- Minecraft [" + valueOr(pack.MinecraftVersion, "unknown") + "]\n")
		b.WriteString("- " + formatLoader(pack.LoaderUID, pack.LoaderVersion) + "\n\n")
	}
}

// ─── Bullet list (markdown) ───────────────────────────────────────────────────

// generateMarkdownBullet builds a markdown bullet-list modlist.
func (m *modlistPage) generateMarkdownBullet() string {
	pack := m.state.Pack
	b := strings.Builder{}
	m.writePackHeader(&b, pack)

	mods := append([]ui.IndexedMod(nil), pack.Mods...)
	if len(mods) == 0 {
		b.WriteString("_No mods found in this pack._")
		return b.String()
	}

	if m.mode == modlistSeparated {
		client, server, both := splitBySide(mods)
		writeBulletSection(&b, "Client", client, m)
		writeBulletSection(&b, "Server", server, m)
		if len(both) > 0 {
			writeBulletSection(&b, "Dual/Unspecified", both, m)
		}
		return b.String()
	}

	writeBulletSection(&b, "All Mods", mods, m)
	return b.String()
}

// writeBulletSection appends a titled bullet-list section to b.
func writeBulletSection(b *strings.Builder, title string, mods []ui.IndexedMod, page *modlistPage) {
	if len(mods) == 0 {
		return
	}
	sortMods(mods, page.sortField, page.sortAsc)
	fmt.Fprintf(b, "## %s (***%d***)\n", title, len(mods))
	for _, mod := range mods {
		b.WriteString(page.formatModBullet(mod))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// formatModBullet renders one mod as a markdown bullet item with optional
// metadata sub-bullets.
func (m *modlistPage) formatModBullet(mod ui.IndexedMod) string {
	name := modDisplayName(mod)

	if m.attachLinks {
		if link := modLink(mod); link != "" {
			name = fmt.Sprintf("[%s](%s)", name, link)
		}
	}

	var details []string
	if m.includeSide && mod.Side != "" {
		details = append(details, fmt.Sprintf("[**Side**] `%s`", titleCase(strings.ToLower(mod.Side))))
	}
	if m.includeSource {
		src := string(mod.Source)
		if src == "" {
			src = "unknown"
		}
		details = append(details, fmt.Sprintf("[**Source**] `%s`", titleCase(strings.ToLower(src))))
	}
	if m.includeVersions && len(mod.GameVersions) > 0 {
		details = append(details, fmt.Sprintf("[**Versions**] `MC %s`", strings.Join(mod.GameVersions, ", ")))
	}
	if m.includeFilename && mod.Filename != "" {
		details = append(details, fmt.Sprintf("[**File**] `%s`", mod.Filename))
	}

	var blk strings.Builder
	fmt.Fprintf(&blk, "- **%s**\n", name)
	for _, d := range details {
		fmt.Fprintf(&blk, "  - %s\n", d)
	}
	return blk.String()
}

// ─── GFM table (markdown) ─────────────────────────────────────────────────────

// generateMarkdownTable builds a GFM pipe-table modlist.
func (m *modlistPage) generateMarkdownTable() string {
	pack := m.state.Pack
	b := strings.Builder{}
	m.writePackHeader(&b, pack)

	mods := append([]ui.IndexedMod(nil), pack.Mods...)
	if len(mods) == 0 {
		b.WriteString("_No mods found in this pack._")
		return b.String()
	}

	if m.mode == modlistSeparated {
		client, server, both := splitBySide(mods)
		m.writeTableSection(&b, "Client", client)
		m.writeTableSection(&b, "Server", server)
		if len(both) > 0 {
			m.writeTableSection(&b, "Dual/Unspecified", both)
		}
		return b.String()
	}

	m.writeTableSection(&b, "All Mods", mods)
	return b.String()
}

// writeTableSection appends a titled GFM table section to b.
func (m *modlistPage) writeTableSection(b *strings.Builder, title string, mods []ui.IndexedMod) {
	if len(mods) == 0 {
		return
	}
	sortMods(mods, m.sortField, m.sortAsc)
	fmt.Fprintf(b, "## %s (***%d***)\n\n", title, len(mods))

	// Build column headers based on active toggles.
	headers := []string{"Name"}
	if m.includeSide {
		headers = append(headers, "Side")
	}
	if m.includeSource {
		headers = append(headers, "Source")
	}
	if m.includeVersions {
		headers = append(headers, "Versions")
	}
	if m.includeFilename {
		headers = append(headers, "Filename")
	}

	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	fmt.Fprintf(b, "| %s |\n", strings.Join(headers, " | "))
	fmt.Fprintf(b, "| %s |\n", strings.Join(seps, " | "))

	for _, mod := range mods {
		b.WriteString(m.formatModTableRow(mod))
	}
	b.WriteString("\n")
}

// formatModTableRow renders one mod as a GFM table row.
func (m *modlistPage) formatModTableRow(mod ui.IndexedMod) string {
	name := modDisplayName(mod)
	// Escape pipes so they don't break the table.
	name = strings.ReplaceAll(name, "|", "\\|")

	if m.attachLinks {
		if link := modLink(mod); link != "" {
			name = fmt.Sprintf("[%s](%s)", name, link)
		}
	}

	cols := []string{fmt.Sprintf("**%s**", name)}
	if m.includeSide {
		side := titleCase(strings.ToLower(mod.Side))
		if side == "" {
			side = "—"
		}
		cols = append(cols, side)
	}
	if m.includeSource {
		src := titleCase(strings.ToLower(string(mod.Source)))
		if src == "" {
			src = "Unknown"
		}
		cols = append(cols, src)
	}
	if m.includeVersions {
		vers := strings.Join(mod.GameVersions, ", ")
		if vers == "" {
			vers = "—"
		}
		cols = append(cols, vers)
	}
	if m.includeFilename {
		fn := mod.Filename
		if fn == "" {
			fn = "—"
		}
		cols = append(cols, fn)
	}

	return fmt.Sprintf("| %s |\n", strings.Join(cols, " | "))
}

// ─── BBCode ───────────────────────────────────────────────────────────────────

// generateBBCode builds a forum-style BBCode modlist.
func (m *modlistPage) generateBBCode() string {
	pack := m.state.Pack
	name := pack.InstanceName
	if name == "" {
		name = "Modlist"
	}

	b := strings.Builder{}
	fmt.Fprintf(&b, "[b]%s[/b]\n", name)

	if m.showProjectMeta {
		if pack.PackAuthor != "" {
			fmt.Fprintf(&b, "Author: %s\n", pack.PackAuthor)
		}
		if pack.PackVersion != "" {
			fmt.Fprintf(&b, "Version: %s\n", pack.PackVersion)
		}
		if pack.PackDescription != "" {
			fmt.Fprintf(&b, "%s\n", pack.PackDescription)
		}
		if pack.WebsiteURL != "" {
			fmt.Fprintf(&b, "[url=%s]Website[/url]\n", pack.WebsiteURL)
		}
	}

	if pack.MinecraftVersion != "" || pack.LoaderUID != "" {
		fmt.Fprintf(&b, "Minecraft %s  |  %s\n",
			valueOr(pack.MinecraftVersion, "?"),
			formatLoader(pack.LoaderUID, pack.LoaderVersion))
	}
	b.WriteString("\n")

	mods := append([]ui.IndexedMod(nil), pack.Mods...)
	if len(mods) == 0 {
		b.WriteString("No mods found in this pack.")
		return b.String()
	}

	if m.mode == modlistSeparated {
		client, server, both := splitBySide(mods)
		m.writeBBCodeSection(&b, "Client", client)
		m.writeBBCodeSection(&b, "Server", server)
		if len(both) > 0 {
			m.writeBBCodeSection(&b, "Dual/Unspecified", both)
		}
		return b.String()
	}

	m.writeBBCodeSection(&b, "All Mods", mods)
	return b.String()
}

// writeBBCodeSection appends a titled BBCode list section to b.
func (m *modlistPage) writeBBCodeSection(b *strings.Builder, title string, mods []ui.IndexedMod) {
	if len(mods) == 0 {
		return
	}
	sortMods(mods, m.sortField, m.sortAsc)
	fmt.Fprintf(b, "[b]%s (%d)[/b]\n[list]\n", title, len(mods))
	for _, mod := range mods {
		b.WriteString(m.formatModBBCode(mod))
	}
	b.WriteString("[/list]\n\n")
}

// formatModBBCode renders one mod as a BBCode list item.
func (m *modlistPage) formatModBBCode(mod ui.IndexedMod) string {
	name := modDisplayName(mod)

	if m.attachLinks {
		if link := modLink(mod); link != "" {
			name = fmt.Sprintf("[url=%s]%s[/url]", link, name)
		}
	}

	var details []string
	if m.includeSide && mod.Side != "" {
		details = append(details, "Side: "+titleCase(strings.ToLower(mod.Side)))
	}
	if m.includeSource {
		src := string(mod.Source)
		if src == "" {
			src = "unknown"
		}
		details = append(details, "Source: "+titleCase(strings.ToLower(src)))
	}
	if m.includeVersions && len(mod.GameVersions) > 0 {
		details = append(details, "MC "+strings.Join(mod.GameVersions, ", "))
	}
	if m.includeFilename && mod.Filename != "" {
		details = append(details, mod.Filename)
	}

	line := fmt.Sprintf("[*][b]%s[/b]", name)
	if len(details) > 0 {
		line += "  —  " + strings.Join(details, "  |  ")
	}
	return line + "\n"
}

// ─── Sort helper ──────────────────────────────────────────────────────────────

// sortMods sorts mods in-place by field, falling back to name for equal values.
func sortMods(mods []ui.IndexedMod, field modlistSort, asc bool) {
	sort.SliceStable(mods, func(i, j int) bool {
		var less bool
		switch field {
		case modlistSortSource:
			si, sj := strings.ToLower(string(mods[i].Source)), strings.ToLower(string(mods[j].Source))
			if si != sj {
				less = si < sj
			} else {
				less = strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name)
			}
		case modlistSortSide:
			si, sj := strings.ToLower(mods[i].Side), strings.ToLower(mods[j].Side)
			if si != sj {
				less = si < sj
			} else {
				less = strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name)
			}
		default: // modlistSortName
			less = strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name)
		}
		if asc {
			return less
		}
		return !less
	})
}

// ─── Shared mod helpers ───────────────────────────────────────────────────────

// modDisplayName returns the best available human-readable name for a mod.
func modDisplayName(mod ui.IndexedMod) string {
	if mod.Name != "" {
		return mod.Name
	}
	if mod.Filename != "" {
		return mod.Filename
	}
	return "Unnamed mod"
}

// modLink returns the canonical URL for a mod, or "" if none is available.
func modLink(mod ui.IndexedMod) string {
	switch mod.Source {
	case ui.SourceModrinth:
		if mod.ModrinthID != "" {
			return fmt.Sprintf("https://modrinth.com/mod/%s", mod.ModrinthID)
		}
	case ui.SourceCurseforge:
		if mod.CurseProject != 0 {
			return fmt.Sprintf("http://curseforge.com/projects/%d", mod.CurseProject)
		}
	}
	return mod.DownloadURL
}

// splitBySide partitions mods into client-only, server-only, and both/unspecified.
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

// ─── Glamour renderer ─────────────────────────────────────────────────────────

// renderMarkdown passes md through glamour for terminal-styled output.
// It caches the renderer by wrap width to avoid unnecessary recreation.
func (m *modlistPage) renderMarkdown(md string) string {
	wrap := m.lastWrap
	if wrap <= 0 {
		wrap = 80
	}

	if m.renderer == nil || m.rendererW != wrap {
		isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
		style := "dark"
		if !isDark {
			style = "light"
		}
		r, err := glamour.NewTermRenderer(
			glamour.WithStylePath(style),
			glamour.WithWordWrap(wrap),
		)
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

// ─── Export / copy ────────────────────────────────────────────────────────────

// exportToFile writes the current output to a file inside the pack directory.
// The filename reflects the output format (.md for markdown, .txt for BBCode).
func (m *modlistPage) exportToFile() string {
	if m.markdown == "" {
		return "Nothing to export"
	}
	filename := "modlist.md"
	if m.outputFormat == modlistFormatBBCode {
		filename = "modlist.txt"
	}
	var path string
	if m.state != nil && m.state.Pack.InstancePath != "" {
		path = filepath.Join(m.state.Pack.InstancePath, filename)
	} else {
		path = filepath.Join(".", filename)
	}
	if err := os.WriteFile(path, []byte(m.markdown), 0o644); err != nil {
		return fmt.Sprintf("Export failed: %v", err)
	}
	return fmt.Sprintf("Exported to %s", path)
}

// copyMarkdown copies the current raw output to the system clipboard.
func (m *modlistPage) copyMarkdown() string {
	if m.markdown == "" {
		return "Nothing to copy"
	}
	if err := clipboard.WriteAll(m.markdown); err != nil {
		return fmt.Sprintf("Copy failed: %v", err)
	}
	switch m.outputFormat {
	case modlistFormatBBCode:
		return "BBCode copied to clipboard"
	case modlistFormatTable:
		return "Markdown table copied to clipboard"
	default:
		return "Markdown copied to clipboard"
	}
}

// ─── Settings hash ────────────────────────────────────────────────────────────

// calculateSettingsHash returns a fast FNV-1a hash of all settings that affect
// the generated output.  rawPreview is intentionally excluded — it only changes
// what the viewport displays, not the underlying raw string.
func (m *modlistPage) calculateSettingsHash() uint64 {
	var h uint64 = 14695981039346656037 // FNV-1a offset basis
	fnv := func(v uint64) { h = h*1099511628211 ^ v }

	fnv(uint64(m.mode))
	fnv(uint64(m.outputFormat))
	fnv(uint64(m.sortField))
	fnv(boolToUint64(m.sortAsc))
	fnv(boolToUint64(m.attachLinks))
	fnv(boolToUint64(m.includeSide))
	fnv(boolToUint64(m.includeSource))
	fnv(boolToUint64(m.includeVersions))
	fnv(boolToUint64(m.includeFilename))
	fnv(boolToUint64(m.showProjectMeta))
	if m.state != nil && m.state.Pack.InstancePath != "" {
		for _, b := range []byte(m.state.Pack.InstancePath) {
			fnv(uint64(b))
		}
	}
	return h
}

// boolToUint64 converts a bool to 0 or 1 for use in hash calculations.
func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// ─── String helpers ───────────────────────────────────────────────────────────

// hardWrapLines hard-wraps every line in s at maxWidth columns, breaking on
// word boundaries where possible and falling back to a hard cut only when a
// single word exceeds maxWidth.  This is used for raw/BBCode content before
// it is handed to the viewport, whose internal renderer does not word-wrap —
// any line longer than previewW would overflow the column and break the layout.
func hardWrapLines(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		out = append(out, wrapLine(line, maxWidth)...)
	}
	return strings.Join(out, "\n")
}

// wrapLine breaks a single line into segments of at most maxWidth runes,
// splitting at the last space within the limit when possible.
func wrapLine(line string, maxWidth int) []string {
	if lipgloss.Width(line) <= maxWidth {
		return []string{line}
	}
	var result []string
	runes := []rune(line)
	for len(runes) > 0 {
		if len(runes) <= maxWidth {
			result = append(result, string(runes))
			break
		}
		split := maxWidth
		// Walk back to find a space to break on.
		for split > 0 && runes[split] != ' ' {
			split--
		}
		if split == 0 {
			split = maxWidth // no space found — hard cut
		}
		result = append(result, string(runes[:split]))
		runes = runes[split:]
		// Trim the leading space on the continuation.
		for len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return result
}

// valueOr returns val if non-empty, otherwise fallback.
func valueOr(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

// titleCase upper-cases only the first rune of s, lowercasing the rest.
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

// formatLoader formats a loader UID and version into a human-readable string.
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

// loaderLabel converts a Prism component UID to a friendly display name.
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
