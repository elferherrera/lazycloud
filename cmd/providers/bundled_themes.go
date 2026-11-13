package providers

import "strconv"

// IsBundledThemeName reports whether a pack name belongs to the seeded
// bundle set. Color edits fork these packs into a custom name instead of
// overwriting the seed, so a bundled palette can always be picked again in
// its original form.
func IsBundledThemeName(name string) bool {
	_, ok := BundledThemeSpecs()[name]
	return ok
}

// CustomThemeForkName returns the name a bundled pack takes when the user
// edits its colors: the seed name plus -custom, with a numeric suffix when
// that name is already taken (monokai-custom, monokai-custom-2, ...). The
// suffix never collides with an existing pack, including other forks.
func CustomThemeForkName(seed string, existing map[string]bool) string {
	base := seed + "-custom"
	if !existing[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if !existing[candidate] {
			return candidate
		}
	}
}

// BundledThemeSpecs returns the palette packs shipped with lazycloud, keyed
// by the name the settings panel cycles. Dark sections carry each palette's
// canonical scheme (monokai, dracula, nord, gruvbox, solarized, tokyo-night
// and one-dark). Gruvbox, solarized and one-dark also ship their canonical
// light variants (gruvbox light, solarized light, one light); the rest
// leave the light section empty so the built-in light palette fills it on
// load. Every dark section pins mode dark so picking a pack paints its own
// colors; the mode rows in the settings panel can still switch it.
func BundledThemeSpecs() map[string]ThemeSpec {
	return map[string]ThemeSpec{
		"monokai": {Dark: Theme{
			Mode:                string(ModeDark),
			Background:          "#272822",
			Labels:              "#a6e22e",
			Text:                "#f8f8f2",
			Borders:             "#75715e",
			Title:               "#fd971f",
			ShortcutLabels:      "#a6e22e",
			ShortcutText:        "#f8f8f2",
			Focus:               "#fd971f",
			StatusLabels:        "#66d9ef",
			StatusText:          "#f8f8f2",
			SummaryLabels:       "#a6e22e",
			SummaryText:         "#f8f8f2",
			DetailLabels:        "#66d9ef",
			DetailText:          "#f8f8f2",
			ActiveRowText:       "#272822",
			ActiveRowBackground: "#fd971f",
			SettingsLabels:      "#a6e22e",
			SettingsText:        "#f8f8f2",
		}},
		"dracula": {Dark: Theme{
			Mode:                string(ModeDark),
			Background:          "#282a36",
			Labels:              "#ff79c6",
			Text:                "#f8f8f2",
			Borders:             "#44475a",
			Title:               "#bd93f9",
			ShortcutLabels:      "#ff79c6",
			ShortcutText:        "#f8f8f2",
			Focus:               "#bd93f9",
			StatusLabels:        "#bd93f9",
			StatusText:          "#f8f8f2",
			SummaryLabels:       "#ff79c6",
			SummaryText:         "#f8f8f2",
			DetailLabels:        "#8be9fd",
			DetailText:          "#f8f8f2",
			ActiveRowText:       "#282a36",
			ActiveRowBackground: "#ff79c6",
			SettingsLabels:      "#ff79c6",
			SettingsText:        "#f8f8f2",
		}},
		"nord": {Dark: Theme{
			Mode:                string(ModeDark),
			Background:          "#2e3440",
			Labels:              "#88c0d0",
			Text:                "#eceff4",
			Borders:             "#4c566a",
			Title:               "#81a1c1",
			ShortcutLabels:      "#88c0d0",
			ShortcutText:        "#eceff4",
			Focus:               "#81a1c1",
			StatusLabels:        "#81a1c1",
			StatusText:          "#eceff4",
			SummaryLabels:       "#88c0d0",
			SummaryText:         "#eceff4",
			DetailLabels:        "#8fbcbb",
			DetailText:          "#eceff4",
			ActiveRowText:       "#2e3440",
			ActiveRowBackground: "#88c0d0",
			SettingsLabels:      "#88c0d0",
			SettingsText:        "#eceff4",
		}},
		"gruvbox": {
			Dark: Theme{
				Mode:                string(ModeDark),
				Background:          "#282828",
				Labels:              "#fabd2f",
				Text:                "#ebdbb2",
				Borders:             "#3c3836",
				Title:               "#fe8019",
				ShortcutLabels:      "#fabd2f",
				ShortcutText:        "#ebdbb2",
				Focus:               "#fe8019",
				StatusLabels:        "#8ec07c",
				StatusText:          "#ebdbb2",
				SummaryLabels:       "#fabd2f",
				SummaryText:         "#ebdbb2",
				DetailLabels:        "#83a598",
				DetailText:          "#ebdbb2",
				ActiveRowText:       "#282828",
				ActiveRowBackground: "#fabd2f",
				SettingsLabels:      "#fabd2f",
				SettingsText:        "#ebdbb2",
			},
			Light: Theme{
				Mode:                string(ModeLight),
				Background:          "#fbf1c7",
				Labels:              "#af3a03",
				Text:                "#3c3836",
				Borders:             "#928374",
				Title:               "#9d0006",
				ShortcutLabels:      "#af3a03",
				ShortcutText:        "#3c3836",
				Focus:               "#af3a03",
				StatusLabels:        "#076678",
				StatusText:          "#3c3836",
				SummaryLabels:       "#af3a03",
				SummaryText:         "#3c3836",
				DetailLabels:        "#8f3f71",
				DetailText:          "#3c3836",
				ActiveRowText:       "#fbf1c7",
				ActiveRowBackground: "#3c3836",
				SettingsLabels:      "#af3a03",
				SettingsText:        "#3c3836",
			},
		},
		"solarized": {
			Dark: Theme{
				Mode:                string(ModeDark),
				Background:          "#002b36",
				Labels:              "#b58900",
				Text:                "#839496",
				Borders:             "#586e75",
				Title:               "#2aa198",
				ShortcutLabels:      "#b58900",
				ShortcutText:        "#93a1a1",
				Focus:               "#2aa198",
				StatusLabels:        "#859900",
				StatusText:          "#93a1a1",
				SummaryLabels:       "#b58900",
				SummaryText:         "#93a1a1",
				DetailLabels:        "#2aa198",
				DetailText:          "#93a1a1",
				ActiveRowText:       "#002b36",
				ActiveRowBackground: "#93a1a1",
				SettingsLabels:      "#b58900",
				SettingsText:        "#93a1a1",
			},
			Light: Theme{
				Mode:                string(ModeLight),
				Background:          "#fdf6e3",
				Labels:              "#b58900",
				Text:                "#657b83",
				Borders:             "#93a1a1",
				Title:               "#2aa198",
				ShortcutLabels:      "#b58900",
				ShortcutText:        "#657b83",
				Focus:               "#2aa198",
				StatusLabels:        "#859900",
				StatusText:          "#657b83",
				SummaryLabels:       "#b58900",
				SummaryText:         "#657b83",
				DetailLabels:        "#2aa198",
				DetailText:          "#657b83",
				ActiveRowText:       "#fdf6e3",
				ActiveRowBackground: "#586e75",
				SettingsLabels:      "#b58900",
				SettingsText:        "#657b83",
			},
		},
		"tokyo-night": {Dark: Theme{
			Mode:                string(ModeDark),
			Background:          "#1a1b26",
			Labels:              "#7aa2f7",
			Text:                "#c0caf5",
			Borders:             "#3b4261",
			Title:               "#bb9af7",
			ShortcutLabels:      "#7aa2f7",
			ShortcutText:        "#c0caf5",
			Focus:               "#7aa2f7",
			StatusLabels:        "#bb9af7",
			StatusText:          "#c0caf5",
			SummaryLabels:       "#7aa2f7",
			SummaryText:         "#c0caf5",
			DetailLabels:        "#9ece6a",
			DetailText:          "#c0caf5",
			ActiveRowText:       "#1a1b26",
			ActiveRowBackground: "#7aa2f7",
			SettingsLabels:      "#7aa2f7",
			SettingsText:        "#c0caf5",
		}},
		"one-dark": {
			Dark: Theme{
				Mode:                string(ModeDark),
				Background:          "#282c34",
				Labels:              "#e5c07b",
				Text:                "#abb2bf",
				Borders:             "#3e4451",
				Title:               "#61afef",
				ShortcutLabels:      "#e5c07b",
				ShortcutText:        "#abb2bf",
				Focus:               "#c678dd",
				StatusLabels:        "#61afef",
				StatusText:          "#abb2bf",
				SummaryLabels:       "#e5c07b",
				SummaryText:         "#abb2bf",
				DetailLabels:        "#98c379",
				DetailText:          "#abb2bf",
				ActiveRowText:       "#282c34",
				ActiveRowBackground: "#61afef",
				SettingsLabels:      "#e5c07b",
				SettingsText:        "#abb2bf",
			},
			Light: Theme{
				Mode:                string(ModeLight),
				Background:          "#fafafa",
				Labels:              "#c18401",
				Text:                "#383a42",
				Borders:             "#a0a1a7",
				Title:               "#4078f2",
				ShortcutLabels:      "#c18401",
				ShortcutText:        "#383a42",
				Focus:               "#4078f2",
				StatusLabels:        "#50a14f",
				StatusText:          "#383a42",
				SummaryLabels:       "#c18401",
				SummaryText:         "#383a42",
				DetailLabels:        "#0184bc",
				DetailText:          "#383a42",
				ActiveRowText:       "#fafafa",
				ActiveRowBackground: "#383a42",
				SettingsLabels:      "#c18401",
				SettingsText:        "#383a42",
			},
		},
	}
}

// seedBundledThemes copies the bundled packs into config's themes map and
// records the seeding in the bundledThemes flag. A pack already present
// under the same name keeps its keys: a user-edited or hand-written pack
// always wins over the bundle. The flag is set even when every bundle was
// already there, so the seeding runs once per config and a deleted bundle
// stays deleted. Returns true when config changed.
func seedBundledThemes(config *Config, defaults Theme) bool {
	changed := false
	for name, spec := range BundledThemeSpecs() {
		if _, ok := config.Themes[name]; ok {
			continue
		}
		spec.Dark = mergeTheme(spec.Dark, defaults)
		spec.Light = CompleteLightTheme(spec.Light)
		if config.Themes == nil {
			config.Themes = map[string]ThemeSpec{}
		}
		config.Themes[name] = spec
		changed = true
	}
	config.BundledThemes = true
	return changed
}
