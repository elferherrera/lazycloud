package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AutoUpdate struct {
	Enabled            bool `yaml:"enabled"`
	ThresholdInMinutes int  `yaml:"threshold_in_minutes"`
}

type Infobar struct {
	ShowShortcuts bool `yaml:"show_shortcuts"`
}

type Config struct {
	Theme      Theme             `yaml:"theme"`
	Shortcuts  map[string]string `yaml:"shortcuts"`
	AutoUpdate AutoUpdate        `yaml:"autoupdate"`
	Infobar    Infobar           `yaml:"infobar"`
}

func ConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "config.yaml"), nil
}

func SaveConfig(config Config) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadConfig(defaultTheme Theme, defaultShortcuts map[string]string, defaultAutoUpdate AutoUpdate, defaultInfobar Infobar) (Config, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, err
		}
		config, err := legacyConfig(defaultTheme, defaultShortcuts, defaultAutoUpdate, defaultInfobar)
		if err != nil {
			return Config{}, err
		}
		config.Theme = mergeTheme(config.Theme, defaultTheme)
		if err := SaveConfig(config); err != nil {
			return Config{}, err
		}
		return config, nil
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	var sections struct {
		AutoUpdate *AutoUpdate `yaml:"autoupdate"`
		Infobar    *Infobar    `yaml:"infobar"`
	}
	if err := yaml.Unmarshal(data, &sections); err != nil {
		return Config{}, err
	}
	rewrote := false
	merged := mergeTheme(config.Theme, defaultTheme)
	if merged != config.Theme {
		config.Theme = merged
		rewrote = true
	}
	if sections.AutoUpdate == nil {
		config.AutoUpdate = defaultAutoUpdate
		rewrote = true
	}
	if sections.Infobar == nil {
		config.Infobar = defaultInfobar
		rewrote = true
	}
	if rewrote {
		_ = SaveConfig(config)
	}
	return config, nil
}

func legacyConfig(defaultTheme Theme, defaultShortcuts map[string]string, defaultAutoUpdate AutoUpdate, defaultInfobar Infobar) (Config, error) {
	config := Config{Theme: defaultTheme, Shortcuts: defaultShortcuts, AutoUpdate: defaultAutoUpdate, Infobar: defaultInfobar}
	themePath, err := themePath()
	if err != nil {
		return Config{}, err
	}
	if data, err := os.ReadFile(themePath); err == nil {
		var theme Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			return Config{}, fmt.Errorf("reading legacy theme file: %w", err)
		}
		config.Theme = theme
	} else if !os.IsNotExist(err) {
		return Config{}, err
	}
	shortcutsPath, err := shortcutsPath()
	if err != nil {
		return Config{}, err
	}
	if data, err := os.ReadFile(shortcutsPath); err == nil {
		var shortcuts map[string]string
		if err := json.Unmarshal(data, &shortcuts); err != nil {
			return Config{}, fmt.Errorf("reading legacy shortcuts file: %w", err)
		}
		config.Shortcuts = shortcuts
	} else if !os.IsNotExist(err) {
		return Config{}, err
	}
	return config, nil
}
