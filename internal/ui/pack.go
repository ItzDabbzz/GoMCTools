package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/pelletier/go-toml/v2"

	"github.com/ItzDabbzz/GoMCTools/internal/config"
)

// ─── Pack source type ─────────────────────────────────────────────────────────

// PackSourceType identifies the launcher / format the pack was imported from.
type PackSourceType string

const (
	PackSourcePrism      PackSourceType = "prism"
	PackSourceCurseForge PackSourceType = "curseforge"
	PackSourceUnknown    PackSourceType = "unknown"
)

// ─── Shared state ─────────────────────────────────────────────────────────────

// SharedState is the single source of truth shared across all pages.
// It holds the currently loaded pack and the active configuration.
type SharedState struct {
	Config        *config.Config
	Pack          PackInfo
	LastLoadError string
}

// NewSharedState creates a SharedState pre-populated with DefaultConfig.
func NewSharedState() *SharedState {
	cfg := config.DefaultConfig()
	return &SharedState{
		Config: &cfg,
	}
}

// ─── PackInfo ─────────────────────────────────────────────────────────────────

// PackInfo represents the loaded pack regardless of source format.
// Fields that are launcher-specific are documented with their origin.
type PackInfo struct {
	SourceType       PackSourceType // which launcher/format this pack came from
	InstancePath     string
	InstanceName     string
	MinecraftDir     string
	ModsDir          string
	IndexDir         string // Prism-only: path to mods/.index; empty for CurseForge
	MinecraftVersion string
	LoaderUID        string // e.g. "net.neoforged", "net.minecraftforge"
	LoaderVersion    string
	PackVersion      string       // pack release version, e.g. "0.12.0-beta"
	PackAuthor       string       // primary author / team name
	PackDescription  string       // short description if available
	WebsiteURL       string       // CurseForge project page URL
	ThumbnailURL     string       // CurseForge thumbnail URL
	Manifest         *MMCManifest // Prism-only: parsed mmc-pack.json; nil for CurseForge
	Mods             []IndexedMod
	Counts           ModCounts
}

// ─── Shared mod types ─────────────────────────────────────────────────────────

// IndexedMod captures the relevant fields from any mod source.
// Fields that cannot be populated for a given source are left as zero values.
type IndexedMod struct {
	Name          string
	Filename      string
	Source        ModSource
	ReleaseType   string
	Side          string   // empty for CurseForge (not exposed by the API)
	Loaders       []string // empty for CurseForge
	GameVersions  []string
	DownloadURL   string
	Hash          string
	HashFormat    string
	UpdateVersion string
	ModrinthID    string // Modrinth-only
	CurseProject  int    // CurseForge-only: addonID / projectID
}

// ModCounts provides quick totals for the dashboard.
type ModCounts struct {
	Total      int
	Modrinth   int
	Curseforge int
	Unknown    int
}

// ModSource classifies how a mod should be grouped in the dashboard.
type ModSource string

const (
	SourceUnknown    ModSource = "unknown"
	SourceModrinth   ModSource = "modrinth"
	SourceCurseforge ModSource = "curseforge"
)

// ─── Prism-specific types ─────────────────────────────────────────────────────

// MMCManifest mirrors Prism's mmc-pack.json.
type MMCManifest struct {
	FormatVersion int            `json:"formatVersion"`
	Components    []MMCComponent `json:"components"`
}

// MMCComponent describes a single manifest component.
type MMCComponent struct {
	UID            string `json:"uid"`
	Version        string `json:"version"`
	CachedName     string `json:"cachedName"`
	CachedVersion  string `json:"cachedVersion"`
	CachedVolatile bool   `json:"cachedVolatile"`
	DependencyOnly bool   `json:"dependencyOnly"`
	Important      bool   `json:"important"`
}

type indexFile struct {
	Filename     string        `toml:"filename"`
	Name         string        `toml:"name"`
	Side         string        `toml:"side"`
	Loaders      []string      `toml:"x-prismlauncher-loaders"`
	GameVersions []string      `toml:"x-prismlauncher-mc-versions"`
	ReleaseType  string        `toml:"x-prismlauncher-release-type"`
	Download     indexDownload `toml:"download"`
	Update       indexUpdate   `toml:"update"`
}

type indexDownload struct {
	Hash       string `toml:"hash"`
	HashFormat string `toml:"hash-format"`
	Mode       string `toml:"mode"`
	URL        string `toml:"url"`
}

