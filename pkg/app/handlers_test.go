package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func newCmdCompletionTestApp(t *testing.T) *app {
	t.Helper()
	a := &app{cmd: tview.NewInputField()}
	appState.CmdMode = modeCmd
	return a
}

func TestCommandNamesSortedAndUnique(t *testing.T) {
	names := commandNames()
	if len(names) == 0 {
		t.Fatal("commandNames() is empty")
	}
	seen := make(map[string]bool)
	for i, name := range names {
		if seen[name] {
			t.Errorf("commandNames() has duplicate %q", name)
		}
		seen[name] = true
		if i > 0 && names[i-1] > name {
			t.Errorf("commandNames() not sorted: %q before %q", names[i-1], name)
		}
	}
	for _, want := range []string{"peerings", "vms", "sort", "write"} {
		if !seen[want] {
			t.Errorf("commandNames() missing %q", want)
		}
	}
}

func TestCompleteCommand(t *testing.T) {
	a := newCmdCompletionTestApp(t)

	a.completeCommand(true)
	assertCmdText(t, a, "accounts")

	a.cmd.SetText("peer")
	a.completeCommand(true)
	assertCmdText(t, a, "peering")

	a.completeCommand(true)
	assertCmdText(t, a, "peerings")

	a.completeCommand(true)
	assertCmdText(t, a, "policies")

	a.completeCommand(false)
	assertCmdText(t, a, "peerings")

	a.cmd.SetText("VM")
	a.completeCommand(true)
	assertCmdText(t, a, "vms")

	a.cmd.SetText("zzz")
	a.completeCommand(true)
	assertCmdText(t, a, "zzz")

	a.cmd.SetText("sort desc name")
	a.completeCommand(true)
	assertCmdText(t, a, "sort desc name")

	a.cmd.SetText("s")
	a.completeCommand(false)
	assertCmdText(t, a, "sort")

	a.cmd.SetText("s")
	a.completeCommand(true)
	assertCmdText(t, a, "sa")
}

func TestSpaceShortcutShowsProjects(t *testing.T) {
	var space *shortcut
	for i := range shortcuts {
		if shortcuts[i].key == ' ' {
			space = &shortcuts[i]
			break
		}
	}
	if space == nil {
		t.Fatal("no space shortcut registered")
	}
	if space.desc != "show projects" {
		t.Fatalf("space shortcut desc = %q, want %q", space.desc, "show projects")
	}
	a := newFilterTestApp(t)
	appState.ProjectID = "p1"
	a.setView(viewVMs)
	space.run(a)
	if appState.ViewKind != viewProjects {
		t.Fatalf("space shortcut left view %q, want %q", appState.ViewKind, viewProjects)
	}
}

func TestShortcutKeyLabel(t *testing.T) {
	if got := shortcutKeyLabel(' '); got != "space" {
		t.Fatalf("shortcutKeyLabel(' ') = %q, want %q", got, "space")
	}
	if got := shortcutKeyLabel('u'); got != "u" {
		t.Fatalf("shortcutKeyLabel('u') = %q, want %q", got, "u")
	}
}

func TestVisibleShortcutsSortedByKeyLabel(t *testing.T) {
	appState.ViewKind = viewProjects
	visible := visibleShortcuts()
	if len(visible) < 2 {
		t.Fatal("visibleShortcuts() is empty")
	}
	for i := 1; i < len(visible); i++ {
		prev, cur := strings.ToLower(visible[i-1].label()), strings.ToLower(visible[i].label())
		if prev > cur {
			t.Fatalf("visibleShortcuts() not sorted: %q before %q", visible[i-1].label(), visible[i].label())
		}
	}
	var colon *shortcut
	for i := range visible {
		if visible[i].key == ':' {
			colon = &visible[i]
		}
	}
	if colon == nil {
		t.Fatal("visibleShortcuts() missing the ':' shortcut")
	}
	if len(colon.commands) == 0 {
		t.Fatal("visibleShortcuts() ':' shortcut lost its command rows")
	}
	var f1, space bool
	for _, s := range visible {
		if s.specialKey == tcell.KeyF1 {
			f1 = true
		}
		if s.key == ' ' {
			space = true
		}
	}
	if !f1 || !space {
		t.Fatalf("visibleShortcuts() missing F1 (%v) or space (%v)", f1, space)
	}
}

