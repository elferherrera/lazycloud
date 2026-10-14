package main

import "github.com/rivo/tview"

type viewKind string

const (
	viewProjects    viewKind = "projects"
	viewIPs         viewKind = "ips"
	viewIPsAll      viewKind = "ips-all"
	viewNetworks    viewKind = "networks"
	viewNetworksAll viewKind = "networks-all"
	viewAccounts    viewKind = "accounts"
	viewAccountsAll viewKind = "accounts-all"
	viewVMs         viewKind = "vms"
	viewVMsAll      viewKind = "vms-all"
)

type cmdMode string

const (
	modeCmd    cmdMode = "cmd"
	modeFilter cmdMode = "filter"
)

type State struct {
	Name          string
	ProjectID     string
	ProjectNumber string
	CmdMode       cmdMode
	ViewKind      viewKind
	AllProjects   bool
}

var appState State

func renderSummary(summary *tview.TextView) {
	if appState.AllProjects {
		summary.SetText("[yellow]Projects: [-]all")
		return
	}
	summary.SetText(
		"[yellow]Name: [-]" + appState.Name +
			"\n[yellow]Project ID: [-]" + appState.ProjectID +
			"\n[yellow]Project Number: [-]" + appState.ProjectNumber)
}

func ipsViewKind() viewKind {
	if appState.AllProjects {
		return viewIPsAll
	}
	return viewIPs
}

func networksViewKind() viewKind {
	if appState.AllProjects {
		return viewNetworksAll
	}
	return viewNetworks
}

func accountsViewKind() viewKind {
	if appState.AllProjects {
		return viewAccountsAll
	}
	return viewAccounts
}

func vmsViewKind() viewKind {
	if appState.AllProjects {
		return viewVMsAll
	}
	return viewVMs
}
