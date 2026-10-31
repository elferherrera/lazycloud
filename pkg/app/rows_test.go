package app

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func TestNodePoolRows(t *testing.T) {
	pools := []providers.ClusterNodePool{
		{Name: "pool-a", Version: "1.31.1-gke.1", NodeCount: 3},
		{Name: "pool-b", Version: "1.31.1-gke.1", NodeCount: 2},
	}
	rows := nodePoolRows(pools, nil)
	if len(rows) != 2 {
		t.Fatalf("nodePoolRows() len = %d, want 2", len(rows))
	}
	want := []string{"pool-a", "1.31.1-gke.1", "3"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("nodePoolRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}

	nodes := []providers.ClusterNode{
		{Name: "gke-cl-1-pool-a-aaa", NodePool: "pool-a"},
		{Name: "gke-cl-1-pool-a-bbb", NodePool: "pool-a"},
		{Name: "gke-cl-1-pool-b-ccc", NodePool: "pool-b"},
	}
	rows = nodePoolRows(pools, nodes)
	if rows[0][2] != "2" {
		t.Fatalf("nodePoolRows()[0] nodes = %q, want 2", rows[0][2])
	}
	if rows[1][2] != "1" {
		t.Fatalf("nodePoolRows()[1] nodes = %q, want 1", rows[1][2])
	}
}

func TestOverlayHeight(t *testing.T) {
	if got := overlayHeight(5, 40); got != 7 {
		t.Errorf("overlayHeight(5, 40) = %d, want 7", got)
	}
	if got := overlayHeight(100, 40); got != 38 {
		t.Errorf("overlayHeight(100, 40) = %d, want 38 (capped to screen)", got)
	}
	if got := overlayHeight(5, 0); got != 7 {
		t.Errorf("overlayHeight(5, 0) = %d, want 7 (no screen info)", got)
	}
	if got := overlayHeight(0, 2); got != 3 {
		t.Errorf("overlayHeight(0, 2) = %d, want 3 (minimum)", got)
	}
}

func TestOverlayWidth(t *testing.T) {
	if got := overlayWidth(120); got != 80 {
		t.Errorf("overlayWidth(120) = %d, want 80", got)
	}
	if got := overlayWidth(60); got != 58 {
		t.Errorf("overlayWidth(60) = %d, want 58 (capped to screen)", got)
	}
	if got := overlayWidth(0); got != 80 {
		t.Errorf("overlayWidth(0) = %d, want 80 (no screen info)", got)
	}
	if got := overlayWidth(5); got != 10 {
		t.Errorf("overlayWidth(5) = %d, want 10 (minimum)", got)
	}
}

func TestVMMatchesNodePool(t *testing.T) {
	if !vmMatchesNodePool(providers.VM{Labels: map[string]string{"goog-k8s-node-pool-name": "pool-a"}}, "pool-a") {
		t.Error("vmMatchesNodePool() = false, want true for label value match")
	}
	if vmMatchesNodePool(providers.VM{Labels: map[string]string{"env": "pool-a"}}, "pool-b") {
		t.Error("vmMatchesNodePool() = true, want false for non-matching pool")
	}
	if vmMatchesNodePool(providers.VM{}, "pool-a") {
		t.Error("vmMatchesNodePool() = true, want false for VM without labels")
	}
}

func TestAppendRowDetails(t *testing.T) {
	data := `{"ProjectID":"p1","Interfaces":[{"InternalIP":"10.0.0.1"}],"Tags":["a","b"],"Empty":null}`
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	appendRowDetails(&sb, parsed, "")
	got := sb.String()
	for _, want := range []string{
		"[yellow]ProjectID: [-]p1",
		"[yellow]Interfaces:[-]",
		"  [yellow]0:[-]",
		"    [yellow]InternalIP: [-]10.0.0.1",
		"[yellow]Tags:[-]",
		"  a",
		"  b",
		"[yellow]Empty: [-]<nil>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("appendRowDetails() output missing %q:\n%s", want, got)
		}
	}
}

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

func TestRouteRows(t *testing.T) {
	routes := []providers.Route{
		{
			ProjectID: "p1", Name: "route-a", Network: "vpc-a", DestRange: "0.0.0.0/0", Priority: 1000,
			NextHopKind: "gateway", NextHop: "default-internet-gateway", Description: "default egress",
			UpdatedAt: "2026-10-12T18:23:12Z",
		},
		{Name: "route-b", UpdatedAt: "not-a-date"},
	}
	rows := routeRows(routes)
	if len(rows) != 2 {
		t.Fatalf("routeRows() len = %d, want 2", len(rows))
	}
	want := []string{"route-a", "vpc-a", "0.0.0.0/0", "1000", "gateway", "default-internet-gateway", "default egress"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("routeRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if len(rows[0]) != 8 {
		t.Fatalf("routeRows()[0] len = %d, want 8", len(rows[0]))
	}
	if rows[1][6] != "" {
		t.Fatalf("routeRows()[1][6] = %q, want empty description", rows[1][6])
	}
	if rows[1][7] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[1][7])
	}

	rowsAll := routeRowsAll(routes)
	if len(rowsAll) != 2 {
		t.Fatalf("routeRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 9 {
		t.Fatalf("routeRowsAll()[0] len = %d, want 9", len(rowsAll[0]))
	}
	if rowsAll[0][8] != "p1" {
		t.Fatalf("routeRowsAll()[0][8] = %q, want project id", rowsAll[0][8])
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

func TestBucketRows(t *testing.T) {
	buckets := []providers.Bucket{
		{ProjectID: "p1", Name: "bucket-a", Location: "EUROPE-WEST1", StorageClass: "STANDARD", Versioning: true, CreatedAt: "2026-10-12T18:23:12Z", UpdatedAt: "2026-10-13T18:23:12Z"},
		{ProjectID: "p1", Name: "bucket-b", Location: "US", StorageClass: "NEARLINE", CreatedAt: "not-a-date", UpdatedAt: "not-a-date"},
	}
	rows := bucketRows(buckets)
	if len(rows) != 2 {
		t.Fatalf("bucketRows() len = %d, want 2", len(rows))
	}
	want := []string{"bucket-a", "EUROPE-WEST1", "STANDARD", "yes"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("bucketRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if rows[1][3] != "no" {
		t.Fatalf("bucketRows()[1][3] = %q, want no", rows[1][3])
	}

	rowsAll := bucketRowsAll(buckets)
	if len(rowsAll) != 2 {
		t.Fatalf("bucketRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 7 {
		t.Fatalf("bucketRowsAll()[0] len = %d, want 7", len(rowsAll[0]))
	}
	if rowsAll[0][6] != "p1" {
		t.Fatalf("bucketRowsAll()[0][6] = %q, want project id", rowsAll[0][6])
	}
}

func TestLBRows(t *testing.T) {
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	defer cache.Close()

	rules := []providers.ForwardingRule{
		{ProjectID: "p1", Name: "fr-web", Region: "global", IPAddress: "34.1.1.1", PortRange: "443", Protocol: "TCP", Scheme: "EXTERNAL", TargetKind: "targetHttpsProxies", Target: "proxy-a", UpdatedAt: "2026-10-12T18:23:12Z"},
		{ProjectID: "p2", Name: "fr-internal", Region: "europe-west1", IPAddress: "10.0.0.5", Protocol: "TCP", Scheme: "INTERNAL", TargetKind: "backendServices", Target: "svc-int", UpdatedAt: "not-a-date"},
	}
	proxies := []providers.TargetProxy{{ProjectID: "p1", Kind: "targetHttpsProxies", Name: "proxy-a", URLMap: "map-a"}}
	urlMaps := []providers.URLMap{{ProjectID: "p1", Name: "map-a", DefaultService: "svc-web", PathRules: []providers.PathRule{{Paths: []string{"/api"}, Service: "svc-api"}}}}
	services := []providers.BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "global", Protocol: "HTTPS"},
		{ProjectID: "p1", Name: "svc-api", Region: "global", Protocol: "HTTPS"},
	}
	if err := cache.SaveForwardingRules("p1", rules[:1]); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	if err := cache.SaveForwardingRules("p2", rules[1:]); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	if err := cache.SaveTargetProxies("p1", proxies); err != nil {
		t.Fatalf("SaveTargetProxies() error = %v", err)
	}
	if err := cache.SaveURLMaps("p1", urlMaps); err != nil {
		t.Fatalf("SaveURLMaps() error = %v", err)
	}
	if err := cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}
	if err := cache.SaveBackendServices("p2", []providers.BackendService{{ProjectID: "p2", Name: "svc-int", Region: "europe-west1", Protocol: "TCP"}}); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}

	p1Rules, err := cache.ListForwardingRules("p1")
	if err != nil {
		t.Fatalf("ListForwardingRules() error = %v", err)
	}
	rows := lbRows(cache, "p1", p1Rules)
	if len(rows) != 1 {
		t.Fatalf("lbRows() len = %d, want 1", len(rows))
	}
	if len(rows[0]) != 7 {
		t.Fatalf("lbRows()[0] len = %d, want 7", len(rows[0]))
	}
	want := []string{"fr-web", "34.1.1.1:443", "global", "TCP", "EXTERNAL", "svc-web (+1)"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("lbRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}

	allRules, err := cache.ListAllForwardingRules()
	if err != nil {
		t.Fatalf("ListAllForwardingRules() error = %v", err)
	}
	rowsAll := lbRowsAll(cache, allRules)
	if len(rowsAll) != 2 {
		t.Fatalf("lbRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 8 {
		t.Fatalf("lbRowsAll()[0] len = %d, want 8", len(rowsAll[0]))
	}
	if rowsAll[0][7] != "p1" || rowsAll[1][7] != "p2" {
		t.Fatalf("lbRowsAll() project column = %q, %q, want p1, p2", rowsAll[0][7], rowsAll[1][7])
	}
	if rowsAll[1][1] != "10.0.0.5" {
		t.Fatalf("lbRowsAll()[1][1] = %q, want IP without port range", rowsAll[1][1])
	}
	if rowsAll[1][5] != "svc-int" {
		t.Fatalf("lbRowsAll()[1][5] = %q, want direct backend service", rowsAll[1][5])
	}
	if rowsAll[1][6] == "" {
		t.Fatal("lbRowsAll()[1][6] updated column empty, want timestamp")
	}
}

func TestLBDetails(t *testing.T) {
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	defer cache.Close()

	rules := []providers.ForwardingRule{{ProjectID: "p1", Name: "fr-web", Region: "global", IPAddress: "34.1.1.1", PortRange: "443", Protocol: "TCP", Scheme: "EXTERNAL", TargetKind: "targetHttpsProxies", Target: "proxy-a"}}
	proxies := []providers.TargetProxy{{ProjectID: "p1", Kind: "targetHttpsProxies", Name: "proxy-a", URLMap: "map-a"}}
	urlMaps := []providers.URLMap{{ProjectID: "p1", Name: "map-a", DefaultService: "svc-web", PathRules: []providers.PathRule{{Paths: []string{"/api", "/api/*"}, Service: "svc-api"}}}}
	services := []providers.BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "global", Protocol: "HTTPS", PortName: "https", Backends: []providers.Backend{{Group: "ig-a", BalancingMode: "UTILIZATION"}}, HealthChecks: []string{"hc-a"}},
		{ProjectID: "p1", Name: "svc-api", Region: "global", Protocol: "HTTPS", PortName: "https", Backends: []providers.Backend{{Group: "neg-api"}}},
	}
	if err := cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	if err := cache.SaveTargetProxies("p1", proxies); err != nil {
		t.Fatalf("SaveTargetProxies() error = %v", err)
	}
	if err := cache.SaveURLMaps("p1", urlMaps); err != nil {
		t.Fatalf("SaveURLMaps() error = %v", err)
	}
	if err := cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}

	lb, err := cache.ResolveLoadBalancer("p1", "global", "fr-web")
	if err != nil {
		t.Fatalf("ResolveLoadBalancer() error = %v", err)
	}
	if lb == nil {
		t.Fatal("ResolveLoadBalancer() = nil, want composed load balancer")
	}
	if lb.Proxy == nil || lb.Proxy.Name != "proxy-a" {
		t.Fatalf("ResolveLoadBalancer() proxy = %+v", lb.Proxy)
	}
	if lb.URLMap == nil || lb.URLMap.Name != "map-a" {
		t.Fatalf("ResolveLoadBalancer() url map = %+v", lb.URLMap)
	}
	if len(lb.BackendServices) != 2 || lb.BackendServices[0].Name != "svc-web" || lb.BackendServices[1].Name != "svc-api" {
		t.Fatalf("ResolveLoadBalancer() backend services = %+v", lb.BackendServices)
	}

	details := lbDetails(lb)
	for _, want := range []string{
		"Name: [-]fr-web",
		"IP:Port: [-]34.1.1.1:443",
		"Target: [-]proxy-a (targetHttpsProxies)",
		"Target proxy: [-]proxy-a",
		"URL map: [-]map-a (default: svc-web)",
		"/api,/api/* -> [-]svc-api",
		"Backend service: [-]svc-web (HTTPS, port https)",
		"ig-a (UTILIZATION)",
		"neg-api",
		"Health checks: [-]hc-a",
	} {
		if !strings.Contains(details, want) {
			t.Errorf("lbDetails() missing %q:\n%s", want, details)
		}
	}
}

func TestLBConsoleURL(t *testing.T) {
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	defer cache.Close()

	rules := []providers.ForwardingRule{
		{ProjectID: "p1", Name: "fr-web", Region: "global", TargetKind: "targetHttpsProxies", Target: "proxy-a"},
		{ProjectID: "p1", Name: "fr-int", Region: "europe-west1", TargetKind: "backendServices", Target: "svc-int"},
	}
	proxies := []providers.TargetProxy{{ProjectID: "p1", Kind: "targetHttpsProxies", Name: "proxy-a", URLMap: "map-a"}}
	urlMaps := []providers.URLMap{{ProjectID: "p1", Name: "map-a", DefaultService: "svc-web"}}
	services := []providers.BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "global"},
		{ProjectID: "p1", Name: "svc-int", Region: "europe-west1"},
	}
	if err := cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	if err := cache.SaveTargetProxies("p1", proxies); err != nil {
		t.Fatalf("SaveTargetProxies() error = %v", err)
	}
	if err := cache.SaveURLMaps("p1", urlMaps); err != nil {
		t.Fatalf("SaveURLMaps() error = %v", err)
	}
	if err := cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}

	if got, want := lbConsoleURL(cache, "p1", "global", "fr-web"), providers.GCPLoadBalancerURL("p1", "map-a"); got != want {
		t.Fatalf("lbConsoleURL() =\n%s\nwant\n%s", got, want)
	}
	if got, want := lbConsoleURL(cache, "p1", "europe-west1", "fr-int"), providers.GCPLoadBalancerFrontendURL("p1", "fr-int"); got != want {
		t.Fatalf("lbConsoleURL() =\n%s\nwant\n%s", got, want)
	}
	if got, want := lbConsoleURL(cache, "p1", "global", "missing"), providers.GCPLoadBalancerFrontendURL("p1", "missing"); got != want {
		t.Fatalf("lbConsoleURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestClusterRows(t *testing.T) {
	clusters := []providers.Cluster{
		{
			ProjectID: "p1",
			Name:      "cluster-a",
			Location:  "europe-west1",
			Status:    "RUNNING",
			Version:   "1.31.1-gke.1",
			NodePools: []providers.ClusterNodePool{{Name: "pool-a", NodeCount: 3}, {Name: "pool-b", NodeCount: 2}},
			CreatedAt: "2026-10-12T18:23:12Z",
			UpdatedAt: "not-a-date",
		},
		{ProjectID: "p1", Name: "cluster-b", Location: "us-central1-a", Status: "PROVISIONING", Version: "1.30.5-gke.1", UpdatedAt: "not-a-date"},
	}
	rows := clusterRows(clusters)
	if len(rows) != 2 {
		t.Fatalf("clusterRows() len = %d, want 2", len(rows))
	}
	want := []string{"cluster-a", "europe-west1", "RUNNING", "1.31.1-gke.1", "pool-a (+1)"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("clusterRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if rows[0][6] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[0][6])
	}
	if rows[1][4] != "" {
		t.Fatalf("clusterRows()[1][4] = %q, want empty for cluster without node pools", rows[1][4])
	}

	rowsAll := clusterRowsAll(clusters)
	if len(rowsAll) != 2 {
		t.Fatalf("clusterRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 8 {
		t.Fatalf("clusterRowsAll()[0] len = %d, want 8", len(rowsAll[0]))
	}
	if rowsAll[0][7] != "p1" {
		t.Fatalf("clusterRowsAll()[0][7] = %q, want project id", rowsAll[0][7])
	}
}

func TestWorkloadRows(t *testing.T) {
	workloads := []providers.Workload{
		{ProjectID: "p1", Cluster: "cluster-a", Kind: "Deployment", Namespace: "default", Name: "web", Replicas: "3/3", Images: []string{"nginx:1.27", "sidecar:1.0"}, UpdatedAt: "not-a-date"},
		{ProjectID: "p1", Cluster: "cluster-a", Kind: "DaemonSet", Namespace: "kube-system", Name: "fluentbit", Replicas: "2/2"},
	}
	rows := workloadRows(workloads)
	if len(rows) != 2 {
		t.Fatalf("workloadRows() len = %d, want 2", len(rows))
	}
	want := []string{"default", "Deployment", "web", "3/3", "nginx:1.27,sidecar:1.0"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("workloadRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if rows[0][5] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[0][5])
	}
	if rows[1][4] != "" {
		t.Fatalf("workloadRows()[1][4] = %q, want empty images", rows[1][4])
	}
	if len(rows[0]) != 6 {
		t.Fatalf("workloadRows()[0] len = %d, want 6", len(rows[0]))
	}
}

func TestCloudRouterRows(t *testing.T) {
	routers := []providers.CloudRouter{
		{
			ProjectID: "p1", Name: "router-a", Region: "europe-west1", Network: "vpc-a", BGPASN: 64512,
			Peers:       []providers.CloudRouterPeer{{Name: "peer-a", Interface: "if-0", IPAddress: "169.254.0.2", PeerIP: "169.254.0.1", PeerASN: 65001}},
			NATs:        []string{"nat-a"},
			Description: "primary",
			UpdatedAt:   "2026-10-12T18:23:12Z",
		},
		{Name: "router-b", UpdatedAt: "not-a-date"},
	}
	rows := cloudRouterRows(routers)
	if len(rows) != 2 {
		t.Fatalf("cloudRouterRows() len = %d, want 2", len(rows))
	}
	want := []string{"router-a", "europe-west1", "vpc-a", "64512", "1", "1", "primary"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("cloudRouterRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if len(rows[0]) != 8 {
		t.Fatalf("cloudRouterRows()[0] len = %d, want 8", len(rows[0]))
	}
	if rows[1][3] != "" {
		t.Fatalf("cloudRouterRows()[1][3] = %q, want empty for router without BGP", rows[1][3])
	}
	if rows[1][4] != "0" || rows[1][5] != "0" {
		t.Fatalf("cloudRouterRows()[1] peers/nats = %q, %q, want 0, 0", rows[1][4], rows[1][5])
	}
	if rows[1][7] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[1][7])
	}

	rowsAll := cloudRouterRowsAll(routers)
	if len(rowsAll) != 2 {
		t.Fatalf("cloudRouterRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 9 {
		t.Fatalf("cloudRouterRowsAll()[0] len = %d, want 9", len(rowsAll[0]))
	}
	if rowsAll[0][8] != "p1" {
		t.Fatalf("cloudRouterRowsAll()[0][8] = %q, want project id", rowsAll[0][8])
	}
}

func TestVPNGatewayRows(t *testing.T) {
	gateways := []providers.VPNGateway{
		{
			ProjectID: "p1", Name: "gw-a", Type: "ha", Region: "europe-west1", Network: "vpc-a",
			IPs:         []string{"34.1.1.1", "34.1.1.2"},
			Description: "primary",
			Tunnels: []providers.VPNTunnel{
				{Name: "tunnel-1", Interface: 0, PeerIP: "203.0.113.1", IKE: 2, Status: "ESTABLISHED", Router: "router-a"},
				{Name: "tunnel-2", Interface: 1, PeerIP: "203.0.113.2", IKE: 2, Status: "DOWN", Router: "router-a"},
			},
			UpdatedAt: "2026-10-12T18:23:12Z",
		},
		{Name: "gw-b", Type: "classic", Region: "us-central1", Network: "vpc-b", UpdatedAt: "not-a-date"},
	}
	rows := vpnGatewayRows(gateways)
	if len(rows) != 2 {
		t.Fatalf("vpnGatewayRows() len = %d, want 2", len(rows))
	}
	want := []string{"ha", "gw-a", "europe-west1", "vpc-a", "34.1.1.1,34.1.1.2", "2", "primary"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("vpnGatewayRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if len(rows[0]) != 8 {
		t.Fatalf("vpnGatewayRows()[0] len = %d, want 8", len(rows[0]))
	}
	if rows[1][4] != "" {
		t.Fatalf("vpnGatewayRows()[1][4] = %q, want empty for classic gateway without interfaces", rows[1][4])
	}
	if rows[1][5] != "0" {
		t.Fatalf("vpnGatewayRows()[1][5] = %q, want 0 tunnels", rows[1][5])
	}
	if rows[1][7] != "not-a-date" {
		t.Fatalf("unparseable UpdatedAt should be kept as-is, got %q", rows[1][7])
	}

	rowsAll := vpnGatewayRowsAll(gateways)
	if len(rowsAll) != 2 {
		t.Fatalf("vpnGatewayRowsAll() len = %d, want 2", len(rowsAll))
	}
	if len(rowsAll[0]) != 9 {
		t.Fatalf("vpnGatewayRowsAll()[0] len = %d, want 9", len(rowsAll[0]))
	}
	if rowsAll[0][8] != "p1" {
		t.Fatalf("vpnGatewayRowsAll()[0][8] = %q, want project id", rowsAll[0][8])
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
	if found != 10 {
		t.Fatalf("shortcuts summary rendered %d numeric shortcuts, want 10", found)
	}
	if table.GetRowCount() != 4 {
		t.Fatalf("shortcuts summary has %d rows, want 4 to fit the 4-row section", table.GetRowCount())
	}
	if table.GetColumnCount() != 6 {
		t.Fatalf("shortcuts summary has %d columns, want 6 (three key/desc pairs)", table.GetColumnCount())
	}
}

func TestRenderSummaryAllProjects(t *testing.T) {
	appState = State{Name: "all", AllProjects: true}
	tv := newSummaryPanel()
	renderSummary(tv)
	text := tv.GetText(false)
	if text != "[yellow]Projects: [white]all" {
		t.Fatalf("renderSummary() = %q", text)
	}
	appState = State{Name: "One", ProjectID: "p1", ProjectNumber: "111"}
	renderSummary(tv)
	text = tv.GetText(false)
	want := "[yellow]Name: [white]One\n[yellow]Project ID: [white]p1\n[yellow]Project Number: [white]111"
	if text != want {
		t.Fatalf("renderSummary() = %q, want %q", text, want)
	}
}

func TestVMsInInstanceGroup(t *testing.T) {
	vms := []providers.VM{
		{Name: "vm-a", Zone: "europe-west2-b"},
		{Name: "vm-a", Zone: "europe-west2-c"},
		{Name: "vm-b", Zone: "europe-west2-b"},
	}
	members := []providers.InstanceGroupMember{
		{Zone: "europe-west2-b", Name: "vm-a"},
		{Zone: "europe-west2-c", Name: "vm-a"},
	}
	matched := vmsInInstanceGroup(vms, members)
	if len(matched) != 2 {
		t.Fatalf("vmsInInstanceGroup() len = %d, want 2", len(matched))
	}
	for _, vm := range matched {
		if vm.Name != "vm-a" {
			t.Fatalf("vmsInInstanceGroup() matched %q, want only vm-a", vm.Name)
		}
	}

	matched = vmsInInstanceGroup(vms, []providers.InstanceGroupMember{{Zone: "europe-west2-b", Name: "vm-missing"}})
	if len(matched) != 0 {
		t.Fatalf("vmsInInstanceGroup() len = %d, want 0 for unknown member", len(matched))
	}
}

func TestInstanceGroupRows(t *testing.T) {
	groups := []providers.InstanceGroup{
		{
			ProjectID: "p1", Name: "ig-a", Zone: "europe-west2-b", Type: "managed", Size: 3,
			Network: "vpc-a", Subnetwork: "sn-a", Template: "tpl-a", Description: "gke nodes",
			UpdatedAt: "2026-10-12T18:23:12Z",
		},
		{Name: "ig-b", UpdatedAt: "not-a-date"},
	}
	rows := instanceGroupRows(groups)
	if len(rows) != 2 {
		t.Fatalf("instanceGroupRows() len = %d, want 2", len(rows))
	}
	want := []string{"ig-a", "europe-west2-b", "managed", "3", "vpc-a", "sn-a", "gke nodes"}
	for col, val := range want {
		if rows[0][col] != val {
			t.Fatalf("instanceGroupRows()[0][%d] = %q, want %q", col, rows[0][col], val)
		}
	}
	if len(rows[0]) != 8 {
		t.Fatalf("instanceGroupRows()[0] len = %d, want 8", len(rows[0]))
	}
	rowsAll := instanceGroupRowsAll(groups)
	if len(rowsAll) != 2 || rowsAll[0][8] != "p1" {
		t.Fatalf("instanceGroupRowsAll() = %v, want project id in column 8", rowsAll)
	}
	if len(rowsAll[0]) != 9 {
		t.Fatalf("instanceGroupRowsAll()[0] len = %d, want 9", len(rowsAll[0]))
	}
}
