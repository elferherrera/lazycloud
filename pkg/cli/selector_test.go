package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

// seedVMs writes VMs with and without a zone into a temp cache, the
// two shapes a GCP and a Nebius refresh produce.
func seedVMs(t *testing.T) *providers.Cache {
	t.Helper()
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt: %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	vms := []providers.VM{
		{ProjectID: "p1", Name: "zoned-vm", Zone: "europe-west1-b", MachineType: "e2-small", Status: "RUNNING", UpdatedAt: "2026-01-02T10:00:00Z"},
		{ProjectID: "p1", Name: "zoneless-vm", Status: "RUNNING", UpdatedAt: "2026-01-02T10:00:00Z"},
	}
	if err := cache.SaveVMs("p1", vms); err != nil {
		t.Fatalf("SaveVMs: %v", err)
	}
	return cache
}

func TestGetVMZonelessSelector(t *testing.T) {
	cache := seedVMs(t)
	item, err := getVM(cache, []string{"p1", "zoneless-vm"})
	if err != nil {
		t.Fatalf("getVM 2-part: %v", err)
	}
	if item.(providers.VM).Name != "zoneless-vm" {
		t.Fatalf("got %q", item.(providers.VM).Name)
	}
}

func TestGetVMZonedSelector(t *testing.T) {
	cache := seedVMs(t)
	item, err := getVM(cache, []string{"p1", "europe-west1-b", "zoned-vm"})
	if err != nil {
		t.Fatalf("getVM 3-part: %v", err)
	}
	if item.(providers.VM).Name != "zoned-vm" {
		t.Fatalf("got %q", item.(providers.VM).Name)
	}
}

func TestGetVMBadSelectorStillErrors(t *testing.T) {
	cache := seedVMs(t)
	for _, sel := range [][]string{{}, {"p1"}, {"p1", "x", "y", "z"}} {
		if _, err := getVM(cache, sel); err != errBadSelector {
			t.Fatalf("sel %v: err = %v, want errBadSelector", sel, err)
		}
	}
}

func TestGetVMShortFormMatchesAnyZone(t *testing.T) {
	// The 2-part form means "the cloud has no zone concept, or the
	// caller does not care": it matches the name across zones, which
	// is also what an agent wants when it only knows project/name.
	cache := seedVMs(t)
	item, err := getVM(cache, []string{"p1", "zoned-vm"})
	if err != nil {
		t.Fatalf("getVM 2-part on zoned row: %v", err)
	}
	if item.(providers.VM).Name != "zoned-vm" {
		t.Fatalf("got %q", item.(providers.VM).Name)
	}
}

func TestGetDiskZonelessSelector(t *testing.T) {
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt: %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	disks := []providers.Disk{
		{ProjectID: "p1", Name: "d1", SizeGB: 10, Status: "READY", UpdatedAt: "2026-01-02T10:00:00Z"},
		{ProjectID: "p1", Name: "d2", Zone: "europe-west1-b", SizeGB: 20, Status: "READY", UpdatedAt: "2026-01-02T10:00:00Z"},
	}
	if err := cache.SaveDisks("p1", disks); err != nil {
		t.Fatalf("SaveDisks: %v", err)
	}
	item, err := getDisk(cache, []string{"p1", "d1"})
	if err != nil {
		t.Fatalf("getDisk 2-part: %v", err)
	}
	if item.(providers.Disk).Name != "d1" {
		t.Fatalf("got %q", item.(providers.Disk).Name)
	}
	item, err = getDisk(cache, []string{"p1", "europe-west1-b", "d2"})
	if err != nil {
		t.Fatalf("getDisk 3-part: %v", err)
	}
	if item.(providers.Disk).Name != "d2" {
		t.Fatalf("got %q", item.(providers.Disk).Name)
	}
}

func TestSelectorHintMentionsBothForms(t *testing.T) {
	// A 4-part selector is the bad shape for both clouds, so the
	// usage error carries the hint.
	stderr := runGetUsage(t, "vms", "p1", "a", "b", "c")
	if !strings.Contains(stderr, "project/name or project/zone/name") {
		t.Fatalf("hint missing from usage: %q", stderr)
	}
}

// runGetUsage triggers a bad-selector usage error and returns stderr.
func runGetUsage(t *testing.T, noun string, sel ...string) string {
	t.Helper()
	d, _, stderr := testDeps(t, Fetchers{})
	args := append([]string{"lazycloud", "get", noun}, sel...)
	code := ExecuteDeps(args, new(strings.Builder), stderr, d)
	if code != exitUsage {
		t.Fatalf("exit = %d, want usage", code)
	}
	return stderr.String()
}
