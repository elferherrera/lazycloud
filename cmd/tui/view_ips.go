package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"

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
	selectRow: func(a *app, row, _ int) {
		if row <= 0 || row >= a.info.GetRowCount() {
			return
		}
		ips, err := a.cache.GetIPAddresses(appState.ProjectID, a.info.GetCell(row, 0).Text, a.info.GetCell(row, 1).Text)
		if err != nil {
			a.statusText("[red]Failed to load IP address: [-]" + err.Error())
			return
		}
		if len(ips) == 0 {
			a.statusText("[red]IP address not found in cache (press u to refresh)[-]")
			return
		}
		a.showIPDetails(&ips[0])
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
	text := tview.NewTextView().SetDynamicColors(true).SetText(ipDetails(ip))
	text.SetBorder(true).SetTitle(fmt.Sprintf(" IP %s ", ip.Name))
	height := strings.Count(text.GetText(true), "\n") + 2
	overlay := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(text, height, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)
	a.pages.RemovePage("ip")
	a.pages.AddPage("ip", overlay, true, true)
}
