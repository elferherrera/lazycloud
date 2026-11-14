package cli

import (
	"reflect"
	"testing"

	"github.com/elferherrera/lazycloud/pkg/cloud"
	"github.com/elferherrera/lazycloud/pkg/providers"
)

func TestResourceNames(t *testing.T) {
	want := []string{"buckets", "clusters", "disks", "iam", "networks", "projects", "serviceaccounts", "vms"}
	if got := ResourceNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("ResourceNames = %v, want %v", got, want)
	}
}

func TestResolveAlias(t *testing.T) {
	cases := map[string]string{
		"vms":            "vms",
		"vm":             "vms",
		"disk":           "disks",
		"cluster":        "clusters",
		"network":        "networks",
		"bucket":         "buckets",
		"sa":             "serviceaccounts",
		"serviceaccount": "serviceaccounts",
		"project":        "projects",
		"dragons":        "dragons",
	}
	for in, want := range cases {
		if got := ResolveAlias(in); got != want {
			t.Errorf("ResolveAlias(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGCPFetchersWired(t *testing.T) {
	p, err := cloud.Get(cloud.GCP)
	if err != nil {
		t.Fatalf("cloud.Get(gcp): %v", err)
	}
	f := p.Fetchers
	fields := reflect.ValueOf(f)
	for i := 0; i < fields.NumField(); i++ {
		if fields.Field(i).IsNil() {
			t.Errorf("DefaultFetchers field %s is nil", fields.Type().Field(i).Name)
		}
	}
}

func TestRowRenderers(t *testing.T) {
	cases := []struct {
		name string
		row  func(any) []string
		v    any
		want []string
	}{
		{
			name: "project",
			row:  projectRow,
			v:    providers.Project{ProjectID: "p1", Name: "one", ProjectNumber: "1"},
			want: []string{"p1", "one", "1"},
		},
		{
			name: "vm",
			row:  vmRow,
			v: providers.VM{Name: "vm-01", Zone: "z", MachineType: "m", Status: "RUNNING",
				Interfaces: []providers.VMInterface{{InternalIP: "a"}, {InternalIP: "b"}, {InternalIP: ""}}, UpdatedAt: "raw"},
			want: []string{"vm-01", "z", "m", "RUNNING", "a,b", "raw"},
		},
		{
			name: "vm empty interfaces",
			row:  vmRow,
			v:    providers.VM{Name: "vm-02", UpdatedAt: "raw"},
			want: []string{"vm-02", "", "", "", "", "raw"},
		},
		{
			name: "disk",
			row:  diskRow,
			v:    providers.Disk{Name: "d1", Zone: "z", SizeGB: 10, Type: "t", Status: "READY", Users: []string{"u1", "u2"}, SourceImage: "img", UpdatedAt: "raw"},
			want: []string{"d1", "z", "10", "t", "READY", "u1,u2", "img", "raw"},
		},
		{
			name: "cluster many pools",
			row:  clusterRow,
			v: providers.Cluster{Name: "c1", Location: "l", Status: "RUNNING", Version: "1.31",
				NodePools: []providers.ClusterNodePool{{Name: "p1"}, {Name: "p2"}},
				CreatedAt: "raw", UpdatedAt: "raw"},
			want: []string{"c1", "l", "RUNNING", "1.31", "p1 (+1)", "raw", "raw"},
		},
		{
			name: "network vpc",
			row:  networkRow,
			v:    providers.Network{Name: "n1", RoutingMode: "GLOBAL", UpdatedAt: "raw"},
			want: []string{"vpc", "n1", "", "", "", "GLOBAL", "", "raw"},
		},
		{
			name: "network subnet",
			row:  networkRow,
			v:    providers.Subnetwork{Name: "s1", Region: "r", Network: "n1", IPRange: "10.0.0.0/24", Status: "READY", UpdatedAt: "raw"},
			want: []string{"subnet", "s1", "r", "n1", "10.0.0.0/24", "", "READY", "raw"},
		},
		{
			name: "network unknown type",
			row:  networkRow,
			v:    "not a network",
		},
		{
			name: "bucket",
			row:  bucketRow,
			v:    providers.Bucket{Name: "b1", Location: "EU", StorageClass: "STANDARD", Versioning: true, CreatedAt: "raw", UpdatedAt: "raw"},
			want: []string{"b1", "EU", "STANDARD", "yes", "raw", "raw"},
		},
		{
			name: "service account",
			row:  serviceAccountRow,
			v:    providers.ServiceAccount{Email: "e@x.com", DisplayName: "d", UniqueID: "1", Disabled: true, UpdatedAt: "raw"},
			want: []string{"e@x.com", "d", "1", "yes", "raw"},
		},
		{
			name: "iam binding",
			row:  iamBindingRow,
			v:    providers.IAMBinding{Member: "user:x@example.com", Role: "roles/viewer", Condition: "", UpdatedAt: "raw"},
			want: []string{"user:x@example.com", "user", "roles/viewer", "", "raw"},
		},
		{
			name: "iam binding member without colon",
			row:  iamBindingRow,
			v:    providers.IAMBinding{Member: "allUsers", Role: "roles/viewer", UpdatedAt: "raw"},
			want: []string{"allUsers", "allUsers", "roles/viewer", "", "raw"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want == nil {
				// Rows panic on unsupported types, like the other row
				// renderers; a blank row would hide registry bugs.
				defer func() {
					if recover() == nil {
						t.Error("row must panic on unsupported type")
					}
				}()
			}
			if got := tt.row(tt.v); tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("row = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodePoolsColumn(t *testing.T) {
	cases := []struct {
		pools []providers.ClusterNodePool
		want  string
	}{
		{pools: nil, want: ""},
		{pools: []providers.ClusterNodePool{{Name: "only"}}, want: "only"},
		{pools: []providers.ClusterNodePool{{Name: "a"}, {Name: "b"}, {Name: "c"}}, want: "a (+2)"},
	}
	for _, tt := range cases {
		if got := nodePoolsColumn(tt.pools); got != tt.want {
			t.Errorf("nodePoolsColumn(%v) = %q, want %q", tt.pools, got, tt.want)
		}
	}
}

func TestIAMMemberType(t *testing.T) {
	cases := map[string]string{
		"user:x@example.com":     "user",
		"serviceAccount:a@b.iam": "serviceAccount",
		"group:g@example.com":    "group",
		"domain:example.com":     "domain",
		"nocolon":                "nocolon",
	}
	for in, want := range cases {
		if got := iamMemberType(in); got != want {
			t.Errorf("iamMemberType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestYesNo(t *testing.T) {
	if yesNo(true) != "yes" || yesNo(false) != "no" {
		t.Error("yesNo broken")
	}
}

func TestInternalIPsSkipsEmpty(t *testing.T) {
	vm := providers.VM{Interfaces: []providers.VMInterface{{InternalIP: "a"}, {InternalIP: ""}, {InternalIP: "c"}}}
	if got := internalIPs(vm); got != "a,c" {
		t.Errorf("internalIPs = %q, want a,c", got)
	}
}

func TestRegistryBadSelectors(t *testing.T) {
	reg := defaultRegistry(Fetchers{})
	cases := []struct {
		noun string
		sel  []string
	}{
		{"projects", []string{"a", "b"}},
		{"vms", []string{"a"}},
		// vms/disks accept project/name and project/zone/name now, so
		// the bad shapes are 1-part and 4-part.
		{"disks", []string{"a"}},
		{"disks", []string{"a", "b", "c", "d"}},
		{"clusters", []string{"a"}},
		{"networks", []string{"a"}},
		{"buckets", []string{"a"}},
		{"serviceaccounts", []string{"a"}},
		{"iam", []string{"a", "b"}},
	}
	for _, tt := range cases {
		t.Run(tt.noun, func(t *testing.T) {
			// Getters run against a nil cache; a bad selector must
			// fail before any cache access, so nil is safe here.
			if _, err := reg[tt.noun].get(nil, tt.sel); err == nil {
				t.Errorf("%s with sel %v must be a bad selector", tt.noun, tt.sel)
			}
		})
	}
}

func TestRegistryViewKinds(t *testing.T) {
	reg := defaultRegistry(Fetchers{})
	want := map[string]string{
		"projects":        "projects",
		"vms":             "vms",
		"disks":           "disks",
		"clusters":        "clusters",
		"networks":        "networks",
		"buckets":         "buckets",
		"serviceaccounts": "accounts",
		"iam":             "iam",
	}
	for noun, view := range want {
		if reg[noun].viewKind != view {
			t.Errorf("%s viewKind = %q, want %q", noun, reg[noun].viewKind, view)
		}
	}
}

func TestRegistryHints(t *testing.T) {
	reg := defaultRegistry(Fetchers{})
	for noun, def := range reg {
		if def.selectorHint == "" {
			t.Errorf("%s has no selectorHint", noun)
		}
	}
}

func TestRegistryHeaders(t *testing.T) {
	reg := defaultRegistry(Fetchers{})
	for noun, def := range reg {
		if len(def.headers) == 0 {
			t.Errorf("%s has no headers", noun)
		}
	}
}
