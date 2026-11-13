package app

import (
	"github.com/elferherrera/lazycloud/cmd/providers"
)

// The list* vars are the seams tests stub so every view fetch runs hermetic:
// no test path reaches the real gcloud clients. Production code calls these
// vars, never providers.List* directly.
var (
	listGcloudProjects         = providers.ListGcloudProjects
	listProjectVMs             = providers.ListProjectVMs
	listProjectDisks           = providers.ListProjectDisks
	listProjectBuckets         = providers.ListProjectBuckets
	listProjectClusters        = providers.ListProjectClusters
	listClusterWorkloads       = providers.ListClusterWorkloads
	listClusterNodes           = providers.ListClusterNodes
	listProjectNetworks        = providers.ListProjectNetworks
	listProjectSubnetworks     = providers.ListProjectSubnetworks
	listProjectPeerings        = providers.ListProjectPeerings
	listProjectRoutes          = providers.ListProjectRoutes
	listProjectFirewallRules   = providers.ListProjectFirewallRules
	listProjectIPAddresses     = providers.ListProjectIPAddresses
	listProjectForwardingRules = providers.ListProjectForwardingRules
	listProjectTargetProxies   = providers.ListProjectTargetProxies
	listProjectURLMaps         = providers.ListProjectURLMaps
	listProjectBackendServices = providers.ListProjectBackendServices
	listProjectSnapshots       = providers.ListProjectSnapshots
	listProjectImages          = providers.ListProjectImages
	listProjectInstanceGroups  = providers.ListProjectInstanceGroups
	listInstanceGroupVMs       = providers.ListInstanceGroupVMs
	listProjectDNSZones        = providers.ListProjectDNSZones
	listZoneDNSRecords         = providers.ListZoneDNSRecords
	listProjectCloudRouters    = providers.ListProjectCloudRouters
	listProjectServiceAttach   = providers.ListProjectServiceAttachments
	listProjectVPNGateways     = providers.ListProjectVPNGateways
	listProjectReservations    = providers.ListProjectReservations
	listProjectInstanceTpl     = providers.ListProjectInstanceTemplates
	listProjectServiceAccounts = providers.ListProjectServiceAccounts
	listProjectIAMPolicy       = providers.ListProjectIAMPolicy
	listProjectCustomRoles     = providers.ListProjectCustomRoles
	listBucketIAMPolicy        = providers.ListBucketIAMPolicy
)