type indexUpdate struct {
	Modrinth   *indexModrinth   `toml:"modrinth"`
	Curseforge *indexCurseforge `toml:"curseforge"`
}

type indexModrinth struct {
	ModID   string `toml:"mod-id"`
	Version string `toml:"version"`
}

type indexCurseforge struct {
	ProjectID int `toml:"project-id"`
	FileID    int `toml:"file-id"`
}

// ─── Messages ─────────────────────────────────────────────────────────────────

// PackLoadedMsg is emitted by LoadPackCmd when pack loading completes.
// If Err is non-nil the load failed and Info should be considered empty.
type PackLoadedMsg struct {
	Info PackInfo
	Err  error
}

// ─── Public API ───────────────────────────────────────────────────────────────

// LoadPackCmd returns a Cmd that loads the pack at root and broadcasts the result.
func LoadPackCmd(root string) tea.Cmd {
	return func() tea.Msg {
		info, err := LoadPack(root)
		return PackLoadedMsg{Info: info, Err: err}
	}
}

// LoadPack auto-detects the pack format (Prism or CurseForge) and loads its metadata.
// Detection walks up from root, preferring mmc-pack.json (Prism) over
// minecraftinstance.json / manifest.json (CurseForge).
func LoadPack(root string) (PackInfo, error) {
	resolved, sourceType, err := resolveRoot(root)
	if err != nil {
		return PackInfo{}, err
	}
	switch sourceType {
	case PackSourceCurseForge:
		return loadCurseForgePack(resolved)
	case PackSourcePrism:
		return loadPrismPack(resolved)
	default:
		return PackInfo{}, fmt.Errorf(
			"no supported pack manifest found at or above %s — expected mmc-pack.json or minecraftinstance.json",
			root,
		)
	}
}

// ─── Root resolution ─────────────────────────────────────────────────────────

// resolveRoot walks up from input until it finds a supported pack manifest.
// mmc-pack.json is checked before CurseForge files so that Prism instances
// are never misidentified even if a CurseForge manifest is present nearby.
func resolveRoot(input string) (string, PackSourceType, error) {
	abs, err := filepath.Abs(strings.TrimSpace(input))
	if err != nil {
		return "", PackSourceUnknown, fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", PackSourceUnknown, fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	for p := abs; ; p = filepath.Dir(p) {
		if fileExists(filepath.Join(p, "mmc-pack.json")) {
			return p, PackSourcePrism, nil
		}
		if fileExists(filepath.Join(p, "minecraftinstance.json")) {
			return p, PackSourceCurseForge, nil
		}
		// manifest.json is common enough that we peek inside before claiming it.
		if fileExists(filepath.Join(p, "manifest.json")) && isCFManifestFile(filepath.Join(p, "manifest.json")) {
			return p, PackSourceCurseForge, nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
	}
	return "", PackSourceUnknown, fmt.Errorf(
		"no supported pack manifest found above %s",
		abs,
	)
}

// isCFManifestFile peeks at a manifest.json to confirm it carries the
// CurseForge "minecraftModpack" manifest type before claiming CurseForge ownership.
func isCFManifestFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	var probe struct {
		ManifestType string `json:"manifestType"`
	}
	if err := json.NewDecoder(f).Decode(&probe); err != nil {
		return false
	}
	return probe.ManifestType == "minecraftModpack"
}

// ─── Prism loader ─────────────────────────────────────────────────────────────
// All logic below is unchanged from the original implementation.

// loadPrismPack loads a Prism Launcher instance from its resolved root directory.
func loadPrismPack(root string) (PackInfo, error) {
	info := PackInfo{
		SourceType:   PackSourcePrism,
		InstancePath: root,
		InstanceName: filepath.Base(root),
		MinecraftDir: filepath.Join(root, "minecraft"),
		ModsDir:      filepath.Join(root, "minecraft", "mods"),
		IndexDir:     filepath.Join(root, "minecraft", "mods", ".index"),
	}

	manifest, err := readMMCManifest(filepath.Join(root, "mmc-pack.json"))
	if err != nil {
		return info, err
	}
	info.Manifest = manifest
	info.MinecraftVersion, info.LoaderUID, info.LoaderVersion = summarizeComponents(manifest)

	// Read instance.cfg for the human-set instance name and description notes.
	// The file is a simple key=value format; we only need "name" and "notes".
	readInstanceCfg(filepath.Join(root, "instance.cfg"), &info)

	mods, counts, err := loadIndexEntries(info.IndexDir)
	if err != nil {
		return info, err
	}
	info.Mods = mods
	info.Counts = counts
	return info, nil
}

