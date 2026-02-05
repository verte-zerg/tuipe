// Package config provides configuration helpers and TOML parsing.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// FileConfig represents the TOML configuration file.
type FileConfig struct {
	Practice PracticeConfig `toml:"practice"`
}

// PracticeConfig maps practice-related settings.
type PracticeConfig struct {
	Lang       *string  `toml:"lang"`
	Words      *int     `toml:"words"`
	CapsPct    *float64 `toml:"caps"`
	PunctPct   *float64 `toml:"punct"`
	PunctSet   *string  `toml:"punct-set"`
	FocusWeak  *bool    `toml:"focus-weak"`
	WeakTop    *int     `toml:"weak-top"`
	WeakFactor *float64 `toml:"weak-factor"`
	WeakWindow *int     `toml:"weak-window"`
}

// LoadConfig reads a TOML config from the given path. Missing file is not an error.
func LoadConfig(path string) (FileConfig, error) {
	if path == "" {
		return FileConfig{}, fmt.Errorf("config path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, nil
		}
		return FileConfig{}, fmt.Errorf("failed to stat config: %w", err)
	}
	var cfg FileConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("failed to decode config: %w", err)
	}
	return cfg, nil
}
