package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu   sync.Mutex
	file *os.File
}

func LogFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud", "lazycloud.log"), nil
}

func NewLogger() (*Logger, error) {
	path, err := LogFilePath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{file: file}, nil
}

func (l *Logger) Update(view, projectID string, count int) {
	l.write("update", view, projectID, fmt.Sprintf("count=%d", count))
}

func (l *Logger) Error(view, projectID, message string) {
	l.write("error", view, projectID, "message="+message)
}

func (l *Logger) write(level, view, projectID, details string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format(time.RFC3339)
	fmt.Fprintf(l.file, "%s level=%s view=%s project=%s %s\n", timestamp, level, view, projectID, details)
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
