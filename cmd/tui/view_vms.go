package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func vmRows(vms []providers.VM) [][]string {
	rows := make([][]string, 0, len(vms))
	for _, vm := range vms {
		internalIPs := make([]string, 0, len(vm.Interfaces))
		for _, iface := range vm.Interfaces {
			if iface.InternalIP != "" {
				internalIPs = append(internalIPs, iface.InternalIP)
			}
		}
		rows = append(rows, []string{
			vm.Name, vm.Zone, vm.MachineType, vm.Status,
			strings.Join(internalIPs, ","), formatUpdatedAt(vm.UpdatedAt),
		})
	}
	return rows
}

func vmRowsAll(vms []providers.VM) [][]string {
	rows := vmRows(vms)
	for i, vm := range vms {
		rows[i] = append(rows[i], vm.ProjectID)
	}
	return rows
}

func copyVM(a *app, projectID, zone, name string) ([]byte, error) {
	vm, err := a.cache.GetVM(projectID, zone, name)
	if err != nil {
		return nil, err
	}
	if vm == nil {
		return nil, fmt.Errorf("VM not found in cache")
	}
	return json.MarshalIndent(vm, "", "  ")
}

var vmsView = viewDef{
	title:   "VMs",
	headers: []string{"Name", "Zone", "Machine Type", "Status", "Internal IPs", "Updated"},
	load: func(a *app) ([][]string, error) {
		vms, err := a.cache.ListVMs(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return vmRows(vms), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d VMs for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching VMs...[-]")
		go func() {
			projectID := appState.ProjectID
			vms, err := providers.ListProjectVMs(projectID)
			if err != nil {
				a.reportError("vms", projectID, "Failed to fetch VMs", err)
				return
			}
			if err := a.cache.SaveVMs(projectID, vms); err != nil {
				a.reportError("vms", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("vms", projectID, len(vms))
			disks, err := providers.ListProjectDisks(projectID)
			if err != nil {
				a.reportError("disks", projectID, "Failed to fetch disks", err)
			} else if err := a.cache.SaveDisks(projectID, disks); err != nil {
				a.reportError("disks", projectID, "Failed to update cache", err)
			} else {
				a.logger.Update("disks", projectID, len(disks))
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewVMs)
				a.statusText(fmt.Sprintf("[green]Fetched %d VMs[-]", len(vms)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyVM(a, appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPVMURL(appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

var vmsAllView = viewDef{
	title:   "VMs",
	headers: []string{"Name", "Zone", "Machine Type", "Status", "Internal IPs", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		vms, err := a.cache.ListAllVMs()
		if err != nil {
			return nil, err
		}
		return vmRowsAll(vms), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d VMs for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching VMs for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("vms", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching VMs for %s...[-]", p.ProjectID))
				})
				vms, err := providers.ListProjectVMs(p.ProjectID)
				if err != nil {
					a.reportError("vms", p.ProjectID, "Failed to fetch VMs for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveVMs(p.ProjectID, vms); err != nil {
					a.reportError("vms", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("vms", p.ProjectID, len(vms))
				disks, err := providers.ListProjectDisks(p.ProjectID)
				if err != nil {
					a.reportError("disks", p.ProjectID, "Failed to fetch disks for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveDisks(p.ProjectID, disks); err != nil {
					a.reportError("disks", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("disks", p.ProjectID, len(disks))
				total += len(vms)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewVMsAll)
				status := fmt.Sprintf("[green]Fetched %d VMs across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyVM(a, a.info.GetCell(row, 6).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPVMURL(a.info.GetCell(row, 6).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func (a *app) lookupCachedDisk(vm *providers.VM, d *providers.VMDisk) *providers.Disk {
	for _, name := range []string{d.Name, d.DeviceName} {
		if name == "" {
			continue
		}
		cached, err := a.cache.GetDisk(vm.ProjectID, vm.Zone, name)
		if err == nil && cached != nil {
			return cached
		}
	}
	return nil
}

func (a *app) vmDetails(vm *providers.VM) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[yellow]Project: [-]%s\n", vm.ProjectID)
	fmt.Fprintf(&sb, "[yellow]Zone: [-]%s\n", vm.Zone)
	fmt.Fprintf(&sb, "[yellow]Machine type: [-]%s\n", vm.MachineType)
	fmt.Fprintf(&sb, "[yellow]Status: [-]%s\n", vm.Status)
	if len(vm.NetworkTags) > 0 {
		fmt.Fprintf(&sb, "[yellow]Network tags: [-]%s\n", strings.Join(vm.NetworkTags, ","))
	}
	if len(vm.Labels) > 0 {
		keys := make([]string, 0, len(vm.Labels))
		for k := range vm.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(&sb, "[yellow]Labels: [-]\n")
		for _, k := range keys {
			fmt.Fprintf(&sb, "  %s=%s\n", k, vm.Labels[k])
		}
	}
	if len(vm.ServiceAccounts) > 0 {
		fmt.Fprintf(&sb, "[yellow]Service accounts: [-]\n")
		for _, sa := range vm.ServiceAccounts {
			line := "  " + sa.Email
			if len(sa.Scopes) > 0 {
				line += "  (" + strings.Join(sa.Scopes, ",") + ")"
			}
			sb.WriteString(line + "\n")
		}
	}
	if vm.InstanceTemplate != "" {
		fmt.Fprintf(&sb, "[yellow]Instance template: [-]%s\n", vm.InstanceTemplate)
	}
	fmt.Fprintf(&sb, "[yellow]Updated: [-]%s\n\n", formatUpdatedAt(vm.UpdatedAt))

	sb.WriteString("[yellow]Network interfaces:[-]\n")
	if len(vm.Interfaces) == 0 {
		sb.WriteString("  none\n")
	}
	for _, iface := range vm.Interfaces {
		line := "  " + iface.Network + "/" + iface.Subnetwork + "  " + iface.InternalIP
		if iface.ExternalIP != "" {
			line += "  (ext: " + iface.ExternalIP + ")"
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n[yellow]Disks:[-]\n")
	if vm.BootDisk == nil && len(vm.DataDisks) == 0 {
		sb.WriteString("  none\n")
	}
	appendDisk := func(label string, d *providers.VMDisk) {
		parts := []string{"  [" + label + "]"}
		name := d.Name
		if name == "" {
			name = d.DeviceName
		}
		parts = append(parts, name)
		if d.DeviceName != "" && d.DeviceName != name {
			parts = append(parts, "device:"+d.DeviceName)
		}
		if d.Mode != "" {
			parts = append(parts, d.Mode)
		}
		if d.AutoDelete {
			parts = append(parts, "auto-delete")
		}
		sizeGB := d.SizeGB
		if cached := a.lookupCachedDisk(vm, d); cached != nil {
			if cached.Type != "" {
				parts = append(parts, cached.Type)
			}
			if cached.Status != "" {
				parts = append(parts, cached.Status)
			}
			if cached.SizeGB > 0 {
				sizeGB = cached.SizeGB
			}
		}
		parts = append(parts, fmt.Sprintf("%d GB", sizeGB))
		sb.WriteString(strings.Join(parts, "  ") + "\n")
	}
	if vm.BootDisk != nil {
		appendDisk("boot", vm.BootDisk)
	}
	for i := range vm.DataDisks {
		appendDisk("data", &vm.DataDisks[i])
	}
	return sb.String()
}

func (a *app) showVMDetails(vm *providers.VM) {
	overlay, text := a.newOverlay(fmt.Sprintf(" VM %s ", vm.Name), a.vmDetails(vm))
	a.pages.RemovePage("vm")
	a.pages.AddPage("vm", overlay, true, true)
	a.tapp.SetFocus(text)
}

func (a *app) openVMLogging() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	projectID := appState.ProjectID
	if appState.ViewKind == viewVMsAll {
		projectID = a.info.GetCell(row, 6).Text
	}
	vm, err := a.cache.GetVM(projectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	if err != nil {
		a.statusText("[red]Failed to load VM: [-]" + err.Error())
		return
	}
	if vm == nil {
		a.statusText("[red]VM not found in cache (press u to refresh)[-]")
		return
	}
	if err := openBrowser(providers.GCPVMLogsURL(projectID, vm.InstanceID, vm.Name)); err != nil {
		a.statusText("[red]Failed to open browser: [-]" + err.Error())
		return
	}
	a.statusText("[green]Opened VM logs in browser[-]")
}

func (a *app) sshVM() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	projectID := appState.ProjectID
	if appState.ViewKind == viewVMsAll {
		projectID = a.info.GetCell(row, 6).Text
	}
	zone := a.info.GetCell(row, 1).Text
	name := a.info.GetCell(row, 0).Text
	if zone == "" || name == "" {
		a.statusText("[red]Cannot SSH: VM not found in cache (press u to refresh)[-]")
		return
	}
	cmd := exec.Command("gcloud", "compute", "ssh", name,
		"--zone", zone, "--project", projectID, "--tunnel-through-iap")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	var err error
	a.tapp.Suspend(func() {
		err = cmd.Run()
	})
	if err != nil {
		a.logger.Error("vms", projectID, "SSH session failed for "+name+" ("+zone+"): "+err.Error())
		a.statusText("[red]SSH session failed: [-]" + err.Error())
		return
	}
	a.statusText("[green]SSH session ended[-]")
}

func showVMs(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(vmsViewKind())
}
