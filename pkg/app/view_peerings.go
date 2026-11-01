package app

import (
	"fmt"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func peeringRouteExchange(p providers.NetworkPeering) string {
	parts := []string{}
	if p.ExchangeSubnetRoutes {
		parts = append(parts, "subnets")
	}
	if p.ExportCustomRoutes {
		parts = append(parts, "export-custom")
	}
	if p.ImportCustomRoutes {
		parts = append(parts, "import-custom")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func peeringRows(peerings []providers.NetworkPeering) [][]string {
	rows := make([][]string, 0, len(peerings))
	for _, p := range peerings {
		rows = append(rows, []string{
			p.Network, p.Name, p.PeerNetwork, p.PeerProject, p.State, peeringRouteExchange(p), formatUpdatedAt(p.UpdatedAt),
		})
	}
	return rows
}

func peeringRowsAll(peerings []providers.NetworkPeering) [][]string {
	rows := peeringRows(peerings)
	for i, p := range peerings {
		rows[i] = append(rows[i], p.ProjectID)
	}
	return rows
}

func peeringResource(a *app, projectID, network, name string) (any, error) {
	p, err := a.cache.GetNetworkPeering(projectID, network, name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("network peering not found in cache")
	}
	return p, nil
}

func fetchProjectPeerings(a *app, projectID string) (string, error) {
	peerings, err := providers.ListProjectPeerings(projectID)
	if err != nil {
		return "", err
	}
	if err := a.cache.SaveNetworkPeerings(projectID, peerings); err != nil {
		return "", err
	}
	a.logger.Update("peerings", projectID, len(peerings))
	return fmt.Sprintf("Fetched %d network peerings", len(peerings)), nil
}

var peeringsView = viewDef{
	title:   "VPC Peerings",
	headers: []string{"Network", "Peering", "Peer Network", "Peer Project", "State", "Route Exchange", "Updated"},
	load: func(a *app) ([][]string, error) {
		peerings, err := a.cache.ListNetworkPeerings(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return peeringRows(peerings), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d network peerings for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewPeerings, "network peerings", fetchProjectPeerings),
	fetch:   fetchProjectPeerings,
	resource: func(a *app, row []string) (any, error) {
		return peeringResource(a, appState.ProjectID, row[0], row[1])
	},
	samples: []any{providers.NetworkPeering{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPNetworkPeeringURL(appState.ProjectID, a.info.GetCell(row, 0).Text, a.info.GetCell(row, 1).Text)
	},
}

var peeringsAllView = viewDef{
	title:   "VPC Peerings",
	headers: []string{"Network", "Peering", "Peer Network", "Peer Project", "State", "Route Exchange", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		peerings, err := a.cache.ListAllNetworkPeerings()
		if err != nil {
			return nil, err
		}
		return peeringRowsAll(peerings), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d network peerings for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching network peerings for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("peerings", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching network peerings for %s...", p.ProjectID))
				})
				peerings, err := providers.ListProjectPeerings(p.ProjectID)
				if err != nil {
					a.reportError("peerings", p.ProjectID, "Failed to fetch network peerings for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveNetworkPeerings(p.ProjectID, peerings); err != nil {
					a.reportError("peerings", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("peerings", p.ProjectID, len(peerings))
				total += len(peerings)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewPeeringsAll)
				status := fmt.Sprintf("[green]Fetched %d network peerings across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return peeringResource(a, row[7], row[0], row[1])
	},
	samples: []any{providers.NetworkPeering{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPNetworkPeeringURL(a.info.GetCell(row, 7).Text, a.info.GetCell(row, 0).Text, a.info.GetCell(row, 1).Text)
	},
}

func showPeerings(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(peeringsViewKind())
}