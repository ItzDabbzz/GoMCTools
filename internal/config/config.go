package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application's persistent settings.
// It is serialised to TOML and loaded from ~/.config/gomctools/config.toml.
type Config struct {
	// AutoLoadPreviousState reopens the last loaded pack on startup when true.
	AutoLoadPreviousState bool `toml:"auto_load_previous_state"`

	// Selector holds settings for the pack-selection page.
	Selector struct {
		// LastPath is the filesystem path of the most recently loaded pack instance.
		LastPath string `toml:"last_path"`
	} `toml:"selector"`

	// Modlist Generator page settings
	Modlist struct {
		Mode            int  `toml:"mode"`              // 0 = merged, 1 = split by side
		AttachLinks     bool `toml:"attach_links"`      // Include mod links
		IncludeSide     bool `toml:"include_side"`      // Include side tag
		IncludeSource   bool `toml:"include_source"`    // Include source (Modrinth/CurseForge)
		IncludeVersions bool `toml:"include_versions"`  // Include game versions
		IncludeFilename bool `toml:"include_filenames"` // Include filenames
	} `toml:"modlist"`

	// Pack Cleaner page settings
	Cleaner struct {
		CustomPresets []CleanerPreset `toml:"custom_presets"`
	} `toml:"cleaner"`
}

// CleanerPreset represents a cleaner preset configuration.
type CleanerPreset struct {
	Name    string `toml:"name"`
	Pattern string `toml:"pattern"`
	Enabled bool   `toml:"enabled"`
}

// DefaultConfig returns a Config populated with sensible default values.
func DefaultConfig() Config {
	return Config{
		AutoLoadPreviousState: true,
		Selector: struct {
			LastPath string `toml:"last_path"`
		}{
			LastPath: "",
		},
		Modlist: struct {
			Mode            int  `toml:"mode"`
			AttachLinks     bool `toml:"attach_links"`
			IncludeSide     bool `toml:"include_side"`
			IncludeSource   bool `toml:"include_source"`
			IncludeVersions bool `toml:"include_versions"`
			IncludeFilename bool `toml:"include_filenames"`
		}{
			Mode:            0, // merged
			AttachLinks:     true,
			IncludeSide:     true,
			IncludeSource:   true,
			IncludeVersions: false,
			IncludeFilename: false,
		},
		Cleaner: struct {
			CustomPresets []CleanerPreset `toml:"custom_presets"`
		}{
			CustomPresets: []CleanerPreset{},
		},
	}
}

// GetConfigPath returns the absolute path to the TOML config file,
// creating the parent directory if it does not yet exist.
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "gomctools")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create config directory: %w", err)
	}

	return filepath.Join(configDir, "config.toml"), nil
}

// Load reads the config file from disk and returns the parsed Config.
// If the file does not exist, DefaultConfig is returned without error.
func Load() (Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Save marshals cfg to TOML and writes it to the config file.
func Save(cfg Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// Reset writes DefaultConfig to disk, discarding any previous settings.
func Reset() error {
	cfg := DefaultConfig()
	return Save(cfg)
}
