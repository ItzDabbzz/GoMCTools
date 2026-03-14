package ui

// pack_curseforge.go — CurseForge instance loading.
//
// CurseForge exports two relevant files:
//
//   manifest.json          — lightweight export: pack metadata + projectID/fileID pairs only.
//                            No mod names. Used as a fallback when the instance file is absent.
//
//   minecraftinstance.json — full CurseForge app database record.
//                            Contains rich installedAddons array (name, authors, download URL,
//                            release type, game versions, etc.).  Can exceed 15 MB for large
//                            packs; this file is streamed rather than loaded into memory whole.
//
// Loading strategy:
//   1. If minecraftinstance.json exists → stream it (full metadata).
//   2. Else if manifest.json exists     → parse it (IDs only, mod names become "Project <N>").

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ─── manifest.json types ──────────────────────────────────────────────────────

// CFManifest mirrors the standalone CurseForge manifest.json export format.
type CFManifest struct {
	Minecraft       CFMinecraft `json:"minecraft"`
	ManifestType    string      `json:"manifestType"`
	ManifestVersion int         `json:"manifestVersion"`
	Name            string      `json:"name"`
	Version         string      `json:"version"`
	Author          string      `json:"author"`
	Files           []CFFile    `json:"files"`
	Overrides       string      `json:"overrides"`
}

// CFMinecraft holds the game version and loader list from manifest.json.
type CFMinecraft struct {
	Version        string        `json:"version"`
	ModLoaders     []CFModLoader `json:"modLoaders"`
	RecommendedRAM int           `json:"recommendedRam"`
}

// CFModLoader is a single entry from manifest.json's modLoaders array.
type CFModLoader struct {
	ID      string `json:"id"`      // e.g. "neoforge-21.1.219"
	Primary bool   `json:"primary"` // true for the primary loader
}

// CFFile is a single mod entry in manifest.json — IDs only, no names.
type CFFile struct {
	ProjectID int  `json:"projectID"`
	FileID    int  `json:"fileID"`
	Required  bool `json:"required"`
	IsLocked  bool `json:"isLocked"`
}

// ─── minecraftinstance.json streaming types ───────────────────────────────────

// cfInstanceData holds the top-level fields we need from minecraftinstance.json.
// The full file is streamed; only these fields are materialised in memory.
type cfInstanceData struct {
	Name         string
	MCVersion    string // from top-level "gameVersion" or baseModLoader.minecraftVersion
	LoaderID     string // raw loader string, e.g. "neoforge-21.1.219"
	PackVersion  string // from embedded manifest.version
	PackAuthor   string // from embedded manifest.author
	WebsiteURL   string // from installedModpack.webSiteURL
	ThumbnailURL string // from installedModpack.thumbnailUrl
	Addons       []CFAddonSummary
}

// CFAddonSummary is the minimal subset decoded from each installedAddon entry.
// Keeping this small is key to low memory usage on large packs — each full
// addon object in the file is several KB; this struct is ~200 bytes.
type CFAddonSummary struct {
	AddonID       int         `json:"addonID"`
	Name          string      `json:"name"`
	PrimaryAuthor string      `json:"primaryAuthor"`
	WebSiteURL    string      `json:"webSiteURL"`
	IsEnabled     bool        `json:"isEnabled"`
	InstalledFile CFAddonFile `json:"installedFile"`
}

// CFAddonFile captures the fields we use from an installedFile sub-object.
type CFAddonFile struct {
	ID          int      `json:"id"`
	FileName    string   `json:"fileName"`
	DownloadURL string   `json:"downloadUrl"`
	ReleaseType int      `json:"releaseType"` // 1=release 2=beta 3=alpha
	GameVersion []string `json:"gameVersion"` // MC version strings
	ProjectID   int      `json:"projectId"`
}

// ─── Entry point ──────────────────────────────────────────────────────────────

