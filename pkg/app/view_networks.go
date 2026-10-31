package app

import (
	"fmt"
	"time"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

const (
	networkKindVPC    = "vpc"
	networkKindSubnet = "subnet"
)

func formatUpdatedAt(updatedAt string) string {
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return updatedAt
}

func networkRows(networks []providers.Network, subnetworks []providers.Subnetwork) [][]string {
	rows := make([][]string, 0, len(networks)+len(subnetworks))
	for _, n := range networks {
		rows = append(rows, []string{
			networkKindVPC, n.Name, "", "", "", n.RoutingMode, "", formatUpdatedAt(n.UpdatedAt),
		})
	}
	for _, s := range subnetworks {
		rows = append(rows, []string{
			networkKindSubnet, s.Name, s.Region, s.Network, s.IPRange, "", s.Status, formatUpdatedAt(s.UpdatedAt),
		})
	}
	return rows
}

func networkRowsAll(networks []providers.Network, subnetworks []providers.Subnetwork) [][]string {
	rows := make([][]string, 0, len(networks)+len(subnetworks))
	for _, n := range networks {
		rows = append(rows, []string{
			networkKindVPC, n.Name, "", "", "", n.RoutingMode, "", formatUpdatedAt(n.UpdatedAt), n.ProjectID,
		})
	}
	for _, s := range subnetworks {
		rows = append(rows, []string{
			networkKindSubnet, s.Name, s.Region, s.Network, s.IPRange, "", s.Status, formatUpdatedAt(s.UpdatedAt), s.ProjectID,
		})
	}
	return rows
}

func networkResource(a *app, projectID, kind, region, name string) (any, error) {
	switch kind {
	case networkKindVPC:
		n, err := a.cache.GetNetwork(projectID, name)
		if err != nil {
			return nil, err
		}
		if n == nil {
			return nil, fmt.Errorf("network not found in cache")
		}
		return n, nil
	case networkKindSubnet:
		s, err := a.cache.GetSubnetwork(projectID, region, name)
		if err != nil {
			return nil, err
		}
		if s == nil {
			return nil, fmt.Errorf("subnetwork not found in cache")
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown network kind %q", kind)
	}
}

func fetchProjectNetworks(a *app, projectID string) (string, error) {
	networks, err := providers.ListProjectNetworks(projectID)
	if err != nil {
		return "", err
	}
	subnetworks, err := providers.ListProjectSubnetworks(projectID)
	if err != nil {
		return "", err
	}
	if err := a.cache.SaveNetworks(projectID, networks, subnetworks); err != nil {
		return "", err
	}
	a.logger.Update("networks", projectID, len(networks)+len(subnetworks))
	return fmt.Sprintf("Fetched %d networks and %d subnetworks", len(networks), len(subnetworks)), nil
}

var networksView = viewDef{
	title:   "Networks",
	headers: []string{"Type", "Name", "Region", "Network", "IP Range", "Routing", "Status", "Updated"},
	load: func(a *app) ([][]string, error) {
		networks, err := a.cache.ListNetworks(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		subnetworks, err := a.cache.ListSubnetworks(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return networkRows(networks, subnetworks), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d network resources for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewNetworks, "networks", fetchProjectNetworks),
	fetch:   fetchProjectNetworks,
	resource: func(a *app, row []string) (any, error) {
		return networkResource(a, appState.ProjectID, row[0], row[2], row[1])
	},
	samples: []any{providers.Network{}, providers.Subnetwork{}},
	consoleURL: func(a *app, row int) string {
		name := a.info.GetCell(row, 1).Text
		region := a.info.GetCell(row, 2).Text
		if a.info.GetCell(row, 0).Text == networkKindVPC {
			return providers.GCPNetworkURL(appState.ProjectID, name)
		}
		return providers.GCPSubnetworkURL(appState.ProjectID, region, name)
	},
}

var networksAllView = viewDef{
	title:   "Networks",
	headers: []string{"Type", "Name", "Region", "Network", "IP Range", "Routing", "Status", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		networks, err := a.cache.ListAllNetworks()
		if err != nil {
			return nil, err
		}
		subnetworks, err := a.cache.ListAllSubnetworks()
		if err != nil {
			return nil, err
		}
		return networkRowsAll(networks, subnetworks), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d network resources for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching networks for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("networks", "all", "Failed to list projects", err)
				return
			}
			totalNetworks, totalSubnets := 0, 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching networks for %s...", p.ProjectID))
				})
				networks, err := providers.ListProjectNetworks(p.ProjectID)
				if err != nil {
					a.reportError("networks", p.ProjectID, "Failed to fetch networks for "+p.ProjectID, err)
					failed++
					continue
				}
				subnetworks, err := providers.ListProjectSubnetworks(p.ProjectID)
				if err != nil {
					a.reportError("networks", p.ProjectID, "Failed to fetch subnetworks for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveNetworks(p.ProjectID, networks, subnetworks); err != nil {
					a.reportError("networks", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("networks", p.ProjectID, len(networks)+len(subnetworks))
				totalNetworks += len(networks)
				totalSubnets += len(subnetworks)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewNetworksAll)
				status := fmt.Sprintf("[green]Fetched %d networks and %d subnetworks across %d projects[-]",
					totalNetworks, totalSubnets, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return networkResource(a, row[8], row[0], row[2], row[1])
	},
	samples: []any{providers.Network{}, providers.Subnetwork{}},
	consoleURL: func(a *app, row int) string {
		projectID := a.info.GetCell(row, 8).Text
		name := a.info.GetCell(row, 1).Text
		region := a.info.GetCell(row, 2).Text
		if a.info.GetCell(row, 0).Text == networkKindVPC {
			return providers.GCPNetworkURL(projectID, name)
		}
		return providers.GCPSubnetworkURL(projectID, region, name)
	},
}
