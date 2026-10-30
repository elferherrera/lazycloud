package app

import (
	"fmt"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func nodePoolRows(pools []providers.ClusterNodePool, nodes []providers.ClusterNode) [][]string {
	rows := make([][]string, 0, len(pools))
	for _, np := range pools {
		count := fmt.Sprintf("%d", np.NodeCount)
		if len(nodes) > 0 {
			live := 0
			for _, n := range nodes {
				if n.NodePool == np.Name {
					live++
				}
			}
			count = fmt.Sprintf("%d", live)
		}
		rows = append(rows, []string{np.Name, np.Version, count})
	}
	return rows
}

func nodePoolResource(a *app, projectID, cluster, name string) (any, error) {
	cl, err := a.cache.GetCluster(projectID, cluster)
	if err != nil {
		return nil, err
	}
	if cl == nil {
		return nil, fmt.Errorf("cluster not found in cache")
	}
	for i := range cl.NodePools {
		if cl.NodePools[i].Name == name {
			return cl.NodePools[i], nil
		}
	}
	return nil, fmt.Errorf("node pool not found in cache")
}

var nodePoolsView = viewDef{
	title:   "Node Pools",
	headers: []string{"Name", "Version", "Nodes"},
	load: func(a *app) ([][]string, error) {
		if appState.ClusterProject == "" || appState.Cluster == "" {
			return nil, fmt.Errorf("no cluster selected")
		}
		cl, err := a.cache.GetCluster(appState.ClusterProject, appState.Cluster)
		if err != nil {
			return nil, err
		}
		if cl == nil {
			return nil, fmt.Errorf("cluster not found in cache")
		}
		nodes, err := a.cache.ListClusterNodes(appState.ClusterProject, appState.Cluster)
		if err != nil {
			return nil, err
		}
		return nodePoolRows(cl.NodePools, nodes), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d node pools for cluster %s (project %s)[-]", count, appState.Cluster, appState.ClusterProject)
	},
	refresh: func(a *app) {
		if appState.ClusterProject == "" || appState.Cluster == "" {
			a.statusText("[red]No cluster selected (press n on a cluster row)[-]")
			return
		}
		cl, err := a.cache.GetCluster(appState.ClusterProject, appState.Cluster)
		if err != nil {
			a.reportError("cluster-nodes", appState.ClusterProject, "Failed to load cluster", err)
			return
		}
		if cl == nil {
			a.statusText("[red]Cluster not found in cache (press u in the clusters view to refresh)[-]")
			return
		}
		a.statusText(fmt.Sprintf("[yellow]Fetching nodes for %s...[-]", appState.Cluster))
		go func() {
			projectID := appState.ClusterProject
			clusterName := appState.Cluster
			nodes, err := providers.ListClusterNodes(projectID, cl.Location, clusterName)
			if err != nil {
				a.reportError("cluster-nodes", projectID+"/"+clusterName, "Failed to fetch nodes", err)
				return
			}
			if err := a.cache.SaveClusterNodes(projectID, clusterName, nodes); err != nil {
				a.reportError("cluster-nodes", projectID+"/"+clusterName, "Failed to update cache", err)
				return
			}
			a.logger.Update("cluster-nodes", projectID+"/"+clusterName, len(nodes))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewNodePools)
				a.statusText(fmt.Sprintf("[green]Fetched %d nodes[-]", len(nodes)))
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return nodePoolResource(a, appState.ClusterProject, appState.Cluster, row[0])
	},
	samples: []any{providers.ClusterNodePool{}},
	consoleURL: func(a *app, row int) string {
		cl, _ := a.cache.GetCluster(appState.ClusterProject, appState.Cluster)
		location := ""
		if cl != nil {
			location = cl.Location
		}
		return providers.GCPClusterNodePoolURL(appState.ClusterProject, location, appState.Cluster, a.info.GetCell(row, 0).Text)
	},
}

func vmMatchesNodePool(vm providers.VM, pool string) bool {
	for _, v := range vm.Labels {
		if v == pool {
			return true
		}
	}
	return false
}

func (a *app) applyNodePoolVMFilter(pool string) {
	vms, err := a.cache.ListVMs(appState.ProjectID)
	if err != nil {
		a.statusText("[red]Failed to load VMs: [-]" + err.Error())
		return
	}
	matched := make([]providers.VM, 0, len(vms))
	for _, vm := range vms {
		if vmMatchesNodePool(vm, pool) {
			matched = append(matched, vm)
		}
	}
	appState.CmdMode = modeFilter
	a.setFilterBase(vmRows(matched))
	a.cmd.SetBorder(true)
	a.cmd.SetLabel(" / ")
	a.cmd.SetText("")
	a.applyFilter()
	a.tapp.SetFocus(a.info)
	if len(matched) == 0 {
		a.statusText(fmt.Sprintf("[red]No cached VMs have a label value %q (press u to refresh)[-]", pool))
		return
	}
	a.statusText(fmt.Sprintf("[yellow]%d VMs for node pool %s (Esc clears)[-]", len(matched), pool))
}

func (a *app) fetchNodePoolVMs(pool string) {
	projectID := appState.ProjectID
	vms, err := providers.ListProjectVMs(projectID)
	if err != nil {
		a.reportError("vms", projectID, "Failed to fetch VMs", err)
		return
	}
	if err := a.cache.SaveVMs(projectID, vms); err != nil {
		a.reportError("vms", projectID, "Failed to update cache", err)
		return
	}
	a.logger.Update("vms", projectID, len(vms))
	disks, err := providers.ListProjectDisks(projectID)
	if err != nil {
		a.reportError("disks", projectID, "Failed to fetch disks", err)
	} else if err := a.cache.SaveDisks(projectID, disks); err != nil {
		a.reportError("disks", projectID, "Failed to update cache", err)
	} else {
		a.logger.Update("disks", projectID, len(disks))
	}
	a.tapp.QueueUpdateDraw(func() {
		a.setView(viewVMs)
		a.applyNodePoolVMFilter(pool)
	})
}

func (a *app) showNodePoolVMs() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	pool := a.info.GetCell(row, 0).Text
	appState.AllProjects = false
	appState.ProjectID = appState.ClusterProject
	if p, err := a.cache.GetProject(appState.ClusterProject); err == nil && p != nil {
		appState.Name = p.Name
		appState.ProjectNumber = p.ProjectNumber
	}
	renderSummary(a.summary)
	a.setView(viewVMs)
	if len(a.allRows) == 0 {
		a.statusText(fmt.Sprintf("[yellow]Fetching VMs for %s...[-]", appState.ProjectID))
		go a.fetchNodePoolVMs(pool)
		return
	}
	a.applyNodePoolVMFilter(pool)
}

func (a *app) openNodePoolEvents() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	pool := a.info.GetCell(row, 0).Text
	eventsURL := providers.GCPNodePoolEventsURL(appState.ClusterProject, appState.Cluster, pool)
	if err := openBrowser(eventsURL); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Opened events for node pool %s in browser[-]", pool))
}

func (a *app) showClusterNodePools() {
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
	a.setView(viewNodePools)
}
