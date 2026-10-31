package app

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func newSortTestApp() *app {
	a := &app{
		tapp:   tview.NewApplication(),
		cmd:    tview.NewInputField(),
		info:   newInfoPanel(),
		status: tview.NewTextView().SetDynamicColors(true),
		pages:  tview.NewPages(),
	}
	a.info.SetBorder(true)
	a.cmd.SetChangedFunc(a.changed)
	a.viewHeaders = []string{"Name", "Machine Type"}
	a.allRows = [][]string{
		{"web-2", "n1-standard"},
		{"web-10", "e2-medium"},
		{"Web-1", "n1-standard"},
	}
	a.setFilterBase(a.allRows)
	a.applyFilter()
	return a
}

func sortTestColumn(a *app, col int) []string {
	got := make([]string, len(a.visibleRows))
	for i, row := range a.visibleRows {
		got[i] = sortCell(row, col)
	}
	return got
}

func TestRunSortCommandAscDesc(t *testing.T) {
	a := newSortTestApp()

	runSortCommand(a, []string{"asc", "name"})
	if got, want := sortTestColumn(a, 0), []string{"Web-1", "web-10", "web-2"}; !equalStringSlices(got, want) {
		t.Errorf("sort asc = %v, want %v", got, want)
	}
	if len(a.allRows) != 3 || a.allRows[0][0] != "web-2" || a.allRows[1][0] != "web-10" || a.allRows[2][0] != "Web-1" {
		t.Errorf("allRows reordered by sort: %v, want untouched original order", a.allRows)
	}

	runSortCommand(a, []string{"DESC", "name"})
	if got, want := sortTestColumn(a, 0), []string{"web-2", "web-10", "Web-1"}; !equalStringSlices(got, want) {
		t.Errorf("sort desc = %v, want %v", got, want)
	}
	if status := a.status.GetText(false); status != "[green]Sorted by Name desc[-]" {
		t.Errorf("status = %q, want [green]Sorted by Name desc[-]", status)
	}
}

func TestRunSortCommandColumnNameWithSpaces(t *testing.T) {
	a := newSortTestApp()

	runSortCommand(a, []string{"asc", "machine", "type"})
	if got, want := sortTestColumn(a, 0), []string{"web-10", "web-2", "Web-1"}; !equalStringSlices(got, want) {
		t.Errorf("sort by spaced column = %v, want %v", got, want)
	}
	if status := a.status.GetText(false); status != "[green]Sorted by Machine Type asc[-]" {
		t.Errorf("status = %q, want [green]Sorted by Machine Type asc[-]", status)
	}
}

func TestRunSortCommandErrors(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"missing direction", nil, "sort: usage: sort asc|desc <column>"},
		{"missing column", []string{"asc"}, "sort: usage: sort asc|desc <column>"},
		{"bad direction", []string{"sideways", "name"}, "sort: direction must be asc or desc"},
		{"unknown column", []string{"asc", "nope"}, `sort: unknown column "nope", columns: Name, Machine Type`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newSortTestApp()
			before := sortTestColumn(a, 0)

			runSortCommand(a, tc.args)

			if got := a.status.GetText(false); !strings.Contains(got, tc.wantMsg) {
				t.Errorf("status = %q, want it to contain %q", got, tc.wantMsg)
			}
			if after := sortTestColumn(a, 0); !equalStringSlices(after, before) {
				t.Errorf("rows changed after error: %v, want unchanged %v", after, before)
			}
		})
	}
}

func TestCommandEnterSort(t *testing.T) {
	a := newSortTestApp()
	a.cmd.SetText("sort desc name")
	a.commandEnter()

	if got, want := sortTestColumn(a, 0), []string{"web-2", "web-10", "Web-1"}; !equalStringSlices(got, want) {
		t.Errorf("rows after :sort desc name = %v, want %v", got, want)
	}
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after command = %q, want cleared", text)
	}
}

func TestCommandEnterSortKeepsFilter(t *testing.T) {
	a := newSortTestApp()
	t.Cleanup(func() { appState.CmdMode = modeCmd })

	appState.CmdMode = modeFilter
	a.cmd.SetText("n1")
	if got, want := sortTestColumn(a, 0), []string{"web-2", "Web-1"}; !equalStringSlices(got, want) {
		t.Fatalf("rows with filter n1 = %v, want %v", got, want)
	}

	a.focusCmd()
	if a.savedFilter != "n1" {
		t.Errorf("savedFilter after : = %q, want n1", a.savedFilter)
	}
	if appState.CmdMode != modeCmd {
		t.Errorf("CmdMode after : = %v, want modeCmd", appState.CmdMode)
	}
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after entering cmd mode = %q, want cleared", text)
	}

	a.cmd.SetText("sort asc name")
	a.commandEnter()

	if got, want := sortTestColumn(a, 0), []string{"Web-1", "web-2"}; !equalStringSlices(got, want) {
		t.Errorf("rows after :sort asc name = %v, want %v", got, want)
	}
	if text := a.cmd.GetText(); text != "n1" {
		t.Errorf("input after sort with active filter = %q, want filter n1 restored", text)
	}
	if appState.CmdMode != modeFilter {
		t.Errorf("CmdMode after sort = %v, want modeFilter", appState.CmdMode)
	}
	if a.savedFilter != "" {
		t.Errorf("savedFilter after sort = %q, want consumed", a.savedFilter)
	}
	if len(a.filterBase) != 2 {
		t.Errorf("filterBase after sort = %d rows, want the 2 filtered rows", len(a.filterBase))
	}
	if len(a.allRows) != 3 || a.allRows[0][0] != "web-2" || a.allRows[1][0] != "web-10" || a.allRows[2][0] != "Web-1" {
		t.Errorf("allRows reordered by sort: %v, want untouched original order", a.allRows)
	}
}

