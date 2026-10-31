package app

import (
	"fmt"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func lbAddrPort(rule providers.ForwardingRule) string {
	if rule.PortRange == "" {
		return rule.IPAddress
	}
	return rule.IPAddress + ":" + rule.PortRange
}

func lbRows(cache *providers.Cache, projectID string, rules []providers.ForwardingRule) [][]string {
	rows := make([][]string, 0, len(rules))
	for _, r := range rules {
		rows = append(rows, []string{
			r.Name, lbAddrPort(r), r.Region, r.Protocol, r.Scheme, lbBackendColumn(cache, projectID, r.Region, r.Name), formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func lbRowsAll(cache *providers.Cache, rules []providers.ForwardingRule) [][]string {
	rows := make([][]string, 0, len(rules))
	for _, r := range rules {
		rows = append(rows, []string{
			r.Name, lbAddrPort(r), r.Region, r.Protocol, r.Scheme, lbBackendColumn(cache, r.ProjectID, r.Region, r.Name), formatUpdatedAt(r.UpdatedAt), r.ProjectID,
		})
	}
	return rows
}

func lbBackendColumn(cache *providers.Cache, projectID, region, name string) string {
	lb, err := cache.ResolveLoadBalancer(projectID, region, name)
	if err != nil || lb == nil || len(lb.BackendServices) == 0 {
		return ""
	}
	column := lb.BackendServices[0].Name
	if len(lb.BackendServices) > 1 {
		column += fmt.Sprintf(" (+%d)", len(lb.BackendServices)-1)
	}
	return column
}

func lbConsoleURL(cache *providers.Cache, projectID, region, name string) string {
	lb, err := cache.ResolveLoadBalancer(projectID, region, name)
	if err != nil || lb == nil {
		return providers.GCPLoadBalancerFrontendURL(projectID, name)
	}
	if lb.URLMap != nil && lb.URLMap.Name != "" {
		return providers.GCPLoadBalancerURL(projectID, lb.URLMap.Name)
	}
	return providers.GCPLoadBalancerFrontendURL(projectID, lb.Rule.Name)
}

func resolveLB(a *app, projectID, region, name string) (*providers.LoadBalancer, error) {
	lb, err := a.cache.ResolveLoadBalancer(projectID, region, name)
	if err != nil {
		return nil, err
	}
	if lb == nil {
		return nil, fmt.Errorf("load balancer not found in cache (press u to refresh)")
	}
	return lb, nil
}

func refreshProjectLBs(a *app, projectID string) (int, error) {
	rules, err := providers.ListProjectForwardingRules(projectID)
	if err != nil {
		return 0, err
	}
	proxies, err := providers.ListProjectTargetProxies(projectID)
	if err != nil {
		return 0, err
	}
	urlMaps, err := providers.ListProjectURLMaps(projectID)
	if err != nil {
		return 0, err
	}
	services, err := providers.ListProjectBackendServices(projectID)
	if err != nil {
		return 0, err
	}
	if err := a.cache.SaveForwardingRules(projectID, rules); err != nil {
		return 0, err
	}
	if err := a.cache.SaveTargetProxies(projectID, proxies); err != nil {
		return 0, err
	}
	if err := a.cache.SaveURLMaps(projectID, urlMaps); err != nil {
		return 0, err
	}
	if err := a.cache.SaveBackendServices(projectID, services); err != nil {
		return 0, err
	}
	a.logger.Update("lbs", projectID, len(rules))
	return len(rules), nil
}

func fetchProjectLBs(a *app, projectID string) (string, error) {
	count, err := refreshProjectLBs(a, projectID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Fetched %d load balancers", count), nil
}

var lbsView = viewDef{
	title:   "Load Balancers",
	headers: []string{"Name", "IP:Port", "Region", "Protocol", "Scheme", "Backends", "Updated"},
	load: func(a *app) ([][]string, error) {
		rules, err := a.cache.ListForwardingRules(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return lbRows(a.cache, appState.ProjectID, rules), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d load balancers for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewLBs, "load balancers", fetchProjectLBs),
	fetch:   fetchProjectLBs,
	resource: func(a *app, row []string) (any, error) {
		return resolveLB(a, appState.ProjectID, row[2], row[0])
	},
	samples: []any{providers.LoadBalancer{}},
	consoleURL: func(a *app, row int) string {
		return lbConsoleURL(a.cache, appState.ProjectID, a.info.GetCell(row, 2).Text, a.info.GetCell(row, 0).Text)
	},
}

var lbsAllView = viewDef{
	title:   "Load Balancers",
	headers: []string{"Name", "IP:Port", "Region", "Protocol", "Scheme", "Backends", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		rules, err := a.cache.ListAllForwardingRules()
		if err != nil {
			return nil, err
		}
		return lbRowsAll(a.cache, rules), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d load balancers for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching load balancers for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("lbs", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching load balancers for %s...", p.ProjectID))
				})
				count, err := refreshProjectLBs(a, p.ProjectID)
				if err != nil {
					a.reportError("lbs", p.ProjectID, "Failed to fetch load balancers for "+p.ProjectID, err)
					failed++
					continue
				}
				total += count
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewLBsAll)
				status := fmt.Sprintf("[green]Fetched %d load balancers across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return resolveLB(a, row[7], row[2], row[0])
	},
	samples: []any{providers.LoadBalancer{}},
	consoleURL: func(a *app, row int) string {
		projectID := a.info.GetCell(row, 7).Text
		return lbConsoleURL(a.cache, projectID, a.info.GetCell(row, 2).Text, a.info.GetCell(row, 0).Text)
	},
}

func lbDetails(lb *providers.LoadBalancer) string {
	var sb strings.Builder
	r := lb.Rule
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Project: [-]%s\n", r.ProjectID)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Name: [-]%s\n", r.Name)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Region: [-]%s\n", r.Region)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"IP:Port: [-]%s\n", lbAddrPort(r))
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Protocol: [-]%s\n", r.Protocol)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Scheme: [-]%s\n", r.Scheme)
	if r.Target != "" {
		if r.TargetKind != "" {
			fmt.Fprintf(&sb, theme.detailLabelTag()+"Target: [-]%s (%s)\n", r.Target, r.TargetKind)
		} else {
			fmt.Fprintf(&sb, theme.detailLabelTag()+"Target: [-]%s\n", r.Target)
		}
	}
	if lb.Proxy != nil {
		fmt.Fprintf(&sb, theme.detailLabelTag()+"Target proxy: [-]%s\n", lb.Proxy.Name)
	}
	if lb.URLMap != nil {
		fmt.Fprintf(&sb, theme.detailLabelTag()+"URL map: [-]%s (default: %s)\n", lb.URLMap.Name, lb.URLMap.DefaultService)
		for _, pr := range lb.URLMap.PathRules {
			fmt.Fprintf(&sb, theme.detailLabelTag()+"  %s -> [-]%s\n", strings.Join(pr.Paths, ","), pr.Service)
		}
	}
	for _, svc := range lb.BackendServices {
		fmt.Fprintf(&sb, theme.detailLabelTag()+"Backend service: [-]%s (%s, port %s)\n", svc.Name, svc.Protocol, svc.PortName)
		for _, b := range svc.Backends {
			if b.BalancingMode != "" {
				fmt.Fprintf(&sb, "  %s (%s)\n", b.Group, b.BalancingMode)
			} else {
				fmt.Fprintf(&sb, "  %s\n", b.Group)
			}
		}
		if len(svc.HealthChecks) > 0 {
			fmt.Fprintf(&sb, theme.detailLabelTag()+"  Health checks: [-]%s\n", strings.Join(svc.HealthChecks, ","))
		}
	}
	return sb.String()
}

func (a *app) showLBDetails(lb *providers.LoadBalancer) {
	overlay, text := a.newOverlay(fmt.Sprintf(" Load Balancer %s ", lb.Rule.Name), lbDetails(lb))
	a.pages.RemovePage("lb")
	a.pages.AddPage("lb", overlay, true, true)
	a.tapp.SetFocus(text)
}

func showLBs(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(lbsViewKind())
}
