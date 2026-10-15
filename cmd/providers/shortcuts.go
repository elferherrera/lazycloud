package providers

import (
	"encoding/json"
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

func SaveShortcuts(shortcuts map[string]string) error {
	path, err := shortcutsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(shortcuts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadShortcuts(defaults map[string]string) (map[string]string, error) {
	path, err := shortcutsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := SaveShortcuts(defaults); err != nil {
				return nil, err
			}
			return defaults, nil
		}
		return nil, err
	}
	var shortcuts map[string]string
	if err := json.Unmarshal(data, &shortcuts); err != nil {
		return nil, err
	}
	return shortcuts, nil
}