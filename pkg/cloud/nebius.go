package cloud

import (
	"path/filepath"

	"github.com/elferherrera/lazycloud/pkg/providers"
	"github.com/elferherrera/lazycloud/pkg/providers/nebius"
)

// Nebius is registered in its own file: adding a new cloud means one
// Register call plus its provider package, with no edits anywhere else
// in the codebase.
func init() {
	Register(Provider{
		Name:            Nebius,
		Fetchers:        nebiusFetchers(),
		TableHeaders:    nebiusTableHeaders(),
		CacheFileSuffix: "nebius",
		NewCache:        NewNebiusCache,
	})
}

// nebiusFetchers adapts the Nebius provider functions to the shared
// Fetchers shape, so the registry and the CLI treat Nebius exactly
// like GCP.
func nebiusFetchers() Fetchers {
	return Fetchers{
		Projects:        nebius.ListProjects,
		VMs:             nebius.ListVMs,
		Disks:           nebius.ListDisks,
		Clusters:        nebius.ListClusters,
		Networks:        nebius.ListNetworks,
		Subnetworks:     nebius.ListSubnetworks,
		Buckets:         nebius.ListBuckets,
		ServiceAccounts: nebius.ListServiceAccounts,
		IAMBindings:     nebius.ListIAM,
	}
}

// nebiusTableHeaders renames the projects columns Nebius fills
// differently: GCP puts its numeric project number in the third
// column, Nebius puts the region there.
func nebiusTableHeaders() map[string][]string {
	return map[string][]string{
		"projects": {"Project ID", "Name", "Region"},
	}
}

// NewNebiusCache opens the Nebius cache db, next to the GCP one.
func NewNebiusCache() (*providers.Cache, error) {
	dir, err := DefaultCacheDir()
	if err != nil {
		return nil, err
	}
	return providers.NewCacheAt(filepath.Join(dir, "cache-nebius.db"))
}
