package utils

import (
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{
			name:     "basic split",
			input:    "a,b,c",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "split with whitespace",
			input:    "a , b , c",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "split with tabs and newlines",
			input:    "a\t,\tb\n,\tc",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty string",
			input:    "",
			sep:      ",",
			expected: []string{""},
		},
		{
			name:     "single element",
			input:    "hello",
			sep:      ",",
			expected: []string{"hello"},
		},
		{
			name:     "empty separator",
			input:    "abc",
			sep:      "",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "all empty parts",
			input:    ",,",
			sep:      ",",
			expected: []string{"", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitAndTrim(tt.input, tt.sep)
			if !sliceEqual(result, tt.expected) {
				t.Errorf("SplitAndTrim(%q, %q) = %v, want %v", tt.input, tt.sep, result, tt.expected)
			}
		})
	}
}

func TestRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"zero length", 0},
		{"small length", 5},
		{"medium length", 32},
		{"large length", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RandomString(tt.length)
			if len(result) != tt.length {
				t.Errorf("RandomString(%d) length = %d, want %d", tt.length, len(result), tt.length)
			}
			// Check that all characters are alphanumeric
			for _, ch := range result {
				if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
					t.Errorf("RandomString contains invalid character: %c", ch)
				}
			}
		})
	}
}

func TestJoinWithTruncation(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		max      int
		expected string
	}{
		{
			name:     "under limit",
			items:    []string{"a", "b", "c"},
			max:      5,
			expected: "a, b, c",
		},
		{
			name:     "at limit",
			items:    []string{"a", "b", "c"},
			max:      3,
			expected: "a, b, c",
		},
		{
			name:     "over limit",
			items:    []string{"a", "b", "c", "d", "e"},
			max:      2,
			expected: "a, b, ...(+3)",
		},
		{
			name:     "empty list",
			items:    []string{},
			max:      5,
			expected: "",
		},
		{
			name:     "max is 0",
			items:    []string{"a", "b"},
			max:      0,
			expected: ", ...(+2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinWithTruncation(tt.items, tt.max)
			if result != tt.expected {
				t.Errorf("JoinWithTruncation(%v, %d) = %q, want %q", tt.items, tt.max, result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{
			name:      "under limit",
			input:     "hello",
			maxLength: 10,
			expected:  "hello",
		},
		{
			name:      "at limit",
			input:     "hello",
			maxLength: 5,
			expected:  "hello",
		},
		{
			name:      "over limit",
			input:     "hello world",
			maxLength: 5,
			expected:  "hello...",
		},
		{
			name:      "empty string",
			input:     "",
			maxLength: 5,
			expected:  "",
		},
		{
			name:      "max is 0",
			input:     "hello",
			maxLength: 0,
			expected:  "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLength)
			if result != tt.expected {
				t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.maxLength, result, tt.expected)
			}
		})
	}
}

func TestIndentText(t *testing.T) {
	tests := []struct {
		name            string
		text            string
		spaces          int
		subsequentSpace []int
		expected        string
	}{
		{
			name:     "simple indent",
			text:     "hello",
			spaces:   2,
			expected: "  hello",
		},
		{
			name:     "multiline indent",
			text:     "hello\nworld",
			spaces:   2,
			expected: "  hello\n  world",
		},
		{
			name:            "with subsequent space",
			text:            "hello\nworld",
			spaces:          2,
			subsequentSpace: []int{4},
			expected:        "  hello\n      world",
		},
		{
			name:     "zero spaces",
			text:     "hello",
			spaces:   0,
			expected: "hello",
		},
		{
			name:     "empty text",
			text:     "",
			spaces:   2,
			expected: "",
		},
		{
			name:     "empty lines",
			text:     "hello\n\nworld",
			spaces:   2,
			expected: "  hello\n\n  world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IndentText(tt.text, tt.spaces, tt.subsequentSpace...)
			if result != tt.expected {
				t.Errorf("IndentText(%q, %d, %v) = %q, want %q", tt.text, tt.spaces, tt.subsequentSpace, result, tt.expected)
			}
		})
	}
}

// Helper function
func sliceEqual(a, b []string) bool {
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
