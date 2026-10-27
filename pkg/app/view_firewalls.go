package app

import (
	"encoding/json"
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

func copyFirewallRule(a *app, projectID, name string) ([]byte, error) {
	r, err := a.cache.GetFirewallRule(projectID, name)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("firewall rule not found in cache")
	}
	return json.MarshalIndent(r, "", "  ")
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
		return fmt.Sprintf("[yellow]%d firewall rules for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching firewall rules...[-]")
		go func() {
			projectID := appState.ProjectID
			rules, err := providers.ListProjectFirewallRules(projectID)
			if err != nil {
				a.reportError("firewalls", projectID, "Failed to fetch firewall rules", err)
				return
			}
			if err := a.cache.SaveFirewallRules(projectID, rules); err != nil {
				a.reportError("firewalls", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("firewalls", projectID, len(rules))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewFirewalls)
				a.statusText(fmt.Sprintf("[green]Fetched %d firewall rules[-]", len(rules)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyFirewallRule(a, appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
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
		return fmt.Sprintf("[yellow]%d firewall rules for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching firewall rules for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("firewalls", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching firewall rules for %s...[-]", p.ProjectID))
				})
				rules, err := providers.ListProjectFirewallRules(p.ProjectID)
				if err != nil {
					a.reportError("firewalls", p.ProjectID, "Failed to fetch firewall rules for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveFirewallRules(p.ProjectID, rules); err != nil {
					a.reportError("firewalls", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("firewalls", p.ProjectID, len(rules))
				total += len(rules)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewFirewallsAll)
				status := fmt.Sprintf("[green]Fetched %d firewall rules across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyFirewallRule(a, a.info.GetCell(row, 8).Text, a.info.GetCell(row, 0).Text)
	},
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
