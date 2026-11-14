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

// TestDeleteThemeRefusesBundledSeeds pins the delete guard: bundled seeds,
// the default fallback and the custom palette survive the button with a
// refusal message and the packs map intact.
func TestDeleteThemeRefusesBundledSeeds(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	for _, seed := range []string{"monokai", providers.DefaultThemeName, customThemeName} {
		configuredThemeName = seed
		configuredTheme = configuredThemePacks[seed]
		a.deleteTheme()
		if providers.IsBundledThemeName(seed) {
			if _, exists := configuredThemePacks[seed]; !exists {
				t.Errorf("deleteTheme removed bundled pack %q", seed)
			}
		}
		if _, exists := configuredThemePacks[seed]; seed != customThemeName && !exists {
			t.Errorf("deleteTheme removed pack %q", seed)
		}
		if configuredThemeName != seed {
			t.Errorf("configuredThemeName = %q after refused delete of %q", configuredThemeName, seed)
		}
	}
}

// TestDeleteThemeRemovesUserPack pins the happy path: deleting a user pack
// persists the loss, snaps the panel to the default pack, and saves.
func TestDeleteThemeRemovesUserPack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	configuredThemePacks["midnight"] = providers.Theme{Background: "navy"}
	configuredThemeName = "midnight"
	configuredTheme = configuredThemePacks["midnight"]
	a.applySettings() // persist the pack so the delete has something real to drop

	a.deleteTheme()
	if _, exists := configuredThemePacks["midnight"]; exists {
		t.Error("midnight survived deleteTheme")
	}
	if configuredThemeName != providers.DefaultThemeName {
		t.Fatalf("configuredThemeName = %q, want %q", configuredThemeName, providers.DefaultThemeName)
	}
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig(), defaultInfoConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := config.Themes["midnight"]; exists {
		t.Error("config still holds the deleted pack")
	}
}

// TestFollowTerminalHidesThemeRows pins the follow-terminal panel shape:
// the theme cycle row stays (it is the way out), the color rows and the
// Delete button all go, because the terminal owns every color while
// follow terminal paints.
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
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		switch item := a.settingsForm.GetFormItem(i).(type) {
		case *focusCheckbox:
			labels = append(labels, item.base)
		}
	}
	if len(labels) != 4 {
		t.Fatalf("checkbox rows = %v, want Auto update cache, Column borders, Shortcut bar and the Theme cycle", labels)
	}
	if labels[0] != "Auto update cache" {
		t.Errorf("first checkbox = %q, want Auto update cache", labels[0])
	}
	if labels[3] != "Theme: "+followTerminalThemeName {
		t.Errorf("theme checkbox = %q, want the follow-terminal selection", labels[3])
	}
	for i := 0; i < a.settingsForm.GetFormItemCount(); i++ {
		if _, ok := a.settingsForm.GetFormItem(i).(*colorPreviewField); ok {
			t.Fatal("color field survived follow-terminal mode")
		}
	}
	if a.settingsForm.GetFormItemCount() != 4 {
		t.Errorf("form items = %d, want 4 (four toggles, no colors)", a.settingsForm.GetFormItemCount())
	}
	if a.settingsForm.GetButtonCount() != 2 {
		t.Errorf("form buttons = %d, want 2 (Open in editor, Close; Delete theme hidden)", a.settingsForm.GetButtonCount())
	}
}