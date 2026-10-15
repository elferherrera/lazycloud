package main

import (
	"encoding/json"
	"fmt"

	"lazycloud/cmd/providers"
)

func dnsZoneRows(zones []providers.DNSZone) [][]string {
	rows := make([][]string, 0, len(zones))
	for _, z := range zones {
		rows = append(rows, []string{
			z.Name, z.DNSName, z.Visibility, z.Description, formatUpdatedAt(z.UpdatedAt),
		})
	}
	return rows
}

func dnsZoneRowsAll(zones []providers.DNSZone) [][]string {
	rows := dnsZoneRows(zones)
	for i, z := range zones {
		rows[i] = append(rows[i], z.ProjectID)
	}
	return rows
}

func copyDNSZone(a *app, projectID, name string) ([]byte, error) {
	z, err := a.cache.GetDNSZone(projectID, name)
	if err != nil {
		return nil, err
	}
	if z == nil {
		return nil, fmt.Errorf("dns zone not found in cache")
	}
	return json.MarshalIndent(z, "", "  ")
}

var dnsZonesView = viewDef{
	title:   "DNS Zones",
	headers: []string{"Name", "DNS Name", "Visibility", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		zones, err := a.cache.ListDNSZones(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return dnsZoneRows(zones), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d dns zones for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching dns zones...[-]")
		go func() {
			projectID := appState.ProjectID
			zones, err := providers.ListProjectDNSZones(projectID)
			if err != nil {
				a.reportError("dns-zones", projectID, "Failed to fetch dns zones", err)
				return
			}
			if err := a.cache.SaveDNSZones(projectID, zones); err != nil {
				a.reportError("dns-zones", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("dns-zones", projectID, len(zones))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewDNSZones)
				a.statusText(fmt.Sprintf("[green]Fetched %d dns zones[-]", len(zones)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyDNSZone(a, appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPDNSZoneURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var dnsZonesAllView = viewDef{
	title:   "DNS Zones",
	headers: []string{"Name", "DNS Name", "Visibility", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		zones, err := a.cache.ListAllDNSZones()
		if err != nil {
			return nil, err
		}
		return dnsZoneRowsAll(zones), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d dns zones for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching dns zones for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("dns-zones", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching dns zones for %s...[-]", p.ProjectID))
				})
				zones, err := providers.ListProjectDNSZones(p.ProjectID)
				if err != nil {
					a.reportError("dns-zones", p.ProjectID, "Failed to fetch dns zones for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveDNSZones(p.ProjectID, zones); err != nil {
					a.reportError("dns-zones", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("dns-zones", p.ProjectID, len(zones))
				total += len(zones)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewDNSZonesAll)
				status := fmt.Sprintf("[green]Fetched %d dns zones across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyDNSZone(a, a.info.GetCell(row, 5).Text, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPDNSZoneURL(a.info.GetCell(row, 5).Text, a.info.GetCell(row, 0).Text)
	},
}

func showDNSZones(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(dnsZonesViewKind())
}