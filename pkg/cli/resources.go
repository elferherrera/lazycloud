package cli

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// errBadSelector reports a get selector with the wrong number of parts
// for the resource.
var errBadSelector = errors.New("bad selector")

// resourceDef wires a CLI noun to the cache and fetchers behind it.
// Entries come from defaultRegistry, which takes the fetchers as a
// parameter so tests can inject fakes for the refresh paths.
type resourceDef struct {
	// all lists the resource across every project (cache read).
	all func(*providers.Cache) ([]any, error)
	// project lists the resource for one project (cache read). nil
	// marks a global resource (projects) that ignores --project.
	project func(*providers.Cache, string) ([]any, error)
	// get returns one resource by selector parts; nil, nil means the
	// selector matched nothing.
	get func(*providers.Cache, []string) (any, error)
	// selectorHint documents the selector shape in usage errors.
	selectorHint string
	// refresh fetches from GCP and saves to the cache.
	refresh func(*providers.Cache, string) error
	// headers are the --table columns, same set as the TUI view.
	headers []string
	// row renders one object as --table cells.
	row func(any) []string
	// viewKind is the TUI view name for this resource, used to
	// record view updates so the TUI's staleness tracking stays
	// honest when the CLI refreshes the shared cache.
	viewKind string
}

// Fetchers is the GCP-facing side of the registry. New wires the real
// providers functions; tests inject fakes, so every refresh path is
// covered without network access.
type Fetchers struct {
	Projects        func() ([]providers.Project, error)
	VMs             func(string) ([]providers.VM, error)
	Disks           func(string) ([]providers.Disk, error)
	Clusters        func(string) ([]providers.Cluster, error)
	Networks        func(string) ([]providers.Network, error)
	Subnetworks     func(string) ([]providers.Subnetwork, error)
	Buckets         func(string) ([]providers.Bucket, error)
	ServiceAccounts func(string) ([]providers.ServiceAccount, error)
	IAMBindings     func(string) ([]providers.IAMBinding, error)
}

// DefaultFetchers returns the real GCP fetchers used by New.
func DefaultFetchers() Fetchers {
	return Fetchers{
		Projects:        providers.ListGcloudProjects,
		VMs:             providers.ListProjectVMs,
		Disks:           providers.ListProjectDisks,
		Clusters:        providers.ListProjectClusters,
		Networks:        providers.ListProjectNetworks,
		Subnetworks:     providers.ListProjectSubnetworks,
		Buckets:         providers.ListProjectBuckets,
		ServiceAccounts: providers.ListProjectServiceAccounts,
		IAMBindings:     providers.ListProjectIAMPolicy,
	}
}

// genericAll and genericProject adapt the typed cache methods to the
// registry's []any shape, so each entry stays a one-liner.
func genericAll[T any](list func(*providers.Cache) ([]T, error)) func(*providers.Cache) ([]any, error) {
	return func(c *providers.Cache) ([]any, error) {
		return toAny(list(c))
	}
}

func genericProject[T any](list func(*providers.Cache, string) ([]T, error)) func(*providers.Cache, string) ([]any, error) {
	return func(c *providers.Cache, projectID string) ([]any, error) {
		return toAny(list(c, projectID))
	}
}

func toAny[T any](items []T, err error) ([]any, error) {
	if err != nil {
		return nil, err
	}
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out, nil
}

// combineAll concatenates two typed lists, used by the networks noun
// whose TUI view mixes VPCs and subnets.
func combineAll[T1, T2 any](l1 func() ([]T1, error), l2 func() ([]T2, error)) ([]any, error) {
	items1, err := l1()
	if err != nil {
		return nil, err
	}
	items2, err := l2()
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(items1)+len(items2))
	for _, item := range items1 {
		out = append(out, item)
	}
	for _, item := range items2 {
		out = append(out, item)
	}
	return out, nil
}

// refreshInto fetches one project's resources and saves them, the
// shared body behind every refresh closure.
func refreshInto[T any](c *providers.Cache, projectID string, fetch func(string) ([]T, error), save func(string, []T) error) error {
	items, err := fetch(projectID)
	if err != nil {
		return err
	}
	return save(projectID, items)
}

