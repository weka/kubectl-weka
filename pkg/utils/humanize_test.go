package utils

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHumanAge(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
		contains bool // If true, check if result contains expected substring
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "-",
		},
		{
			name:     "seconds",
			input:    time.Duration(30 * time.Second),
			expected: "30s",
		},
		{
			name:     "minutes",
			input:    time.Duration(5 * time.Minute),
			expected: "5m",
		},
		{
			name:     "hours",
			input:    time.Duration(3 * time.Hour),
			expected: "3h",
		},
		{
			name:     "days",
			input:    time.Duration(10 * 24 * time.Hour),
			expected: "10d",
		},
		{
			name:     "string passthrough",
			input:    "custom value",
			expected: "custom value",
		},
		{
			name:     "invalid type",
			input:    123,
			expected: "-",
		},
		{
			name:     "time.Time",
			input:    time.Now().Add(-time.Hour),
			contains: true,
			expected: "h",
		},
		{
			name:     "metav1.Time",
			input:    metav1.NewTime(time.Now().Add(-time.Minute * 5)),
			contains: true,
			expected: "m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanAge(tt.input)
			if tt.contains {
				if !contains(result, tt.expected) {
					t.Errorf("HumanAge(%v) = %q, want to contain %q", tt.input, result, tt.expected)
				}
			} else {
				if result != tt.expected {
					t.Errorf("HumanAge(%v) = %q, want %q", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    1024,
			expected: "1.0 KiB",
		},
		{
			name:     "megabytes",
			bytes:    1024 * 1024,
			expected: "1.0 MiB",
		},
		{
			name:     "gigabytes",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GiB",
		},
		{
			name:     "terabytes",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TiB",
		},
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "partial kilobyte",
			bytes:    512,
			expected: "512 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("HumanBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestHumanMbps(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "Unknown/No link",
		},
		{
			name:     "negative",
			input:    -100,
			expected: "Unknown/No link",
		},
		{
			name:     "mbps",
			input:    100,
			expected: "100 Mbps",
		},
		{
			name:     "gbps",
			input:    10000,
			expected: "10 Gbps",
		},
		{
			name:     "non-integer",
			input:    "invalid",
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanMbps(tt.input)
			if result != tt.expected {
				t.Errorf("HumanMbps(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFirstOrNone(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: "<none>",
		},
		{
			name:     "single element",
			input:    []string{"hello"},
			expected: "hello",
		},
		{
			name:     "multiple elements",
			input:    []string{"hello", "world"},
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    []string{""},
			expected: "<none>",
		},
		{
			name:     "whitespace only",
			input:    []string{"   "},
			expected: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FirstOrNone(tt.input)
			if result != tt.expected {
				t.Errorf("FirstOrNone(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "uppercase",
			input:    "HELLO",
			expected: "HELLO",
		},
		{
			name:     "mixed",
			input:    "hELLO",
			expected: "HELLO",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "single char",
			input:    "a",
			expected: "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CapitalizeFirst(tt.input)
			if result != tt.expected {
				t.Errorf("CapitalizeFirst(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s[len(s)-1:] == substr ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr)
}
