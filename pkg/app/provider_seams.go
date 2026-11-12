package app

import (
	"github.com/elferherrera/lazycloud/pkg/providers/gcp"
)

// The list* vars are the seams tests stub so every view fetch runs hermetic:
// no test path reaches the real gcloud clients. Production code calls these
// vars, never providers.List* directly.
var (
	listGcloudProjects         = gcp.ListGcloudProjects
	listProjectVMs             = gcp.ListProjectVMs
	listProjectDisks           = gcp.ListProjectDisks
	listProjectBuckets         = gcp.ListProjectBuckets
	listProjectClusters        = gcp.ListProjectClusters
	listClusterWorkloads       = gcp.ListClusterWorkloads
	listClusterNodes           = gcp.ListClusterNodes
	listProjectNetworks        = gcp.ListProjectNetworks
	listProjectSubnetworks     = gcp.ListProjectSubnetworks
	listProjectPeerings        = gcp.ListProjectPeerings
	listProjectRoutes          = gcp.ListProjectRoutes
	listProjectFirewallRules   = gcp.ListProjectFirewallRules
	listProjectIPAddresses     = gcp.ListProjectIPAddresses
	listProjectForwardingRules = gcp.ListProjectForwardingRules
	listProjectTargetProxies   = gcp.ListProjectTargetProxies
	listProjectURLMaps         = gcp.ListProjectURLMaps
	listProjectBackendServices = gcp.ListProjectBackendServices
	listProjectSnapshots       = gcp.ListProjectSnapshots
	listProjectImages          = gcp.ListProjectImages
	listProjectInstanceGroups  = gcp.ListProjectInstanceGroups
	listInstanceGroupVMs       = gcp.ListInstanceGroupVMs
	listProjectDNSZones        = gcp.ListProjectDNSZones
	listZoneDNSRecords         = gcp.ListZoneDNSRecords
	listProjectCloudRouters    = gcp.ListProjectCloudRouters
	listProjectServiceAttach   = gcp.ListProjectServiceAttachments
	listProjectVPNGateways     = gcp.ListProjectVPNGateways
	listProjectReservations    = gcp.ListProjectReservations
	listProjectInstanceTpl     = gcp.ListProjectInstanceTemplates
	listProjectServiceAccounts = gcp.ListProjectServiceAccounts
	listProjectIAMPolicy       = gcp.ListProjectIAMPolicy
	listBucketIAMPolicy        = gcp.ListBucketIAMPolicy
)
