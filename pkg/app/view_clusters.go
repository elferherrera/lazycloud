package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
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

func clusterResource(a *app, projectID, name string) (any, error) {
	cl, err := a.cache.GetCluster(projectID, name)
	if err != nil {
		return nil, err
	}
	if cl == nil {
		return nil, fmt.Errorf("cluster not found in cache")
	}
	return cl, nil
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
	resource: func(a *app, row []string) (any, error) {
		return clusterResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.Cluster{}},
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
	resource: func(a *app, row []string) (any, error) {
		return clusterResource(a, row[7], row[0])
	},
	samples: []any{providers.Cluster{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPClusterURL(a.info.GetCell(row, 7).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func (a *app) getClusterCredentials() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	projectID := appState.ProjectID
	if appState.ViewKind == viewClustersAll {
		projectID = a.info.GetCell(row, 7).Text
	}
	name := a.info.GetCell(row, 0).Text
	location := a.info.GetCell(row, 1).Text
	if name == "" || location == "" {
		a.statusText("[red]Cannot fetch credentials: cluster not found in cache (press u to refresh)[-]")
		return
	}
	scopeFlag := "--region"
	if strings.Count(location, "-") > 1 {
		scopeFlag = "--zone"
	}
	cmd := exec.Command("gcloud", "container", "clusters", "get-credentials", name,
		scopeFlag, location, "--project", projectID)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	var err error
	a.tapp.Suspend(func() {
		err = cmd.Run()
	})
	if err != nil {
		a.logger.Error("clusters", projectID, "get-credentials failed for "+name+" ("+location+"): "+err.Error())
		a.statusText("[red]get-credentials failed: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Fetched credentials for %s[-]", name))
}

func (a *app) openClusterNodeEvents() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	projectID := appState.ProjectID
	if appState.ViewKind == viewClustersAll {
		projectID = a.info.GetCell(row, 7).Text
	}
	clusterName := a.info.GetCell(row, 0).Text
	if err := openBrowser(providers.GCPClusterEventsURL(projectID, clusterName)); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Opened node events for %s in browser[-]", clusterName))
}

func showClusters(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(clustersViewKind())
}

func clusterDetails(cl *providers.Cluster) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[yellow]Project: [-]%s\n", cl.ProjectID)
	fmt.Fprintf(&sb, "[yellow]Name: [-]%s\n", cl.Name)
	fmt.Fprintf(&sb, "[yellow]Location: [-]%s\n", cl.Location)
	fmt.Fprintf(&sb, "[yellow]Status: [-]%s\n", cl.Status)
	fmt.Fprintf(&sb, "[yellow]Version: [-]%s\n", cl.Version)
	fmt.Fprintf(&sb, "[yellow]Endpoint: [-]%s\n", cl.Endpoint)
	fmt.Fprintf(&sb, "[yellow]Description: [-]%s\n", cl.Description)
	fmt.Fprintf(&sb, "[yellow]Node pools: [-]\n")
	for _, np := range cl.NodePools {
		fmt.Fprintf(&sb, "  %s (v%s, %d nodes)\n", np.Name, np.Version, np.NodeCount)
	}
	fmt.Fprintf(&sb, "[yellow]Created: [-]%s\n", formatUpdatedAt(cl.CreatedAt))
	fmt.Fprintf(&sb, "[yellow]Updated: [-]%s\n", formatUpdatedAt(cl.UpdatedAt))
	return sb.String()
}

func (a *app) showClusterDetails(cl *providers.Cluster) {
	overlay, text := a.newOverlay(fmt.Sprintf(" Cluster %s ", cl.Name), clusterDetails(cl))
	a.pages.RemovePage("cluster")
	a.pages.AddPage("cluster", overlay, true, true)
	a.tapp.SetFocus(text)
}
