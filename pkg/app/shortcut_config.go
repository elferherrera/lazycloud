package app

import (
	"fmt"
	"os"
	"slices"
)

var numericShortcutKeys = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}

var defaultNumericShortcuts = map[string]viewKind{
	"1": viewProjects,
	"2": viewVMs,
	"3": viewBuckets,
	"4": viewAccounts,
	"5": viewIPs,
	"6": viewNetworks,
	"7": viewLBs,
	"8": viewClusters,
	"9": viewRoutes,
	"0": viewFirewalls,
}

func viewShortcutAction(kind viewKind) func(a *app) {
	switch kind {
	case viewProjects:
		return func(a *app) { a.setView(viewProjects) }
	case viewVMs:
		return showVMs
	case viewBuckets:
		return showBuckets
	case viewAccounts:
		return showAccounts
	case viewIPs:
		return (*app).showIPs
	case viewNetworks:
		return (*app).showNetworks
	case viewLBs:
		return showLBs
	case viewClusters:
		return showClusters
	case viewRoutes:
		return showRoutes
	case viewFirewalls:
		return showFirewalls
	case viewDNSZones:
		return showDNSZones
	case viewCloudRouters:
		return showCloudRouters
	case viewVPNs:
		return showVPNs
	case viewInstanceGroups:
		return showInstanceGroups
	case viewWorkloads:
		return func(a *app) {
			if appState.Cluster == "" {
				a.statusText("[red]No cluster selected[-]")
				return
			}
			a.setView(viewWorkloads)
		}
	case viewNodePools:
		return func(a *app) {
			if appState.Cluster == "" {
				a.statusText("[red]No cluster selected[-]")
				return
			}
			a.setView(viewNodePools)
		}
	case viewDNSRecords:
		return func(a *app) {
			if appState.Zone == "" {
				a.statusText("[red]No DNS zone selected[-]")
				return
			}
			a.setView(viewDNSRecords)
		}
	case viewIPsAll, viewNetworksAll, viewRoutesAll, viewFirewallsAll, viewAccountsAll,
		viewVMsAll, viewBucketsAll, viewLBsAll, viewClustersAll, viewDNSZonesAll, viewCloudRoutersAll,
		viewVPNsAll, viewInstanceGroupsAll:
		return func(a *app) { a.setView(kind) }
	}
	return nil
}

func defaultShortcutConfig() map[string]string {
	config := make(map[string]string, len(defaultNumericShortcuts))
	for key, kind := range defaultNumericShortcuts {
		config[key] = string(kind)
	}
	return config
}

func applyShortcutConfig(config map[string]string) {
	kinds := make(map[string]viewKind, len(defaultNumericShortcuts))
	for key, kind := range defaultNumericShortcuts {
		kinds[key] = kind
	}
	for key, name := range config {
		if !slices.Contains(numericShortcutKeys, key) {
			fmt.Fprintf(os.Stderr, "shortcuts config: ignoring non-numeric key %q\n", key)
			continue
		}
		kind := viewKind(name)
		if viewShortcutAction(kind) == nil {
			fmt.Fprintf(os.Stderr, "shortcuts config: unknown view %q for key %q, keeping default\n", name, key)
			continue
		}
		kinds[key] = kind
	}
	kept := make([]shortcut, 0, len(shortcuts))
	for _, s := range shortcuts {
		if s.key >= '0' && s.key <= '9' {
			continue
		}
		kept = append(kept, s)
	}
	for _, key := range numericShortcutKeys {
		kind, ok := kinds[key]
		if !ok {
			continue
		}
		kept = append(kept, shortcut{
			key:  rune(key[0]),
			desc: "show " + views[kind].title,
			run:  viewShortcutAction(kind),
		})
	}
	shortcuts = kept
}