// getProject selects one project by id alone.
func getProject(c *providers.Cache, sel []string) (any, error) {
	if len(sel) != 1 {
		return nil, errBadSelector
	}
	p, err := c.GetProject(sel[0])
	if err != nil || p == nil {
		return nil, err
	}
	return *p, nil
}

func projectRow(v any) []string {
	p := v.(providers.Project)
	return []string{p.ProjectID, p.Name, p.ProjectNumber}
}

// getVM selects by project/zone/name.
func getVM(c *providers.Cache, sel []string) (any, error) {
	if len(sel) != 3 {
		return nil, errBadSelector
	}
	vm, err := c.GetVM(sel[0], sel[1], sel[2])
	if err != nil || vm == nil {
		return nil, err
	}
	return *vm, nil
}

func vmRow(v any) []string {
	vm := v.(providers.VM)
	return []string{vm.Name, vm.Zone, vm.MachineType, vm.Status, internalIPs(vm), formatUpdatedAt(vm.UpdatedAt)}
}

func getDisk(c *providers.Cache, sel []string) (any, error) {
	if len(sel) != 3 {
		return nil, errBadSelector
	}
	d, err := c.GetDisk(sel[0], sel[1], sel[2])
	if err != nil || d == nil {
		return nil, err
	}
	return *d, nil
}

func diskRow(v any) []string {
	d := v.(providers.Disk)
	return []string{d.Name, d.Zone, strconv.FormatInt(d.SizeGB, 10), d.Type, d.Status, strings.Join(d.Users, ","), d.SourceImage, formatUpdatedAt(d.UpdatedAt)}
}

// getCluster selects by project/name.
func getCluster(c *providers.Cache, sel []string) (any, error) {
	if len(sel) != 2 {
		return nil, errBadSelector
	}
	cl, err := c.GetCluster(sel[0], sel[1])
	if err != nil || cl == nil {
		return nil, err
	}
	return *cl, nil
}

func clusterRow(v any) []string {
	cl := v.(providers.Cluster)
	return []string{cl.Name, cl.Location, cl.Status, cl.Version, nodePoolsColumn(cl.NodePools), formatUpdatedAt(cl.CreatedAt), formatUpdatedAt(cl.UpdatedAt)}
}

// getNetworks selects a VPC by project/name or a subnet by
// project/region/name.
func getNetworks(c *providers.Cache, sel []string) (any, error) {
	switch len(sel) {
	case 2:
		n, err := c.GetNetwork(sel[0], sel[1])
		if err != nil || n == nil {
			return nil, err
		}
		return *n, nil
	case 3:
		s, err := c.GetSubnetwork(sel[0], sel[1], sel[2])
		if err != nil || s == nil {
			return nil, err
		}
		return *s, nil
	}
	return nil, errBadSelector
}

func networkRow(v any) []string {
	switch n := v.(type) {
	case providers.Network:
		return []string{"vpc", n.Name, "", "", "", n.RoutingMode, "", formatUpdatedAt(n.UpdatedAt)}
	case providers.Subnetwork:
		return []string{"subnet", n.Name, n.Region, n.Network, n.IPRange, "", n.Status, formatUpdatedAt(n.UpdatedAt)}
	}
	return []string{}
}

func getBucket(c *providers.Cache, sel []string) (any, error) {
	if len(sel) != 2 {
		return nil, errBadSelector
	}
	b, err := c.GetBucket(sel[0], sel[1])
	if err != nil || b == nil {
		return nil, err
	}
	return *b, nil
}

func bucketRow(v any) []string {
	b := v.(providers.Bucket)
	return []string{b.Name, b.Location, b.StorageClass, yesNo(b.Versioning), formatUpdatedAt(b.CreatedAt), formatUpdatedAt(b.UpdatedAt)}
}

