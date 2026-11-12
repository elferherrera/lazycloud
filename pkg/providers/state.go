package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type LastProject struct {
	ProjectID     string `json:"project_id"`
	Name          string `json:"name"`
	ProjectNumber string `json:"project_number"`
}

func lastProjectPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "last_project.json"), nil
}

func SaveLastProject(p LastProject) error {
	path, err := lastProjectPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadLastProject() (LastProject, error) {
	var p LastProject
	path, err := lastProjectPath()
	if err != nil {
		return p, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return p, err
	}
	err = json.Unmarshal(data, &p)
	return p, err
}
