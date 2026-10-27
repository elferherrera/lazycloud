package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func dnsRecordRows(records []providers.DNSRecord) [][]string {
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		rows = append(rows, []string{
			r.Name, r.Type, strconv.FormatInt(r.TTL, 10), strings.Join(r.Data, ","), formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func copyDNSRecord(a *app, projectID, zone, name, recordType string) ([]byte, error) {
	r, err := a.cache.GetDNSRecord(projectID, zone, name, recordType)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("dns record not found in cache")
	}
	return json.MarshalIndent(r, "", "  ")
}

var dnsRecordsView = viewDef{
	title:   "DNS Records",
	headers: []string{"Name", "Type", "TTL", "Data", "Updated"},
	load: func(a *app) ([][]string, error) {
		records, err := a.cache.ListZoneDNSRecords(appState.ZoneProject, appState.Zone)
		if err != nil {
			return nil, err
		}
		return dnsRecordRows(records), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d dns records for zone %s (project %s)[-]", count, appState.Zone, appState.ZoneProject)
	},
	refresh: func(a *app) {
		if appState.ZoneProject == "" || appState.Zone == "" {
			a.statusText("[red]No dns zone selected (press d on a zone row)[-]")
			return
		}
		a.statusText(fmt.Sprintf("[yellow]Fetching dns records for %s...[-]", appState.Zone))
		go func() {
			projectID := appState.ZoneProject
			zone := appState.Zone
			records, err := providers.ListZoneDNSRecords(projectID, zone)
			if err != nil {
				a.reportError("dns-records", projectID+"/"+zone, "Failed to fetch dns records", err)
				return
			}
			if err := a.cache.SaveDNSRecords(projectID, zone, records); err != nil {
				a.reportError("dns-records", projectID+"/"+zone, "Failed to update cache", err)
				return
			}
			a.logger.Update("dns-records", projectID+"/"+zone, len(records))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewDNSRecords)
				a.statusText(fmt.Sprintf("[green]Fetched %d dns records[-]", len(records)))
			})
		}()
	},
	copyRow: func(a *app, row int) ([]byte, error) {
		return copyDNSRecord(a, appState.ZoneProject, appState.Zone,
			a.info.GetCell(row, 0).Text, a.info.GetCell(row, 1).Text)
	},
	consoleURL: func(a *app, row int) string {
		return providers.GCPDNSZoneURL(appState.ZoneProject, appState.Zone)
	},
}

func (a *app) showZoneDNSRecords() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	projectID := appState.ProjectID
	if appState.ViewKind == viewDNSZonesAll {
		projectID = a.info.GetCell(row, 5).Text
	}
	appState.ZoneProject = projectID
	appState.Zone = a.info.GetCell(row, 0).Text
	a.setView(viewDNSRecords)
}

func dnsRecordDetails(r *providers.DNSRecord) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[yellow]Project: [-]%s\n", r.ProjectID)
	fmt.Fprintf(&sb, "[yellow]Zone: [-]%s\n", r.Zone)
	fmt.Fprintf(&sb, "[yellow]Name: [-]%s\n", r.Name)
	fmt.Fprintf(&sb, "[yellow]Type: [-]%s\n", r.Type)
	fmt.Fprintf(&sb, "[yellow]TTL: [-]%d\n", r.TTL)
	fmt.Fprintf(&sb, "[yellow]Data: [-]\n")
	for _, d := range r.Data {
		fmt.Fprintf(&sb, "  %s\n", d)
	}
	fmt.Fprintf(&sb, "[yellow]Updated: [-]%s\n", formatUpdatedAt(r.UpdatedAt))
	return sb.String()
}

func (a *app) showDNSRecordDetails(r *providers.DNSRecord) {
	overlay, text := a.newOverlay(fmt.Sprintf(" DNS %s ", r.Type), dnsRecordDetails(r))
	a.pages.RemovePage("dns")
	a.pages.AddPage("dns", overlay, true, true)
	a.tapp.SetFocus(text)
}