func getServiceAccount(c *providers.Cache, sel []string) (any, error) {
	if len(sel) != 2 {
		return nil, errBadSelector
	}
	sa, err := c.GetServiceAccount(sel[0], sel[1])
	if err != nil || sa == nil {
		return nil, err
	}
	return *sa, nil
}

func serviceAccountRow(v any) []string {
	sa := v.(providers.ServiceAccount)
	return []string{sa.Email, sa.DisplayName, sa.UniqueID, yesNo(sa.Disabled), formatUpdatedAt(sa.UpdatedAt)}
}

// getIAMBinding selects by project/member/role, where the role itself
// contains slashes (roles/viewer), so everything after the member joins
// back into the role.
func getIAMBinding(c *providers.Cache, sel []string) (any, error) {
	if len(sel) < 3 {
		return nil, errBadSelector
	}
	b, err := c.GetIAMBinding(sel[0], sel[1], strings.Join(sel[2:], "/"))
	if err != nil || b == nil {
		return nil, err
	}
	return *b, nil
}

func iamBindingRow(v any) []string {
	b := v.(providers.IAMBinding)
	return []string{b.Member, iamMemberType(b.Member), b.Role, b.Condition, formatUpdatedAt(b.UpdatedAt)}
}

// defaultRegistry maps nouns to their resource plumbing. Adding a noun
// is one entry: cache reads, a fetch-and-save, headers, a row renderer.
func defaultRegistry(f Fetchers) map[string]resourceDef {
	return map[string]resourceDef{
		"projects": {
			all:          genericAll((*providers.Cache).ListProjects),
			project:      nil,
			get:          getProject,
			selectorHint: "project",
			refresh: func(c *providers.Cache, _ string) error {
				items, err := f.Projects()
				if err != nil {
					return err
				}
				return c.SaveProjects(items)
			},
			headers:  []string{"Project ID", "Name", "Project Number"},
			row:      projectRow,
			viewKind: "projects",
		},
		"vms": {
			all:          genericAll((*providers.Cache).ListAllVMs),
			project:      genericProject((*providers.Cache).ListVMs),
			get:          getVM,
			selectorHint: "project/zone/name",
			refresh: func(c *providers.Cache, projectID string) error {
				return refreshInto(c, projectID, f.VMs, c.SaveVMs)
			},
			headers:  []string{"Name", "Zone", "Machine Type", "Status", "Internal IPs", "Updated"},
			row:      vmRow,
			viewKind: "vms",
		},
		"disks": {
			all:          genericAll((*providers.Cache).ListAllDisks),
			project:      genericProject((*providers.Cache).ListDisks),
			get:          getDisk,
			selectorHint: "project/zone/name",
			refresh: func(c *providers.Cache, projectID string) error {
				return refreshInto(c, projectID, f.Disks, c.SaveDisks)
			},
			headers:  []string{"Name", "Zone", "Size GB", "Type", "Status", "In Use By", "Source Image", "Updated"},
			row:      diskRow,
			viewKind: "disks",
		},
		"clusters": {
			all:          genericAll((*providers.Cache).ListAllClusters),
			project:      genericProject((*providers.Cache).ListClusters),
			get:          getCluster,
			selectorHint: "project/name",
			refresh: func(c *providers.Cache, projectID string) error {
				return refreshInto(c, projectID, f.Clusters, c.SaveClusters)
			},
			headers:  []string{"Name", "Location", "Status", "Version", "Node Pools", "Created", "Updated"},
			row:      clusterRow,
			viewKind: "clusters",
		},
		"networks": {
			all: func(c *providers.Cache) ([]any, error) {
				return combineAll(
					func() ([]providers.Network, error) { return c.ListAllNetworks() },
					func() ([]providers.Subnetwork, error) { return c.ListAllSubnetworks() },
				)
			},
			project: func(c *providers.Cache, projectID string) ([]any, error) {
				return combineAll(
					func() ([]providers.Network, error) { return c.ListNetworks(projectID) },
					func() ([]providers.Subnetwork, error) { return c.ListSubnetworks(projectID) },
				)
			},
			get:          getNetworks,
			selectorHint: "project/name or project/region/name",
			refresh: func(c *providers.Cache, projectID string) error {
				networks, err := f.Networks(projectID)
				if err != nil {
					return err
				}
				subnetworks, err := f.Subnetworks(projectID)
				if err != nil {
					return err
				}
				return c.SaveNetworks(projectID, networks, subnetworks)
			},
			headers:  []string{"Type", "Name", "Region", "Network", "IP Range", "Routing", "Status", "Updated"},
			row:      networkRow,
			viewKind: "networks",
		},
		"buckets": {
			all:          genericAll((*providers.Cache).ListAllBuckets),
			project:      genericProject((*providers.Cache).ListBuckets),
			get:          getBucket,
			selectorHint: "project/name",
			refresh: func(c *providers.Cache, projectID string) error {
				return refreshInto(c, projectID, f.Buckets, c.SaveBuckets)
			},
			headers:  []string{"Name", "Location", "Storage Class", "Versioning", "Created", "Updated"},
			row:      bucketRow,
			viewKind: "buckets",
		},
		"serviceaccounts": {
			all:          genericAll((*providers.Cache).ListAllServiceAccounts),
			project:      genericProject((*providers.Cache).ListServiceAccounts),
			get:          getServiceAccount,
			selectorHint: "project/email",
			refresh: func(c *providers.Cache, projectID string) error {
				return refreshInto(c, projectID, f.ServiceAccounts, c.SaveServiceAccounts)
			},
			headers:  []string{"Email", "Display Name", "Unique ID", "Disabled", "Updated"},
			row:      serviceAccountRow,
			viewKind: "accounts",
		},
		"iam": {
			all:          genericAll((*providers.Cache).ListAllIAMBindings),
			project:      genericProject((*providers.Cache).ListIAMBindings),
			get:          getIAMBinding,
			selectorHint: "project/member/role",
			refresh: func(c *providers.Cache, projectID string) error {
				return refreshInto(c, projectID, f.IAMBindings, c.SaveIAMBindings)
			},
			headers:  []string{"Member", "Type", "Role", "Condition", "Updated"},
			row:      iamBindingRow,
			viewKind: "iam",
		},
	}
}