// loadCurseForgePack loads a CurseForge instance from its resolved root.
// It prefers minecraftinstance.json for rich metadata and falls back to
// manifest.json when the full instance file is not present.
func loadCurseForgePack(root string) (PackInfo, error) {
	info := PackInfo{
		SourceType:   PackSourceCurseForge,
		InstancePath: root,
		InstanceName: filepath.Base(root), // overwritten below if the JSON has a name
		MinecraftDir: root,                // CurseForge has no "minecraft" sub-folder
		ModsDir:      filepath.Join(root, "mods"),
		// IndexDir is intentionally empty: CurseForge has no .index TOML directory.
	}

	instancePath := filepath.Join(root, "minecraftinstance.json")
	manifestPath := filepath.Join(root, "manifest.json")

	switch {
	case fileExists(instancePath):
		return loadFromCFInstance(root, instancePath, manifestPath, info)
	case fileExists(manifestPath):
		return loadFromCFManifest(manifestPath, info)
	default:
		return info, fmt.Errorf(
			"neither minecraftinstance.json nor manifest.json found in %s", root,
		)
	}
}

// ─── minecraftinstance.json path ─────────────────────────────────────────────

// loadFromCFInstance streams minecraftinstance.json and optionally cross-references
// manifest.json for any fields the instance file did not populate.
func loadFromCFInstance(root, instancePath, manifestPath string, info PackInfo) (PackInfo, error) {
	data, err := streamCFInstance(instancePath)
	if err != nil {
		return info, fmt.Errorf("parse minecraftinstance.json: %w", err)
	}

	if data.Name != "" {
		info.InstanceName = data.Name
	}
	info.MinecraftVersion = data.MCVersion
	info.LoaderUID, info.LoaderVersion = parseLoaderID(data.LoaderID)
	info.PackVersion = data.PackVersion
	info.PackAuthor = data.PackAuthor
	info.WebsiteURL = data.WebsiteURL
	info.ThumbnailURL = data.ThumbnailURL

	// Cross-reference manifest.json to fill any gaps.
	if fileExists(manifestPath) {
		if m, err := readCFManifest(manifestPath); err == nil {
			if info.InstanceName == filepath.Base(root) && m.Name != "" {
				info.InstanceName = m.Name
			}
			if info.MinecraftVersion == "" {
				info.MinecraftVersion = m.Minecraft.Version
			}
			if info.LoaderUID == "" {
				for _, l := range m.Minecraft.ModLoaders {
					if l.Primary {
						info.LoaderUID, info.LoaderVersion = parseLoaderID(l.ID)
						break
					}
				}
			}
			if info.PackVersion == "" {
				info.PackVersion = m.Version
			}
			if info.PackAuthor == "" {
				info.PackAuthor = m.Author
			}
		}
	}

	info.Mods, info.Counts = cfAddonsToMods(data.Addons)
	return info, nil
}

// ─── manifest.json fallback path ─────────────────────────────────────────────

// loadFromCFManifest builds a PackInfo from manifest.json only.
// Because manifest.json contains only projectID/fileID pairs, mod names
// are synthesised as "Project <N>" and all other per-mod fields are empty.
func loadFromCFManifest(manifestPath string, info PackInfo) (PackInfo, error) {
	m, err := readCFManifest(manifestPath)
	if err != nil {
		return info, fmt.Errorf("parse manifest.json: %w", err)
	}

	if m.Name != "" {
		info.InstanceName = m.Name
	}
	info.MinecraftVersion = m.Minecraft.Version
	info.PackVersion = m.Version
	info.PackAuthor = m.Author
	for _, l := range m.Minecraft.ModLoaders {
		if l.Primary {
			info.LoaderUID, info.LoaderVersion = parseLoaderID(l.ID)
			break
		}
	}

	var mods []IndexedMod
	var counts ModCounts
	for _, f := range m.Files {
		if !f.Required {
			continue
		}
		mods = append(mods, IndexedMod{
			Name:          fmt.Sprintf("Project %d", f.ProjectID),
			Source:        SourceCurseforge,
			CurseProject:  f.ProjectID,
			UpdateVersion: fmt.Sprintf("project %d file %d", f.ProjectID, f.FileID),
		})
		counts.Curseforge++
	}
	counts.Total = len(mods)
	info.Mods = mods
	info.Counts = counts
	return info, nil
}

// ─── minecraftinstance.json streaming decoder ─────────────────────────────────

