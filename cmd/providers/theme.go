package providers

import (
	"os"
	"path/filepath"
)

type Theme struct {
	Mode                string `json:"mode" yaml:"mode"`
	Background          string `json:"background" yaml:"background"`
	Labels              string `json:"labels" yaml:"labels"`
	Text                string `json:"text" yaml:"text"`
	Borders             string `json:"borders" yaml:"borders"`
	Title               string `json:"title" yaml:"title"`
	ShortcutLabels      string `json:"shortcutLabels" yaml:"shortcutLabels"`
	ShortcutText        string `json:"shortcutText" yaml:"shortcutText"`
	Focus               string `json:"focus" yaml:"focus"`
	StatusLabels        string `json:"statusLabels" yaml:"statusLabels"`
	StatusText          string `json:"statusText" yaml:"statusText"`
	SummaryLabels       string `json:"summaryLabels" yaml:"summaryLabels"`
	SummaryText         string `json:"summaryText" yaml:"summaryText"`
	DetailLabels        string `json:"detailLabels" yaml:"detailLabels"`
	DetailText          string `json:"detailText" yaml:"detailText"`
	ActiveRowText       string `json:"activeRowText" yaml:"activeRowText"`
	ActiveRowBackground string `json:"activeRowBackground" yaml:"activeRowBackground"`
}

// ThemeMode selects which palette lazycloud paints the UI with.
type ThemeMode string

const (
	// ModeSystem resolves the palette from the terminal background.
	ModeSystem ThemeMode = "system"
	// ModeLight always uses the light palette.
	ModeLight ThemeMode = "light"
	// ModeDark always uses the dark palette.
	ModeDark ThemeMode = "dark"
)

// ParseThemeMode maps a config value onto a ThemeMode. Unknown or empty
// values resolve to dark, the safe default for a TUI on an unknown terminal.
func ParseThemeMode(value string) ThemeMode {
	switch ThemeMode(value) {
	case ModeLight:
		return ModeLight
	case ModeSystem:
		return ModeSystem
	default:
		return ModeDark
	}
}

// DefaultLightTheme is the built-in light palette. It is also the set of
// defaults a partial themeLight config section merges against, so an empty
// key falls back to the built-in value instead of the dark palette.
// Labels use a dark bronze hex rather than the olive name because terminals
// render olive too light for the white background (4.2:1 measured, below
// the 4.5:1 AA floor; the bronze reads 7.1:1 on the same background).
func DefaultLightTheme() Theme {
	return Theme{
		Mode:                string(ModeLight),
		Background:          "white",
		Labels:              "#705400",
		Text:                "black",
		Borders:             "black",
		Title:               "black",
		ShortcutLabels:      "#705400",
		ShortcutText:        "black",
		Focus:               "#705400",
		StatusLabels:        "#705400",
		StatusText:          "black",
		SummaryLabels:       "#705400",
		SummaryText:         "black",
		DetailLabels:        "#705400",
		DetailText:          "black",
		ActiveRowText:       "white",
		ActiveRowBackground: "black",
	}
}

// CompleteLightTheme fills the empty keys of a themeLight section from the
// built-in light palette.
func CompleteLightTheme(theme Theme) Theme {
	return mergeTheme(theme, DefaultLightTheme())
}

func themePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "theme.json"), nil
}

func mergeTheme(theme, defaults Theme) Theme {
	if theme.Mode == "" {
		theme.Mode = defaults.Mode
	}
	if theme.Background == "" {
		theme.Background = defaults.Background
	}
	if theme.Labels == "" {
		theme.Labels = defaults.Labels
	}
	if theme.Text == "" {
		theme.Text = defaults.Text
	}
	if theme.Borders == "" {
		theme.Borders = defaults.Borders
	}
	if theme.Title == "" {
		theme.Title = defaults.Title
	}
	if theme.ShortcutLabels == "" {
		theme.ShortcutLabels = theme.Labels
	}
	if theme.ShortcutText == "" {
		theme.ShortcutText = theme.Text
	}
	if theme.Focus == "" {
		theme.Focus = theme.ShortcutLabels
	}
	if theme.StatusLabels == "" {
		theme.StatusLabels = defaults.StatusLabels
	}
	if theme.StatusText == "" {
		theme.StatusText = theme.Text
	}
	if theme.SummaryLabels == "" {
		theme.SummaryLabels = theme.Labels
	}
	if theme.SummaryText == "" {
		theme.SummaryText = theme.Text
	}
	if theme.DetailLabels == "" {
		theme.DetailLabels = defaults.DetailLabels
	}
	if theme.DetailText == "" {
		theme.DetailText = theme.Text
	}
	if theme.ActiveRowText == "" {
		theme.ActiveRowText = defaults.ActiveRowText
	}
	if theme.ActiveRowBackground == "" {
		theme.ActiveRowBackground = theme.Text
	}
	return theme
}
