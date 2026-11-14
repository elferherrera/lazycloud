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
	settingsLabels      string
	settingsText        string
	activeRowText       string
	activeRowBackground string
}

var theme = themeColors{
	background:          "black",
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
	settingsLabels:      "yellow",
	settingsText:        "white",
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

// focusColor drives every focused-control affordance in the settings form:
// checkbox box, label marker, button activation, and the row caret.
func (t themeColors) focusColor() tcell.Color {
	if c := tcell.GetColor(t.focus); c != tcell.ColorDefault {
		return c
	}
	return tcell.GetColor(t.labels)
}

// focusStyle styles the focused checkbox: accent box background with
// readable black-or-white check text, matching how color swatches pick
// their text color.
func (t themeColors) focusStyle() tcell.Style {
	return tcell.StyleDefault.
		Background(t.focusColor()).
		Foreground(readableOnColor(t.focusColor()))
}

// readableOnColor returns the text color that stays legible on a given
// background: black on light, white otherwise.
func readableOnColor(color tcell.Color) tcell.Color {
	if color == tcell.ColorDefault || isLightColor(color) {
		return tcell.ColorBlack
	}
	return tcell.ColorWhite
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

// settingsLabelColor paints the settings panel labels: the form's row
// names, the checkbox labels and the key hint under the form.
func (t themeColors) settingsLabelColor() tcell.Color {
	return tcell.GetColor(t.settingsLabels)
}

// settingsTextColor paints the settings panel values: color field text,
// the config path row and the button labels.
func (t themeColors) settingsTextColor() tcell.Color {
	return tcell.GetColor(t.settingsText)
}

func (t themeColors) activeRowTextColor() tcell.Color {
	return tcell.GetColor(t.activeRowText)
}

func (t themeColors) activeRowBackgroundColor() tcell.Color {
	return tcell.GetColor(t.activeRowBackground)
}

// themeColor resolves a config value to the color string the UI paints
// with: "default" defers the slot to the terminal's own color, any tcell
// name or hex pins it, and an unparseable value falls back to the built-in
// palette so a typo never blanks a slot.
func themeColor(value, fallback string) string {
	switch {
	case value == "":
		return fallback
	case value == "default":
		return "default"
	case tcell.GetColor(value) == tcell.ColorDefault:
		return fallback
	default:
		return value
	}
}

// applyActiveTheme applies the palette the current settings select: the
// terminal-follow palette when the Follow terminal toggle is on (every
// slot defers to the user's terminal colors), the selected pack otherwise.
// applyActiveThemeFn is the seam tests use to observe the call.
var applyActiveThemeFn = applyActiveTheme

func applyActiveTheme() {
	if configuredFollowTerminal {
		applyTheme(providers.FollowTerminalTheme())
		return
	}
	applyTheme(configuredTheme)
}

// applyThemeChrome pushes the theme's box colors into the widgets that
// were built before this palette existed. tview's Box-backed primitives
// copy Styles.PrimitiveBackgroundColor and Styles.BorderColor at
// construction and never read the globals again, so a pack switch that
// only mutates tview.Styles leaves every widget built earlier painted
// with the startup palette: the settings overlay kept its old backdrop,
// border and title after a switch. Repaint the box chrome of every
// widget the app keeps alive; things created later (info table cells,
// detail overlays) read the already-updated globals or the live theme
// var at construction. Text, fields and cells are handled by
// applyThemeToWidgets, which runs next to this in both callers.
func (a *app) applyThemeChrome() {
	bg := tcell.GetColor(theme.background)
	border := theme.borderColor()
	// All of these embed *tview.Box, which is reachable from outside, so
	// one pass repaints background and border for the whole live tree:
	// the Pages backdrop, every layout flex and every panel.
	boxes := make([]*tview.Box, 0, 20)
	if a.pages != nil {
		boxes = append(boxes, a.pages.Box)
	}
	if a.rootView != nil {
		boxes = append(boxes, a.rootView.Box)
	}
	if a.mainView != nil {
		boxes = append(boxes, a.mainView.Box)
	}
	if a.searchColumn != nil {
		boxes = append(boxes, a.searchColumn.Box)
	}
	if a.searchPanel != nil {
		boxes = append(boxes, a.searchPanel.Box)
	}
	if a.searchInput != nil {
		boxes = append(boxes, a.searchInput.Box)
	}
	if a.searchResults != nil {
		boxes = append(boxes, a.searchResults.Box)
	}
	if a.info != nil {
		boxes = append(boxes, a.info.Box)
	}
	if a.summary != nil {
		boxes = append(boxes, a.summary.Box)
	}
	if a.status != nil {
		boxes = append(boxes, a.status.Box)
	}
	if a.infobar != nil {
		boxes = append(boxes, a.infobar.Box)
	}
	if a.cmd != nil {
		boxes = append(boxes, a.cmd.Box)
	}
	if a.shortcutsSummary != nil {
		boxes = append(boxes, a.shortcutsSummary.Box)
	}
	if a.helpText != nil {
		boxes = append(boxes, a.helpText.Box)
	}
	if a.helpRow != nil {
		boxes = append(boxes, a.helpRow.Box)
	}
	if a.helpColumn != nil {
		boxes = append(boxes, a.helpColumn.Box)
	}
	if a.settingsForm != nil {
		boxes = append(boxes, a.settingsForm.Box)
	}
	if a.settingsHint != nil {
		boxes = append(boxes, a.settingsHint.Box)
	}
	if a.settingsRow != nil {
		boxes = append(boxes, a.settingsRow.Box)
	}
	if a.settingsColumn != nil {
		boxes = append(boxes, a.settingsColumn.Box)
	}
	for _, b := range boxes {
		b.SetBackgroundColor(bg)
		b.SetBorderColor(border)
	}
	// Chrome beyond box colors: titles and the selected row follow the
	// theme var, and the widgets cannot pick that up on their own.
	if a.info != nil {
		a.info.SetTitleColor(theme.titleColor())
		a.info.SetSelectedStyle(tcell.StyleDefault.
			Foreground(theme.activeRowTextColor()).
			Background(theme.activeRowBackgroundColor()))
	}
	if a.searchResults != nil {
		a.searchResults.SetTitleColor(theme.titleColor())
	}
	if a.helpText != nil {
		a.helpText.SetTitleColor(theme.titleColor())
	}
	if a.settingsForm != nil {
		a.settingsForm.SetTitleColor(theme.titleColor())
		// Form.Draw restyles every item from these form-level colors on each
		// draw, so the preview path must refresh them or the open panel keeps
		// painting with the palette it was built with.
		a.settingsForm.SetLabelColor(theme.settingsLabelColor())
		a.settingsForm.SetFieldTextColor(theme.settingsTextColor())
		a.settingsForm.SetButtonTextColor(theme.settingsTextColor())
	}
	if a.settingsHint != nil {
		a.settingsHint.SetTextColor(theme.settingsLabelColor())
	}
}

func applyTheme(config providers.Theme) {
	// Every fallback chain ends in the shipped default palette, never in
	// the pack that painted before it: a pack switch must not leak the
	// old pack's accents, and an invalid hand-edited value must resolve
	// to a real color instead of poisoning the chain below it. Related
	// slots still fall back to each other (shortcutLabels to labels,
	// statusText to text) so a sparse pack stays coherent.
	def := defaultThemeConfig()
	background := themeColor(config.Background, def.Background)
	labels := themeColor(config.Labels, def.Labels)
	text := themeColor(config.Text, def.Text)
	shortcutLabels := themeColor(config.ShortcutLabels, labels)
	shortcutText := themeColor(config.ShortcutText, text)
	theme = themeColors{
		background:          background,
		labels:              labels,
		text:                text,
		borders:             themeColor(config.Borders, def.Borders),
		title:               themeColor(config.Title, def.Title),
		shortcutLabels:      shortcutLabels,
		shortcutText:        shortcutText,
		focus:               themeColor(config.Focus, shortcutLabels),
		statusLabels:        themeColor(config.StatusLabels, def.StatusLabels),
		statusText:          themeColor(config.StatusText, text),
		summaryLabels:       themeColor(config.SummaryLabels, labels),
		summaryText:         themeColor(config.SummaryText, text),
		detailLabels:        themeColor(config.DetailLabels, def.DetailLabels),
		detailText:          themeColor(config.DetailText, text),
		settingsLabels:      themeColor(config.SettingsLabels, def.SettingsLabels),
		settingsText:        themeColor(config.SettingsText, text),
		activeRowText:       themeColor(config.ActiveRowText, def.ActiveRowText),
		activeRowBackground: themeColor(config.ActiveRowBackground, text),
	}
	// These styles are set unconditionally, "default" included: ColorDefault
	// is what makes the terminal-follow palette defer to the terminal's own
	// foreground and background, so a stale style from the previous pack
	// must not survive a switch.
	tview.Styles.SecondaryTextColor = tcell.GetColor(theme.labels)
	tview.Styles.PrimaryTextColor = tcell.GetColor(theme.text)
	tview.Styles.BorderColor = tcell.GetColor(theme.borders)
	tview.Styles.TitleColor = tcell.GetColor(theme.title)
	tview.Styles.PrimitiveBackgroundColor = tcell.GetColor(theme.background)
}

// defaultThemeConfig is the palette handed to LoadConfig as the fill
// default; the lazycloud_dark bundle is the single source of that palette.
func defaultThemeConfig() providers.Theme {
	return providers.BundledThemes()[providers.DefaultThemeName]
}