// streamCFInstance streams minecraftinstance.json key-by-key, decoding only the
// fields we care about and skipping everything else with a single RawMessage
// allocation.  The installedAddons array is decoded one element at a time via
// CFAddonSummary, keeping peak memory well below the raw file size.
func streamCFInstance(path string) (cfInstanceData, error) {
	f, err := os.Open(path)
	if err != nil {
		return cfInstanceData{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var result cfInstanceData
	dec := json.NewDecoder(f)

	// Consume opening '{'.
	if tok, err := dec.Token(); err != nil {
		return result, fmt.Errorf("read opening brace: %w", err)
	} else if d, ok := tok.(json.Delim); !ok || d != '{' {
		return result, fmt.Errorf("expected '{', got %v", tok)
	}

	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return result, fmt.Errorf("read object key: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			if err := skipJSONValue(dec); err != nil {
				return result, err
			}
			continue
		}

		switch key {

		case "name":
			if err := dec.Decode(&result.Name); err != nil {
				return result, fmt.Errorf("decode name: %w", err)
			}

		// Top-level "gameVersion" is the Minecraft version string.
		case "gameVersion":
			if err := dec.Decode(&result.MCVersion); err != nil {
				return result, fmt.Errorf("decode gameVersion: %w", err)
			}

		// baseModLoader carries the loader identity and, as a fallback,
		// the Minecraft version again.
		case "baseModLoader":
			var bml struct {
				Name             string `json:"name"`
				MinecraftVersion string `json:"minecraftVersion"`
			}
			if err := dec.Decode(&bml); err != nil {
				return result, fmt.Errorf("decode baseModLoader: %w", err)
			}
			result.LoaderID = bml.Name
			if result.MCVersion == "" {
				result.MCVersion = bml.MinecraftVersion
			}

		// The embedded "manifest" object mirrors manifest.json and carries
		// pack name, version, author, and an alternative loader list.
		case "manifest":
			var m struct {
				Minecraft struct {
					Version    string `json:"version"`
					ModLoaders []struct {
						ID      string `json:"id"`
						Primary bool   `json:"primary"`
					} `json:"modLoaders"`
				} `json:"minecraft"`
				Name    string `json:"name"`
				Version string `json:"version"`
				Author  string `json:"author"`
			}
			if err := dec.Decode(&m); err != nil {
				return result, fmt.Errorf("decode manifest: %w", err)
			}
			if result.MCVersion == "" {
				result.MCVersion = m.Minecraft.Version
			}
			if result.LoaderID == "" {
				for _, l := range m.Minecraft.ModLoaders {
					if l.Primary {
						result.LoaderID = l.ID
						break
					}
				}
			}
			if result.Name == "" && m.Name != "" {
				result.Name = m.Name
			}
			result.PackVersion = m.Version
			result.PackAuthor = m.Author

		// installedModpack carries the project-level metadata: URL, thumbnail, authors.
		case "installedModpack":
			var imp struct {
				PrimaryAuthor string `json:"primaryAuthor"`
				WebSiteURL    string `json:"webSiteURL"`
				ThumbnailURL  string `json:"thumbnailUrl"`
				Authors       []struct {
					Name string `json:"name"`
				} `json:"authors"`
			}
			if err := dec.Decode(&imp); err != nil {
				return result, fmt.Errorf("decode installedModpack: %w", err)
			}
			if result.PackAuthor == "" {
				result.PackAuthor = imp.PrimaryAuthor
				// Fall back to first author in the list if primary is empty.
				if result.PackAuthor == "" && len(imp.Authors) > 0 {
					result.PackAuthor = imp.Authors[0].Name
				}
			}
			result.WebsiteURL = imp.WebSiteURL
			result.ThumbnailURL = imp.ThumbnailURL

		// installedAddons is the large array — stream it one element at a time.
		case "installedAddons":
			// Consume opening '['.
			arrTok, err := dec.Token()
			if err != nil {
				return result, fmt.Errorf("read installedAddons '[': %w", err)
			}
			if d, ok := arrTok.(json.Delim); !ok || d != '[' {
				return result, fmt.Errorf("expected '[' for installedAddons, got %v", arrTok)
			}
			for dec.More() {
				var addon CFAddonSummary
				if err := dec.Decode(&addon); err != nil {
					return result, fmt.Errorf("decode addon entry: %w", err)
				}
				result.Addons = append(result.Addons, addon)
			}
			// Consume closing ']'.
			if _, err := dec.Token(); err != nil {
				return result, fmt.Errorf("read installedAddons ']': %w", err)
			}

		default:
			// Skip any field we don't need.
			if err := skipJSONValue(dec); err != nil {
				return result, fmt.Errorf("skip field %q: %w", key, err)
			}
		}
	}

	return result, nil
}