func TestHelpPanelCommandsSorted(t *testing.T) {
	appState.ViewKind = viewProjects
	table := newShortcutsPanel()
	prev := ""
	seen := 0
	for r := 0; r < table.GetRowCount(); r++ {
		key := table.GetCell(r, 0).Text
		if !strings.HasPrefix(key, ":") {
			continue
		}
		seen++
		if prev != "" && prev > key {
			t.Fatalf("help panel commands not sorted: %q before %q", prev, key)
		}
		prev = key
	}
	if seen < 2 {
		t.Fatalf("help panel shows %d command rows, want the : command list", seen)
	}
}

func TestToggleHelpRestoresInfoFocus(t *testing.T) {
	a := newFilterTestApp(t)
	mainView := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.summary, 4, 1, false).
		AddItem(a.cmd, 0, 0, false).
		AddItem(a.info, 0, 1, false).
		AddItem(a.status, 1, 1, false)
	a.pages.AddPage("main", mainView, true, true)
	a.pages.AddPage("help", a.helpRow, true, false)
	a.tapp.SetRoot(a.pages, true)
	a.tapp.SetFocus(a.info)

	a.toggleHelp()
	if got := a.tapp.GetFocus(); got != a.shortcuts {
		t.Fatalf("focus with help open = %T, want the help shortcuts table", got)
	}

	a.toggleHelp()
	if got := a.tapp.GetFocus(); got != a.info {
		t.Fatalf("focus after closing help = %T, want the info table", got)
	}
}

func TestF1ShortcutOpensConfigFile(t *testing.T) {
	var f1 *shortcut
	for i := range shortcuts {
		if shortcuts[i].specialKey == tcell.KeyF1 {
			f1 = &shortcuts[i]
			break
		}
	}
	if f1 == nil {
		t.Fatal("no F1 shortcut registered")
	}
	if f1.desc != "open config file in editor" {
		t.Fatalf("F1 shortcut desc = %q, want %q", f1.desc, "open config file in editor")
	}
	if f1.run == nil {
		t.Fatal("F1 shortcut has no run action")
	}
	if got := f1.label(); got != "F1" {
		t.Fatalf("F1 shortcut label = %q, want %q", got, "F1")
	}
	if !f1.matches(tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone)) {
		t.Fatal("F1 shortcut does not match an F1 key event")
	}
	if f1.matches(tcell.NewEventKey(tcell.KeyRune, 'u', tcell.ModNone)) {
		t.Fatal("F1 shortcut matches a rune key event")
	}
}

func TestReloadConfigAppliesEditedValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "lazycloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	configYaml := "theme:\n  labels: orange\n  text: silver\nautoupdate:\n  enabled: false\n  threshold_in_minutes: 15\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYaml), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	originalTheme := theme
	originalShortcuts := shortcuts
	originalStaleViewAfter := staleViewAfter
	secondary, primary := tview.Styles.SecondaryTextColor, tview.Styles.PrimaryTextColor
	t.Cleanup(func() {
		theme = originalTheme
		shortcuts = originalShortcuts
		staleViewAfter = originalStaleViewAfter
		tview.Styles.SecondaryTextColor = secondary
		tview.Styles.PrimaryTextColor = primary
	})

	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	a.staleAutoRefresh = true
	a.reloadConfig()
	if theme.labels != "orange" || theme.text != "silver" {
		t.Fatalf("reloadConfig() theme = %+v, want orange/silver", theme)
	}
	if tview.Styles.SecondaryTextColor != tcell.GetColor("orange") {
		t.Errorf("reloadConfig() SecondaryTextColor = %v, want orange", tview.Styles.SecondaryTextColor)
	}
	if tview.Styles.PrimaryTextColor != tcell.GetColor("silver") {
		t.Errorf("reloadConfig() PrimaryTextColor = %v, want silver", tview.Styles.PrimaryTextColor)
	}
	if a.staleAutoRefresh {
		t.Error("reloadConfig() staleAutoRefresh = true, want false")
	}
	if staleViewAfter != 15*time.Minute {
		t.Errorf("reloadConfig() staleViewAfter = %v, want 15m", staleViewAfter)
	}
	if got := a.status.GetText(false); !strings.Contains(got, "Config reloaded") {
		t.Errorf("reloadConfig() status = %q, want Config reloaded", got)
	}
}

