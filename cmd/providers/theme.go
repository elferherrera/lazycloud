package providers

import (
	"os"
	"path/filepath"
)

type Theme struct {
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
	SettingsLabels      string `json:"settingsLabels" yaml:"settingsLabels"`
	SettingsText        string `json:"settingsText" yaml:"settingsText"`
	ActiveRowText       string `json:"activeRowText" yaml:"activeRowText"`
	ActiveRowBackground string `json:"activeRowBackground" yaml:"activeRowBackground"`
}

// FollowTerminalTheme is the palette used when followTerminal hides the
// theme and color rows: every value is a tcell palette name or "default",
// so the terminal resolves each color from its own user-configured palette
// the way lazygit and htop defer to the terminal. "default" leaves the
// slot to the terminal's default foreground/background, so the app sits on
// whatever base colors the terminal ships. Named accents (yellow, cyan,
// green) read well over both dark and light default backgrounds.
func FollowTerminalTheme() Theme {
	return Theme{
		Background:          "default",
		Labels:              "yellow",
		Text:                "default",
		Borders:             "default",
		Title:               "cyan",
		ShortcutLabels:      "yellow",
		ShortcutText:        "default",
		Focus:               "green",
		StatusLabels:        "yellow",
		StatusText:          "default",
		SummaryLabels:       "yellow",
		SummaryText:         "default",
		DetailLabels:        "cyan",
		DetailText:          "default",
		SettingsLabels:      "yellow",
		SettingsText:        "default",
		ActiveRowText:       "black",
		ActiveRowBackground: "cyan",
	}
}

// themePath is the pre-config-file theme.json location; LoadConfig still
// imports it on a fresh config.yaml so an old install keeps its palette.
func themePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "theme.json"), nil
}

// mergeTheme fills the empty keys of theme from defaults. Deriving roles
// with real fallback chains (shortcutLabels falls back to labels) rather
// than the same defaults table keeps a partially-specified pack coherent.
func mergeTheme(theme, defaults Theme) Theme {
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
	if theme.SettingsLabels == "" {
		theme.SettingsLabels = defaults.SettingsLabels
	}
	if theme.SettingsText == "" {
		theme.SettingsText = theme.Text
	}
	if theme.ActiveRowText == "" {
		theme.ActiveRowText = defaults.ActiveRowText
	}
	if theme.ActiveRowBackground == "" {
		theme.ActiveRowBackground = theme.Text
	}
	return theme
}
