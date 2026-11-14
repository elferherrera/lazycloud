package providers

import (
	"os"
	"path/filepath"
)

func shortcutsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "shortcuts.json"), nil
}
