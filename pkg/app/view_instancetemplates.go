package app

import (
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func templateBootDisk(t providers.InstanceTemplate) *providers.InstanceTemplateDisk {
	for i := range t.Disks {
		if t.Disks[i].Boot {
			return &t.Disks[i]
		}
	}
	if len(t.Disks) > 0 {
		return &t.Disks[0]
	}
	return nil
}

func instanceTemplateRows(templates []providers.InstanceTemplate) [][]string {
	rows := make([][]string, 0, len(templates))
	for _, t := range templates {
		bootImage := ""
		diskGB := ""
		diskType := ""
		if d := templateBootDisk(t); d != nil {
			bootImage = d.SourceImage
			diskGB = strconv.FormatInt(d.DiskSizeGB, 10)
			diskType = d.DiskType
		}
		network := ""
		subnetwork := ""
		if len(t.Interfaces) > 0 {
			network = t.Interfaces[0].Network
			subnetwork = t.Interfaces[0].Subnetwork
		}
		rows = append(rows, []string{
			t.Name, t.Location, t.MachineType, bootImage, diskGB, diskType,
			network, subnetwork, t.Description, formatUpdatedAt(t.UpdatedAt),
		})
	}
	return rows
}

func instanceTemplateRowsAll(templates []providers.InstanceTemplate) [][]string {
	rows := instanceTemplateRows(templates)
	for i, t := range templates {
		rows[i] = append(rows[i], t.ProjectID)
	}
	return rows
}

func instanceTemplateResource(a *app, projectID, location, name string) (any, error) {
	template, err := a.cache.GetInstanceTemplate(projectID, location, name)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("instance template not found in cache")
	}
	return template, nil
}

func fetchProjectInstanceTemplates(a *app, projectID string) (string, error) {
	templates, err := providers.ListProjectInstanceTemplates(projectID)
	if err != nil {
		return "", err
	}
	if err := a.cache.SaveInstanceTemplates(projectID, templates); err != nil {
		return "", err
	}
	a.logger.Update("instance-templates", projectID, len(templates))
	return fmt.Sprintf("Fetched %d instance templates", len(templates)), nil
}

var instanceTemplatesView = viewDef{
	title:   "Instance templates",
	headers: []string{"Name", "Location", "Machine Type", "Boot Image", "Disk GB", "Disk Type", "Network", "Subnetwork", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		templates, err := a.cache.ListInstanceTemplates(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return instanceTemplateRows(templates), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d instance templates for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewInstanceTemplates, "instance templates", fetchProjectInstanceTemplates),
	fetch:   fetchProjectInstanceTemplates,
	resource: func(a *app, row []string) (any, error) {
		return instanceTemplateResource(a, appState.ProjectID, row[1], row[0])
	},
	samples: []any{providers.InstanceTemplate{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPInstanceTemplateURL(appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

var instanceTemplatesAllView = viewDef{
	title:   "Instance templates",
	headers: []string{"Name", "Location", "Machine Type", "Boot Image", "Disk GB", "Disk Type", "Network", "Subnetwork", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		templates, err := a.cache.ListAllInstanceTemplates()
		if err != nil {
			return nil, err
		}
		return instanceTemplateRowsAll(templates), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d instance templates for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching instance templates for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("instance-templates", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching instance templates for %s...", p.ProjectID))
				})
				templates, err := providers.ListProjectInstanceTemplates(p.ProjectID)
				if err != nil {
					a.reportError("instance-templates", p.ProjectID, "Failed to fetch instance templates for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveInstanceTemplates(p.ProjectID, templates); err != nil {
					a.reportError("instance-templates", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("instance-templates", p.ProjectID, len(templates))
				total += len(templates)
			}
			a.markViewUpdated(viewInstanceTemplatesAll, appState.ProjectID)
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewInstanceTemplatesAll)
				status := fmt.Sprintf("[green]Fetched %d instance templates across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return instanceTemplateResource(a, row[10], row[1], row[0])
	},
	samples: []any{providers.InstanceTemplate{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPInstanceTemplateURL(a.info.GetCell(row, 10).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func showInstanceTemplates(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(instanceTemplatesViewKind())
}

func vmsFromTemplate(vms []providers.VM, template string) []providers.VM {
	matched := make([]providers.VM, 0, len(vms))
	for _, vm := range vms {
		if vm.InstanceTemplate == template {
			matched = append(matched, vm)
		}
	}
	return matched
}

func (a *app) applyTemplateVMFilter(template string) {
	vms, err := a.cache.ListVMs(appState.ProjectID)
	if err != nil {
		a.statusText("[red]Failed to load VMs: [-]" + err.Error())
		return
	}
	matched := vmsFromTemplate(vms, template)
	appState.CmdMode = modeFilter
	a.setFilterBase(vmRows(matched))
	a.cmd.SetBorder(true)
	a.cmd.SetLabel(" / ")
	a.cmd.SetText("")
	a.applyFilter()
	a.tapp.SetFocus(a.info)
	if len(matched) == 0 {
		a.statusText(fmt.Sprintf("[red]No cached VMs use template %q (press u to refresh)[-]", template))
		return
	}
	a.statusInfo(fmt.Sprintf("%d VMs for template %s (Esc clears)", len(matched), template))
}

func (a *app) fetchTemplateVMs(template string) {
	projectID := appState.ProjectID
	vms, err := providers.ListProjectVMs(projectID)
	if err != nil {
		a.reportError("vms", projectID, "Failed to fetch VMs", err)
		return
	}
	if err := a.cache.SaveVMs(projectID, vms); err != nil {
		a.reportError("vms", projectID, "Failed to update cache", err)
		return
	}
	a.logger.Update("vms", projectID, len(vms))
	a.tapp.QueueUpdateDraw(func() {
		a.setView(viewVMs)
		a.applyTemplateVMFilter(template)
	})
}

func (a *app) showTemplateVMs() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	template := a.info.GetCell(row, 0).Text
	projectID := appState.ProjectID
	if appState.ViewKind == viewInstanceTemplatesAll {
		projectID = a.info.GetCell(row, 10).Text
	}
	appState.AllProjects = false
	appState.ProjectID = projectID
	if p, err := a.cache.GetProject(projectID); err == nil && p != nil {
		appState.Name = p.Name
		appState.ProjectNumber = p.ProjectNumber
	}
	renderSummary(a.summary)
	a.setView(viewVMs)
	if len(a.allRows) == 0 {
		a.statusInfo(fmt.Sprintf("Fetching VMs for %s...", appState.ProjectID))
		go a.fetchTemplateVMs(template)
		return
	}
	a.applyTemplateVMFilter(template)
}
