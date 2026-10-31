package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func TestThemeColor(t *testing.T) {
	if got := themeColor("aqua", "yellow"); got != "aqua" {
		t.Errorf("themeColor(aqua) = %q, want aqua", got)
	}
	if got := themeColor("#ff9900", "yellow"); got != "#ff9900" {
		t.Errorf("themeColor(#ff9900) = %q, want #ff9900", got)
	}
	if got := themeColor("#fff", "yellow"); got != "yellow" {
		t.Errorf("themeColor(#fff) = %q, want yellow fallback (not #rrggbb)", got)
	}
	if got := themeColor("", "yellow"); got != "yellow" {
		t.Errorf("themeColor(empty) = %q, want yellow", got)
	}
	if got := themeColor("not-a-color", "yellow"); got != "yellow" {
		t.Errorf("themeColor(invalid) = %q, want yellow", got)
	}
}

func TestApplyTheme(t *testing.T) {
	secondary, primary, border, title := tview.Styles.SecondaryTextColor, tview.Styles.PrimaryTextColor, tview.Styles.BorderColor, tview.Styles.TitleColor
	original := theme
	t.Cleanup(func() {
		tview.Styles.SecondaryTextColor = secondary
		tview.Styles.PrimaryTextColor = primary
		tview.Styles.BorderColor = border
		tview.Styles.TitleColor = title
		theme = original
	})

	applyTheme(providers.Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange"})
	if theme.labels != "aqua" || theme.text != "gray" || theme.borders != "green" || theme.title != "orange" {
		t.Errorf("applyTheme() = %+v, want custom colors", theme)
	}
	if theme.shortcutLabels != "aqua" || theme.shortcutText != "gray" {
		t.Errorf("applyTheme() shortcut colors = %q %q, want new labels/text fallback", theme.shortcutLabels, theme.shortcutText)
	}
	if theme.statusLabels != "yellow" || theme.statusText != "gray" {
		t.Errorf("applyTheme() status colors = %q %q, want yellow and new text fallback", theme.statusLabels, theme.statusText)
	}
	if theme.summaryLabels != "aqua" || theme.summaryText != "gray" {
		t.Errorf("applyTheme() summary colors = %q %q, want new labels/text fallback", theme.summaryLabels, theme.summaryText)
	}
	if theme.detailLabels != "yellow" || theme.detailText != "gray" {
		t.Errorf("applyTheme() detail colors = %q %q, want yellow and new text fallback", theme.detailLabels, theme.detailText)
	}
	if theme.activeRowText != "black" || theme.activeRowBackground != "gray" {
		t.Errorf("applyTheme() active row colors = %q %q, want black and new text fallback", theme.activeRowText, theme.activeRowBackground)
	}
	if tview.Styles.SecondaryTextColor != tcell.ColorAqua {
		t.Errorf("applyTheme() SecondaryTextColor = %v, want aqua", tview.Styles.SecondaryTextColor)
	}
	if tview.Styles.PrimaryTextColor != tcell.ColorGray {
		t.Errorf("applyTheme() PrimaryTextColor = %v, want gray", tview.Styles.PrimaryTextColor)
	}
	if tview.Styles.BorderColor != tcell.ColorGreen {
		t.Errorf("applyTheme() BorderColor = %v, want green", tview.Styles.BorderColor)
	}
	if tview.Styles.TitleColor != tcell.ColorOrange {
		t.Errorf("applyTheme() TitleColor = %v, want orange", tview.Styles.TitleColor)
	}

	applyTheme(providers.Theme{Labels: "not-a-color"})
	if theme.labels != "aqua" {
		t.Errorf("applyTheme() invalid labels = %q, want aqua fallback", theme.labels)
	}

	applyTheme(providers.Theme{Labels: "#ff9900", Borders: "#00aa55"})
	if theme.labels != "#ff9900" {
		t.Errorf("applyTheme() hex labels = %q, want #ff9900", theme.labels)
	}
	if tview.Styles.SecondaryTextColor != tcell.GetColor("#ff9900") {
		t.Errorf("applyTheme() hex SecondaryTextColor = %v, want %v", tview.Styles.SecondaryTextColor, tcell.GetColor("#ff9900"))
	}
	if tview.Styles.BorderColor != tcell.GetColor("#00aa55") {
		t.Errorf("applyTheme() hex BorderColor = %v, want %v", tview.Styles.BorderColor, tcell.GetColor("#00aa55"))
	}

	applyTheme(providers.Theme{ShortcutLabels: "orange", ShortcutText: "silver"})
	if theme.shortcutLabels != "orange" || theme.shortcutText != "silver" {
		t.Errorf("applyTheme() shortcut colors = %q %q, want orange silver", theme.shortcutLabels, theme.shortcutText)
	}
	if theme.labels != "#ff9900" {
		t.Errorf("applyTheme() labels = %q, want #ff9900 kept", theme.labels)
	}
	if theme.statusLabels != "yellow" {
		t.Errorf("applyTheme() status labels = %q, want yellow kept", theme.statusLabels)
	}

	applyTheme(providers.Theme{StatusLabels: "aqua", StatusText: "silver"})
	if theme.statusLabels != "aqua" || theme.statusText != "silver" {
		t.Errorf("applyTheme() status colors = %q %q, want aqua silver", theme.statusLabels, theme.statusText)
	}
	if theme.labels != "#ff9900" {
		t.Errorf("applyTheme() labels = %q, want #ff9900 kept", theme.labels)
	}
	if theme.shortcutLabels != "#ff9900" {
		t.Errorf("applyTheme() shortcut labels = %q, want labels fallback", theme.shortcutLabels)
	}

	applyTheme(providers.Theme{SummaryLabels: "orange", SummaryText: "silver"})
	if theme.summaryLabels != "orange" || theme.summaryText != "silver" {
		t.Errorf("applyTheme() summary colors = %q %q, want orange silver", theme.summaryLabels, theme.summaryText)
	}
	if theme.labels != "#ff9900" {
		t.Errorf("applyTheme() labels = %q, want #ff9900 kept", theme.labels)
	}

	applyTheme(providers.Theme{DetailLabels: "orange", DetailText: "silver"})
	if theme.detailLabels != "orange" || theme.detailText != "silver" {
		t.Errorf("applyTheme() detail colors = %q %q, want orange silver", theme.detailLabels, theme.detailText)
	}
	if theme.labels != "#ff9900" {
		t.Errorf("applyTheme() labels = %q, want #ff9900 kept", theme.labels)
	}

	applyTheme(providers.Theme{ActiveRowText: "orange", ActiveRowBackground: "silver"})
	if theme.activeRowText != "orange" || theme.activeRowBackground != "silver" {
		t.Errorf("applyTheme() active row colors = %q %q, want orange silver", theme.activeRowText, theme.activeRowBackground)
	}
	if theme.activeRowTextColor() != tcell.GetColor("orange") || theme.activeRowBackgroundColor() != tcell.GetColor("silver") {
		t.Errorf("applyTheme() active row colors resolve wrong: %v %v", theme.activeRowTextColor(), theme.activeRowBackgroundColor())
	}
	if theme.labels != "#ff9900" {
		t.Errorf("applyTheme() labels = %q, want #ff9900 kept", theme.labels)
	}
}

func TestSummaryColors(t *testing.T) {
	original := theme
	t.Cleanup(func() { theme = original })
	theme = themeColors{summaryLabels: "orange", summaryText: "silver"}

	appState = State{AllProjects: true}
	tv := newSummaryPanel()
	renderSummary(tv)
	if got, want := tv.GetText(false), "[orange]Projects: [silver]all"; got != want {
		t.Errorf("renderSummary() = %q, want %q", got, want)
	}
}

func TestDetailColors(t *testing.T) {
	original := theme
	t.Cleanup(func() { theme = original })
	theme = themeColors{detailLabels: "orange"}

	var sb strings.Builder
	appendRowDetails(&sb, map[string]any{"a": 1}, "")
	if got, want := sb.String(), "[orange]a: [-]1\n"; got != want {
		t.Errorf("appendRowDetails() = %q, want %q", got, want)
	}

	details := ipDetails(&providers.IPAddress{ProjectID: "p1"})
	if !strings.Contains(details, "[orange]Project: [-]p1\n") {
		t.Errorf("ipDetails() = %q, want [orange] Project label", details)
	}
}

func TestStatusInfo(t *testing.T) {
	original := theme
	t.Cleanup(func() { theme = original })
	theme = themeColors{labels: "yellow", text: "white", borders: "white", title: "white", shortcutLabels: "yellow", shortcutText: "white", statusLabels: "orange", statusText: "silver"}

	a := &app{status: tview.NewTextView().SetDynamicColors(true).SetTextColor(theme.statusTextColor())}
	a.statusInfo("Fetching VMs...")
	if got := a.status.GetText(false); got != "[orange]Fetching VMs...[-]" {
		t.Errorf("statusInfo() = %q, want [orange]Fetching VMs...[-]", got)
	}
}

func TestShortcutPanelColors(t *testing.T) {
	original := theme
	t.Cleanup(func() { theme = original })
	theme = themeColors{labels: "yellow", text: "white", borders: "white", title: "white", shortcutLabels: "orange", shortcutText: "silver"}

	appState.ViewKind = viewProjects
	panel := newShortcutsPanel()
	key, _, _ := panel.GetCell(0, 0).Style.Decompose()
	if key != tcell.GetColor("orange") {
		t.Errorf("shortcuts panel key color = %v, want orange", key)
	}
	desc, _, _ := panel.GetCell(0, 1).Style.Decompose()
	if desc != tcell.GetColor("silver") {
		t.Errorf("shortcuts panel desc color = %v, want silver", desc)
	}

	summary := newShortcutsSummaryPanel()
	key, _, _ = summary.GetCell(0, 0).Style.Decompose()
	if key != tcell.GetColor("orange") {
		t.Errorf("shortcuts summary key color = %v, want orange", key)
	}
	desc, _, _ = summary.GetCell(0, 1).Style.Decompose()
	if desc != tcell.GetColor("silver") {
		t.Errorf("shortcuts summary desc color = %v, want silver", desc)
	}
}
