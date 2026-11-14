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
	// Theme mirrors the selected pack's palette so hand edits of the
	// file keep working; the settings panel writes both this section
	// and the pack.
	Theme Theme `yaml:"theme"`
	// Themes holds every named palette pack; ThemeName selects one.
	// Each pack is a single palette.
	Themes    map[string]Theme `yaml:"themes,omitempty"`
	ThemeName string           `yaml:"themeName,omitempty"`
	// FollowTerminal hides the theme and color rows and paints the UI
	// with tcell palette colors and terminal defaults, so the user's
	// terminal palette governs every color.
	FollowTerminal bool `yaml:"followTerminal,omitempty"`
	// BundledThemes records that the shipped palette packs were seeded
	// into the themes map, so the seeding runs once per config file.
	BundledThemes bool `yaml:"bundledThemes,omitempty"`
	// BundledThemeSet names the bundle generation the seeding filled. A
	// config written by an older lazycloud carries bundles from an
	// earlier set under the same flag; the generation key lets the
	// seeding re-run once for the new set while deletions keep
	// persisting after it.
	BundledThemeSet string            `yaml:"bundledThemeSet,omitempty"`
	Shortcuts       map[string]string `yaml:"shortcuts"`
	AutoUpdate      AutoUpdate        `yaml:"autoupdate"`
	Infobar         Infobar           `yaml:"infobar"`
}

// DefaultThemeName is the built-in pack users get on a fresh config.
const DefaultThemeName = "lazycloud_dark"

// bundledThemeSetKey is the current bundle generation. Bump it when the
// bundle set changes shape (renames, new packs, new palettes) so every
// existing config seeds the new set once.
const bundledThemeSetKey = "single-palette-1"

// legacyDefaultPackName is the house pack name from before the redesign.
// Its pristine palette is byte-identical to lazycloud_dark, so the
// migration renames it instead of leaving a duplicate in the cycle list.
const legacyDefaultPackName = "default"

// legacySection is a config theme section from before the mode key was
// dropped: the palette keys inline, plus the mode that used to select
// between a dark and a light section.
type legacySection struct {
	Theme `yaml:",inline"`
	Mode  string `yaml:"mode"`
}

// themeEntry is one pack as it appears in a config file, in either shape:
// a flat palette (keys read straight into the embedded Theme) or the old
// dark/light pair that the migration collapses.
type themeEntry struct {
	Theme `yaml:",inline"`
	Dark  *legacySection `yaml:"dark"`
	Light *legacySection `yaml:"light"`
}

