package app

import (
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func firewallRows(rules []providers.FirewallRule) [][]string {
	rows := make([][]string, 0, len(rules))
	for _, r := range rules {
		rows = append(rows, []string{
			r.Name, r.Network, r.Direction, strconv.FormatInt(r.Priority, 10),
			r.Action, r.Source, r.Targets, formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func firewallRowsAll(rules []providers.FirewallRule) [][]string {
	rows := firewallRows(rules)
	for i, r := range rules {
		rows[i] = append(rows[i], r.ProjectID)
	}
	return rows
}

func firewallRuleResource(a *app, projectID, name string) (any, error) {
	r, err := a.cache.GetFirewallRule(projectID, name)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("firewall rule not found in cache")
	}
	return r, nil
}

func fetchProjectFirewalls(a *app, projectID string) (int, string, error) {
	rules, err := listProjectFirewallRules(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveFirewallRules(projectID, rules); err != nil {
		return 0, "", err
	}
	a.logger.Update("firewalls", projectID, len(rules))
	return len(rules), fmt.Sprintf("Fetched %d firewall rules", len(rules)), nil
}

var firewallsView = viewDef{
	title:   "Firewall Rules",
	headers: []string{"Name", "Network", "Direction", "Priority", "Action", "Source", "Targets", "Updated"},
	load: func(a *app) ([][]string, error) {
		rules, err := a.cache.ListFirewallRules(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return firewallRows(rules), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d firewall rules for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewFirewalls, "firewall rules", fetchProjectFirewalls),
	fetch:   fetchProjectFirewalls,
	resource: func(a *app, row []string) (any, error) {
		return firewallRuleResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.FirewallRule{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPFirewallsURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var firewallsAllView = viewDef{
	title:   "Firewall Rules",
	headers: []string{"Name", "Network", "Direction", "Priority", "Action", "Source", "Targets", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		rules, err := a.cache.ListAllFirewallRules()
		if err != nil {
			return nil, err
		}
		return firewallRowsAll(rules), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d firewall rules for all projects", count)
	},
	refresh: allProjectsRefresh(viewFirewallsAll, "firewalls", "firewall rules", "firewall rules", fetchProjectFirewalls),
	resource: func(a *app, row []string) (any, error) {
		return firewallRuleResource(a, row[8], row[0])
	},
	samples: []any{providers.FirewallRule{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPFirewallsURL(a.info.GetCell(row, 8).Text, a.info.GetCell(row, 0).Text)
	},
}

func showFirewalls(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(firewallsViewKind())
}
