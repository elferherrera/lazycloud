package app

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	mmaid "github.com/aaronsb/mmaid-go"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func routeRows(routes []providers.Route) [][]string {
	rows := make([][]string, 0, len(routes))
	for _, r := range routes {
		rows = append(rows, []string{
			r.Name, r.Network, r.DestRange, strconv.FormatInt(r.Priority, 10),
			r.NextHopKind, r.NextHop, r.Description, formatUpdatedAt(r.UpdatedAt),
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

func routeResource(a *app, projectID, name string) (any, error) {
	r, err := a.cache.GetRoute(projectID, name)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("route not found in cache")
	}
	return r, nil
}

var routesView = viewDef{
	title:   "Routes",
	headers: []string{"Name", "Network", "Destination", "Priority", "Next Hop Type", "Next Hop", "Description", "Updated"},
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
	resource: func(a *app, row []string) (any, error) {
		return routeResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.Route{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPRoutesURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var routesAllView = viewDef{
	title:   "Routes",
	headers: []string{"Name", "Network", "Destination", "Priority", "Next Hop Type", "Next Hop", "Description", "Updated", "Project"},
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
	resource: func(a *app, row []string) (any, error) {
		return routeResource(a, row[8], row[0])
	},
	samples: []any{providers.Route{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPRoutesURL(a.info.GetCell(row, 8).Text, a.info.GetCell(row, 0).Text)
	},
}

func showRoutes(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(routesViewKind())
}

type hopTarget struct {
	id      string
	label   string
	rounded bool
	rule    providers.ForwardingRule
}

func vmMatchesIP(vm providers.VM, ip string) bool {
	for _, iface := range vm.Interfaces {
		if iface.InternalIP == ip || iface.ExternalIP == ip {
			return true
		}
	}
	return false
}

func (a *app) routeHopTargets(r *providers.Route) []hopTarget {
	if net.ParseIP(r.NextHop) == nil {
		return nil
	}
	var targets []hopTarget
	vms := 0
	lbs := 0
	if cached, err := a.cache.ListVMs(r.ProjectID); err == nil {
		for _, vm := range cached {
			if vmMatchesIP(vm, r.NextHop) {
				vms++
				targets = append(targets, hopTarget{
					id:    fmt.Sprintf("vm%d", vms),
					label: "VM: " + vm.Name + " (" + vm.Zone + ")",
				})
			}
		}
	}
	if rules, err := a.cache.ListForwardingRules(r.ProjectID); err == nil {
		for _, rule := range rules {
			if rule.IPAddress != r.NextHop {
				continue
			}
			lbs++
			label := "LB: " + rule.Name + " (" + rule.Region
			if rule.Scheme != "" {
				label += ", " + rule.Scheme
			}
			targets = append(targets, hopTarget{
				id:      fmt.Sprintf("lb%d", lbs),
				label:   label + ")",
				rounded: true,
				rule:    rule,
			})
		}
	}
	return targets
}

func routeHopLabel(r *providers.Route) string {
	switch {
	case r.NextHop == "":
		return "none"
	case r.NextHopKind == "":
		return r.NextHop
	}
	return r.NextHopKind + ": " + r.NextHop
}

func routeHopMermaid(a *app, r *providers.Route) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")
	if r.DestRange != "" {
		fmt.Fprintf(&sb, "  route[%q] -->|dest %s| hop{%q}\n", r.Name, r.DestRange, routeHopLabel(r))
	} else {
		fmt.Fprintf(&sb, "  route[%q] --> hop{%q}\n", r.Name, routeHopLabel(r))
	}
	targets := a.routeHopTargets(r)
	if len(targets) == 0 {
		if net.ParseIP(r.NextHop) != nil {
			sb.WriteString("  hop -.-> unknown[\"no VM or LB in cache\"]\n")
		}
		return sb.String()
	}
	for _, t := range targets {
		if t.rounded {
			fmt.Fprintf(&sb, "  hop --> %s(%q)\n", t.id, t.label)
			sb.WriteString(a.lbBackendsMermaid(t.id, t.rule))
		} else {
			fmt.Fprintf(&sb, "  hop --> %s[%q]\n", t.id, t.label)
		}
	}
	return sb.String()
}

func (a *app) lbBackendsMermaid(lbID string, rule providers.ForwardingRule) string {
	lb, err := a.cache.ResolveLoadBalancer(rule.ProjectID, rule.Region, rule.Name)
	if err != nil || lb == nil {
		return ""
	}
	var sb strings.Builder
	for i, svc := range lb.BackendServices {
		svcID := fmt.Sprintf("%s-svc%d", lbID, i+1)
		fmt.Fprintf(&sb, "  %s --> %s[%q]\n", lbID, svcID, "fwr: "+svc.Name)
		group := 0
		for _, b := range svc.Backends {
			if b.Group == "" {
				continue
			}
			group++
			fmt.Fprintf(&sb, "  %s --> %s{{%q}}\n", svcID, fmt.Sprintf("%s-grp%d", svcID, group), "group: "+b.Group)
		}
	}
	return sb.String()
}

func renderMermaidArt(source string) string {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldColumns, hadColumns := os.LookupEnv("COLUMNS")
	r, w, err := os.Pipe()
	if err != nil {
		return mmaid.Render(source)
	}
	os.Stdout = w
	os.Stderr = w
	os.Setenv("COLUMNS", "80")
	art := mmaid.Render(source)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	w.Close()
	io.Copy(io.Discard, r)
	r.Close()
	if hadColumns {
		os.Setenv("COLUMNS", oldColumns)
	} else {
		os.Unsetenv("COLUMNS")
	}
	return art
}

func (a *app) showRouteHopChart() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	def := views[appState.ViewKind]
	if def.resource == nil {
		return
	}
	res, err := def.resource(a, a.tableRowCells(row))
	if err != nil {
		a.statusText("[red]Failed to load route: [-]" + err.Error())
		return
	}
	route, ok := res.(*providers.Route)
	if !ok {
		a.statusText("[red]Failed to load route from cache[-]")
		return
	}
	art := renderMermaidArt(routeHopMermaid(a, route))
	overlay, text := a.newArtOverlay(fmt.Sprintf(" Route %s ", route.Name), art)
	a.pages.RemovePage("route")
	a.pages.AddPage("route", overlay, true, true)
	a.tapp.SetFocus(text)
}
