package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

type themeColors struct {
	background          string
	labels              string
	text                string
	borders             string
	title               string
	shortcutLabels      string
	shortcutText        string
	focus               string
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
	background:         "black",
	labels:              "yellow",
	text:                "white",
	borders:             "white",
	title:               "white",
	shortcutLabels:      "yellow",
	shortcutText:        "white",
	focus:               "yellow",
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

func (t themeColors) borderColor() tcell.Color {
	return tcell.GetColor(t.borders)
}

// focusBorderColor marks the focused section while the search panel is open.
// Same hue family as labels so the highlight reads as this app's accent, not a
// new color.
func (t themeColors) focusBorderColor() tcell.Color {
	if c := tcell.GetColor(t.focus); c != tcell.ColorDefault {
		return c
	}
	return t.borderColor()
}

func (t themeColors) titleColor() tcell.Color {
	return tcell.GetColor(t.title)
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

// detectBackground is the seam tests use to pin the terminal background
// instead of querying the real terminal.
var detectBackground = providers.DetectBackground

// resolveTheme returns the palette the UI should paint with. Explicit dark
// mode and a dark terminal under system mode keep the configured palette, so
// existing color customizations survive. Only an explicit light mode or a
// detected light terminal background switches to the built-in light palette.
func resolveTheme(config providers.Theme, background providers.BackgroundKind) providers.Theme {
	switch providers.ParseThemeMode(config.Mode) {
	case providers.ModeLight:
		if palette, ok := palettes[providers.BackgroundLight]; ok {
			return palette
		}
		return config
	case providers.ModeDark:
		return config
	default:
		if palette, ok := palettes[background]; ok {
			return palette
		}
		return config
	}
}

// palettes holds the built-in palettes keyed by terminal background kind.
// A new palette is a data addition here, nothing else to touch.
var palettes = map[providers.BackgroundKind]providers.Theme{
	providers.BackgroundLight: {
		Background:         "white",
		Labels:              "olive",
		Text:                "black",
		Borders:             "black",
		Title:               "black",
		ShortcutLabels:      "olive",
		ShortcutText:        "black",
		StatusLabels:        "olive",
		StatusText:          "black",
		SummaryLabels:       "olive",
		SummaryText:         "black",
		DetailLabels:        "olive",
		DetailText:          "black",
		ActiveRowText:       "white",
		ActiveRowBackground: "black",
	},
}

// applyConfigTheme resolves the configured theme mode against the current
// terminal background and applies the resulting palette.
func applyConfigTheme(config providers.Theme) {
	applyTheme(resolveTheme(config, detectBackground()))
}

func applyTheme(config providers.Theme) {
	background := themeColor(config.Background, theme.background)
	labels := themeColor(config.Labels, theme.labels)
	text := themeColor(config.Text, theme.text)
	shortcutLabels := themeColor(config.ShortcutLabels, labels)
	shortcutText := themeColor(config.ShortcutText, text)
	theme = themeColors{
		background:         background,
		labels:              labels,
		text:                text,
		borders:             themeColor(config.Borders, theme.borders),
		title:               themeColor(config.Title, theme.title),
		shortcutLabels:      shortcutLabels,
		shortcutText:        shortcutText,
		focus:               themeColor(config.Focus, shortcutLabels),
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
	if c := tcell.GetColor(theme.background); c != tcell.ColorDefault {
		tview.Styles.PrimitiveBackgroundColor = c
	}
}

func defaultThemeConfig() providers.Theme {
	return providers.Theme{
		Mode:                string(providers.ModeDark),
		Background:          "black",
		Labels:              "yellow",
		Text:                "white",
		Borders:             "white",
		Title:               "white",
		ShortcutLabels:      "yellow",
		ShortcutText:        "white",
		Focus:               "yellow",
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
