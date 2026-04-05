package utils

import (
	"testing"
)

func TestNormalizeValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic text",
			input:    "Hello",
			expected: "hello",
		},
		{
			name:     "with spaces",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "uppercase",
			input:    "HELLO",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeValue(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeSet(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]struct{}
	}{
		{
			name:     "basic list",
			input:    []string{"a", "b", "c"},
			expected: map[string]struct{}{"a": {}, "b": {}, "c": {}},
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c"},
			expected: map[string]struct{}{"a": {}, "b": {}, "c": {}},
		},
		{
			name:     "with case differences",
			input:    []string{"A", "a", "B"},
			expected: map[string]struct{}{"a": {}, "b": {}},
		},
		{
			name:     "with spaces",
			input:    []string{"  a  ", "b", "  c  "},
			expected: map[string]struct{}{"a": {}, "b": {}, "c": {}},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "empty strings",
			input:    []string{"", "a", ""},
			expected: map[string]struct{}{"a": {}},
		},
		{
			name:     "only empty strings",
			input:    []string{"", "  ", ""},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSet(tt.input)
			if !mapsEqual(result, tt.expected) {
				t.Errorf("NormalizeSet(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with v prefix",
			input:    "v1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "without v prefix",
			input:    "1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only v",
			input:    "v",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeVersion(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBoolToOkError(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "true",
			input:    true,
			expected: "OK",
		},
		{
			name:     "false",
			input:    false,
			expected: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BoolToOkError(tt.input)
			if result != tt.expected {
				t.Errorf("BoolToOkError(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper function
func mapsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
