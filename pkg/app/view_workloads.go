package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/elferherrera/lazycloud/cmd/providers"
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

func workloadResource(a *app, projectID, cluster, kind, namespace, name string) (any, error) {
	w, err := a.cache.GetWorkload(projectID, cluster, kind, namespace, name)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("workload not found in cache")
	}
	return w, nil
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
		a.statusProgress(fmt.Sprintf("Fetching workloads for %s...", appState.Cluster))
		a.goBackground(func() {
			projectID := appState.ClusterProject
			clusterName := appState.Cluster
			workloads, err := listClusterWorkloads(projectID, cl.Location, clusterName)
			if err != nil {
				a.reportError("workloads", projectID+"/"+clusterName, "Failed to fetch workloads", err)
				return
			}
			if err := a.cache.SaveWorkloads(projectID, clusterName, workloads); err != nil {
				a.reportError("workloads", projectID+"/"+clusterName, "Failed to update cache", err)
				return
			}
			a.logger.Update("workloads", projectID+"/"+clusterName, len(workloads))
			a.markViewUpdated(viewWorkloads, projectID)
			a.queueUpdateDraw(func() {
				a.setView(viewWorkloads)
				a.statusText(fmt.Sprintf("[green]Fetched %d workloads[-]", len(workloads)))
			})
		})
	},
	resource: func(a *app, row []string) (any, error) {
		return workloadResource(a, appState.ClusterProject, appState.Cluster, row[1], row[0], row[2])
	},
	samples: []any{providers.Workload{}},
	consoleURL: func(a *app, row int) string {
		cl, err := a.cache.GetCluster(appState.ClusterProject, appState.Cluster)
		if err != nil || cl == nil {
			return ""
		}
		return providers.GCPWorkloadDetailsURL(appState.ClusterProject, cl.Location, appState.Cluster,
			a.info.GetCell(row, 0).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 2).Text)
	},
}

func (a *app) focusedWorkload() (cl *providers.Cluster, namespace, name string, w *providers.Workload, ok bool) {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return nil, "", "", nil, false
	}
	cl, err := a.cache.GetCluster(appState.ClusterProject, appState.Cluster)
	if err != nil {
		a.statusText("[red]Failed to load cluster: [-]" + err.Error())
		return nil, "", "", nil, false
	}
	if cl == nil {
		a.statusText("[red]Cluster not found in cache (press u to refresh)[-]")
		return nil, "", "", nil, false
	}
	namespace = a.info.GetCell(row, 0).Text
	kind := a.info.GetCell(row, 1).Text
	name = a.info.GetCell(row, 2).Text
	w, err = a.cache.GetWorkload(appState.ClusterProject, appState.Cluster, kind, namespace, name)
	if err != nil {
		a.statusText("[red]Failed to load workload: [-]" + err.Error())
		return nil, "", "", nil, false
	}
	return cl, namespace, name, w, true
}

func (a *app) openWorkloadLogs() {
	cl, namespace, name, w, ok := a.focusedWorkload()
	if !ok {
		return
	}
	if w != nil && len(w.Pods) > 1 {
		a.showPodPicker(w.Pods, func(pod string) { a.openPodLogs(cl.Location, namespace, pod) })
		return
	}
	if w != nil && len(w.Pods) == 1 {
		a.openPodLogs(cl.Location, namespace, w.Pods[0])
		return
	}
	logsURL := providers.GCPWorkloadLogsURL(appState.ClusterProject, cl.Location, appState.Cluster, namespace, name)
	if err := openBrowser(logsURL); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText("[green]Opened workload logs in browser[-]")
}

func (a *app) openPodLogs(location, namespace, pod string) {
	logsURL := providers.GCPWorkloadPodLogsURL(appState.ClusterProject, location, appState.Cluster, namespace, pod)
	if err := openBrowser(logsURL); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Opened logs for %s in browser[-]", pod))
}

func (a *app) showPodPicker(pods []string, open func(pod string)) {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Pods ")
	for _, pod := range pods {
		list.AddItem(pod, "", 0, nil)
	}
	list.SetSelectedFunc(func(_ int, pod, _ string, _ rune) {
		a.pages.RemovePage("pods")
		a.tapp.SetFocus(a.info)
		open(pod)
	})
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}
		}
		return event
	})
	width := 0
	for _, pod := range pods {
		if n := utf8.RuneCountInString(pod) + 4; n > width {
			width = n
		}
	}
	a.pages.RemovePage("pods")
	a.pages.AddPage("pods", a.centeredOverlay(list, len(pods)+2, width), true, true)
	a.tapp.SetFocus(list)
}

func (a *app) openWorkloadEvents() {
	cl, namespace, name, w, ok := a.focusedWorkload()
	if !ok {
		return
	}
	if w != nil && len(w.Pods) > 1 {
		a.showPodPicker(w.Pods, func(pod string) { a.openPodEvents(cl.Location, namespace, pod) })
		return
	}
	if w != nil && len(w.Pods) == 1 {
		a.openPodEvents(cl.Location, namespace, w.Pods[0])
		return
	}
	eventsURL := providers.GCPWorkloadEventsURL(appState.ClusterProject, cl.Location, appState.Cluster, name)
	if err := openBrowser(eventsURL); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Opened events for %s in browser[-]", name))
}

