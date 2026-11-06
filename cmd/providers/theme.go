package providers

import (
	"os"
	"path/filepath"
)

type Theme struct {
	Labels              string `json:"labels" yaml:"labels"`
	Text                string `json:"text" yaml:"text"`
	Borders             string `json:"borders" yaml:"borders"`
	Title               string `json:"title" yaml:"title"`
	ShortcutLabels      string `json:"shortcutLabels" yaml:"shortcutLabels"`
	ShortcutText        string `json:"shortcutText" yaml:"shortcutText"`
	StatusLabels        string `json:"statusLabels" yaml:"statusLabels"`
	StatusText          string `json:"statusText" yaml:"statusText"`
	SummaryLabels       string `json:"summaryLabels" yaml:"summaryLabels"`
	SummaryText         string `json:"summaryText" yaml:"summaryText"`
	DetailLabels        string `json:"detailLabels" yaml:"detailLabels"`
	DetailText          string `json:"detailText" yaml:"detailText"`
	ActiveRowText       string `json:"activeRowText" yaml:"activeRowText"`
	ActiveRowBackground string `json:"activeRowBackground" yaml:"activeRowBackground"`
}

func themePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "theme.json"), nil
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
