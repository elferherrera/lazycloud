package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"lazycloud/cmd/providers"
)

func workloadRows(workloads []providers.Workload) [][]string {
	rows := make([][]string, 0, len(workloads))
	for _, w := range workloads {
		rows = append(rows, []string{
			w.Namespace, w.Kind, w.Name, w.Replicas, strings.Join(w.Images, ","), formatUpdatedAt(w.UpdatedAt),
		})
	}
	return rows
}

func copyWorkload(a *app, projectID, cluster, kind, namespace, name string) ([]byte, error) {
	w, err := a.cache.GetWorkload(projectID, cluster, kind, namespace, name)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("workload not found in cache")
	}
	return json.MarshalIndent(w, "", "  ")
}

var workloadsView = viewDef{
	title:   "Workloads",
	headers: []string{"Namespace", "Kind", "Name", "Replicas", "Images", "Updated"},
	load: func(a *app) ([][]string, error) {
		workloads, err := a.cache.ListClusterWorkloads(appState.ClusterProject, appState.Cluster)
		if err != nil {
			return nil, err
		}
		return workloadRows(workloads), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d workloads for cluster %s (project %s)[-]", count, appState.Cluster, appState.ClusterProject)
	},
	refresh: func(a *app) {
		if appState.ClusterProject == "" || appState.Cluster == "" {
			a.statusText("[red]No cluster selected (press w on a cluster row)[-]")
			return
		}
		cl, err := a.cache.GetCluster(appState.ClusterProject, appState.Cluster)
		if err != nil {
			a.reportError("workloads", appState.ClusterProject, "Failed to load cluster", err)
			return
		}
		if cl == nil {
			a.statusText("[red]Cluster not found in cache (press u in the clusters view to refresh)[-]")
			return
		}
		a.statusText(fmt.Sprintf("[yellow]Fetching workloads for %s...[-]", appState.Cluster))
		go func() {
			projectID := appState.ClusterProject
			clusterName := appState.Cluster
			workloads, err := providers.ListClusterWorkloads(projectID, cl.Location, clusterName)
			if err != nil {
				a.reportError("workloads", projectID+"/"+clusterName, "Failed to fetch workloads", err)
				return
			}
			if err := a.cache.SaveWorkloads(projectID, clusterName, workloads); err != nil {
				a.reportError("workloads", projectID+"/"+clusterName, "Failed to update cache", err)
				return
			}
			a.logger.Update("workloads", projectID+"/"+clusterName, len(workloads))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewWorkloads)
				a.statusText(fmt.Sprintf("[green]Fetched %d workloads[-]", len(workloads)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyWorkload(a, appState.ClusterProject, appState.Cluster,
			a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text, a.info.GetCell(row, 2).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPWorkloadOverviewURL(appState.ClusterProject, a.info.GetCell(row, 0).Text)
	},
}

func (a *app) openWorkloadLogs() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	cl, err := a.cache.GetCluster(appState.ClusterProject, appState.Cluster)
	if err != nil {
		a.statusText("[red]Failed to load cluster: [-]" + err.Error())
		return
	}
	if cl == nil {
		a.statusText("[red]Cluster not found in cache (press u to refresh)[-]")
		return
	}
	logsURL := providers.GCPWorkloadLogsURL(appState.ClusterProject, cl.Location, appState.Cluster,
		a.info.GetCell(row, 0).Text, a.info.GetCell(row, 2).Text)
	if err := openBrowser(logsURL); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText("[green]Opened workload logs in browser[-]")
}

func (a *app) showClusterWorkloads() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	projectID := appState.ProjectID
	if appState.ViewKind == viewClustersAll {
		projectID = a.info.GetCell(row, 7).Text
	}
	appState.ClusterProject = projectID
	appState.Cluster = a.info.GetCell(row, 0).Text
	a.setView(viewWorkloads)
}
