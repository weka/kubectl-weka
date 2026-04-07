package completion

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ListMatchingEntriesInDirectory lists entries in a directory that match a prefix and a callback function.
func ListMatchingEntriesInDirectory(dir string, prefix string, matchFunc func(entry os.DirEntry, fullPath string) bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		fullPath := filepath.Join(dir, name)
		if matchFunc(entry, fullPath) {
			matches = append(matches, name)
		}
	}
	return matches, nil
}

// ListPatternMatchesInDirectory lists entries matching a toComplete pattern and a callback function.
// The callback receives (os.DirEntry, fullPath) and returns true if the entry should be included.
func ListPatternMatchesInDirectory(toComplete string, matchFunc func(entry os.DirEntry, fullPath string) bool) ([]string, error) {
	if toComplete == "" {
		toComplete = "."
	}

	// Expand ~ and ..
	expanded := toComplete
	if strings.HasPrefix(toComplete, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			expanded = home + toComplete[1:]
		}
	}
	if strings.HasPrefix(expanded, "..") || strings.HasPrefix(expanded, ".") {
		abs, err := filepath.Abs(expanded)
		if err == nil {
			expanded = abs
		}
	}

	var suggestions []string
	info, err := os.Stat(expanded)
	if err == nil && info.IsDir() {
		// List entries inside this directory
		matches, err := ListMatchingEntriesInDirectory(expanded, "", matchFunc)
		if err != nil {
			return nil, err
		}
		for _, name := range matches {
			full := filepath.Join(toComplete, name)
			if matchFunc != nil {
				// If it's a directory, add slash
				if entryInfo, err := os.Stat(filepath.Join(expanded, name)); err == nil && entryInfo.IsDir() {
					full += "/"
				}
			}
			suggestions = append(suggestions, full)
		}
		slices.Sort(suggestions)
		return suggestions, nil
	}

	// Otherwise, get parent directory and filter entries by prefix
	parent := filepath.Dir(expanded)
	prefix := filepath.Base(expanded)
	matches, err := ListMatchingEntriesInDirectory(parent, prefix, matchFunc)
	if err != nil {
		return nil, err
	}
	for _, name := range matches {
		full := toComplete + name[len(prefix):]
		if matchFunc != nil {
			if entryInfo, err := os.Stat(filepath.Join(parent, name)); err == nil && entryInfo.IsDir() {
				full += "/"
			}
		}
		suggestions = append(suggestions, full)
	}
	// Remove duplicates and sort
	m := map[string]struct{}{}
	var out []string
	for _, s := range suggestions {
		if _, ok := m[s]; !ok {
			m[s] = struct{}{}
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out, nil
}
