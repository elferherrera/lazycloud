package app

import (
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func cloudRouterASN(asn uint32) string {
	if asn == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(asn), 10)
}

func cloudRouterRows(routers []providers.CloudRouter) [][]string {
	rows := make([][]string, 0, len(routers))
	for _, r := range routers {
		rows = append(rows, []string{
			r.Name, r.Region, r.Network, cloudRouterASN(r.BGPASN),
			strconv.Itoa(len(r.Peers)), strconv.Itoa(len(r.NATs)),
			r.Description, formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func cloudRouterRowsAll(routers []providers.CloudRouter) [][]string {
	rows := cloudRouterRows(routers)
	for i, r := range routers {
		rows[i] = append(rows[i], r.ProjectID)
	}
	return rows
}

func cloudRouterResource(a *app, projectID, region, name string) (any, error) {
	r, err := a.cache.GetCloudRouter(projectID, region, name)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("cloud router not found in cache")
	}
	return r, nil
}

func fetchProjectCloudRouters(a *app, projectID string) (int, string, error) {
	routers, err := listProjectCloudRouters(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveCloudRouters(projectID, routers); err != nil {
		return 0, "", err
	}
	a.logger.Update("cloud-routers", projectID, len(routers))
	return len(routers), fmt.Sprintf("Fetched %d cloud routers", len(routers)), nil
}

var cloudRoutersView = viewDef{
	title:   "Cloud Routers",
	headers: []string{"Name", "Region", "Network", "ASN", "Peers", "NATs", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		routers, err := a.cache.ListCloudRouters(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return cloudRouterRows(routers), nil
	},
	refresh: projectRefresh(viewCloudRouters, "cloud routers", fetchProjectCloudRouters),
	fetch:   fetchProjectCloudRouters,
	resource: func(a *app, row []string) (any, error) {
		return cloudRouterResource(a, appState.ProjectID, row[1], row[0])
	},
	samples: []any{providers.CloudRouter{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPCloudRouterURL(appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

var cloudRoutersAllView = viewDef{
	title:   "Cloud Routers",
	headers: []string{"Name", "Region", "Network", "ASN", "Peers", "NATs", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		routers, err := a.cache.ListAllCloudRouters()
		if err != nil {
			return nil, err
		}
		return cloudRouterRowsAll(routers), nil
	},
	refresh: allProjectsRefresh(viewCloudRoutersAll, "cloud-routers", "cloud routers", "cloud routers", fetchProjectCloudRouters),
	resource: func(a *app, row []string) (any, error) {
		return cloudRouterResource(a, row[8], row[1], row[0])
	},
	samples: []any{providers.CloudRouter{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPCloudRouterURL(a.info.GetCell(row, 8).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func showCloudRouters(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(cloudRoutersViewKind())
}