func (a *app) openPodEvents(location, namespace, pod string) {
	eventsURL := providers.GCPWorkloadPodEventsURL(appState.ClusterProject, location, appState.Cluster, namespace, pod)
	if err := openBrowser(eventsURL); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Opened events for %s in browser[-]", pod))
}

func shellPath() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func workloadShellCommand() *exec.Cmd {
	shell := exec.Command(shellPath())
	shell.Env = append(os.Environ(), "LAZYCLOUD_CONTEXT=true")
	return shell
}

func kubectlSetContextArgs(contextName, namespace string) []string {
	return []string{"config", "set-context", contextName, "--namespace", namespace}
}

func findClusterContextName(names []string, projectID, location, cluster string) string {
	exact := fmt.Sprintf("gke_%s_%s_%s", projectID, location, cluster)
	var suffixProject, suffixAny, containsProject, containsAny []string
	for _, name := range names {
		if name == exact {
			return name
		}
		hasProject := strings.Contains(name, projectID)
		switch {
		case strings.HasSuffix(name, "_"+cluster) && hasProject:
			suffixProject = append(suffixProject, name)
		case strings.HasSuffix(name, "_"+cluster):
			suffixAny = append(suffixAny, name)
		case strings.Contains(name, cluster) && hasProject:
			containsProject = append(containsProject, name)
		case strings.Contains(name, cluster):
			containsAny = append(containsAny, name)
		}
	}
	for _, group := range [][]string{suffixProject, suffixAny, containsProject, containsAny} {
		if len(group) > 0 {
			return group[0]
		}
	}
	return ""
}

func kubectlClusterContext(projectID, location, cluster string) (string, error) {
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("kubectl get-contexts: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	names := make([]string, 0, 8)
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	name := findClusterContextName(names, projectID, location, cluster)
	if name == "" {
		return "", fmt.Errorf("no kubectl context found for cluster %q (press c on the cluster in the clusters view to fetch credentials first)", cluster)
	}
	return name, nil
}

func (a *app) openWorkloadShell() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	namespace := a.info.GetCell(row, 0).Text
	if namespace == "" || appState.Cluster == "" {
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
	contextName, err := kubectlClusterContext(appState.ClusterProject, cl.Location, appState.Cluster)
	if err != nil {
		a.logger.Error("workloads", appState.ClusterProject, "context lookup failed for "+appState.Cluster+" (namespace "+namespace+"): "+err.Error())
		a.statusText("[red]Failed to find kubectl context: [-]" + err.Error())
		return
	}
	setCtx := exec.Command("kubectl", kubectlSetContextArgs(contextName, namespace)...)
	setCtx.Stdin = os.Stdin
	setCtx.Stdout = os.Stdout
	setCtx.Stderr = os.Stderr
	shell := workloadShellCommand()
	shell.Stdin = os.Stdin
	shell.Stdout = os.Stdout
	shell.Stderr = os.Stderr
	var setErr, shellErr error
	a.tapp.Suspend(func() {
		setErr = setCtx.Run()
		shellErr = shell.Run()
	})
	if setErr != nil {
		a.logger.Error("workloads", appState.ClusterProject, "set-context failed for "+contextName+" (namespace "+namespace+"): "+setErr.Error())
		a.statusText("[red]kubectl set-context failed: [-]" + setErr.Error())
		return
	}
	var exitErr *exec.ExitError
	if shellErr != nil && !errors.As(shellErr, &exitErr) {
		a.logger.Error("workloads", appState.ClusterProject, "shell failed for "+contextName+" (namespace "+namespace+"): "+shellErr.Error())
		a.statusText("[red]Failed to open shell: [-]" + shellErr.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Shell session for %s/%s ended[-]", contextName, namespace))
}

func zellijInstalled() bool {
	_, err := exec.LookPath("zellij")
	return err == nil
}

func k9sViewForKind(kind string) string {
	switch strings.ToLower(kind) {
	case "deployment":
		return "dp"
	case "statefulset":
		return "sts"
	case "daemonset":
		return "ds"
	case "replicaset":
		return "rs"
	case "pod":
		return "po"
	}
	return "po"
}

func k9sCommand(kind, name string) string {
	view := k9sViewForKind(kind)
	if name == "" {
		return view
	}
	return view + " /" + name
}

func zellijRunArgs(namespace, kind, name string) []string {
	return []string{"run", "--floating", "--", "k9s", "--namespace", namespace, "--command", k9sCommand(kind, name)}
}

func (a *app) openWorkloadK9s() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	namespace := a.info.GetCell(row, 0).Text
	kind := a.info.GetCell(row, 1).Text
	name := a.info.GetCell(row, 2).Text
	if namespace == "" || kind == "" {
		return
	}
	if !zellijInstalled() {
		a.statusText("[red]zellij is not installed (install it to open k9s in a floating window)[-]")
		return
	}
	out, err := exec.Command("zellij", zellijRunArgs(namespace, kind, name)...).CombinedOutput()
	if err != nil {
		a.logger.Error("workloads", appState.ClusterProject, "zellij k9s failed for namespace "+namespace+": "+err.Error())
		if msg := strings.TrimSpace(string(out)); msg != "" {
			a.statusText("[red]Failed to open k9s in zellij: [-]" + msg)
			return
		}
		a.statusText("[red]Failed to open k9s in zellij: [-]" + err.Error())
		return
	}
	a.statusText(fmt.Sprintf("[green]Opened k9s for namespace %s in zellij[-]", namespace))
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
