package app

import (
	"os"
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func newRouteHopTestApp(t *testing.T) *app {
	t.Helper()
	a := newFilterTestApp(t)
	vms := []providers.VM{
		{ProjectID: "p1", Name: "web-vm", Zone: "europe-west2-b", Status: "RUNNING",
			Interfaces: []providers.VMInterface{{Network: "net", Subnetwork: "sub", InternalIP: "10.10.0.5", ExternalIP: "104.44.44.44"}}},
		{ProjectID: "p1", Name: "db-vm", Zone: "europe-west2-c", Status: "RUNNING",
			Interfaces: []providers.VMInterface{{Network: "net", Subnetwork: "sub", InternalIP: "10.10.0.9"}}},
	}
	if err := a.cache.SaveVMs("p1", vms); err != nil {
		t.Fatalf("SaveVMs() error = %v", err)
	}
	rules := []providers.ForwardingRule{
		{ProjectID: "p1", Name: "web-fr", Region: "europe-west2", IPAddress: "10.10.0.5", Scheme: "EXTERNAL", Protocol: "TCP"},
		{ProjectID: "p1", Name: "other-fr", Region: "europe-west2", IPAddress: "10.99.9.9", Scheme: "INTERNAL"},
	}
	if err := a.cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	return a
}

func TestRouteHopMermaidGatewayHop(t *testing.T) {
	a := newRouteHopTestApp(t)
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "default-internet", DestRange: "0.0.0.0/0",
		NextHopKind: "gateway", NextHop: "default-internet-gateway",
	})
	want := `route["default-internet"] -->|dest 0.0.0.0/0| hop{"gateway: default-internet-gateway"}`
	if !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
	if strings.Contains(src, "vm1") || strings.Contains(src, "lb1") || strings.Contains(src, "unknown") {
		t.Errorf("gateway hop should not resolve targets:\n%s", src)
	}
}

func TestRouteHopMermaidIPHopResolvesVMAndLB(t *testing.T) {
	a := newRouteHopTestApp(t)
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "to-web", DestRange: "10.20.0.0/20",
		NextHopKind: "ip", NextHop: "10.10.0.5",
	})
	if want := `hop --> vm1["VM: web-vm (europe-west2-b)"]`; !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
	if want := `hop --> lb1("LB: web-fr (europe-west2, EXTERNAL)")`; !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
	if strings.Contains(src, "unknown") {
		t.Errorf("resolved hop should not render the unknown node:\n%s", src)
	}
	if strings.Contains(src, "fwr:") || strings.Contains(src, "group:") {
		t.Errorf("LB without a cached backend chain should not continue the chart:\n%s", src)
	}
}

func TestRouteHopMermaidLBContinuesToBackendGroups(t *testing.T) {
	a := newRouteHopTestApp(t)
	rules := []providers.ForwardingRule{
		{ProjectID: "p1", Name: "web-fr", Region: "europe-west2", IPAddress: "10.10.0.5", Scheme: "EXTERNAL", Protocol: "TCP", TargetKind: "backendServices", Target: "svc-web"},
	}
	if err := a.cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	services := []providers.BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "europe-west2", Protocol: "HTTP",
			Backends: []providers.Backend{{Group: "ig-a", BalancingMode: "UTILIZATION"}, {Group: "ig-b"}}},
	}
	if err := a.cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "to-web", DestRange: "10.20.0.0/20",
		NextHopKind: "ip", NextHop: "10.10.0.5",
	})
	for _, want := range []string{
		`hop --> lb1("LB: web-fr (europe-west2, EXTERNAL)")`,
		`lb1 --> lb1-svc1["fwr: svc-web"]`,
		`lb1-svc1 --> lb1-svc1-grp1{{"group: ig-a"}}`,
		`lb1-svc1 --> lb1-svc1-grp2{{"group: ig-b"}}`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("mermaid source missing %q:\n%s", want, src)
		}
	}
}

