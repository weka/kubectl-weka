package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SanitizeFilename(s string) string {
	r := strings.NewReplacer(
		"/", "_",
		":", "_",
		"@", "_",
		"\\", "_",
	)
	s = r.Replace(strings.TrimSpace(s))
	if s == "" {
		return "image"
	}
	return s
}

func FilePathJoinSafe(baseDir, name string) (string, error) {
	clean := filepath.Clean(name)
	if clean == "." || clean == "" {
		return baseDir, nil
	}
	target := filepath.Join(baseDir, clean)
	rel, err := filepath.Rel(baseDir, target)
	if err != nil {
		return "", fmt.Errorf("validate archive path %q: %w", name, err)
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("archive contains path traversal entry %q", name)
	}
	return target, nil
}

// WriteFile writes content to a file
func WriteFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

// ReadFile reads content from a file
func ReadFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
