// Package config provides XDG path helpers.
package config

import (
	"os"
	"path/filepath"
)

// XDGConfigHome returns the XDG config home or a default fallback.
func XDGConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return filepath.Join(home, ".config")
}

// XDGDataHome returns the XDG data home or a default fallback.
func XDGDataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return filepath.Join(home, ".local", "share")
}

// DefaultWordListPath builds the default word list path for a language.
func DefaultWordListPath(lang string) string {
	return filepath.Join(XDGConfigHome(), "tuipe", "wordlists", lang+".txt")
}

// DefaultWordListDir returns the default directory for word lists.
func DefaultWordListDir() string {
	return filepath.Join(XDGConfigHome(), "tuipe", "wordlists")
}

// DefaultDBPath returns the default path for the SQLite database.
func DefaultDBPath() string {
	return filepath.Join(XDGDataHome(), "tuipe", "tuipe.db")
}

// DefaultWordfreqCacheDir returns the cache directory for wordfreq wheels.
func DefaultWordfreqCacheDir() string {
	return filepath.Join(XDGDataHome(), "tuipe", "wordfreq")
}

// DefaultConfigPath returns the default TOML config path.
func DefaultConfigPath() string {
	return filepath.Join(XDGConfigHome(), "tuipe", "config.toml")
}