func TestRouteHopMermaidLBContinuesThroughURLMap(t *testing.T) {
	a := newRouteHopTestApp(t)
	rules := []providers.ForwardingRule{
		{ProjectID: "p1", Name: "web-fr", Region: "europe-west2", IPAddress: "10.10.0.5", Scheme: "EXTERNAL", Protocol: "TCP", TargetKind: "targetHttpsProxies", Target: "proxy-a"},
	}
	if err := a.cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	proxies := []providers.TargetProxy{{ProjectID: "p1", Kind: "targetHttpsProxies", Name: "proxy-a", URLMap: "map-a"}}
	if err := a.cache.SaveTargetProxies("p1", proxies); err != nil {
		t.Fatalf("SaveTargetProxies() error = %v", err)
	}
	urlMaps := []providers.URLMap{{ProjectID: "p1", Name: "map-a", DefaultService: "svc-web", PathRules: []providers.PathRule{{Paths: []string{"/api", "/api/*"}, Service: "svc-api"}}}}
	if err := a.cache.SaveURLMaps("p1", urlMaps); err != nil {
		t.Fatalf("SaveURLMaps() error = %v", err)
	}
	services := []providers.BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "europe-west2", Backends: []providers.Backend{{Group: "ig-a"}}},
		{ProjectID: "p1", Name: "svc-api", Region: "global", Backends: []providers.Backend{{Group: "neg-api"}}},
	}
	if err := a.cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "to-web", DestRange: "10.20.0.0/20",
		NextHopKind: "ip", NextHop: "10.10.0.5",
	})
	for _, want := range []string{
		`lb1 --> lb1-svc1["fwr: svc-web"]`,
		`lb1-svc1 --> lb1-svc1-grp1{{"group: ig-a"}}`,
		`lb1 --> lb1-svc2["fwr: svc-api"]`,
		`lb1-svc2 --> lb1-svc2-grp1{{"group: neg-api"}}`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("mermaid source missing %q:\n%s", want, src)
		}
	}
}

func TestRouteHopMermaidLBWithoutBackendsStopsAtLB(t *testing.T) {
	a := newRouteHopTestApp(t)
	services := []providers.BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "europe-west2", Protocol: "HTTP"},
	}
	if err := a.cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}
	rules := []providers.ForwardingRule{
		{ProjectID: "p1", Name: "web-fr", Region: "europe-west2", IPAddress: "10.10.0.5", Scheme: "EXTERNAL", Protocol: "TCP", TargetKind: "backendServices", Target: "svc-web"},
	}
	if err := a.cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "to-web", DestRange: "10.20.0.0/20",
		NextHopKind: "ip", NextHop: "10.10.0.5",
	})
	if !strings.Contains(src, `lb1 --> lb1-svc1["fwr: svc-web"]`) {
		t.Errorf("mermaid source missing service node:\n%s", src)
	}
	if strings.Contains(src, "group:") {
		t.Errorf("service without backends should not render group nodes:\n%s", src)
	}
}

func TestRouteHopMermaidIPHopMatchesExternalIP(t *testing.T) {
	a := newRouteHopTestApp(t)
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "to-web-ext", DestRange: "10.30.0.0/20",
		NextHopKind: "ip", NextHop: "104.44.44.44",
	})
	if want := `hop --> vm1["VM: web-vm (europe-west2-b)"]`; !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
}

func TestRouteHopMermaidIPHopUnresolved(t *testing.T) {
	a := newRouteHopTestApp(t)
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "to-nowhere", DestRange: "10.40.0.0/20",
		NextHopKind: "ip", NextHop: "10.50.50.50",
	})
	if want := `hop -.-> unknown["no VM or LB in cache"]`; !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
	if strings.Contains(src, "vm1") || strings.Contains(src, "lb1") {
		t.Errorf("unresolved hop should not render targets:\n%s", src)
	}
}

