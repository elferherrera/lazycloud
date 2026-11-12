package app

import (
	"fmt"

	"github.com/elferherrera/lazycloud/cmd/providers"
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

func dnsZoneResource(a *app, projectID, name string) (any, error) {
	z, err := a.cache.GetDNSZone(projectID, name)
	if err != nil {
		return nil, err
	}
	if z == nil {
		return nil, fmt.Errorf("dns zone not found in cache")
	}
	return z, nil
}

func fetchProjectDNSZones(a *app, projectID string) (int, string, error) {
	zones, err := listProjectDNSZones(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveDNSZones(projectID, zones); err != nil {
		return 0, "", err
	}
	a.logger.Update("dns-zones", projectID, len(zones))
	return len(zones), fmt.Sprintf("Fetched %d dns zones", len(zones)), nil
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
		return fmt.Sprintf("%d dns zones for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewDNSZones, "dns zones", fetchProjectDNSZones),
	fetch:   fetchProjectDNSZones,
	resource: func(a *app, row []string) (any, error) {
		return dnsZoneResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.DNSZone{}},
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
		return fmt.Sprintf("%d dns zones for all projects", count)
	},
	refresh: allProjectsRefresh(viewDNSZonesAll, "dns-zones", "dns zones", "dns zones", fetchProjectDNSZones),
	resource: func(a *app, row []string) (any, error) {
		return dnsZoneResource(a, row[5], row[0])
	},
	samples: []any{providers.DNSZone{}},
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
