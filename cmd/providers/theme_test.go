package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestThemeRoundtrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	defaults := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	got, err := LoadTheme(defaults)
	if err != nil {
		t.Fatalf("LoadTheme() on missing file error = %v", err)
	}
	if got != defaults {
		t.Fatalf("LoadTheme() on missing file = %+v, want defaults %+v", got, defaults)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "lazycloud", "theme.json")); err != nil {
		t.Fatalf("LoadTheme() did not write defaults: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "lazycloud", "theme.json"))
	if err != nil {
		t.Fatalf("reading generated theme file: %v", err)
	}
	var generated Theme
	if err := json.Unmarshal(data, &generated); err != nil {
		t.Fatalf("unmarshalling generated theme file: %v", err)
	}
	if generated != defaults {
		t.Fatalf("generated theme file = %+v, want defaults %+v", generated, defaults)
	}

	custom := Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "red", ShortcutText: "silver", StatusLabels: "teal", StatusText: "olive", SummaryLabels: "maroon", SummaryText: "navy", DetailLabels: "purple", DetailText: "pink", ActiveRowText: "lime", ActiveRowBackground: "teal"}
	if err := SaveTheme(custom); err != nil {
		t.Fatalf("SaveTheme() error = %v", err)
	}
	got, err = LoadTheme(defaults)
	if err != nil {
		t.Fatalf("LoadTheme() error = %v", err)
	}
	if got != custom {
		t.Fatalf("LoadTheme() = %+v, want saved theme %+v", got, custom)
	}
}

func TestThemeMigrate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	defaults := Theme{Labels: "yellow", Text: "white", Borders: "white", Title: "white", ShortcutLabels: "yellow", ShortcutText: "white", StatusLabels: "yellow", StatusText: "white", SummaryLabels: "yellow", SummaryText: "white", DetailLabels: "yellow", DetailText: "white", ActiveRowText: "black", ActiveRowBackground: "white"}
	legacy := Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange"}
	if err := SaveTheme(legacy); err != nil {
		t.Fatalf("SaveTheme() error = %v", err)
	}

	got, err := LoadTheme(defaults)
	if err != nil {
		t.Fatalf("LoadTheme() error = %v", err)
	}
	want := Theme{Labels: "aqua", Text: "gray", Borders: "green", Title: "orange", ShortcutLabels: "aqua", ShortcutText: "gray", StatusLabels: "yellow", StatusText: "gray", SummaryLabels: "aqua", SummaryText: "gray", DetailLabels: "yellow", DetailText: "gray", ActiveRowText: "black", ActiveRowBackground: "gray"}
	if got != want {
		t.Fatalf("LoadTheme() = %+v, want migrated %+v", got, want)
	}

	var onDisk Theme
	data, err := os.ReadFile(filepath.Join(home, ".config", "lazycloud", "theme.json"))
	if err != nil {
		t.Fatalf("reading theme file: %v", err)
	}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("unmarshalling theme file: %v", err)
	}
	if onDisk != want {
		t.Fatalf("theme file = %+v, want migrated %+v", onDisk, want)
	}

	if err := os.WriteFile(filepath.Join(home, ".config", "lazycloud", "theme.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("writing theme file: %v", err)
	}
	got, err = LoadTheme(defaults)
	if err != nil {
		t.Fatalf("LoadTheme() error = %v", err)
	}
	if got != defaults {
		t.Fatalf("LoadTheme() = %+v, want defaults %+v", got, defaults)
	}
}