// collapse folds a pack entry into the single palette it was painting
// with: an old two-section pack keeps the section its mode selected (the
// dark section unless mode was light), a flat pack fills its empty keys
// from defaults. An old entry with no section at all reports ok=false so
// the caller drops it.
// A pack with mode: light but no light section keeps its dark palette:
// that is exactly what the old renderer painted (mode selected between
// sections, and a missing section fell back to the dark one), so the
// migration preserves the on-screen result instead of guessing a light
// palette the user never saw.
func (e themeEntry) collapse(defaults Theme) (Theme, bool) {
	switch {
	case e.Dark != nil && e.Light != nil:
		if e.Dark.Mode == "light" {
			return MergeTheme(e.Light.Theme, defaults), true
		}
		return MergeTheme(e.Dark.Theme, defaults), true
	case e.Dark != nil:
		return MergeTheme(e.Dark.Theme, defaults), true
	case e.Light != nil:
		return MergeTheme(e.Light.Theme, defaults), true
	default:
		if e.Theme == (Theme{}) {
			return Theme{}, false
		}
		return MergeTheme(e.Theme, defaults), true
	}
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

// LoadConfig reads the config file, filling defaults and migrating older
// schemas in place: two-section packs collapse to the palette their mode
// selected, theme.mode "system" becomes followTerminal, and the bundled
// packs seed once per file.
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
		config.Theme = MergeTheme(config.Theme, defaultTheme)
		config.Themes = map[string]Theme{DefaultThemeName: config.Theme}
		config.ThemeName = DefaultThemeName
		seedBundledThemes(&config, defaultTheme)
		config.BundledThemeSet = bundledThemeSetKey
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
		AutoUpdate *AutoUpdate           `yaml:"autoupdate"`
		Infobar    *Infobar              `yaml:"infobar"`
		Theme      *legacySection        `yaml:"theme"`
		ThemeLight *legacySection        `yaml:"themeLight"`
		Follow     *bool                 `yaml:"followTerminal"`
		Entries    map[string]themeEntry `yaml:"themes"`
	}
	if err := yaml.Unmarshal(data, &sections); err != nil {
		return Config{}, err
	}
	rewrote := false
	if sections.Theme != nil {
		merged := MergeTheme(config.Theme, defaultTheme)
		if merged != config.Theme {
			config.Theme = merged
			rewrote = true
		}
	} else {
		config.Theme = defaultTheme
		rewrote = true
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
	// active palette lives in the legacy sections: theme.mode light was
	// painting with themeLight, anything else with theme.
	if sections.Entries == nil {
		pack := config.Theme
		if sections.ThemeLight != nil && sections.Theme != nil && sections.Theme.Mode == "light" {
			pack = MergeTheme(sections.ThemeLight.Theme, defaultTheme)
		}
		config.Themes = map[string]Theme{DefaultThemeName: pack}
		if config.ThemeName == "" {
			config.ThemeName = DefaultThemeName
		}
		rewrote = true
	} else {
		// Collapse every pack to one palette: old two-section packs to
		// the section their mode used, flat packs unchanged beyond the
		// empty-key fill. An entry with nothing to keep is dropped.
		collapsed := make(map[string]Theme, len(sections.Entries))
		for name, entry := range sections.Entries {
			pack, ok := entry.collapse(defaultTheme)
			if !ok {
				rewrote = true
				continue
			}
			collapsed[name] = pack
			if existing, present := config.Themes[name]; !present || existing != pack {
				rewrote = true
			}
		}
		config.Themes = collapsed
		if config.ThemeName == "" {
			config.ThemeName = DefaultThemeName
			rewrote = true
		}
	}
	// theme.mode: system asked the app to follow the terminal background;
	// the followTerminal flag is the single-palette equivalent, so the
	// migration keeps the intent instead of silently pinning a palette.
	if sections.Follow == nil && sections.Theme != nil && sections.Theme.Mode == "system" {
		config.FollowTerminal = true
		rewrote = true
	}
	if _, ok := config.Themes[config.ThemeName]; !ok {
		// A missing selection falls back to the default pack rather than
		// painting with empty colors.
		config.ThemeName = DefaultThemeName
		if _, ok := config.Themes[config.ThemeName]; !ok {
			config.Themes[config.ThemeName] = defaultTheme
		}
		rewrote = true
	}
	// The bundled palettes seed once per bundle generation: user packs
	// and hand-edited bundles keep their keys, deleted bundles stay
	// deleted for the rest of that generation. A config from an older
	// set re-seeds once so the new packs appear, then respects deletions
	// again.
	if !config.BundledThemes || config.BundledThemeSet != bundledThemeSetKey {
		seedBundledThemes(&config, defaultTheme)
		config.BundledThemeSet = bundledThemeSetKey
		rewrote = true
	}
	// The pristine legacy house pack is byte-identical to the new
	// lazycloud_dark; rename it so the cycle list does not carry the
	// same palette twice. An edited pack keeps its name: it is a custom
	// pack now, and the user picked those colors.
	// A user-authored lazycloud_dark (hand-edited file) also keeps the
	// legacy pack alive: rewiring the selection onto it would repaint
	// the UI with colors the user never selected under that name.
	if pack, ok := config.Themes[legacyDefaultPackName]; ok && pack == lazycloudDarkPalette() {
		samePalette, exists := config.Themes[DefaultThemeName]
		if !exists || samePalette == pack {
			config.Themes[DefaultThemeName] = pack
			delete(config.Themes, legacyDefaultPackName)
			if config.ThemeName == legacyDefaultPackName {
				config.ThemeName = DefaultThemeName
			}
			rewrote = true
		}
	}
	// The mirror section follows the selected pack so hand edits of the
	// file stay meaningful.
	if pack, ok := config.Themes[config.ThemeName]; ok {
		mirrored := pack
		if mirrored != config.Theme {
			config.Theme = mirrored
			rewrote = true
		}
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
