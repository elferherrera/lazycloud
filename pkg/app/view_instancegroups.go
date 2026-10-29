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

func instanceGroupMemberKeys(members []providers.InstanceGroupMember) map[string]bool {
	keys := make(map[string]bool, len(members))
	for _, m := range members {
		keys[m.Zone+"/"+m.Name] = true
	}
	return keys
}

func vmsInInstanceGroup(vms []providers.VM, members []providers.InstanceGroupMember) []providers.VM {
	keys := instanceGroupMemberKeys(members)
	matched := make([]providers.VM, 0, len(vms))
	for _, vm := range vms {
		if keys[vm.Zone+"/"+vm.Name] {
			matched = append(matched, vm)
		}
	}
	return matched
}

func (a *app) showInstanceGroupVMs() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	group := a.info.GetCell(row, 0).Text
	location := a.info.GetCell(row, 1).Text
	projectID := appState.ProjectID
	if appState.ViewKind == viewInstanceGroupsAll {
		projectID = a.info.GetCell(row, 8).Text
	}
	appState.AllProjects = false
	appState.ProjectID = projectID
	if p, err := a.cache.GetProject(projectID); err == nil && p != nil {
		appState.Name = p.Name
		appState.ProjectNumber = p.ProjectNumber
	}
	renderSummary(a.summary)
	a.statusText(fmt.Sprintf("[yellow]Fetching VMs for instance group %s...[-]", group))
	go a.fetchInstanceGroupVMs(location, group)
}

func (a *app) fetchInstanceGroupVMs(location, group string) {
	projectID := appState.ProjectID
	vms, err := a.cache.ListVMs(projectID)
	if err != nil {
		a.reportError("vms", projectID, "Failed to load VMs", err)
		return
	}
	if len(vms) == 0 {
		vms, err = providers.ListProjectVMs(projectID)
		if err != nil {
			a.reportError("vms", projectID, "Failed to fetch VMs", err)
			return
		}
		if err := a.cache.SaveVMs(projectID, vms); err != nil {
			a.reportError("vms", projectID, "Failed to update cache", err)
			return
		}
		a.logger.Update("vms", projectID, len(vms))
	}
	members, err := providers.ListInstanceGroupVMs(projectID, location, group)
	if err != nil {
		a.reportError("instance-groups", projectID+"/"+group, "Failed to fetch instance group VMs", err)
		return
	}
	a.tapp.QueueUpdateDraw(func() {
		a.setView(viewVMs)
		a.applyInstanceGroupVMFilter(group, members)
	})
}

func (a *app) applyInstanceGroupVMFilter(group string, members []providers.InstanceGroupMember) {
	vms, err := a.cache.ListVMs(appState.ProjectID)
	if err != nil {
		a.statusText("[red]Failed to load VMs: [-]" + err.Error())
		return
	}
	matched := vmsInInstanceGroup(vms, members)
	appState.CmdMode = modeFilter
	a.filterBase = vmRows(matched)
	a.cmd.SetBorder(true)
	a.cmd.SetLabel(" / ")
	a.cmd.SetText("")
	a.applyFilter()
	a.tapp.SetFocus(a.info)
	if len(matched) == 0 {
		a.statusText(fmt.Sprintf("[red]No cached VMs are members of %s (press u to refresh)[-]", group))
		return
	}
	a.statusText(fmt.Sprintf("[yellow]%d VMs for instance group %s (Esc clears)[-]", len(matched), group))
}