func TestRouteHopMermaidNonIPValueSkipsResolution(t *testing.T) {
	a := newRouteHopTestApp(t)
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "peering-route", DestRange: "10.60.0.0/16",
		NextHopKind: "peering", NextHop: "servicenetworking.googleapis.com",
	})
	if want := `hop{"peering: servicenetworking.googleapis.com"}`; !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
	if strings.Contains(src, "unknown") || strings.Contains(src, "vm1") || strings.Contains(src, "lb1") {
		t.Errorf("non-IP hop should not resolve targets:\n%s", src)
	}
}

func TestRouteHopMermaidNoDestRange(t *testing.T) {
	a := newRouteHopTestApp(t)
	src := routeHopMermaid(a, &providers.Route{
		ProjectID: "p1", Name: "no-dest", NextHopKind: "instance", NextHop: "vm-a",
	})
	if want := `route["no-dest"] --> hop{"instance: vm-a"}`; !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
}

func TestRouteHopMermaidNoNextHop(t *testing.T) {
	a := newRouteHopTestApp(t)
	src := routeHopMermaid(a, &providers.Route{ProjectID: "p1", Name: "blackhole", DestRange: "10.70.0.0/16"})
	if want := `hop{"none"}`; !strings.Contains(src, want) {
		t.Errorf("mermaid source missing %q:\n%s", want, src)
	}
}

func TestRouteHopTargetsLimitedToRouteProject(t *testing.T) {
	a := newRouteHopTestApp(t)
	targets := a.routeHopTargets(&providers.Route{
		ProjectID: "p2", Name: "other-project", NextHopKind: "ip", NextHop: "10.10.0.5",
	})
	if len(targets) != 0 {
		t.Errorf("targets = %v, want none for other project", targets)
	}
}

func TestRenderMermaidArtPinsWidthAndRestoresEnv(t *testing.T) {
	t.Setenv("COLUMNS", "300")
	src := "graph TD\n  route[\"peering-route-1\"] -->|dest 10.20.0.0/20| hop{\"ip: 10.154.12.5\"}\n  hop --> vm1[\"VM: api-server-1 (europe-west1-b)\"]\n  hop --> lb1(\"LB: web-fr (europe-west1)\")"
	art := renderMermaidArt(src)
	if !strings.Contains(art, "peering-route-1") || !strings.Contains(art, "api-server-1") || !strings.Contains(art, "web-fr") {
		t.Errorf("rendered art is missing node labels:\n%s", art)
	}
	max := 0
	for _, line := range strings.Split(strings.TrimRight(art, "\n"), "\n") {
		if len(line) > max {
			max = len(line)
		}
	}
	if max >= 200 {
		t.Errorf("art stretched to %d columns, width pinning failed:\n%s", max, art)
	}
	if os.Getenv("COLUMNS") != "300" {
		t.Errorf("COLUMNS = %q, want restored value %q", os.Getenv("COLUMNS"), "300")
	}
}

func TestRenderMermaidArtKeepsStdoutAndStderr(t *testing.T) {
	stdout, stderr := os.Stdout, os.Stderr
	renderMermaidArt("graph TD\n  a[\"x\"] --> b[\"y\"]")
	if os.Stdout != stdout || os.Stderr != stderr {
		t.Error("os.Stdout/os.Stderr were not restored after render")
	}
}

func TestArtOverlayWidth(t *testing.T) {
	if w := artOverlayWidth(strings.Repeat("x", 100)+"\n", 200); w != 102 {
		t.Errorf("artOverlayWidth = %d, want 102", w)
	}
	if w := artOverlayWidth(strings.Repeat("┌", 100)+"\n", 200); w != 102 {
		t.Errorf("artOverlayWidth = %d, want 102 for rune-counted line", w)
	}
	if w := artOverlayWidth(strings.Repeat("x", 120)+"\n", 120); w != 118 {
		t.Errorf("artOverlayWidth = %d, want 118 capped to screen", w)
	}
	if w := artOverlayWidth("abc\nabcdefg\n", 200); w != 10 {
		t.Errorf("artOverlayWidth = %d, want 10 minimum", w)
	}
	if w := artOverlayWidth("", 200); w != 10 {
		t.Errorf("artOverlayWidth = %d, want 10 minimum", w)
	}
}
