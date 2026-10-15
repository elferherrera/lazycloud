package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShortcutsRoundtrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	defaults := map[string]string{"1": "projects", "2": "vms", "0": "firewalls"}
	got, err := LoadShortcuts(defaults)
	if err != nil {
		t.Fatalf("LoadShortcuts() on missing file error = %v", err)
	}
	if len(got) != len(defaults) {
		t.Fatalf("LoadShortcuts() on missing file = %v, want defaults", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "lazycloud", "shortcuts.json")); err != nil {
		t.Fatalf("LoadShortcuts() did not write defaults: %v", err)
	}

	overridden := map[string]string{"1": "clusters", "5": "ips"}
	if err := SaveShortcuts(overridden); err != nil {
		t.Fatalf("SaveShortcuts() error = %v", err)
	}
	got, err = LoadShortcuts(defaults)
	if err != nil {
		t.Fatalf("LoadShortcuts() error = %v", err)
	}
	if got["1"] != "clusters" || got["5"] != "ips" {
		t.Fatalf("LoadShortcuts() = %v, want saved shortcuts", got)
	}
}