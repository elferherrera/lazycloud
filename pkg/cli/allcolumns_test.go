package cli

import (
	"testing"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

// TestProjectIDAllTypes locks the all-projects Project column: every
// registry object must yield its owning project.
func TestProjectIDAllTypes(t *testing.T) {
	cases := []struct {
		v    any
		want string
	}{
		{providers.VM{ProjectID: "p1"}, "p1"},
		{providers.Disk{ProjectID: "p2"}, "p2"},
		{providers.Cluster{ProjectID: "p3"}, "p3"},
		{providers.Network{ProjectID: "p4"}, "p4"},
		{providers.Subnetwork{ProjectID: "p5"}, "p5"},
		{providers.Bucket{ProjectID: "p6"}, "p6"},
		{providers.ServiceAccount{ProjectID: "p7"}, "p7"},
		{providers.IAMBinding{ProjectID: "p8"}, "p8"},
		{"not a resource", ""},
	}
	for _, tt := range cases {
		if got := projectID(tt.v); got != tt.want {
			t.Errorf("projectID(%T) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

// TestRegistryAllColumns verifies the per-entry defaults: scoped
// entries get a Project-terminated header set and a rowAll that
// appends the project id, global entries keep neither.
func TestRegistryAllColumns(t *testing.T) {
	reg := defaultRegistry(Fetchers{})
	for noun, def := range reg {
		if def.project == nil {
			if len(def.headersAll) != 0 || def.rowAll != nil {
				t.Errorf("%s is global, must not gain all-projects columns", noun)
			}
			continue
		}
		if len(def.headersAll) != len(def.headers)+1 {
			t.Errorf("%s headersAll len = %d, want %d", noun, len(def.headersAll), len(def.headers)+1)
		}
		if got := def.headersAll[len(def.headersAll)-1]; got != "Project" {
			t.Errorf("%s headersAll ends with %q, want Project", noun, got)
		}
		if def.rowAll == nil {
			t.Errorf("%s rowAll must default", noun)
		}
	}
	// The projects row itself has no project column; a global entry
	// reuses def.row through renderItems untouched.
	if reg["projects"].headersAll != nil {
		t.Error("projects must stay global")
	}
}
