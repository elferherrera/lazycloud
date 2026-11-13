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
	ShowShortcuts bool   `yaml:"show_shortcuts"`
	SortOrder     string `yaml:"sort_order"`
}

const (
	// InfobarSortCurrent keeps the built-in order: panel shortcuts first,
	// then globals in infobarGlobalOrder.
	InfobarSortCurrent = "current"
	// InfobarSortAlpha sorts every entry by key label.
	InfobarSortAlpha = "alphabetical"
)

type Config struct {
	Theme      Theme `yaml:"theme"`
	ThemeLight Theme `yaml:"themeLight"`
	// Themes holds every named palette pack; ThemeName selects one.
	// Kept alongside the two legacy sections so old files load and the
	// settings panel can switch packs without a second file.
	Themes     map[string]ThemeSpec `yaml:"themes,omitempty"`
	ThemeName  string               `yaml:"themeName,omitempty"`
	Shortcuts  map[string]string    `yaml:"shortcuts"`
	AutoUpdate AutoUpdate           `yaml:"autoupdate"`
	Infobar    Infobar              `yaml:"infobar"`
}

// ThemeSpec is one named theme pack: a palette for each terminal
// background, so a single themeName works on dark and light terminals.
type ThemeSpec struct {
	Dark  Theme `yaml:"dark"`
	Light Theme `yaml:"light"`
}

// DefaultThemeName is the built-in pack users get on a fresh config.
const DefaultThemeName = "default"

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
		config.ThemeLight = CompleteLightTheme(config.ThemeLight)
		config.Themes = map[string]ThemeSpec{DefaultThemeName: {Dark: config.Theme, Light: config.ThemeLight}}
		config.ThemeName = DefaultThemeName
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
		AutoUpdate *AutoUpdate          `yaml:"autoupdate"`
		Infobar    *Infobar             `yaml:"infobar"`
		ThemeLight *Theme               `yaml:"themeLight"`
		Themes     map[string]ThemeSpec `yaml:"themes"`
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
	if sections.ThemeLight == nil {
		config.ThemeLight = CompleteLightTheme(Theme{})
		rewrote = true
	} else {
		mergedLight := CompleteLightTheme(config.ThemeLight)
		if mergedLight != config.ThemeLight {
			config.ThemeLight = mergedLight
			rewrote = true
		}
	}
	if sections.AutoUpdate == nil {
		config.AutoUpdate = defaultAutoUpdate
		rewrote = true
	}
	if sections.Infobar == nil {
		config.Infobar = defaultInfobar
		rewrote = true
	} else if config.Infobar.SortOrder == "" {
		config.Infobar.SortOrder = defaultInfobar.SortOrder
		rewrote = true
	}
	// Files without a themes map predate multi-theme support; their
	// active palette lives in the two legacy sections. Move it into a
	// pack named DefaultThemeName so the settings panel treats it like
	// any other pack. Files that already carry packs keep them; only the
	// empty keys of each pack are filled in.
	if sections.Themes == nil {
		config.Themes = map[string]ThemeSpec{DefaultThemeName: {Dark: config.Theme, Light: config.ThemeLight}}
		if config.ThemeName == "" {
			config.ThemeName = DefaultThemeName
		}
		rewrote = true
	} else {
		for name, spec := range config.Themes {
			mergedDark := mergeTheme(spec.Dark, defaultTheme)
			mergedLight := CompleteLightTheme(spec.Light)
			if mergedDark != spec.Dark || mergedLight != spec.Light {
				rewrote = true
			}
			spec.Dark = mergedDark
			spec.Light = mergedLight
			config.Themes[name] = spec
		}
		if config.ThemeName == "" {
			config.ThemeName = DefaultThemeName
			rewrote = true
		}
	}
	if _, ok := config.Themes[config.ThemeName]; !ok {
		// A missing selection falls back to the default pack rather than
		// painting with empty colors.
		config.ThemeName = DefaultThemeName
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
