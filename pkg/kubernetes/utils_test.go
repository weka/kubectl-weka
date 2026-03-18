package kubernetes

import (
	"github.com/weka/kubectl-weka/pkg/utils"
	"testing"
)

// TestParseSelector tests the parseSelector function
func TestParseSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:     "single label",
			selector: "app=weka",
			expected: map[string]string{"app": "weka"},
			wantErr:  false,
		},
		{
			name:     "multiple labels",
			selector: "app=weka,env=prod",
			expected: map[string]string{"app": "weka", "env": "prod"},
			wantErr:  false,
		},
		{
			name:     "single label with spaces",
			selector: "app = weka",
			expected: map[string]string{"app": "weka"},
			wantErr:  false,
		},
		{
			name:     "empty selector",
			selector: "",
			expected: map[string]string{},
			wantErr:  false,
		},
		{
			name:     "complex labels",
			selector: "tier=frontend,version=v1.0",
			expected: map[string]string{"tier": "frontend", "version": "v1.0"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.ParseSelector(tt.selector)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("parseSelector(%q) returned %d items, expected %d", tt.selector, len(result), len(tt.expected))
			}

			// Check values
			for key, expectedValue := range tt.expected {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("parseSelector(%q) missing key %q", tt.selector, key)
				} else if actualValue != expectedValue {
					t.Errorf("parseSelector(%q)[%q] = %q, expected %q", tt.selector, key, actualValue, expectedValue)
				}
			}
		})
	}
}

// TestSortNodeNamesNumerically tests the sortNodeNamesNumerically function
func TestSortNodeNamesNumerically(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "simple numeric names",
			input:    []string{"node3", "node1", "node2"},
			expected: []string{"node1", "node2", "node3"},
		},
		{
			name:     "mixed numeric names",
			input:    []string{"node11", "node2", "node1", "node20"},
			expected: []string{"node1", "node2", "node11", "node20"},
		},
		{
			name:     "names with suffixes",
			input:    []string{"worker-3", "worker-1", "worker-2"},
			expected: []string{"worker-1", "worker-2", "worker-3"},
		},
		{
			name:     "single item",
			input:    []string{"node1"},
			expected: []string{"node1"},
		},
		{
			name:     "already sorted",
			input:    []string{"node1", "node2", "node3"},
			expected: []string{"node1", "node2", "node3"},
		},
		{
			name:     "empty list",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "names with multiple numeric parts",
			input:    []string{"node-2-1", "node-1-2", "node-1-1"},
			expected: []string{"node-1-1", "node-1-2", "node-2-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy input to avoid modifying original
			input := make([]string, len(tt.input))
			copy(input, tt.input)

			SortNodeNamesNumerically(input)

			// Check length
			if len(input) != len(tt.expected) {
				t.Errorf("sortNodeNamesNumerically() returned %d items, expected %d", len(input), len(tt.expected))
			}

			// Check values in order
			for i, expectedValue := range tt.expected {
				if i >= len(input) {
					break
				}
				if input[i] != expectedValue {
					t.Errorf("sortNodeNamesNumerically()[%d] = %q, expected %q", i, input[i], expectedValue)
				}
			}
		})
	}
}

// TestRandomString tests the randomString function
func TestRandomString(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{
			name:    "valid length 8",
			length:  8,
			wantErr: false,
		},
		{
			name:    "valid length 16",
			length:  16,
			wantErr: false,
		},
		{
			name:    "zero length",
			length:  0,
			wantErr: false,
		},
		{
			name:    "large length",
			length:  1000,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.RandomString(tt.length)

			// Check length
			if len(result) != tt.length {
				t.Errorf("randomString(%d) returned string of length %d, expected %d", tt.length, len(result), tt.length)
			}

			// Check that consecutive calls produce different results (for non-zero length)
			if tt.length > 0 {
				result2 := utils.RandomString(tt.length)
				if result == result2 {
					t.Errorf("randomString(%d) produced same string twice: %q", tt.length, result)
				}
			}

			// Check that string only contains valid characters (alphanumeric)
			for _, ch := range result {
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
					t.Errorf("randomString(%d) contains invalid character: %c", tt.length, ch)
				}
			}
		})
	}
}

func TestIndentText(t *testing.T) {
	cases := []struct {
		name            string
		input           string
		spaces          int
		expect          string
		subsequentSpace int
		useSubsequent   bool
	}{
		{
			name:   "single line, no newline",
			input:  "hello",
			spaces: 2,
			expect: "  hello",
		},
		{
			name:   "single line, with newline",
			input:  "hello\n",
			spaces: 2,
			expect: "  hello\n",
		},
		{
			name:   "multi-line",
			input:  "foo\nbar",
			spaces: 4,
			expect: "    foo\n    bar",
		},
		{
			name:   "multi-line with trailing newline",
			input:  "foo\nbar\n",
			spaces: 3,
			expect: "   foo\n   bar\n",
		},
		{
			name:   "empty string",
			input:  "",
			spaces: 2,
			expect: "",
		},
		{
			name:   "zero spaces",
			input:  "hello\nworld",
			spaces: 0,
			expect: "hello\nworld",
		},
		// New cases for subsequentSpace
		{
			name:            "multi-line with subsequentSpace",
			input:           "foo\nbar\nbaz",
			spaces:          2,
			subsequentSpace: 3,
			useSubsequent:   true,
			expect:          "  foo\n     bar\n     baz",
		},
		{
			name:            "multi-line with subsequentSpace and trailing newline",
			input:           "foo\nbar\nbaz\n",
			spaces:          1,
			subsequentSpace: 2,
			useSubsequent:   true,
			expect:          " foo\n   bar\n   baz\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out string
			if c.useSubsequent {
				out = utils.IndentText(c.input, c.spaces, c.subsequentSpace)
			} else {
				out = utils.IndentText(c.input, c.spaces)
			}
			if out != c.expect {
				t.Errorf("indentText(%q, %d, %d) = %q, want %q", c.input, c.spaces, c.subsequentSpace, out, c.expect)
			}
		})
	}
}
