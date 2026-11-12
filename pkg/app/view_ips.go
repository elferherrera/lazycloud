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

type subnetKey struct {
	project, region, name string
}

func subnetNetworkMap(subnetworks []providers.Subnetwork) map[subnetKey]string {
	networks := make(map[subnetKey]string, len(subnetworks))
	for _, s := range subnetworks {
		networks[subnetKey{s.ProjectID, s.Region, s.Name}] = s.Network
	}
	return networks
}

func resolveIPNetworks(ips []providers.IPAddress, networks map[subnetKey]string) []providers.IPAddress {
	resolved := make([]providers.IPAddress, len(ips))
	copy(resolved, ips)
	for i := range resolved {
		ip := &resolved[i]
		if ip.Network == "" && ip.Subnetwork != "" && ip.Region != "" && ip.Region != "global" {
			if network, ok := networks[subnetKey{ip.ProjectID, ip.Region, ip.Subnetwork}]; ok && network != "" {
				ip.Network = network
			}
		}
	}
	return resolved
}

func ipAddressRows(ips []providers.IPAddress) [][]string {
	rows := make([][]string, 0, len(ips))
	for _, a := range ips {
		updatedAt := a.UpdatedAt
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			updatedAt = t.Local().Format("2006-01-02 15:04")
		}
		rows = append(rows, []string{a.Name, a.Address, a.AddressType, a.Region, a.Network, a.Subnetwork, a.Status, updatedAt})
	}
	return rows
}

func fetchProjectIPs(a *app, projectID string) (int, string, error) {
	ips, err := listProjectIPAddresses(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveIPAddresses(projectID, ips); err != nil {
		return 0, "", err
	}
	a.logger.Update("ips", projectID, len(ips))
	return len(ips), fmt.Sprintf("Fetched %d IP addresses", len(ips)), nil
}

var ipsView = viewDef{
	title:   "IP Addresses",
	headers: []string{"Name", "Address", "Type", "Region", "Network", "Subnet", "Status", "Updated"},
	load: func(a *app) ([][]string, error) {
		ips, err := a.cache.ListIPAddresses(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		networks := map[subnetKey]string{}
		if subs, err := a.cache.ListSubnetworks(appState.ProjectID); err == nil {
			networks = subnetNetworkMap(subs)
		}
		return ipAddressRows(resolveIPNetworks(ips, networks)), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d IP addresses for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewIPs, "IP addresses", fetchProjectIPs),
	fetch:   fetchProjectIPs,
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
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Project: [-]%s\n", ip.ProjectID)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Name: [-]%s\n", ip.Name)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Address: [-]%s\n", ip.Address)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Type: [-]%s\n", ip.AddressType)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Status: [-]%s\n", ip.Status)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Region: [-]%s\n", ip.Region)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Network: [-]%s\n", ip.Network)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Subnetwork: [-]%s\n", ip.Subnetwork)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Updated: [-]%s\n", formatUpdatedAt(ip.UpdatedAt))
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
		rows = append(rows, []string{a.Name, a.Address, a.AddressType, a.Region, a.Network, a.Subnetwork, a.Status, updatedAt, a.ProjectID})
	}
	return rows
}

var ipsAllView = viewDef{
	title:   "IP Addresses",
	headers: []string{"Name", "Address", "Type", "Region", "Network", "Subnet", "Status", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		ips, err := a.cache.ListAllIPAddresses()
		if err != nil {
			return nil, err
		}
		networks := map[subnetKey]string{}
		if subs, err := a.cache.ListAllSubnetworks(); err == nil {
			networks = subnetNetworkMap(subs)
		}
		return ipAddressRowsAll(resolveIPNetworks(ips, networks)), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d IP addresses for all projects", count)
	},
	refresh: allProjectsRefresh(viewIPsAll, "ips", "IPs", "IP addresses", fetchProjectIPs),
	resource: func(a *app, row []string) (any, error) {
		return ipAddressResource(a, row[8], row[0], row[1])
	},
	samples: []any{providers.IPAddress{}},
	consoleURL: func(a *app, row int) string {
		projectID := a.info.GetCell(row, 8).Text
		return providers.GCPAddressesURL(projectID, a.info.GetCell(row, 0).Text)
	},
}
