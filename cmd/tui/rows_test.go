package main

import (
	"path/filepath"
	"strings"
	"testing"

	"lazycloud/cmd/providers"
)

func TestProjectRows(t *testing.T) {
	projects := []providers.Project{
		{ProjectID: "p1", Name: "One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "Two", ProjectNumber: "222"},
	}
	rows := projectRows(projects)
	if len(rows) != 2 {
		t.Fatalf("projectRows() len = %d, want 2", len(rows))
	}
	want := []string{"p1", "One", "111"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("projectRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
}

func TestIPAddressRows(t *testing.T) {
	ips := []providers.IPAddress{
		{Name: "ip-a", Address: "10.0.0.1", Region: "europe-west1", Status: "RESERVED", UpdatedAt: "2026-10-12T18:23:12Z"},
		{Name: "ip-b", Address: "34.1.1.1", Region: "global", Status: "IN_USE", UpdatedAt: "not-a-date"},
	}
	rows := ipAddressRows(ips)
	if len(rows) != 2 {
		t.Fatalf("ipAddressRows() len = %d, want 2", len(rows))
	}
	if rows[0][0] != "ip-a" || rows[0][1] != "10.0.0.1" || rows[0][2] != "europe-west1" || rows[0][3] != "RESERVED" {
		t.Fatalf("ipAddressRows()[0] = %v", rows[0])
	}
	if len(rows[0]) != 5 {
		t.Fatalf("ipAddressRows()[0] len = %d, want 5", len(rows[0]))
	}
	if rows[1][4] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[1][4])
	}
}

func TestIPAddressRowsAll(t *testing.T) {
	ips := []providers.IPAddress{
		{ProjectID: "p1", Name: "ip-a", Address: "10.0.0.1", Region: "europe-west1", Status: "RESERVED", UpdatedAt: "2026-10-12T18:23:12Z"},
	}
	rows := ipAddressRowsAll(ips)
	if len(rows) != 1 {
		t.Fatalf("ipAddressRowsAll() len = %d, want 1", len(rows))
	}
	if len(rows[0]) != 6 {
		t.Fatalf("ipAddressRowsAll()[0] len = %d, want 6", len(rows[0]))
	}
	if rows[0][5] != "p1" {
		t.Fatalf("ipAddressRowsAll()[0][5] = %q, want project id", rows[0][5])
	}
}

func TestNetworkRows(t *testing.T) {
	networks := []providers.Network{
		{Name: "vpc-a", RoutingMode: "REGIONAL", UpdatedAt: "2026-10-12T18:23:12Z"},
	}
	subnetworks := []providers.Subnetwork{
		{Name: "sn-a", Region: "europe-west1", Network: "vpc-a", IPRange: "10.0.0.0/24", Status: "READY", UpdatedAt: "not-a-date"},
	}
	rows := networkRows(networks, subnetworks)
	if len(rows) != 2 {
		t.Fatalf("networkRows() len = %d, want 2", len(rows))
	}
	wantVPC := []string{"vpc", "vpc-a", "", "", "", "REGIONAL", ""}
	for col, val := range wantVPC {
		if rows[0][col] != val {
			t.Fatalf("networkRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	wantSubnet := []string{"subnet", "sn-a", "europe-west1", "vpc-a", "10.0.0.0/24", "", "READY"}
	for col, val := range wantSubnet {
		if rows[1][col] != val {
			t.Fatalf("networkRows()[1][%d] = %q, want %q", col, rows[1][col], val)
		}
	}
	if rows[1][7] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[1][7])
	}
}

func TestNetworkRowsAll(t *testing.T) {
	networks := []providers.Network{
		{ProjectID: "p1", Name: "vpc-a", RoutingMode: "REGIONAL", UpdatedAt: "2026-10-12T18:23:12Z"},
	}
	subnetworks := []providers.Subnetwork{
		{ProjectID: "p2", Name: "sn-a", Region: "europe-west1", Network: "vpc-b", IPRange: "10.0.0.0/24", Status: "READY", UpdatedAt: "not-a-date"},
	}
	rows := networkRowsAll(networks, subnetworks)
	if len(rows) != 2 {
		t.Fatalf("networkRowsAll() len = %d, want 2", len(rows))
	}
	if len(rows[0]) != 9 {
		t.Fatalf("networkRowsAll()[0] len = %d, want 9", len(rows[0]))
	}
	if rows[0][8] != "p1" {
		t.Fatalf("networkRowsAll()[0][8] = %q, want project id", rows[0][8])
	}
	if rows[1][8] != "p2" {
		t.Fatalf("networkRowsAll()[1][8] = %q, want project id", rows[1][8])
	}
}

func TestServiceAccountRows(t *testing.T) {
	accounts := []providers.ServiceAccount{
		{ProjectID: "p1", Email: "sa-a@p1.iam.gserviceaccount.com", DisplayName: "SA A", UniqueID: "111", Disabled: false, UpdatedAt: "2026-10-12T18:23:12Z"},
		{ProjectID: "p1", Email: "sa-b@p1.iam.gserviceaccount.com", DisplayName: "SA B", UniqueID: "222", Disabled: true, UpdatedAt: "not-a-date"},
	}
	rows := serviceAccountRows(accounts)
	if len(rows) != 2 {
		t.Fatalf("serviceAccountRows() len = %d, want 2", len(rows))
	}
	want := []string{"sa-a@p1.iam.gserviceaccount.com", "SA A", "111", "no"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("serviceAccountRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if rows[1][3] != "yes" {
		t.Fatalf("serviceAccountRows()[1][3] = %q, want yes", rows[1][3])
	}
	if rows[1][4] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[1][4])
	}

	rowsAll := serviceAccountRowsAll(accounts)
	if len(rowsAll) != 2 {
		t.Fatalf("serviceAccountRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 6 {
		t.Fatalf("serviceAccountRowsAll()[0] len = %d, want 6", len(rowsAll[0]))
	}
	if rowsAll[0][5] != "p1" {
		t.Fatalf("serviceAccountRowsAll()[0][5] = %q, want project id", rowsAll[0][5])
	}
}

func TestVMRows(t *testing.T) {
	vms := []providers.VM{
		{
			ProjectID:   "p1",
			Name:        "vm-a",
			Zone:        "europe-west1-b",
			MachineType: "n1-standard-1",
			Status:      "RUNNING",
			Interfaces: []providers.VMInterface{
				{Network: "vpc-a", Subnetwork: "sn-a", InternalIP: "10.0.0.3", ExternalIP: "34.1.1.1"},
				{Network: "vpc-a", Subnetwork: "sn-b", InternalIP: "10.1.0.3"},
			},
			BootDisk:  &providers.VMDisk{Name: "vm-a", DeviceName: "boot", Mode: "READ_WRITE", SizeGB: 20},
			DataDisks: []providers.VMDisk{{Name: "data-1", DeviceName: "data-1", Mode: "READ_WRITE", SizeGB: 100}},
			UpdatedAt: "2026-10-12T18:23:12Z",
		},
		{Name: "vm-b", Zone: "us-central1-a", Status: "TERMINATED", UpdatedAt: "not-a-date"},
	}
	rows := vmRows(vms)
	if len(rows) != 2 {
		t.Fatalf("vmRows() len = %d, want 2", len(rows))
	}
	if rows[0][0] != "vm-a" || rows[0][1] != "europe-west1-b" || rows[0][2] != "n1-standard-1" || rows[0][3] != "RUNNING" {
		t.Fatalf("vmRows()[0] = %v", rows[0])
	}
	if rows[0][4] != "10.0.0.3,10.1.0.3" {
		t.Fatalf("vmRows()[0][4] = %q, want joined internal IPs", rows[0][4])
	}
	if rows[1][4] != "" {
		t.Fatalf("vmRows()[1][4] = %q, want empty for VM without interfaces", rows[1][4])
	}
	if rows[1][5] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[1][5])
	}

	rowsAll := vmRowsAll(vms)
	if len(rowsAll) != 2 {
		t.Fatalf("vmRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 7 {
		t.Fatalf("vmRowsAll()[0] len = %d, want 7", len(rowsAll[0]))
	}
	if rowsAll[0][6] != "p1" {
		t.Fatalf("vmRowsAll()[0][6] = %q, want project id", rowsAll[0][6])
	}
}

func TestVMDetails(t *testing.T) {
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	defer cache.Close()
	a := &app{cache: cache}

	vm := providers.VM{
		ProjectID:   "p1",
		Name:        "vm-a",
		Zone:        "europe-west1-b",
		MachineType: "n1-standard-1",
		Status:      "RUNNING",
		Interfaces: []providers.VMInterface{
			{Network: "vpc-a", Subnetwork: "sn-a", InternalIP: "10.0.0.3", ExternalIP: "34.1.1.1"},
		},
		BootDisk:  &providers.VMDisk{Name: "vm-a", DeviceName: "boot", Mode: "READ_WRITE", AutoDelete: true, SizeGB: 20},
		DataDisks: []providers.VMDisk{{Name: "data-1", DeviceName: "data-1", Mode: "READ_WRITE", SizeGB: 100}},
	}
	if err := cache.SaveVMs("p1", []providers.VM{vm}); err != nil {
		t.Fatalf("SaveVMs() error = %v", err)
	}
	if err := cache.SaveDisks("p1", []providers.Disk{
		{Name: "data-1", Zone: "europe-west1-b", SizeGB: 100, Type: "pd-balanced", Status: "READY"},
	}); err != nil {
		t.Fatalf("SaveDisks() error = %v", err)
	}

	details := a.vmDetails(&vm)
	for _, want := range []string{
		"Project: [-]p1",
		"Zone: [-]europe-west1-b",
		"Status: [-]RUNNING",
		"vpc-a/sn-a  10.0.0.3  (ext: 34.1.1.1)",
		"[boot]",
		"[data]",
		"pd-balanced",
		"READY",
		"100 GB",
	} {
		if !strings.Contains(details, want) {
			t.Errorf("vmDetails() missing %q:\n%s", want, details)
		}
	}
}

func TestShortcutsSummaryPanel(t *testing.T) {
	table := newShortcutsSummaryPanel()
	var found int
	for r := 0; r < table.GetRowCount(); r++ {
		for c := 0; c < table.GetColumnCount(); c += 2 {
			key := table.GetCell(r, c)
			if key.Text == "" {
				continue
			}
			desc := table.GetCell(r, c+1).Text
			for _, s := range shortcuts {
				if string(s.key) == key.Text && desc == strings.TrimPrefix(s.desc, "show ") {
					found++
				}
			}
		}
	}
	if found != 5 {
		t.Fatalf("shortcuts summary rendered %d numeric shortcuts, want 5", found)
	}
	if table.GetRowCount() != 3 {
		t.Fatalf("shortcuts summary has %d rows, want 3 to fit the 3-row section", table.GetRowCount())
	}
}

func TestRenderSummaryAllProjects(t *testing.T) {
	appState = State{Name: "all", AllProjects: true}
	tv := newSummaryPanel()
	renderSummary(tv)
	text := tv.GetText(false)
	if text != "[yellow]Projects: [-]all" {
		t.Fatalf("renderSummary() = %q", text)
	}
	appState = State{Name: "One", ProjectID: "p1", ProjectNumber: "111"}
	renderSummary(tv)
	text = tv.GetText(false)
	want := "[yellow]Name: [-]One\n[yellow]Project ID: [-]p1\n[yellow]Project Number: [-]111"
	if text != want {
		t.Fatalf("renderSummary() = %q, want %q", text, want)
	}
}
