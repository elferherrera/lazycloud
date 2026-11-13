package app

import (
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

// themeForkTestState snapshots the globals the fork flow touches, so each
// test starts from the bundled seeds and restores the palette afterwards.
type themeForkTestState struct {
	t          *testing.T
	origPacks  map[string]providers.Theme
	origName   string
	origTheme  providers.Theme
	origFollow bool
}

func forkTestState(t *testing.T) *themeForkTestState {
	t.Helper()
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

// TestForkBundledThemeOnFirstColorEdit pins the core contract: editing a
// color on a bundled pack forks it before the edit lands, the seed keeps
// its shipped palette, and the fork carries the edit.
func TestForkBundledThemeOnFirstColorEdit(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	seed := "monokai"
	seedPalette := providers.BundledThemes()[seed]
	edit := themeColorKeys[0]

	// The changed callback writes the value into the palette first, then
	// forks, so the fork snapshot carries the edit in flight.
	edit.set(&configuredTheme, "orange")
	a.forkBundledTheme()

	if configuredThemeName != "monokai-custom" {
		t.Fatalf("configuredThemeName = %q, want %q", configuredThemeName, "monokai-custom")
	}
	if got := configuredThemePacks[seed]; got != seedPalette {
		t.Errorf("seed palette drifted: %+v, want %+v", got, seedPalette)
	}
	fork, ok := configuredThemePacks["monokai-custom"]
	if !ok {
		t.Fatal("fork pack missing from configuredThemePacks")
	}
	if fork.Background != "orange" {
		t.Errorf("fork background = %q, want orange (the edit in flight)", fork.Background)
	}
	if !a.sessionForks["monokai-custom"] {
		t.Error("sessionForks does not track the auto-fork")
	}
}

// TestForkNameCollisionsNumberUp pins the fork-name picker: monokai-custom
// taken means monokai-custom-2, then -3; never a silent overwrite.
func TestForkNameCollisionsNumberUp(t *testing.T) {
	existing := map[string]bool{"monokai": true, "monokai-custom": true}
	if got := providers.CustomThemeForkName("monokai", existing); got != "monokai-custom-2" {
		t.Errorf("CustomThemeForkName = %q, want monokai-custom-2", got)
	}
	existing["monokai-custom-2"] = true
	if got := providers.CustomThemeForkName("monokai", existing); got != "monokai-custom-3" {
		t.Errorf("CustomThemeForkName = %q, want monokai-custom-3", got)
	}
}

// TestTypedSaveAsNameRenamesFork pins close-time consumption: a name typed
// over the suggestion renames the pack and absorbs this session's fork, so
// only the typed pack survives in the config.
func TestTypedSaveAsNameRenamesFork(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	a.forkBundledTheme()
	if a.settingsSaveAs == nil {
		t.Fatal("save-as field missing after rebuild")
	}
	a.settingsSaveAs.SetText("midnight")
	if !a.applyTypedThemeName() {
		t.Fatalf("applyTypedThemeName refused a free name")
	}
	if configuredThemeName != "midnight" {
		t.Fatalf("configuredThemeName = %q, want midnight", configuredThemeName)
	}
	if _, exists := configuredThemePacks["monokai-custom"]; exists {
		t.Error("monokai-custom fork survived a save-as rename; want it absorbed")
	}
}

// TestTypedSaveAsNameRejectsExistingNames pins the no-silent-overwrite rule
// at close time: an existing pack name or a bundled seed name keeps the
// panel open instead of clobbering the pack.
func TestTypedSaveAsNameRejectsExistingNames(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	a.forkBundledTheme()
	for _, typed := range []string{"monokai", "  solarized  "} {
		a.settingsSaveAs.SetText(typed)
		if a.applyTypedThemeName() {
			t.Errorf("applyTypedThemeName accepted existing name %q", strings.TrimSpace(typed))
		}
		if configuredThemeName != "monokai-custom" {
			t.Errorf("configuredThemeName = %q after refused rename", configuredThemeName)
		}
	}
}

// TestCloseSettingsWithoutEditsKeepsBundledPack pins the zero-edit path:
// closing on an untouched bundled pack leaves the config without a stray
// -custom copy, because the suggestion was never a typed rename.
func TestCloseSettingsWithoutEditsKeepsBundledPack(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	if !a.applyTypedThemeName() {
		t.Fatal("close refused on an untouched bundled pack")
	}
	if _, exists := configuredThemePacks["monokai-custom"]; exists {
		t.Error("closing without edits created monokai-custom; want no stray fork")
	}
	if configuredThemeName != "monokai" {
		t.Errorf("configuredThemeName = %q, want monokai", configuredThemeName)
	}
}

// TestDeleteThemeClearsTypedSaveAsName pins the post-delete state: a name
// typed for the deleted pack must not rename the fallback pack on close,
// so the field goes back to the suggestion after the delete rebuilds.
func TestDeleteThemeClearsTypedSaveAsName(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	a.forkBundledTheme()
	a.settingsSaveAs.SetText("midnight")
	a.deleteTheme()

	if text := a.settingsSaveAs.GetText(); text != providers.CustomThemeForkName(providers.DefaultThemeName, nil) {
		t.Errorf("save-as text after delete = %q, want the %q suggestion", text, providers.CustomThemeForkName(providers.DefaultThemeName, nil))
	}
	if !a.applyTypedThemeName() {
		t.Fatal("close refused after delete")
	}
	if configuredThemeName != providers.DefaultThemeName {
		t.Errorf("configuredThemeName = %q after delete and close; want the fallback untouched", configuredThemeName)
	}
}

// TestDeleteThemeRefusesBundledSeeds pins the delete guard: bundled seeds
// (and the default fallback) survive the button with a refusal message and
// the packs map intact.
func TestDeleteThemeRefusesBundledSeeds(t *testing.T) {
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	for _, seed := range []string{"monokai", providers.DefaultThemeName} {
		configuredThemeName = seed
		configuredTheme = configuredThemePacks[seed]
		a.deleteTheme()
		if providers.IsBundledThemeName(seed) {
			if _, exists := configuredThemePacks[seed]; !exists {
				t.Errorf("deleteTheme removed bundled pack %q", seed)
			}
		}
		if configuredThemeName != seed {
			t.Errorf("configuredThemeName = %q after refused delete of %q", configuredThemeName, seed)
		}
	}
}

// TestDeleteThemeRemovesCustomPack pins the happy path: deleting a custom
// pack persists the loss, snaps the panel to the default pack, and saves.
func TestDeleteThemeRemovesCustomPack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	forkTestState(t)
	a := settingsTestApp(t)
	a.buildSettingsPage()
	a.rebuildSettingsForm()

	a.forkBundledTheme()
	edit := themeColorKeys[0]
	edit.set(&configuredTheme, "orange")
	a.applySettings() // persist the fork so the delete has something real to drop

	a.deleteTheme()
	if _, exists := configuredThemePacks["monokai-custom"]; exists {
		t.Error("monokai-custom survived deleteTheme")
	}
	if configuredThemeName != providers.DefaultThemeName {
		t.Fatalf("configuredThemeName = %q, want %q", configuredThemeName, providers.DefaultThemeName)
	}
	config, err := providers.LoadConfig(defaultThemeConfig(), defaultShortcutConfig(), defaultAutoUpdateConfig(), defaultInfobarConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := config.Themes["monokai-custom"]; exists {
		t.Error("config still holds the deleted pack")
	}
}

// TestFollowTerminalHidesThemeRows pins the follow-terminal panel shape:
// the toggle row stays, the theme pack row, the save-as input, the color
// rows and the Delete button all go, because the terminal owns every color
// while the toggle is on.
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
	if len(labels) != 3 {
		t.Fatalf("checkbox rows = %v, want only Follow terminal, Shortcut bar and Auto update cache", labels)
	}
	if labels[0] != "Follow terminal" {
		t.Errorf("first checkbox = %q, want Follow terminal", labels[0])
	}
	if a.settingsSaveAs != nil {
		t.Error("save-as field survived follow-terminal mode")
	}
	if a.settingsForm.GetFormItemCount() != 4 {
		t.Errorf("form items = %d, want 4 (three toggles, config path, no colors or save-as)", a.settingsForm.GetFormItemCount())
	}
	if a.settingsForm.GetButtonCount() != 2 {
		t.Errorf("form buttons = %d, want 2 (Open in editor, Close; Delete theme hidden)", a.settingsForm.GetButtonCount())
	}
}