func TestModeEntryClearsInput(t *testing.T) {
	a := newSortTestApp()
	t.Cleanup(func() { appState.CmdMode = modeCmd })

	appState.CmdMode = modeFilter
	a.cmd.SetText("n1")
	a.tapp.SetFocus(a.info)

	a.focusCmd()
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after entering cmd mode = %q, want cleared", text)
	}
	if a.savedFilter != "n1" {
		t.Errorf("savedFilter after entering cmd mode = %q, want n1", a.savedFilter)
	}

	a.cmd.SetText("leftover")
	a.focusCmd()
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after re-entering cmd mode = %q, want cleared", text)
	}
	if a.savedFilter != "n1" {
		t.Errorf("savedFilter after re-entering cmd mode = %q, want untouched n1", a.savedFilter)
	}

	a.cmd.SetText("leftover")
	a.startFilter()
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after entering filter mode = %q, want cleared", text)
	}

	appState.CmdMode = modeFilter
	a.cmd.SetText("n1")
	a.tapp.SetFocus(a.info)
	a.startFilter()
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after re-entering filter mode = %q, want cleared", text)
	}
	if len(a.filterBase) != 2 {
		t.Errorf("filterBase after / in refinement = %d rows, want the 2 visible rows", len(a.filterBase))
	}
}

func TestCommandEnterSortKeepsRefinedFilter(t *testing.T) {
	a := newSortTestApp()
	t.Cleanup(func() { appState.CmdMode = modeCmd })

	appState.CmdMode = modeFilter
	a.cmd.SetText("n1")
	a.tapp.SetFocus(a.info)
	a.startFilter()
	if len(a.filterBase) != 2 {
		t.Fatalf("filterBase after refinement = %d rows, want the 2 visible rows", len(a.filterBase))
	}

	a.focusCmd()
	if len(a.filterBase) != 2 {
		t.Errorf("filterBase reset by entering cmd mode = %d rows, want the refined 2", len(a.filterBase))
	}
	if got, want := sortTestColumn(a, 0), []string{"web-2", "Web-1"}; !equalStringSlices(got, want) {
		t.Errorf("rows after entering cmd mode = %v, want refined rows unchanged %v", got, want)
	}
	if !a.filterStashed {
		t.Errorf("filterStashed after entering cmd mode = false, want the refinement stashed")
	}

	a.cmd.SetText("sort asc name")
	a.commandEnter()

	if got, want := sortTestColumn(a, 0), []string{"Web-1", "web-2"}; !equalStringSlices(got, want) {
		t.Errorf("rows after :sort asc name = %v, want %v", got, want)
	}
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after sort = %q, want the refined empty filter restored", text)
	}
	if appState.CmdMode != modeFilter {
		t.Errorf("CmdMode after sort = %v, want modeFilter (refinement restored)", appState.CmdMode)
	}
	if a.filterStashed {
		t.Errorf("filterStashed after sort = true, want consumed")
	}
	if len(a.filterBase) != 2 {
		t.Errorf("filterBase after sort = %d rows, want the 2 refined rows", len(a.filterBase))
	}
	if len(a.allRows) != 3 || a.allRows[0][0] != "web-2" || a.allRows[1][0] != "web-10" || a.allRows[2][0] != "Web-1" {
		t.Errorf("allRows reordered by sort: %v, want untouched original order", a.allRows)
	}
}

func TestStartFilterResumesRefinedFilter(t *testing.T) {
	a := newSortTestApp()
	t.Cleanup(func() { appState.CmdMode = modeCmd })

	appState.CmdMode = modeFilter
	a.cmd.SetText("n1")
	a.tapp.SetFocus(a.info)
	a.startFilter()
	if len(a.filterBase) != 2 {
		t.Fatalf("filterBase after refinement = %d rows, want the 2 visible rows", len(a.filterBase))
	}

	a.focusCmd()
	a.tapp.SetFocus(a.info)
	a.startFilter()

	if got, want := sortTestColumn(a, 0), []string{"web-2", "Web-1"}; !equalStringSlices(got, want) {
		t.Errorf("rows after / resuming refinement = %v, want %v", got, want)
	}
	if len(a.filterBase) != 2 {
		t.Errorf("filterBase after / resuming refinement = %d rows, want the 2 refined rows", len(a.filterBase))
	}
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after / resuming refinement = %q, want cleared", text)
	}
	if appState.CmdMode != modeFilter {
		t.Errorf("CmdMode after / resuming refinement = %v, want modeFilter", appState.CmdMode)
	}
}

func TestCommandEnterUnknownCommand(t *testing.T) {
	a := newSortTestApp()
	a.cmd.SetText("frobnicate now")
	a.commandEnter()

	if got := a.status.GetText(false); !strings.Contains(got, "[red]Unknown command: [-]frobnicate") {
		t.Errorf("status = %q, want it to contain unknown command message", got)
	}
	if text := a.cmd.GetText(); text != "" {
		t.Errorf("input after unknown command = %q, want cleared", text)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
