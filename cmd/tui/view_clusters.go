package main

import (
	"encoding/json"
	"fmt"

	"lazycloud/cmd/providers"
)

func nodePoolsColumn(pools []providers.ClusterNodePool) string {
	switch len(pools) {
	case 0:
		return ""
	case 1:
		return pools[0].Name
	default:
		return fmt.Sprintf("%s (+%d)", pools[0].Name, len(pools)-1)
	}
}

func clusterRows(clusters []providers.Cluster) [][]string {
	rows := make([][]string, 0, len(clusters))
	for _, cl := range clusters {
		rows = append(rows, []string{
			cl.Name, cl.Location, cl.Status, cl.Version, nodePoolsColumn(cl.NodePools), formatUpdatedAt(cl.CreatedAt), formatUpdatedAt(cl.UpdatedAt),
		})
	}
	return rows
}

func clusterRowsAll(clusters []providers.Cluster) [][]string {
	rows := clusterRows(clusters)
	for i, cl := range clusters {
		rows[i] = append(rows[i], cl.ProjectID)
	}
	return rows
}

func copyCluster(a *app, projectID, name string) ([]byte, error) {
	cl, err := a.cache.GetCluster(projectID, name)
	if err != nil {
		return nil, err
	}
	if cl == nil {
		return nil, fmt.Errorf("cluster not found in cache")
	}
	return json.MarshalIndent(cl, "", "  ")
}

var clustersView = viewDef{
	title:   "Kubernetes Clusters",
	headers: []string{"Name", "Location", "Status", "Version", "Node Pools", "Created", "Updated"},
	load: func(a *app) ([][]string, error) {
		clusters, err := a.cache.ListClusters(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return clusterRows(clusters), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d clusters for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching clusters...[-]")
		go func() {
			projectID := appState.ProjectID
			clusters, err := providers.ListProjectClusters(projectID)
			if err != nil {
				a.reportError("clusters", projectID, "Failed to fetch clusters", err)
				return
			}
			if err := a.cache.SaveClusters(projectID, clusters); err != nil {
				a.reportError("clusters", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("clusters", projectID, len(clusters))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewClusters)
				a.statusText(fmt.Sprintf("[green]Fetched %d clusters[-]", len(clusters)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyCluster(a, appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPClusterURL(appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

var clustersAllView = viewDef{
	title:   "Kubernetes Clusters",
	headers: []string{"Name", "Location", "Status", "Version", "Node Pools", "Created", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		clusters, err := a.cache.ListAllClusters()
		if err != nil {
			return nil, err
		}
		return clusterRowsAll(clusters), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d clusters for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching clusters for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("clusters", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching clusters for %s...[-]", p.ProjectID))
				})
				clusters, err := providers.ListProjectClusters(p.ProjectID)
				if err != nil {
					a.reportError("clusters", p.ProjectID, "Failed to fetch clusters for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveClusters(p.ProjectID, clusters); err != nil {
					a.reportError("clusters", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("clusters", p.ProjectID, len(clusters))
				total += len(clusters)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewClustersAll)
				status := fmt.Sprintf("[green]Fetched %d clusters across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyCluster(a, a.info.GetCell(row, 7).Text, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPClusterURL(a.info.GetCell(row, 7).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func showClusters(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(clustersViewKind())
}