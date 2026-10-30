package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

const vpnKindHA = "ha"

func vpnGatewayRows(gateways []providers.VPNGateway) [][]string {
	rows := make([][]string, 0, len(gateways))
	for _, g := range gateways {
		rows = append(rows, []string{
			g.Type, g.Name, g.Region, g.Network, strings.Join(g.IPs, ","),
			strconv.Itoa(len(g.Tunnels)), g.Description, formatUpdatedAt(g.UpdatedAt),
		})
	}
	return rows
}

func vpnGatewayRowsAll(gateways []providers.VPNGateway) [][]string {
	rows := vpnGatewayRows(gateways)
	for i, g := range gateways {
		rows[i] = append(rows[i], g.ProjectID)
	}
	return rows
}

func vpnGatewayResource(a *app, projectID, region, name string) (any, error) {
	g, err := a.cache.GetVPNGateway(projectID, region, name)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, fmt.Errorf("vpn gateway not found in cache")
	}
	return g, nil
}

var vpnsView = viewDef{
	title:   "VPNs",
	headers: []string{"Type", "Name", "Region", "Network", "IPs", "Tunnels", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		gateways, err := a.cache.ListVPNGateways(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return vpnGatewayRows(gateways), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d VPN gateways for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching VPN gateways...[-]")
		go func() {
			projectID := appState.ProjectID
			gateways, err := providers.ListProjectVPNGateways(projectID)
			if err != nil {
				a.reportError("vpns", projectID, "Failed to fetch VPN gateways", err)
				return
			}
			if err := a.cache.SaveVPNGateways(projectID, gateways); err != nil {
				a.reportError("vpns", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("vpns", projectID, len(gateways))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewVPNs)
				a.statusText(fmt.Sprintf("[green]Fetched %d VPN gateways[-]", len(gateways)))
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return vpnGatewayResource(a, appState.ProjectID, row[2], row[1])
	},
	samples: []any{providers.VPNGateway{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPVPNGatewayURL(appState.ProjectID, a.info.GetCell(row, 2).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text == vpnKindHA)
	},
}

var vpnsAllView = viewDef{
	title:   "VPNs",
	headers: []string{"Type", "Name", "Region", "Network", "IPs", "Tunnels", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		gateways, err := a.cache.ListAllVPNGateways()
		if err != nil {
			return nil, err
		}
		return vpnGatewayRowsAll(gateways), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d VPN gateways for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching VPN gateways for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("vpns", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching VPN gateways for %s...[-]", p.ProjectID))
				})
				gateways, err := providers.ListProjectVPNGateways(p.ProjectID)
				if err != nil {
					a.reportError("vpns", p.ProjectID, "Failed to fetch VPN gateways for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveVPNGateways(p.ProjectID, gateways); err != nil {
					a.reportError("vpns", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("vpns", p.ProjectID, len(gateways))
				total += len(gateways)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewVPNsAll)
				status := fmt.Sprintf("[green]Fetched %d VPN gateways across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return vpnGatewayResource(a, row[8], row[2], row[1])
	},
	samples: []any{providers.VPNGateway{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPVPNGatewayURL(a.info.GetCell(row, 8).Text, a.info.GetCell(row, 2).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text == vpnKindHA)
	},
}

func showVPNs(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(vpnsViewKind())
}
