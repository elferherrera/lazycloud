package main

import (
	"encoding/json"
	"fmt"
	"time"

	"lazycloud/cmd/providers"
)

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
	copyRow: func(a *app, row int) ([]byte, error) {
		projectID := a.info.GetCell(row, 5).Text
		ips, err := a.cache.GetIPAddresses(projectID, a.info.GetCell(row, 0).Text, a.info.GetCell(row, 1).Text)
		if err != nil {
			return nil, err
		}
		switch len(ips) {
		case 0:
			return nil, fmt.Errorf("IP address not found in cache")
		case 1:
			return json.MarshalIndent(ips[0], "", "  ")
		default:
			return json.MarshalIndent(ips, "", "  ")
		}
	},
	consoleURL: func(a *app, row int) string {
		projectID := a.info.GetCell(row, 5).Text
		return providers.GCPAddressesURL(projectID, a.info.GetCell(row, 0).Text)
	},
}
