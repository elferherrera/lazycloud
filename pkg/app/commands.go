package app

import (
	"fmt"
	"sort"
	"strings"
)

type commandFunc func(a *app, args []string)

var commands = map[string]commandFunc{
	"sort": runSortCommand,
}

func (a *app) runCommand(fields []string) {
	if len(fields) == 0 {
		a.cmd.SetText("")
		return
	}
	cmd := fields[0]
	args := fields[1:]
	a.cmd.SetText("")
	if run, ok := commands[cmd]; ok {
		run(a, args)
	} else {
		a.statusText("[red]Unknown command: [-]" + cmd)
	}
}

func runSortCommand(a *app, args []string) {
	if len(args) < 2 {
		a.statusText("[red]sort: usage: sort asc|desc <column>[-]")
		return
	}
	direction := strings.ToLower(args[0])
	if direction != "asc" && direction != "desc" {
		a.statusText("[red]sort: direction must be asc or desc[-]")
		return
	}
	column := strings.Join(args[1:], " ")
	col := -1
	for i, header := range a.viewHeaders {
		if strings.EqualFold(header, column) {
			col = i
			break
		}
	}
	if col < 0 {
		a.statusText(fmt.Sprintf("[red]sort: unknown column %q, columns: %s[-]", column, strings.Join(a.viewHeaders, ", ")))
		return
	}
	a.sortRows(direction == "desc", col)
	a.restoreFilter()
	a.statusText(fmt.Sprintf("[green]Sorted by %s %s[-]", a.viewHeaders[col], direction))
}

func (a *app) restoreFilter() {
	if !a.filterStashed {
		return
	}
	filter := a.savedFilter
	a.clearFilterStash()
	appState.CmdMode = modeFilter
	a.cmd.SetLabel(" / ")
	a.cmd.SetText(filter)
}

func (a *app) clearFilterStash() {
	a.savedFilter = ""
	a.filterStashed = false
}

func (a *app) sortRows(desc bool, col int) {
	sort.SliceStable(a.visibleRows, func(i, j int) bool {
		cmp := strings.Compare(strings.ToLower(sortCell(a.visibleRows[i], col)), strings.ToLower(sortCell(a.visibleRows[j], col)))
		if desc {
			cmp = -cmp
		}
		return cmp < 0
	})
	a.setFilterBase(a.visibleRows)
	a.applyFilter()
}

func sortCell(row []string, col int) string {
	if col < len(row) {
		return row[col]
	}
	return ""
}
