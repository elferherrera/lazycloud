package app

import (
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// themeForkTestState snapshots the globals the theme pack flow touches, so
// each test starts from the bundled seeds and restores the palette
// afterwards.
type themeForkTestState struct {
	t          *testing.T
	origPacks  map[string]providers.Theme
	origName   string
	origTheme  providers.Theme
	origFollow bool
}

func forkTestState(t *testing.T) *themeForkTestState {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	s := &themeForkTestState{
		t:          t,
		origPacks:  configuredThemePacks,
		origName:   configuredThemeName,
		origTheme:  configuredTheme,
		origFollow: configuredFollowTerminal,
	}
	packs := map[string]providers.Theme{}
	for name, pack := range providers.BundledThemes() {
		packs[name] = pack
	}
	configuredThemePacks = packs
	seed := "monokai"
	configuredThemeName = seed
	configuredTheme = packs[seed]
	configuredFollowTerminal = false
	t.Cleanup(func() {
		configuredThemePacks = s.origPacks
		configuredThemeName = s.origName
		configuredTheme = s.origTheme
		configuredFollowTerminal = s.origFollow
	})
	return s
}

// TestFollowTerminalHidesThemeRows pins the follow-terminal panel shape:
// the theme cycle row stays (it is the way out) and the color rows go,
// because the terminal owns every color while follow terminal paints.
func TestFollowTerminalHidesThemeRows(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	configuredFollowTerminal = true
	a.rebuildSettingsForm()

	if a.settingsForm == nil {
		t.Fatal("settings form missing")
	}
	var labels []string
	var themeItem *themeSelectionField
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		switch item := a.settingsForm.GetFormItem(i).(type) {
		case *focusCheckbox:
			labels = append(labels, item.base)
		case *themeSelectionField:
			themeItem = item
		}
	}
	if len(labels) != 3 {
		t.Fatalf("checkbox rows = %v, want Auto update cache, Column borders and Shortcut bar", labels)
	}
	if labels[0] != "Auto update cache" {
		t.Errorf("first checkbox = %q, want Auto update cache", labels[0])
	}
	if themeItem == nil {
		t.Fatal("settings form is missing the Theme cycle row")
	}
	if themeItem.GetText() != followTerminalThemeName {
		t.Errorf("theme selection = %q, want the follow-terminal entry", themeItem.GetText())
	}
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		if _, ok := a.settingsForm.GetFormItem(i).(*colorPreviewField); ok {
			t.Fatal("color field survived follow-terminal mode")
		}
	}
	if a.settingsForm.GetFormItemCount() != 4 {
		t.Errorf("form items = %d, want 4 (four toggles, no colors)", a.settingsForm.GetFormItemCount())
	}
	if a.settingsForm.GetButtonCount() != 1 {
		t.Errorf("form buttons = %d, want 1 (Open in editor)", a.settingsForm.GetButtonCount())
	}
}