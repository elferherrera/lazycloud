package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Theme struct {
	Labels              string `json:"labels"`
	Text                string `json:"text"`
	Borders             string `json:"borders"`
	Title               string `json:"title"`
	ShortcutLabels      string `json:"shortcutLabels"`
	ShortcutText        string `json:"shortcutText"`
	StatusLabels        string `json:"statusLabels"`
	StatusText          string `json:"statusText"`
	SummaryLabels       string `json:"summaryLabels"`
	SummaryText         string `json:"summaryText"`
	DetailLabels        string `json:"detailLabels"`
	DetailText          string `json:"detailText"`
	ActiveRowText       string `json:"activeRowText"`
	ActiveRowBackground string `json:"activeRowBackground"`
}

func themePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "theme.json"), nil
}

func SaveTheme(theme Theme) error {
	path, err := themePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func LoadTheme(defaults Theme) (Theme, error) {
	path, err := themePath()
	if err != nil {
		return Theme{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := SaveTheme(defaults); err != nil {
				return Theme{}, err
			}
			return defaults, nil
		}
		return Theme{}, err
	}
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return Theme{}, err
	}
	merged := mergeTheme(theme, defaults)
	if merged != theme {
		_ = SaveTheme(merged)
	}
	return merged, nil
}

func mergeTheme(theme, defaults Theme) Theme {
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
