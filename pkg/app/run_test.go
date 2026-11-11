package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func TestCheckNestedExecution(t *testing.T) {
	t.Setenv("LAZYCLOUD_CONTEXT", "true")
	err := checkNestedExecution()
	if err == nil || err.Error() != "Detected nested lazycloud execution. Exit this context to start lazycloud" {
		t.Errorf("checkNestedExecution() = %v, want nested execution error", err)
	}
	t.Setenv("LAZYCLOUD_CONTEXT", "false")
	if err := checkNestedExecution(); err != nil {
		t.Errorf("checkNestedExecution() = %v, want nil for false", err)
	}
	t.Setenv("LAZYCLOUD_CONTEXT", "")
	if err := checkNestedExecution(); err != nil {
		t.Errorf("checkNestedExecution() = %v, want nil for empty", err)
	}
}

// TestRunAppliesConfigThemeBeforeFailure proves the theme call site runs
// before Run can fail on later setup: last_project.json as a directory
// makes LoadLastProject fail after config and theme application.
func TestRunAppliesConfigThemeBeforeFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "lazycloud", "last_project.json"), 0o755); err != nil {
		t.Fatalf("creating directory as last_project.json: %v", err)
	}

	applied := false
	originalDetect := detectBackground
	detectBackground = func() providers.BackgroundKind {
		applied = true
		return providers.BackgroundDark
	}
	t.Cleanup(func() { detectBackground = originalDetect })

	_ = Run()

	if !applied {
		t.Error("Run() returned before applying the config theme")
	}
}

// TestResolveThemeMissingPalette keeps resolveTheme total when a palette
// lookup misses: it must fall back to the configured theme.
func TestResolveThemeMissingPalette(t *testing.T) {
	original := palettes
	palettes = map[providers.BackgroundKind]providers.Theme{}
	t.Cleanup(func() { palettes = original })

	config := providers.Theme{Mode: "light", Labels: "orange", Text: "aqua"}
	if got := resolveTheme(config, providers.BackgroundLight); got != config {
		t.Errorf("resolveTheme(missing palette) = %+v, want the configured theme", got)
	}
	config.Mode = "system"
	if got := resolveTheme(config, providers.BackgroundLight); got != config {
		t.Errorf("resolveTheme(system, missing palette) = %+v, want the configured theme", got)
	}
}
