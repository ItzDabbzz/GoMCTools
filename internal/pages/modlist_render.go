package pages

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/glow/v2/utils"
	"itzdabbzz.me/gomctools/internal/ui"
)

// generateMarkdown builds the full markdown modlist string from the currently
// loaded pack and the page's display settings.
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

// writeSection appends a titled section of sorted mods to b.
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

// formatMod renders a single mod entry as a markdown list item with optional
// metadata sub-bullets according to the current display settings.
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

// renderMarkdown passes md through glamour for terminal-styled output.
// It caches the renderer by wrap width to avoid unnecessary recreation.
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

// exportToFile writes the current markdown to modlist.md inside the pack directory.
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

// copyMarkdown copies the raw markdown text to the system clipboard.
func (m *modlistPage) copyMarkdown() string {
	if m.markdown == "" {
		return "Nothing to copy"
	}
	if err := clipboard.WriteAll(m.markdown); err != nil {
		return fmt.Sprintf("Copy failed: %v", err)
	}
	return "Markdown copied to clipboard"
}

// calculateSettingsHash returns a fast hash of all settings that affect markdown
// output. It is used to skip regeneration when nothing has changed.
func (m *modlistPage) calculateSettingsHash() uint64 {
	var hash uint64 = 14695981039346656037 // FNV offset basis
	hash = hash*1099511628211 ^ uint64(m.mode)
	hash = hash*1099511628211 ^ boolToUint64(m.attachLinks)
	hash = hash*1099511628211 ^ boolToUint64(m.includeSide)
	hash = hash*1099511628211 ^ boolToUint64(m.includeSource)
	hash = hash*1099511628211 ^ boolToUint64(m.includeVersions)
	hash = hash*1099511628211 ^ boolToUint64(m.includeFilename)
	if m.state != nil && m.state.Pack.InstancePath != "" {
		for _, b := range []byte(m.state.Pack.InstancePath) {
			hash = hash*1099511628211 ^ uint64(b)
		}
	}
	return hash
}

// boolToUint64 converts a bool to 0 or 1 for use in hash calculations.
func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// --- string helpers ---

// valueOr returns val if non-empty, otherwise fallback.
func valueOr(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

// titleCase upper-cases only the first rune of s.
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

// loaderLabel converts a Prism component UID to a friendly name.
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
