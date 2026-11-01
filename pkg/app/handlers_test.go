package app

import (
	"testing"

	"github.com/rivo/tview"
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
	assertCmdText(t, a, "projects")

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