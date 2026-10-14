package main

import (
	"encoding/json"
	"fmt"
	"time"

	"lazycloud/cmd/providers"
)

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
	copyRow: func(a *app, row int) ([]byte, error) {
		ips, err := a.cache.GetIPAddresses(appState.ProjectID, a.info.GetCell(row, 0).Text, a.info.GetCell(row, 1).Text)
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
		return providers.GCPAddressesURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}
