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
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d cloud routers for %s", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusInfo("Fetching cloud routers...")
		go func() {
			projectID := appState.ProjectID
			routers, err := providers.ListProjectCloudRouters(projectID)
			if err != nil {
				a.reportError("cloud-routers", projectID, "Failed to fetch cloud routers", err)
				return
			}
			if err := a.cache.SaveCloudRouters(projectID, routers); err != nil {
				a.reportError("cloud-routers", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("cloud-routers", projectID, len(routers))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewCloudRouters)
				a.statusText(fmt.Sprintf("[green]Fetched %d cloud routers[-]", len(routers)))
			})
		}()
	},
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
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d cloud routers for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching cloud routers for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("cloud-routers", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching cloud routers for %s...", p.ProjectID))
				})
				routers, err := providers.ListProjectCloudRouters(p.ProjectID)
				if err != nil {
					a.reportError("cloud-routers", p.ProjectID, "Failed to fetch cloud routers for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveCloudRouters(p.ProjectID, routers); err != nil {
					a.reportError("cloud-routers", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("cloud-routers", p.ProjectID, len(routers))
				total += len(routers)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewCloudRoutersAll)
				status := fmt.Sprintf("[green]Fetched %d cloud routers across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
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
