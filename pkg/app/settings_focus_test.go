package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// focusFormRows builds the settings page, focuses the form the way the app
// does (SetFocus on the form, which delegates to the focused element) and
// returns the app so tests can move focus and re-render.
func focusFormRows(t *testing.T, width, focusIndex int) (*app, []string) {
	t.Helper()
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: "dark"}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	a.buildSettingsPage()
	form := a.settingsForm
	h := settingsFormHeight(a)
	form.SetFocus(focusIndex)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(width, h)
	form.SetRect(10, 0, width-20, h)
	// Same delegation Application.SetFocus performs: the delegate hands
	// focus down the primitive chain, firing the items' focus callbacks.
	var focus func(p tview.Primitive)
	focus = func(p tview.Primitive) { p.Focus(focus) }
	form.Focus(focus)
	form.Draw(screen)
	screen.Show()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var row strings.Builder
		cells, cw, _ := screen.GetContents()
		for x := 0; x < cw; x++ {
			row.WriteRune(cells[y*cw+x].Runes[0])
		}
		rows[y] = row.String()
	}
	return a, rows
}

// TestFocusedCheckboxRowShowsMarker pins the focus affordance: the focused
// checkbox row carries the ▸ prefix and an accent-colored label, while
// unfocused rows keep their plain labels so the column stays aligned.
func TestFocusedCheckboxRowShowsMarker(t *testing.T) {
	_, rows := focusFormRows(t, 80, 1) // Light theme, first toggleable row
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, focusMarker+"Light theme") {
		t.Errorf("focused checkbox row is missing the %q marker; form:\n%s", focusMarker, joined)
	}
	if !strings.Contains(joined, noFocusMarker+"Dark theme") {
		t.Errorf("unfocused checkbox row lost its padding prefix; form:\n%s", joined)
	}
}

// TestFocusedColorFieldRowShowsMarker pins the same affordance on the color
// swatch rows, whose labels live on the embedded InputField.
func TestFocusedColorFieldRowShowsMarker(t *testing.T) {
	_, rows := focusFormRows(t, 80, 6) // first color field (Background)
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, focusMarker+"Background") {
		t.Errorf("focused color row is missing the %q marker; form:\n%s", focusMarker, joined)
	}
	if !strings.Contains(joined, noFocusMarker+"Labels") {
		t.Errorf("unfocused color row lost its padding prefix; form:\n%s", joined)
	}
}

// TestRebuildSettingsFormKeepsFocusedRow guards the focus-restore logic:
// toggling a checkbox rebuilds the form, and the rebuilt row must keep focus
// (its marker and styles) so the caret does not jump back to the top.
func TestRebuildSettingsFormKeepsFocusedRow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: "dark"}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	a.buildSettingsPage()
	form := a.settingsForm

	// Focus the Shortcut bar row (index 4) and rebuild, like a toggle does.
	form.SetFocus(4)
	a.rebuildSettingsForm()

	item, button := form.GetFocusedItemIndex()
	if item != 4 || button != -1 {
		t.Fatalf("after rebuild focused item=%d button=%d, want item=4 button=-1", item, button)
	}
	checkbox, ok := form.GetFormItem(4).(*focusCheckbox)
	if !ok {
		t.Fatalf("form item 4 is %T, want *focusCheckbox", form.GetFormItem(4))
	}
	if !strings.HasPrefix(checkbox.GetLabel(), focusMarker) {
		t.Errorf("rebuilt focused checkbox label = %q, want %q prefix", checkbox.GetLabel(), focusMarker)
	}
}

// TestRebuildSettingsFormResetsFocusWithoutFormFocus pins the other half of
// the restore rule: when the form subtree has no focus (panel closed or the
// app focus sits elsewhere), a rebuild must not steal focus for it.
func TestRebuildSettingsFormResetsFocusWithoutFormFocus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := settingsTestApp(t)
	configuredTheme = providers.Theme{Mode: "dark"}
	configuredThemesDark = configuredTheme
	configuredThemesLight = providers.DefaultLightTheme()
	a.buildSettingsPage()

	form := a.settingsForm
	if form.HasFocus() {
		t.Fatal("form must not have focus before the app grants it")
	}
	a.rebuildSettingsForm()
	if form.HasFocus() {
		t.Error("rebuild without prior focus must not focus the form")
	}
}
