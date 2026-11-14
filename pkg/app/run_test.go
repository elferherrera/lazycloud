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
	originalActive := applyActiveThemeFn
	applyActiveThemeFn = func() {
		applied = true
	}
	t.Cleanup(func() {
		applyActiveThemeFn = originalActive
	})

	_ = Run()

	if !applied {
		t.Error("Run() returned before applying the config theme")
	}
}

// TestFollowTerminalThemeUsesTerminalColors pins the follow-terminal
// palette: every slot is a tcell palette name or "default", never a hex
// pin, so the terminal resolves each color from its own configuration.
func TestFollowTerminalThemeUsesTerminalColors(t *testing.T) {
	follow := providers.FollowTerminalTheme()
	if follow.Background != "default" || follow.Text != "default" || follow.Borders != "default" {
		t.Errorf("FollowTerminalTheme() base slots = %+v, want default so the terminal paints them", follow)
	}
	if follow.Labels != "yellow" || follow.Title != "cyan" || follow.Focus != "green" {
		t.Errorf("FollowTerminalTheme() accents = %+v, want named tcell palette colors", follow)
	}
	if follow.ActiveRowText != "black" || follow.ActiveRowBackground != "cyan" {
		t.Errorf("FollowTerminalTheme() active row = %+v, want black on cyan", follow)
	}
}

// TestApplyActiveThemeFollowTerminal pins the settings selection: the
// follow-terminal toggle paints with the follow palette, a pack selection
// paints with the pack, and the toggle wins over the selected pack.
func TestApplyActiveThemeFollowTerminal(t *testing.T) {
	configuredFollowTerminal = true
	configuredTheme = providers.Theme{Background: "#272822", Labels: "#a6e22e"}
	t.Cleanup(func() {
		configuredFollowTerminal = false
		configuredTheme = providers.Theme{}
	})
	applyActiveTheme()
	if theme.background != "default" || theme.labels != "yellow" {
		t.Errorf("applyActiveTheme() follow-terminal slots = %q/%q, want default/yellow", theme.background, theme.labels)
	}
	configuredFollowTerminal = false
	applyActiveTheme()
	if theme.background != "#272822" || theme.labels != "#a6e22e" {
		t.Errorf("applyActiveTheme() pack slots = %q/%q, want the pack palette", theme.background, theme.labels)
	}
}
