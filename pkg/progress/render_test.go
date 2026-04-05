package progress

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "single byte",
			bytes:    1,
			expected: "1 B",
		},
		{
			name:     "512 bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "megabytes",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "terabytes",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TB",
		},
		{
			name:     "partial kilobyte",
			bytes:    1536,
			expected: "1.5 KB",
		},
		{
			name:     "partial megabyte",
			bytes:    1024*1024 + 512*1024,
			expected: "1.5 MB",
		},
		{
			name:     "large kilobytes",
			bytes:    1024 * 512,
			expected: "512.0 KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestRenderProgressEdgeCases(t *testing.T) {
	// These tests just verify that RenderProgress doesn't panic
	tests := []struct {
		name      string
		current   int64
		total     int64
		category  string
		operation string
	}{
		{
			name:      "zero total",
			current:   0,
			total:     0,
			category:  "test",
			operation: "testing",
		},
		{
			name:      "current exceeds total",
			current:   150,
			total:     100,
			category:  "test",
			operation: "testing",
		},
		{
			name:      "normal progress",
			current:   50,
			total:     100,
			category:  "test",
			operation: "testing",
		},
		{
			name:      "complete progress",
			current:   100,
			total:     100,
			category:  "test",
			operation: "testing",
		},
		{
			name:      "large numbers",
			current:   1024 * 1024 * 1024,
			total:     2 * 1024 * 1024 * 1024,
			category:  "test",
			operation: "testing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just call RenderProgress and make sure it doesn't panic
			// We won't test the actual output since it goes to stdout with \r
			RenderProgress(tt.current, tt.total, tt.category, tt.operation)
		})
	}
}