// skipJSONValue consumes and discards the next value from the decoder.
func skipJSONValue(dec *json.Decoder) error {
	var raw json.RawMessage
	return dec.Decode(&raw)
}

// ─── Conversion helpers ───────────────────────────────────────────────────────

// readCFManifest parses a CurseForge manifest.json file.
func readCFManifest(path string) (CFManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CFManifest{}, fmt.Errorf("read manifest.json: %w", err)
	}
	var m CFManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return CFManifest{}, fmt.Errorf("parse manifest.json: %w", err)
	}
	return m, nil
}

// cfAddonsToMods converts a slice of CFAddonSummary to IndexedMods + counts.
// Results are sorted alphabetically by name (matching Prism behaviour).
func cfAddonsToMods(addons []CFAddonSummary) ([]IndexedMod, ModCounts) {
	mods := make([]IndexedMod, 0, len(addons))
	var counts ModCounts

	for _, addon := range addons {
		mods = append(mods, IndexedMod{
			Name:        addon.Name,
			Filename:    addon.InstalledFile.FileName,
			Source:      SourceCurseforge,
			ReleaseType: cfReleaseTypeName(addon.InstalledFile.ReleaseType),
			// Side is not available from the CurseForge API at the per-addon level.
			GameVersions: addon.InstalledFile.GameVersion,
			DownloadURL:  addon.InstalledFile.DownloadURL,
			CurseProject: addon.AddonID,
			UpdateVersion: fmt.Sprintf(
				"project %d file %d", addon.AddonID, addon.InstalledFile.ID,
			),
		})
		counts.Curseforge++
	}

	sort.Slice(mods, func(i, j int) bool {
		l, r := strings.ToLower(mods[i].Name), strings.ToLower(mods[j].Name)
		if l == r {
			return mods[i].Filename < mods[j].Filename
		}
		return l < r
	})
	counts.Total = len(mods)
	return mods, counts
}

// ─── Loader ID parsing ────────────────────────────────────────────────────────

// parseLoaderID splits a raw CurseForge loader identifier into a canonical UID
// (matching the UIDs used in Prism's mmc-pack.json) and a version string.
//
// Examples:
//
//	"neoforge-21.1.219"       → "net.neoforged",             "21.1.219"
//	"forge-1.21.1-53.1.0"     → "net.minecraftforge",        "53.1.0"
//	"fabric-0.16.10"          → "net.fabricmc.fabric-loader", "0.16.10"
//	"quilt-0.26.3"            → "org.quiltmc.quilt-loader",   "0.26.3"
//	"unknownloader-1.0"       → "unknownloader-1.0",          "1.0"
func parseLoaderID(id string) (uid, version string) {
	if id == "" {
		return "", ""
	}
	lower := strings.ToLower(id)
	dashIdx := strings.IndexByte(lower, '-')
	if dashIdx < 0 {
		// No separator: treat the whole thing as the uid with no version.
		return id, ""
	}
	prefix := lower[:dashIdx]
	rest := id[dashIdx+1:] // preserve original casing for the version string

	switch prefix {
	case "neoforge":
		return "net.neoforged", rest

	case "forge":
		// Format: forge-<mcVersion>-<forgeVersion>
		// We want only the forge component (after the MC version).
		if idx := strings.IndexByte(rest, '-'); idx >= 0 {
			return "net.minecraftforge", rest[idx+1:]
		}
		return "net.minecraftforge", rest

	case "fabric":
		return "net.fabricmc.fabric-loader", rest

	case "quilt":
		return "org.quiltmc.quilt-loader", rest

	default:
		// Unknown loader: use the whole raw ID as uid, rest as version.
		return id, rest
	}
}

// cfReleaseTypeName maps a CurseForge numeric releaseType to a human-readable label.
func cfReleaseTypeName(t int) string {
	switch t {
	case 1:
		return "release"
	case 2:
		return "beta"
	case 3:
		return "alpha"
	default:
		return "unknown"
	}
}
