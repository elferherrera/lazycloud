package providers

import "testing"

func TestLastProjectRoundtrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	p, err := LoadLastProject()
	if err != nil {
		t.Fatalf("LoadLastProject() on missing file error = %v", err)
	}
	if p.ProjectID != "" {
		t.Fatalf("LoadLastProject() on missing file = %+v, want zero value", p)
	}

	want := LastProject{ProjectID: "p1", Name: "P One", ProjectNumber: "111"}
	if err := SaveLastProject(want); err != nil {
		t.Fatalf("SaveLastProject() error = %v", err)
	}

	got, err := LoadLastProject()
	if err != nil {
		t.Fatalf("LoadLastProject() error = %v", err)
	}
	if got != want {
		t.Fatalf("LoadLastProject() = %+v, want %+v", got, want)
	}

	overridden := LastProject{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}
	if err := SaveLastProject(overridden); err != nil {
		t.Fatalf("SaveLastProject() overwrite error = %v", err)
	}
	got, err = LoadLastProject()
	if err != nil {
		t.Fatalf("LoadLastProject() error = %v", err)
	}
	if got != overridden {
		t.Fatalf("LoadLastProject() = %+v, want %+v", got, overridden)
	}
}