func TestReloadConfigAppliesInfobarSetting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "lazycloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	configYaml := "theme:\n  labels: orange\ninfobar:\n  show_shortcuts: false\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYaml), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	originalInfobar := infobarEnabled
	originalTheme := theme
	originalShortcuts := shortcuts
	t.Cleanup(func() {
		infobarEnabled = originalInfobar
		theme = originalTheme
		shortcuts = originalShortcuts
	})

	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	infobarEnabled = true
	a.reloadConfig()
	if infobarEnabled {
		t.Fatal("reloadConfig() with show_shortcuts: false left the infobar enabled")
	}

	configYaml = "theme:\n  labels: orange\ninfobar:\n  show_shortcuts: true\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYaml), 0o644); err != nil {
		t.Fatalf("rewriting config file: %v", err)
	}
	a.reloadConfig()
	if !infobarEnabled {
		t.Fatal("reloadConfig() with show_shortcuts: true left the infobar disabled")
	}
	if got := a.status.GetText(false); !strings.Contains(got, "Config reloaded") {
		t.Errorf("reloadConfig() status = %q, want Config reloaded", got)
	}
}

func TestReloadConfigKeepsValuesOnBrokenFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "lazycloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("theme:\n  labels: [broken"), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	originalTheme := theme
	t.Cleanup(func() { theme = originalTheme })

	appState.ViewKind = viewProjects
	a := newFilterTestApp(t)
	originalLabels := theme.labels
	a.reloadConfig()
	if theme.labels != originalLabels {
		t.Errorf("reloadConfig() theme.labels = %q, want %q kept", theme.labels, originalLabels)
	}
	if got := a.status.GetText(false); !strings.Contains(got, "Failed to reload config") {
		t.Errorf("reloadConfig() status = %q, want failure message", got)
	}
}

func TestRowCopyPayloadIncludesColumns(t *testing.T) {
	payload, err := rowCopyPayload(
		[]string{"Network", "Peering", "Route Exchange"},
		[]string{"app-net", "peer-1", "subnets, export-custom"},
		providers.NetworkPeering{ProjectID: "p1", Network: "app-net", Name: "peer-1", ExchangeSubnetRoutes: true, ExportCustomRoutes: true},
	)
	if err != nil {
		t.Fatalf("rowCopyPayload() error = %v", err)
	}
	if payload["ProjectID"] != "p1" || payload["ExchangeSubnetRoutes"] != true {
		t.Fatalf("rowCopyPayload() resource fields missing: %#v", payload)
	}
	cols, ok := payload["Columns"].(map[string]string)
	if !ok {
		t.Fatalf("rowCopyPayload() Columns missing: %#v", payload)
	}
	if cols["Network"] != "app-net" || cols["Peering"] != "peer-1" {
		t.Fatalf("rowCopyPayload() Columns wrong: %#v", cols)
	}
	if cols["Route Exchange"] != "subnets, export-custom" {
		t.Fatalf("rowCopyPayload() derived column missing: %#v", cols)
	}
}

func TestRowCopyPayloadNonObjectResource(t *testing.T) {
	payload, err := rowCopyPayload([]string{"Name"}, []string{"a"}, []string{"not", "an", "object"})
	if err != nil {
		t.Fatalf("rowCopyPayload() error = %v", err)
	}
	if res, ok := payload["Resource"].([]string); !ok || len(res) != 3 {
		t.Fatalf("rowCopyPayload() Resource fallback missing: %#v", payload)
	}
	if cols, ok := payload["Columns"].(map[string]string); !ok || cols["Name"] != "a" {
		t.Fatalf("rowCopyPayload() Columns missing: %#v", payload)
	}
}
