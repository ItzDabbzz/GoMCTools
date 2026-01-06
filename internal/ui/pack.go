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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pelletier/go-toml/v2"
)

// SharedState keeps data that multiple pages need, such as the currently loaded pack.
type SharedState struct {
	Pack          PackInfo
	LastLoadError string
}

func NewSharedState() *SharedState {
	return &SharedState{}
}

// PackInfo represents the Prism instance that was loaded from disk.
type PackInfo struct {
	InstancePath     string
	InstanceName     string
	MinecraftDir     string
	ModsDir          string
	IndexDir         string
	MinecraftVersion string
	LoaderUID        string
	LoaderVersion    string
	Manifest         *MMCManifest
	Mods             []IndexedMod
	Counts           ModCounts
}

// IndexedMod captures the important parts of a Prism .index entry.
type IndexedMod struct {
	Name          string
	Filename      string
	Source        ModSource
	ReleaseType   string
	Side          string
	Loaders       []string
	GameVersions  []string
	DownloadURL   string
	Hash          string
	HashFormat    string
	UpdateVersion string
	ModrinthID    string
	CurseProject  int
}

// ModCounts provides quick totals for the dashboard.
type ModCounts struct {
	Total      int
	Modrinth   int
	Curseforge int
	Unknown    int
}

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

// ModSource classifies how a mod should be grouped in the dashboard.
type ModSource string

const (
	SourceUnknown    ModSource = "unknown"
	SourceModrinth   ModSource = "modrinth"
	SourceCurseforge ModSource = "curseforge"
)

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

type PackLoadedMsg struct {
	Info PackInfo
	Err  error
}

func LoadPackCmd(root string) tea.Cmd {
	return func() tea.Msg {
		info, err := LoadPack(root)
		return PackLoadedMsg{Info: info, Err: err}
	}
}

// LoadPack scans the Prism instance on disk and returns its metadata and mods.
func LoadPack(root string) (PackInfo, error) {
	resolvedRoot, err := resolveInstanceRoot(root)
	if err != nil {
		return PackInfo{}, err
	}
	root = resolvedRoot

	info := PackInfo{
		InstancePath: root,
		InstanceName: filepath.Base(root),
		MinecraftDir: filepath.Join(root, "minecraft"),
		ModsDir:      filepath.Join(root, "minecraft", "mods"),
		IndexDir:     filepath.Join(root, "minecraft", "mods", ".index"),
	}

	manifest, err := readManifest(filepath.Join(root, "mmc-pack.json"))
	if err != nil {
		return info, err
	}
	info.Manifest = manifest
	info.MinecraftVersion, info.LoaderUID, info.LoaderVersion = summarizeComponents(manifest)

	mods, counts, err := loadIndexEntries(info.IndexDir)
	if err != nil {
		return info, err
	}
	info.Mods = mods
	info.Counts = counts

	return info, nil
}

// resolveInstanceRoot walks up from the provided path until it finds mmc-pack.json.
// It allows users to pass the instance root, minecraft folder, or mods folder.
func resolveInstanceRoot(input string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(input))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	for p := abs; ; p = filepath.Dir(p) {
		if fileExists(filepath.Join(p, "mmc-pack.json")) {
			return p, nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
	}

	return "", fmt.Errorf("could not find mmc-pack.json above %s", abs)
}

func readManifest(path string) (*MMCManifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mmc-pack.json: %w", err)
	}

	var manifest MMCManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse mmc-pack.json: %w", err)
	}
	return &manifest, nil
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

	for _, preferred := range []string{"net.neoforged", "net.minecraftforge", "net.fabricmc.fabric-loader", "net.fabricmc.intermediary", "org.quiltmc.quilt-loader"} {
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
		left := strings.ToLower(mods[i].Name)
		right := strings.ToLower(mods[j].Name)
		if left == right {
			return mods[i].Filename < mods[j].Filename
		}
		return left < right
	})

	counts.Total = len(mods)
	return mods, counts, nil
}

func toIndexedMod(idx indexFile) IndexedMod {
	mod := IndexedMod{
		Name:          idx.Name,
		Filename:      idx.Filename,
		ReleaseType:   idx.ReleaseType,
		Side:          idx.Side,
		Loaders:       idx.Loaders,
		GameVersions:  idx.GameVersions,
		DownloadURL:   idx.Download.URL,
		Hash:          idx.Download.Hash,
		HashFormat:    idx.Download.HashFormat,
		UpdateVersion: "",
		ModrinthID:    "",
		CurseProject:  0,
	}

	if idx.Update.Modrinth != nil {
		mod.Source = SourceModrinth
		mod.UpdateVersion = idx.Update.Modrinth.Version
		mod.ModrinthID = idx.Update.Modrinth.ModID
	} else if idx.Update.Curseforge != nil {
		mod.Source = SourceCurseforge
		mod.UpdateVersion = fmt.Sprintf("project %d file %d", idx.Update.Curseforge.ProjectID, idx.Update.Curseforge.FileID)
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
