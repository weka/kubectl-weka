package utils

import (
	"strings"
)

// NormalizeValue normalizes a string by trimming whitespace and converting to lowercase
func NormalizeValue(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func NormalizeSet(values []string) map[string]struct{} {
	if values == nil || len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = NormalizeValue(v)
		if v != "" {
			out[v] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func NormalizeVersion(version string) string {
	if version == "" {
		return ""
	}
	if strings.HasPrefix(version, "v") {
		return version[1:]
	}
	return version
}

// BoolToOkError converts a bool to "OK" or "ERROR"
func BoolToOkError(v bool) string {
	if v {
		return "OK"
	}
	return "ERROR"
}
