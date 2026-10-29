package app

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func instanceGroupRows(groups []providers.InstanceGroup) [][]string {
	rows := make([][]string, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, []string{
			g.Name, g.Zone, g.Type, strconv.Itoa(int(g.Size)), g.Network,
			g.Subnetwork, g.Description, formatUpdatedAt(g.UpdatedAt),
		})
	}
	return rows
}

func instanceGroupRowsAll(groups []providers.InstanceGroup) [][]string {
	rows := instanceGroupRows(groups)
	for i, g := range groups {
		rows[i] = append(rows[i], g.ProjectID)
	}
	return rows
}

func copyInstanceGroup(a *app, projectID, zone, name string) ([]byte, error) {
	g, err := a.cache.GetInstanceGroup(projectID, zone, name)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, fmt.Errorf("instance group not found in cache")
	}
	return json.MarshalIndent(g, "", "  ")
}

var instanceGroupsView = viewDef{
	title:   "Instance Groups",
	headers: []string{"Name", "Location", "Type", "Size", "Network", "Subnetwork", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		groups, err := a.cache.ListInstanceGroups(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return instanceGroupRows(groups), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d instance groups for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching instance groups...[-]")
		go func() {
			projectID := appState.ProjectID
			groups, err := providers.ListProjectInstanceGroups(projectID)
			if err != nil {
				a.reportError("instance-groups", projectID, "Failed to fetch instance groups", err)
				return
			}
			if err := a.cache.SaveInstanceGroups(projectID, groups); err != nil {
				a.reportError("instance-groups", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("instance-groups", projectID, len(groups))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewInstanceGroups)
				a.statusText(fmt.Sprintf("[green]Fetched %d instance groups[-]", len(groups)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyInstanceGroup(a, appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPInstanceGroupURL(appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

var instanceGroupsAllView = viewDef{
	title:   "Instance Groups",
	headers: []string{"Name", "Location", "Type", "Size", "Network", "Subnetwork", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		groups, err := a.cache.ListAllInstanceGroups()
		if err != nil {
			return nil, err
		}
		return instanceGroupRowsAll(groups), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d instance groups for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching instance groups for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("instance-groups", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching instance groups for %s...[-]", p.ProjectID))
				})
				groups, err := providers.ListProjectInstanceGroups(p.ProjectID)
				if err != nil {
					a.reportError("instance-groups", p.ProjectID, "Failed to fetch instance groups for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveInstanceGroups(p.ProjectID, groups); err != nil {
					a.reportError("instance-groups", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("instance-groups", p.ProjectID, len(groups))
				total += len(groups)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewInstanceGroupsAll)
				status := fmt.Sprintf("[green]Fetched %d instance groups across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyInstanceGroup(a, a.info.GetCell(row, 8).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPInstanceGroupURL(a.info.GetCell(row, 8).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func showInstanceGroups(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(instanceGroupsViewKind())
}
