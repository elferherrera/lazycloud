package app

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func routeRows(routes []providers.Route) [][]string {
	rows := make([][]string, 0, len(routes))
	for _, r := range routes {
		rows = append(rows, []string{
			r.Name, r.Network, r.DestRange, strconv.FormatInt(r.Priority, 10),
			r.NextHopKind, r.NextHop, formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func routeRowsAll(routes []providers.Route) [][]string {
	rows := routeRows(routes)
	for i, r := range routes {
		rows[i] = append(rows[i], r.ProjectID)
	}
	return rows
}

func copyRoute(a *app, projectID, name string) ([]byte, error) {
	r, err := a.cache.GetRoute(projectID, name)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("route not found in cache")
	}
	return json.MarshalIndent(r, "", "  ")
}

var routesView = viewDef{
	title:   "Routes",
	headers: []string{"Name", "Network", "Destination", "Priority", "Next Hop Type", "Next Hop", "Updated"},
	load: func(a *app) ([][]string, error) {
		routes, err := a.cache.ListRoutes(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return routeRows(routes), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d routes for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching routes...[-]")
		go func() {
			projectID := appState.ProjectID
			routes, err := providers.ListProjectRoutes(projectID)
			if err != nil {
				a.reportError("routes", projectID, "Failed to fetch routes", err)
				return
			}
			if err := a.cache.SaveRoutes(projectID, routes); err != nil {
				a.reportError("routes", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("routes", projectID, len(routes))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewRoutes)
				a.statusText(fmt.Sprintf("[green]Fetched %d routes[-]", len(routes)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyRoute(a, appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPRoutesURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var routesAllView = viewDef{
	title:   "Routes",
	headers: []string{"Name", "Network", "Destination", "Priority", "Next Hop Type", "Next Hop", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		routes, err := a.cache.ListAllRoutes()
		if err != nil {
			return nil, err
		}
		return routeRowsAll(routes), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d routes for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching routes for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("routes", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching routes for %s...[-]", p.ProjectID))
				})
				routes, err := providers.ListProjectRoutes(p.ProjectID)
				if err != nil {
					a.reportError("routes", p.ProjectID, "Failed to fetch routes for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveRoutes(p.ProjectID, routes); err != nil {
					a.reportError("routes", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("routes", p.ProjectID, len(routes))
				total += len(routes)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewRoutesAll)
				status := fmt.Sprintf("[green]Fetched %d routes across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyRoute(a, a.info.GetCell(row, 7).Text, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPRoutesURL(a.info.GetCell(row, 7).Text, a.info.GetCell(row, 0).Text)
	},
}

func showRoutes(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(routesViewKind())
}
