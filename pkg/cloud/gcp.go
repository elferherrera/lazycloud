package cloud

import (
	"github.com/elferherrera/lazycloud/pkg/providers/gcp"
)

// GCP is registered in its own file: adding a new cloud means one
// Register call plus its provider package, with no edits anywhere
// else in the codebase.
func gcpFetchers() Fetchers {
	return Fetchers{
		Projects:        gcp.ListGcloudProjects,
		VMs:             gcp.ListProjectVMs,
		Disks:           gcp.ListProjectDisks,
		Clusters:        gcp.ListProjectClusters,
		Networks:        gcp.ListProjectNetworks,
		Subnetworks:     gcp.ListProjectSubnetworks,
		Buckets:         gcp.ListProjectBuckets,
		ServiceAccounts: gcp.ListProjectServiceAccounts,
		IAMBindings:     gcp.ListProjectIAMPolicy,
	}
}
