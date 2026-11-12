package cloud

import (
	"github.com/elferherrera/lazycloud/pkg/providers/gcp"
)

// GCP registration mirrors nebius.go: one cloud, one registration
// file, no edits anywhere else to add or change it.
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
