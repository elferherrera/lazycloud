package app

import (
	"reflect"
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// settingsGlobalsSnapshot captures every package-level variable the
// settings panel and theme plumbing mutate, so one cleanup restores the
// whole set instead of each test remembering a different subset.
type settingsGlobalsSnapshot struct {
	theme                 themeColors
	configuredThemePacks  map[string]providers.Theme
	configuredThemeName   string
	configuredTheme       providers.Theme
	configuredFollowTerm  bool
	infobarEnabled        bool
	applyActiveThemeFnVar func()
}

func snapshotSettingsGlobals(t *testing.T) settingsGlobalsSnapshot {
	t.Helper()
	snap := settingsGlobalsSnapshot{
		theme:                 theme,
		configuredThemePacks:  configuredThemePacks,
		configuredThemeName:   configuredThemeName,
		configuredTheme:       configuredTheme,
		configuredFollowTerm:  configuredFollowTerminal,
		infobarEnabled:        infobarEnabled,
		applyActiveThemeFnVar: applyActiveThemeFn,
	}
	t.Cleanup(func() {
		theme = snap.theme
		configuredThemePacks = snap.configuredThemePacks
		configuredThemeName = snap.configuredThemeName
		configuredTheme = snap.configuredTheme
		configuredFollowTerminal = snap.configuredFollowTerm
		infobarEnabled = snap.infobarEnabled
		applyActiveThemeFn = snap.applyActiveThemeFnVar
	})
	return snap
}

// TestThemeColorKeysCoverEveryThemeField pins the editor's coverage: the
// color rows must expose every string field of the Theme struct, so a
// field added later without a row cannot slip past review.
func TestThemeColorKeysCoverEveryThemeField(t *testing.T) {
	snapshotSettingsGlobals(t)
	themeType := reflect.TypeOf(providers.Theme{})
	fields := make([]string, 0, themeType.NumField())
	for i := 0; i < themeType.NumField(); i++ {
		f := themeType.Field(i)
		if f.Type.Kind() == reflect.String {
			fields = append(fields, f.Name)
		}
	}
	if len(fields) != len(themeColorKeys) {
		t.Fatalf("Theme has %d string fields but the panel exposes %d rows", len(fields), len(themeColorKeys))
	}
	exposed := map[string]bool{}
	for _, key := range themeColorKeys {
		theme := providers.Theme{}
		marker := "sentinel-" + key.label
		key.set(&theme, marker)
		value := reflect.ValueOf(theme)
		for _, name := range fields {
			if value.FieldByName(name).String() == marker {
				exposed[name] = true
				break
			}
		}
	}
	missing := make([]string, 0)
	for _, name := range fields {
		if !exposed[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("themeColorKeys misses Theme fields: %s", strings.Join(missing, ", "))
	}
}
