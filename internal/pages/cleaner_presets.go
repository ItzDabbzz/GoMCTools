package pages

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// cleanerPreset describes a single named pattern the Pack Cleaner can delete.
// BuiltIn presets ship with the application and cannot be edited or deleted.
type cleanerPreset struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Enabled bool   `json:"enabled"`
	BuiltIn bool   `json:"-"`
}

// cleanerConfig is the on-disk representation of custom presets stored inside
// the Minecraft directory as gomctools.cleaner.json.
type cleanerConfig struct {
	Custom []cleanerPreset `json:"custom_presets"`
}

// defaultCleanerPresets returns the built-in preset list.
// All presets are disabled by default so the user must opt-in.
func defaultCleanerPresets() []cleanerPreset {
	names := []string{
		".cache/",
		".curseclient/",
		".mixin.out/",
		".probe/",
		".vscode/",
		"bluemap/",
		"cachecoremods/",
		"crash-reports/",
		"data/",
		"Distant_Horizons_server_data/",
		"downloads/",
		"dynamic-data-pack-cache/",
		"dynamic-resource-pack-cache/",
		"fancymenu_data/",
		"journeymap/",
		"local/",
		"logs/",
		"midi_files/",
		"moddata/",
		"moonlight-global-datapacks/",
		"patchouli_books/",
		"saves/",
		"screenshots/",
		"waypoints/",
		"command_history.txt",
		"patchouli_data.json",
		"servers.dat",
		"servers.dat_old",
		"usercache.json",
		"usernamecache.json",
		"modlist.html",
		"kubejs/",
	}

	presets := make([]cleanerPreset, 0, len(names))
	for _, n := range names {
		presets = append(presets, cleanerPreset{
			Name:    displayName(n),
			Pattern: normalizePattern(n),
			Enabled: false,
			BuiltIn: true,
		})
	}
	return presets
}

// deletePreset removes the filesystem path described by preset.Pattern under
// root. It refuses to delete paths that escape the root directory.
func deletePreset(root string, preset cleanerPreset) (int, error) {
	if root == "" {
		return 0, errors.New("missing root path")
	}

	pattern := strings.TrimSpace(preset.Pattern)
	if pattern == "" {
		return 0, nil
	}
	pattern = normalizePattern(pattern)

	rel := strings.TrimPrefix(pattern, string(os.PathSeparator))
	target := filepath.Clean(filepath.Join(root, rel))

	if !strings.HasPrefix(target, filepath.Clean(root)) {
		return 0, fmt.Errorf("refusing to delete outside root: %s", target)
	}
	if target == filepath.Clean(root) {
		return 0, fmt.Errorf("refusing to delete root directory")
	}

	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	count := 1
	if info.IsDir() {
		c, cerr := countDirEntries(target)
		if cerr == nil {
			count = c
		}
		return count, os.RemoveAll(target)
	}
	return count, os.Remove(target)
}

// countEntries returns the number of filesystem items the preset would remove.
func countEntries(root string, preset cleanerPreset) (int, error) {
	target := filepath.Clean(filepath.Join(root, normalizePattern(preset.Pattern)))
	if !strings.HasPrefix(target, filepath.Clean(root)) {
		return 0, fmt.Errorf("outside root")
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	if info.IsDir() {
		return countDirEntries(target)
	}
	return 1, nil
}

// countDirEntries walks dir and returns the number of files it contains.
// An empty directory counts as one unit of work.
func countDirEntries(dir string) (int, error) {
	count := 0
	walkErr := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		count++
		return nil
	})
	if walkErr != nil {
		return count, walkErr
	}
	if count == 0 {
		count = 1
	}
	return count, nil
}

// readCleanerConfig loads custom presets from root/gomctools.cleaner.json.
// Returns nil, nil when the file does not exist.
func readCleanerConfig(root string) ([]cleanerPreset, error) {
	if root == "" {
		return nil, errors.New("missing root path")
	}
	path := filepath.Join(root, "gomctools.cleaner.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg cleanerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return normalizeCustom(cfg.Custom), nil
}

// writeCleanerConfig persists custom presets to root/gomctools.cleaner.json.
func writeCleanerConfig(root string, presets []cleanerPreset) error {
	if root == "" {
		return errors.New("missing root path")
	}
	path := filepath.Join(root, "gomctools.cleaner.json")
	cfg := cleanerConfig{Custom: normalizeCustom(presets)}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// normalizePattern strips leading separators and "./" prefixes from a preset
// pattern so it is always relative to the Minecraft directory root.
func normalizePattern(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, string(os.PathSeparator))
	return p
}

// normalizeCustom filters and cleans a slice of custom presets, removing any
// entries with empty names or patterns and resetting BuiltIn to false.
func normalizeCustom(list []cleanerPreset) []cleanerPreset {
	out := make([]cleanerPreset, 0, len(list))
	for _, p := range list {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Pattern) == "" {
			continue
		}
		p.BuiltIn = false
		p.Pattern = normalizePattern(p.Pattern)
		out = append(out, p)
	}
	return out
}

// filterCustom returns only the non-built-in presets from list.
func filterCustom(list []cleanerPreset) []cleanerPreset {
	out := []cleanerPreset{}
	for _, p := range list {
		if p.BuiltIn {
			continue
		}
		out = append(out, p)
	}
	return out
}

// displayName converts a path-style preset name into a human-readable label.
func displayName(path string) string {
	trimmed := strings.TrimSuffix(path, string(os.PathSeparator))
	trimmed = strings.TrimPrefix(trimmed, ".")
	if trimmed == "" {
		return path
	}
	return trimmed
}
