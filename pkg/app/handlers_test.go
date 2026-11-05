package app

import (
	"testing"

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
	assertCmdText(t, a, "private-service-connect")

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