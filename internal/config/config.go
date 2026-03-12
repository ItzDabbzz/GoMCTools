package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application's persistent settings.
// Comments are preserved when saving to TOML format.
type Config struct {
	// Global settings
	AutoLoadPreviousState bool   `toml:"auto_load_previous_state"`
	TelemetryEnabled      bool   `toml:"telemetry_enabled"`
	LastPackPath          string `toml:"last_pack_path"`

	// Selector page settings
	Selector struct {
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

// DefaultConfig returns a config with default values.
func DefaultConfig() Config {
	return Config{
		AutoLoadPreviousState: true,
		TelemetryEnabled:      true,
		LastPackPath:          "",
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

// GetConfigPath returns the path to the config file.
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

// Load loads the config from disk.
// Returns DefaultConfig if the file doesn't exist.
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

// Save writes the config to disk.
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

// Reset resets the config to defaults and saves it.
func Reset() error {
	cfg := DefaultConfig()
	return Save(cfg)
}
