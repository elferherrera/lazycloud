package app

import "testing"

func TestProjectViewKindsFetchable(t *testing.T) {
	seen := make(map[viewKind]bool, len(projectViewKinds))
	for _, kind := range projectViewKinds {
		if seen[kind] {
			t.Errorf("projectViewKinds lists %q twice", kind)
		}
		seen[kind] = true
		def, ok := views[kind]
		if !ok {
			t.Errorf("projectViewKinds contains unregistered kind %q", kind)
			continue
		}
		if def.fetch == nil {
			t.Errorf("project view %q has no fetch", kind)
		}
		if def.refresh == nil {
			t.Errorf("project view %q has no refresh", kind)
		}
		if def.title == "" {
			t.Errorf("project view %q has no title", kind)
		}
	}
}

func TestProjectViewKindsCoverFetchableViews(t *testing.T) {
	listed := make(map[viewKind]bool, len(projectViewKinds))
	for _, kind := range projectViewKinds {
		listed[kind] = true
	}
	for kind, def := range views {
		if def.fetch != nil && !listed[kind] {
			t.Errorf("view %q has fetch but is missing from projectViewKinds", kind)
		}
	}
}
