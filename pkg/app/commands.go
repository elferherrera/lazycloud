package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type commandFunc func(a *app, args []string)

var commands = map[string]commandFunc{
	"sort":  runSortCommand,
	"write": runWriteCommand,
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

func runWriteCommand(a *app, args []string) {
	if len(args) != 1 {
		a.statusText("[red]write: usage: write <file.(csv|json|yaml)>[-]")
		return
	}
	name := args[0]
	var data []byte
	var err error
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		data, err = a.csvBytes()
	case ".json":
		data, err = a.resourceBytes(func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") })
	case ".yaml", ".yml":
		data, err = a.resourceBytes(yaml.Marshal)
	default:
		a.statusText(fmt.Sprintf("[red]write: unsupported extension %q, want .csv, .json or .yaml[-]", filepath.Ext(name)))
		return
	}
	if err != nil {
		a.statusText("[red]write: [-]" + err.Error())
		return
	}
	if err := os.WriteFile(name, data, 0o644); err != nil {
		a.statusText("[red]write: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Wrote %d elements to %s[-]", len(a.visibleRows), name))
}

func (a *app) csvBytes() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(a.viewHeaders); err != nil {
		return nil, err
	}
	for _, row := range a.visibleRows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *app) resourceBytes(marshal func(any) ([]byte, error)) ([]byte, error) {
	resources, err := a.viewResources()
	if err != nil {
		return nil, err
	}
	data, err := marshal(resources)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data, nil
}

func (a *app) viewResources() ([]any, error) {
	def := views[appState.ViewKind]
	if def.resource == nil {
		return nil, errors.New("this view has no resource details")
	}
	resources := make([]any, 0, len(a.visibleRows))
	for _, row := range a.visibleRows {
		res, err := def.resource(a, row)
		if err != nil {
			return nil, err
		}
		resources = append(resources, res)
	}
	return resources, nil
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
