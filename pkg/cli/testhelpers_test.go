package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

// testDeps builds deps with a real temp cache, injected fetchers, no
// LAZYCLOUD_PROJECT, and a no-op view-update recorder.
func testDeps(t *testing.T, fetchers Fetchers) (deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cachePath := filepath.Join(t.TempDir(), "cache.db")
	openCache := func() (*providers.Cache, error) { return providers.NewCacheAt(cachePath) }
	return deps{
		openCache:  openCache,
		fetchers:   fetchers,
		envProject: func() string { return "" },
		saveViewUpdate: func(_ *providers.Cache, _, _ string) error {
			return nil
		},
	}, new(bytes.Buffer), new(bytes.Buffer)
}

// fixedFetchers returns fetchers yielding one canned object per
// resource, so refresh writes known data for every noun.
func fixedFetchers() Fetchers {
	return Fetchers{
		Projects: func() ([]providers.Project, error) {
			return []providers.Project{{ProjectID: "p1", Name: "one", ProjectNumber: "1"}}, nil
		},
		VMs: func(string) ([]providers.VM, error) {
			return []providers.VM{{ProjectID: "p1", Name: "vm-01", Zone: "europe-west1-b", MachineType: "e2-small", Status: "RUNNING", Interfaces: []providers.VMInterface{{InternalIP: "10.0.0.5"}}, UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
		Disks: func(string) ([]providers.Disk, error) {
			return []providers.Disk{{ProjectID: "p1", Name: "disk-01", Zone: "europe-west1-b", SizeGB: 10, Type: "pd-ssd", Status: "READY", Users: []string{"vm-01"}, UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
		Clusters: func(string) ([]providers.Cluster, error) {
			return []providers.Cluster{{ProjectID: "p1", Name: "cluster-01", Location: "europe-west1", Status: "RUNNING", Version: "1.31", NodePools: []providers.ClusterNodePool{{Name: "pool-1"}, {Name: "pool-2"}}, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
		Networks: func(string) ([]providers.Network, error) {
			return []providers.Network{{ProjectID: "p1", Name: "net-01", RoutingMode: "GLOBAL", UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
		Subnetworks: func(string) ([]providers.Subnetwork, error) {
			return []providers.Subnetwork{{ProjectID: "p1", Name: "subnet-01", Region: "europe-west1", Network: "net-01", IPRange: "10.0.0.0/24", Status: "READY", UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
		Buckets: func(string) ([]providers.Bucket, error) {
			return []providers.Bucket{{ProjectID: "p1", Name: "bucket-01", Location: "EU", StorageClass: "STANDARD", Versioning: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
		ServiceAccounts: func(string) ([]providers.ServiceAccount, error) {
			return []providers.ServiceAccount{{ProjectID: "p1", Email: "sa-01@p1.iam.gserviceaccount.com", DisplayName: "sa one", UniqueID: "123", UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
		IAMBindings: func(string) ([]providers.IAMBinding, error) {
			return []providers.IAMBinding{{ProjectID: "p1", Member: "user:x@example.com", Role: "roles/viewer", UpdatedAt: "2026-01-02T10:00:00Z"}}, nil
		},
	}
}
