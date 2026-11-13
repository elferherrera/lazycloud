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

func fetchProjectVPNs(a *app, projectID string) (int, string, error) {
	gateways, err := listProjectVPNGateways(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveVPNGateways(projectID, gateways); err != nil {
		return 0, "", err
	}
	a.logger.Update("vpns", projectID, len(gateways))
	return len(gateways), fmt.Sprintf("Fetched %d VPN gateways", len(gateways)), nil
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
	refresh: projectRefresh(viewVPNs, "VPN gateways", fetchProjectVPNs),
	fetch:   fetchProjectVPNs,
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
	refresh: allProjectsRefresh(viewVPNsAll, "vpns", "VPN gateways", "VPN gateways", fetchProjectVPNs),
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
