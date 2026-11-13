package app

import (
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

// themeForkTestState snapshots the globals the fork flow touches, so each
// test starts from the bundled seeds and restores the palette afterwards.
type themeForkTestState struct {
	t         *testing.T
	origPacks map[string]providers.ThemeSpec
	origName  string
	origDark  providers.Theme
	origLight providers.Theme
	origMode  providers.Theme
}

func forkTestState(t *testing.T) *themeForkTestState {
	t.Helper()
	s := &themeForkTestState{
		t:         t,
		origPacks: configuredThemePacks,
		origName:  configuredThemeName,
		origDark:  configuredThemesDark,
		origLight: configuredThemesLight,
		origMode:  configuredTheme,
	}
	packs := map[string]providers.ThemeSpec{}
	for name, spec := range providers.BundledThemeSpecs() {
		packs[name] = spec
	}
	configuredThemePacks = packs
	seed := "monokai"
	configuredThemeName = seed
	spec := packs[seed]
	configuredThemesDark = spec.Dark
	configuredThemesLight = spec.Light
	configuredTheme = spec.Dark
	t.Cleanup(func() {
		configuredThemePacks = s.origPacks
		configuredThemeName = s.origName
		configuredThemesDark = s.origDark
		configuredThemesLight = s.origLight
		configuredTheme = s.origMode
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
	seedPalette := providers.BundledThemeSpecs()[seed]
	edit := themeColorKeys[0]

	// The changed callback writes the value into the section first, then
	// forks, so the fork snapshot carries the edit in flight.
	edit.set(&configuredThemesDark, "orange")
	a.forkBundledTheme()

	if configuredThemeName != "monokai-custom" {
		t.Fatalf("configuredThemeName = %q, want %q", configuredThemeName, "monokai-custom")
	}
	if got := configuredThemePacks[seed]; got.Dark != seedPalette.Dark {
		t.Errorf("seed palette drifted: %+v, want %+v", got.Dark, seedPalette.Dark)
	}
	fork, ok := configuredThemePacks["monokai-custom"]
	if !ok {
		t.Fatal("fork pack missing from configuredThemePacks")
	}
	if fork.Dark.Background != "orange" {
		t.Errorf("fork background = %q, want orange (the edit in flight)", fork.Dark.Background)
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

	if text := a.settingsSaveAs.GetText(); text != providers.DefaultThemeName {
		t.Errorf("save-as text after delete = %q, want the %q suggestion", text, providers.DefaultThemeName)
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
	edit.set(&configuredThemesDark, "orange")
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
