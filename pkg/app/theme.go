package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

type themeColors struct {
	labels              string
	text                string
	borders             string
	title               string
	shortcutLabels      string
	shortcutText        string
	statusLabels        string
	statusText          string
	summaryLabels       string
	summaryText         string
	detailLabels        string
	detailText          string
	activeRowText       string
	activeRowBackground string
}

var theme = themeColors{
	labels:              "yellow",
	text:                "white",
	borders:             "white",
	title:               "white",
	shortcutLabels:      "yellow",
	shortcutText:        "white",
	statusLabels:        "yellow",
	statusText:          "white",
	summaryLabels:       "yellow",
	summaryText:         "white",
	detailLabels:        "yellow",
	detailText:          "white",
	activeRowText:       "black",
	activeRowBackground: "white",
}

func (t themeColors) shortcutLabelColor() tcell.Color {
	return tcell.GetColor(t.shortcutLabels)
}

func (t themeColors) shortcutTextColor() tcell.Color {
	return tcell.GetColor(t.shortcutText)
}

func (t themeColors) statusLabelTag() string {
	return "[" + t.statusLabels + "]"
}

func (t themeColors) statusTextColor() tcell.Color {
	return tcell.GetColor(t.statusText)
}

func (t themeColors) summaryLabelTag() string {
	return "[" + t.summaryLabels + "]"
}

func (t themeColors) summaryTextTag() string {
	return "[" + t.summaryText + "]"
}

func (t themeColors) summaryTextColor() tcell.Color {
	return tcell.GetColor(t.summaryText)
}

func (t themeColors) detailLabelTag() string {
	return "[" + t.detailLabels + "]"
}

func (t themeColors) detailTextColor() tcell.Color {
	return tcell.GetColor(t.detailText)
}

func (t themeColors) activeRowTextColor() tcell.Color {
	return tcell.GetColor(t.activeRowText)
}

func (t themeColors) activeRowBackgroundColor() tcell.Color {
	return tcell.GetColor(t.activeRowBackground)
}

func themeColor(value, fallback string) string {
	if tcell.GetColor(value) == tcell.ColorDefault {
		return fallback
	}
	return value
}

func applyTheme(config providers.Theme) {
	labels := themeColor(config.Labels, theme.labels)
	text := themeColor(config.Text, theme.text)
	theme = themeColors{
		labels:              labels,
		text:                text,
		borders:             themeColor(config.Borders, theme.borders),
		title:               themeColor(config.Title, theme.title),
		shortcutLabels:      themeColor(config.ShortcutLabels, labels),
		shortcutText:        themeColor(config.ShortcutText, text),
		statusLabels:        themeColor(config.StatusLabels, theme.statusLabels),
		statusText:          themeColor(config.StatusText, text),
		summaryLabels:       themeColor(config.SummaryLabels, labels),
		summaryText:         themeColor(config.SummaryText, text),
		detailLabels:        themeColor(config.DetailLabels, theme.detailLabels),
		detailText:          themeColor(config.DetailText, text),
		activeRowText:       themeColor(config.ActiveRowText, theme.activeRowText),
		activeRowBackground: themeColor(config.ActiveRowBackground, text),
	}
	if c := tcell.GetColor(theme.labels); c != tcell.ColorDefault {
		tview.Styles.SecondaryTextColor = c
	}
	if c := tcell.GetColor(theme.text); c != tcell.ColorDefault {
		tview.Styles.PrimaryTextColor = c
	}
	if c := tcell.GetColor(theme.borders); c != tcell.ColorDefault {
		tview.Styles.BorderColor = c
	}
	if c := tcell.GetColor(theme.title); c != tcell.ColorDefault {
		tview.Styles.TitleColor = c
	}
}

func defaultThemeConfig() providers.Theme {
	return providers.Theme{
		Labels:              "yellow",
		Text:                "white",
		Borders:             "white",
		Title:               "white",
		ShortcutLabels:      "yellow",
		ShortcutText:        "white",
		StatusLabels:        "yellow",
		StatusText:          "white",
		SummaryLabels:       "yellow",
		SummaryText:         "white",
		DetailLabels:        "yellow",
		DetailText:          "white",
		ActiveRowText:       "black",
		ActiveRowBackground: "white",
	}
}
