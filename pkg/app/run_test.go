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

// TestResolveThemeUnknownBackground pins the system-mode fallback: a
// background kind the detection cannot classify keeps the dark section,
// and a light terminal paints with the configured light section.
func TestResolveThemeUnknownBackground(t *testing.T) {
	light := providers.CompleteLightTheme(providers.Theme{Labels: "orange"})
	config := providers.Theme{Mode: "light", Labels: "orange", Text: "aqua"}
	if got := resolveTheme(config, light, providers.BackgroundLight); got != light {
		t.Errorf("resolveTheme(light) = %+v, want the light section", got)
	}
	config.Mode = "system"
	if got := resolveTheme(config, light, providers.BackgroundLight); got != light {
		t.Errorf("resolveTheme(system, light background) = %+v, want the configured light section", got)
	}
	if got := resolveTheme(config, light, providers.BackgroundDark); got != config {
		t.Errorf("resolveTheme(system, dark background) = %+v, want the configured theme section", got)
	}
}
