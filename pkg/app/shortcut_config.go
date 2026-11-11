package app

import (
	"fmt"
	"os"
	"slices"
)

var numericShortcutKeys = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}

// builtinNumericShortcuts holds the shipped entries, captured the first
// time a config is applied, so later reloads can restore them verbatim.
var builtinNumericShortcuts map[string]shortcut

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
	case viewPeerings:
		return showPeerings
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
	case viewPSC:
		return showPSC
	case viewInstanceGroups:
		return showInstanceGroups
	case viewReservations:
		return showReservations
	case viewImages:
		return showImages
	case viewPublicImages:
		return showPublicImages
	case viewInstanceTemplates:
		return showInstanceTemplates
	case viewDisks:
		return showDisks
	case viewSnapshots:
		return showSnapshots
	case viewIAM:
		return showIAM
	case viewRoles:
		return showRoles
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
	case viewBucketIAM:
		return func(a *app) {
			if appState.Bucket == "" {
				a.statusText("[red]No bucket selected[-]")
				return
			}
			a.setView(viewBucketIAM)
		}
	case viewIPsAll, viewNetworksAll, viewPeeringsAll, viewRoutesAll, viewFirewallsAll, viewAccountsAll,
		viewVMsAll, viewBucketsAll, viewLBsAll, viewClustersAll, viewDNSZonesAll, viewCloudRoutersAll,
		viewVPNsAll, viewPSCAll, viewInstanceGroupsAll, viewReservationsAll, viewImagesAll,
		viewInstanceTemplatesAll, viewDisksAll, viewSnapshotsAll, viewIAMAll, viewRolesAll:
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
	if builtinNumericShortcuts == nil {
		builtinNumericShortcuts = make(map[string]shortcut, len(defaultNumericShortcuts))
		for _, s := range shortcuts {
			if s.key >= '0' && s.key <= '9' {
				builtinNumericShortcuts[string(s.key)] = s
			}
		}
	}
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
		// A key keeping its default view also keeps the built-in entry, so
		// desc and actions stay identical to a config-less boot.
		if kind == defaultNumericShortcuts[key] {
			if s, ok := builtinNumericShortcuts[key]; ok {
				kept = append(kept, s)
			}
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
