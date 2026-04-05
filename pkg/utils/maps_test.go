package utils

import (
	"errors"
	"testing"
)

func TestKeysOf(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]struct{}
		expected []string
	}{
		{
			name:     "empty map",
			input:    map[string]struct{}{},
			expected: nil,
		},
		{
			name:     "single key",
			input:    map[string]struct{}{"a": {}},
			expected: []string{"a"},
		},
		{
			name:     "multiple keys",
			input:    map[string]struct{}{"a": {}, "b": {}, "c": {}},
			expected: []string{"a", "b", "c"}, // Note: map iteration order is random
		},
		{
			name:     "nil map",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KeysOf(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("KeysOf(%v) length = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			// For single and empty cases, we can check equality
			if len(tt.expected) <= 1 {
				if !sliceEqualStrings(result, tt.expected) {
					t.Errorf("KeysOf(%v) = %v, want %v", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestKeysOfSorted(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]struct{}
		expected []string
	}{
		{
			name:     "empty map",
			input:    map[string]struct{}{},
			expected: nil,
		},
		{
			name:     "single key",
			input:    map[string]struct{}{"a": {}},
			expected: []string{"a"},
		},
		{
			name:     "multiple keys sorted",
			input:    map[string]struct{}{"c": {}, "a": {}, "b": {}},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "nil map",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KeysOfSorted(tt.input)
			if !sliceEqualStrings(result, tt.expected) {
				t.Errorf("KeysOfSorted(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBoolPtr(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		ptr := BoolPtr(true)
		if ptr == nil || *ptr != true {
			t.Error("BoolPtr(true) should return pointer to true")
		}
	})

	t.Run("false", func(t *testing.T) {
		ptr := BoolPtr(false)
		if ptr == nil || *ptr != false {
			t.Error("BoolPtr(false) should return pointer to false")
		}
	})
}

func TestShortErr(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantSubstr string
	}{
		{
			name:       "simple error",
			err:        errors.New("simple error"),
			wantSubstr: "simple error",
		},
		{
			name:       "error with newlines",
			err:        errors.New("error\nwith\nnewlines"),
			wantSubstr: "error with newlines",
		},
		{
			name:       "error with leading/trailing spaces",
			err:        errors.New("   error with spaces   "),
			wantSubstr: "error with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShortErr(tt.err)
			if result != tt.wantSubstr {
				t.Errorf("ShortErr() = %q, want %q", result, tt.wantSubstr)
			}
			// Check no newlines in result
			for _, ch := range result {
				if ch == '\n' {
					t.Error("ShortErr should not contain newlines")
				}
			}
		})
	}
}

// Helper function
func sliceEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
