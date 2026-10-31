package app

import (
	"fmt"
	neturl "net/url"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func projectRows(projects []providers.Project) [][]string {
	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{p.ProjectID, p.Name, p.ProjectNumber})
	}
	return rows
}

var projectsView = viewDef{
	title:   "Projects",
	headers: []string{"Project ID", "Name", "Project Number"},
	load: func(a *app) ([][]string, error) {
		projects, err := a.cache.ListProjects()
		if err != nil {
			return nil, err
		}
		return projectRows(projects), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching projects...")
		go func() {
			projects, err := providers.ListGcloudProjects()
			if err != nil {
				a.reportError("projects", "all", "Failed to fetch projects", err)
				return
			}
			if err := a.cache.SaveProjects(projects); err != nil {
				a.reportError("projects", "all", "Failed to update cache", err)
				return
			}
			saved, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("projects", "all", "Failed to list projects", err)
				return
			}
			a.logger.Update("projects", "all", len(saved))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewProjects)
				a.statusText(fmt.Sprintf("[green]Fetched %d projects[-]", len(saved)))
			})
		}()
	},
	selectRow: func(a *app, row, _ int) {
		appState.ProjectID = a.info.GetCell(row, 0).Text
		appState.Name = a.info.GetCell(row, 1).Text
		appState.ProjectNumber = a.info.GetCell(row, 2).Text
		appState.AllProjects = false
		renderSummary(a.summary)
		if err := providers.SaveLastProject(providers.LastProject{
			ProjectID:     appState.ProjectID,
			Name:          appState.Name,
			ProjectNumber: appState.ProjectNumber,
		}); err != nil {
			a.logger.Error("projects", appState.ProjectID, "Failed to save last project: "+err.Error())
		}
	},
	resource: func(a *app, row []string) (any, error) {
		p, err := a.cache.GetProject(row[0])
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, fmt.Errorf("project not found in cache")
		}
		return p, nil
	},
	samples: []any{providers.Project{}},
	consoleURL: func(a *app, row int) string {
		return "https://console.cloud.google.com/home/dashboard?project=" +
			neturl.QueryEscape(a.info.GetCell(row, 0).Text)
	},
}