// formatUpdatedAt follows the TUI's helper of the same name: RFC3339
// timestamps become local "2006-01-02 15:04", anything else prints
// raw.
func formatUpdatedAt(updatedAt string) string {
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return updatedAt
}

// internalIPs joins every interface's internal IP, matching the TUI's
// Internal IPs column.
func internalIPs(vm providers.VM) string {
	ips := make([]string, 0, len(vm.Interfaces))
	for _, iface := range vm.Interfaces {
		if iface.InternalIP != "" {
			ips = append(ips, iface.InternalIP)
		}
	}
	return strings.Join(ips, ",")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func nodePoolsColumn(pools []providers.ClusterNodePool) string {
	switch len(pools) {
	case 0:
		return ""
	case 1:
		return pools[0].Name
	default:
		return fmt.Sprintf("%s (+%d)", pools[0].Name, len(pools)-1)
	}
}

// iamMemberType extracts the prefix before the first colon of an IAM
// member (user, serviceAccount, group, domain).
func iamMemberType(member string) string {
	if i := strings.Index(member, ":"); i > 0 {
		return member[:i]
	}
	return member
}

// Aliases maps alternate nouns to registry keys. Registry keys are
// valid nouns themselves.
var Aliases = map[string]string{
	"vm":             "vms",
	"disk":           "disks",
	"cluster":        "clusters",
	"network":        "networks",
	"bucket":         "buckets",
	"serviceaccount": "serviceaccounts",
	"sa":             "serviceaccounts",
	"project":        "projects",
}

// ResourceNames returns the sorted registry keys.
func ResourceNames() []string {
	reg := defaultRegistry(Fetchers{})
	names := make([]string, 0, len(reg))
	for name := range reg {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveAlias normalizes a noun to its registry key.
func ResolveAlias(name string) string {
	if canonical, ok := Aliases[name]; ok {
		return canonical
	}
	return name
}
