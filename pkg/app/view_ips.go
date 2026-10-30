package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func ipAddressResource(a *app, projectID, name, address string) (any, error) {
	ips, err := a.cache.GetIPAddresses(projectID, name, address)
	if err != nil {
		return nil, err
	}
	switch len(ips) {
	case 0:
		return nil, fmt.Errorf("IP address not found in cache")
	case 1:
		return ips[0], nil
	default:
		return ips, nil
	}
}

func ipAddressRows(ips []providers.IPAddress) [][]string {
	rows := make([][]string, 0, len(ips))
	for _, a := range ips {
		updatedAt := a.UpdatedAt
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			updatedAt = t.Local().Format("2006-01-02 15:04")
		}
		rows = append(rows, []string{a.Name, a.Address, a.Region, a.Status, updatedAt})
	}
	return rows
}

var ipsView = viewDef{
	title:   "IP Addresses",
	headers: []string{"Name", "Address", "Region", "Status", "Updated"},
	load: func(a *app) ([][]string, error) {
		ips, err := a.cache.ListIPAddresses(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return ipAddressRows(ips), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d IP addresses for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching IP addresses...[-]")
		go func() {
			projectID := appState.ProjectID
			ips, err := providers.ListProjectIPAddresses(projectID)
			if err != nil {
				a.reportError("ips", projectID, "Failed to fetch IPs", err)
				return
			}
			if err := a.cache.SaveIPAddresses(projectID, ips); err != nil {
				a.reportError("ips", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("ips", projectID, len(ips))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewIPs)
				a.statusText(fmt.Sprintf("[green]Fetched %d IP addresses[-]", len(ips)))
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return ipAddressResource(a, appState.ProjectID, row[0], row[1])
	},
	samples: []any{providers.IPAddress{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPAddressesURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

func ipDetails(ip *providers.IPAddress) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[yellow]Project: [-]%s\n", ip.ProjectID)
	fmt.Fprintf(&sb, "[yellow]Name: [-]%s\n", ip.Name)
	fmt.Fprintf(&sb, "[yellow]Address: [-]%s\n", ip.Address)
	fmt.Fprintf(&sb, "[yellow]Type: [-]%s\n", ip.AddressType)
	fmt.Fprintf(&sb, "[yellow]Status: [-]%s\n", ip.Status)
	fmt.Fprintf(&sb, "[yellow]Region: [-]%s\n", ip.Region)
	fmt.Fprintf(&sb, "[yellow]Network: [-]%s\n", ip.Network)
	fmt.Fprintf(&sb, "[yellow]Subnetwork: [-]%s\n", ip.Subnetwork)
	fmt.Fprintf(&sb, "[yellow]Updated: [-]%s\n", formatUpdatedAt(ip.UpdatedAt))
	return sb.String()
}

func (a *app) showIPDetails(ip *providers.IPAddress) {
	if ip.Network == "" && ip.Subnetwork != "" && ip.Region != "" && ip.Region != "global" {
		if sub, err := a.cache.GetSubnetwork(ip.ProjectID, ip.Region, ip.Subnetwork); err == nil && sub != nil && sub.Network != "" {
			resolved := *ip
			resolved.Network = sub.Network
			ip = &resolved
		}
	}
	overlay, text := a.newOverlay(fmt.Sprintf(" IP %s ", ip.Name), ipDetails(ip))
	a.pages.RemovePage("ip")
	a.pages.AddPage("ip", overlay, true, true)
	a.tapp.SetFocus(text)
}

func ipAddressRowsAll(ips []providers.IPAddress) [][]string {
	rows := make([][]string, 0, len(ips))
	for _, a := range ips {
		updatedAt := a.UpdatedAt
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			updatedAt = t.Local().Format("2006-01-02 15:04")
		}
		rows = append(rows, []string{a.Name, a.Address, a.Region, a.Status, updatedAt, a.ProjectID})
	}
	return rows
}

var ipsAllView = viewDef{
	title:   "IP Addresses",
	headers: []string{"Name", "Address", "Region", "Status", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		ips, err := a.cache.ListAllIPAddresses()
		if err != nil {
			return nil, err
		}
		return ipAddressRowsAll(ips), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d IP addresses for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching IP addresses for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("ips", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching IPs for %s...[-]", p.ProjectID))
				})
				ips, err := providers.ListProjectIPAddresses(p.ProjectID)
				if err != nil {
					a.reportError("ips", p.ProjectID, "Failed to fetch IPs for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveIPAddresses(p.ProjectID, ips); err != nil {
					a.reportError("ips", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("ips", p.ProjectID, len(ips))
				total += len(ips)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewIPsAll)
				status := fmt.Sprintf("[green]Fetched %d IP addresses across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return ipAddressResource(a, row[5], row[0], row[1])
	},
	samples: []any{providers.IPAddress{}},
	consoleURL: func(a *app, row int) string {
		projectID := a.info.GetCell(row, 5).Text
		return providers.GCPAddressesURL(projectID, a.info.GetCell(row, 0).Text)
	},
}