func readMMCManifest(path string) (*MMCManifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mmc-pack.json: %w", err)
	}
	var m MMCManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse mmc-pack.json: %w", err)
	}
	return &m, nil
}

func summarizeComponents(manifest *MMCManifest) (minecraft, loaderUID, loaderVersion string) {
	if manifest == nil {
		return "", "", ""
	}
	for _, c := range manifest.Components {
		if c.UID == "net.minecraft" {
			minecraft = c.Version
			break
		}
	}
	for _, preferred := range []string{
		"net.neoforged",
		"net.minecraftforge",
		"net.fabricmc.fabric-loader",
		"net.fabricmc.intermediary",
		"org.quiltmc.quilt-loader",
	} {
		for _, c := range manifest.Components {
			if c.UID == preferred {
				loaderUID = c.UID
				loaderVersion = c.Version
				return minecraft, loaderUID, loaderVersion
			}
		}
	}
	return minecraft, loaderUID, loaderVersion
}

func loadIndexEntries(indexDir string) ([]IndexedMod, ModCounts, error) {
	var mods []IndexedMod
	var counts ModCounts

	entries, err := os.ReadDir(indexDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, counts, fmt.Errorf("mods index not found at %s", indexDir)
		}
		return nil, counts, fmt.Errorf("read mods index: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(indexDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, counts, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		var idx indexFile
		if err := toml.Unmarshal(data, &idx); err != nil {
			return nil, counts, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		mod := toIndexedMod(idx)
		mods = append(mods, mod)
		switch mod.Source {
		case SourceModrinth:
			counts.Modrinth++
		case SourceCurseforge:
			counts.Curseforge++
		default:
			counts.Unknown++
		}
	}

	sort.Slice(mods, func(i, j int) bool {
		l, r := strings.ToLower(mods[i].Name), strings.ToLower(mods[j].Name)
		if l == r {
			return mods[i].Filename < mods[j].Filename
		}
		return l < r
	})
	counts.Total = len(mods)
	return mods, counts, nil
}

func toIndexedMod(idx indexFile) IndexedMod {
	mod := IndexedMod{
		Name:         idx.Name,
		Filename:     idx.Filename,
		ReleaseType:  idx.ReleaseType,
		Side:         idx.Side,
		Loaders:      idx.Loaders,
		GameVersions: idx.GameVersions,
		DownloadURL:  idx.Download.URL,
		Hash:         idx.Download.Hash,
		HashFormat:   idx.Download.HashFormat,
	}
	if idx.Update.Modrinth != nil {
		mod.Source = SourceModrinth
		mod.UpdateVersion = idx.Update.Modrinth.Version
		mod.ModrinthID = idx.Update.Modrinth.ModID
	} else if idx.Update.Curseforge != nil {
		mod.Source = SourceCurseforge
		mod.UpdateVersion = fmt.Sprintf("project %d file %d",
			idx.Update.Curseforge.ProjectID, idx.Update.Curseforge.FileID)
		mod.CurseProject = idx.Update.Curseforge.ProjectID
	} else if strings.Contains(strings.ToLower(idx.Download.URL), "modrinth") {
		mod.Source = SourceModrinth
	} else if strings.Contains(strings.ToLower(idx.Download.URL), "curseforge") {
		mod.Source = SourceCurseforge
	} else {
		mod.Source = SourceUnknown
	}
	return mod
}

// ─── Utilities ────────────────────────────────────────────────────────────────

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// readInstanceCfg parses a Prism Launcher instance.cfg file and writes the
// fields it finds into info.  The file uses a simple key=value format (an INI
// [General] header may be present but is ignored).  Only "name" and "notes"
// are extracted; all other lines are skipped.
//
// Failures are silently swallowed: instance.cfg is optional metadata and a
// missing or malformed file must never abort a pack load.
func readInstanceCfg(path string, info *PackInfo) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		switch strings.ToLower(k) {
		case "name":
			if v != "" {
				info.InstanceName = v
			}
		case "notes":
			if v != "" {
				info.PackDescription = v
			}
		}
	}
}
