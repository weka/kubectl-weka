package types

import (
	"testing"
)

func TestCheckStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   CheckStatus
		expected string
	}{
		{
			name:     "StatusSkipped",
			status:   StatusSkipped,
			expected: "⏭️ SKIPPED (Node not ready)",
		},
		{
			name:     "StatusPass",
			status:   StatusPass,
			expected: "✅ OK",
		},
		{
			name:     "StatusWarn",
			status:   StatusWarn,
			expected: "⚠️ WARNING",
		},
		{
			name:     "StatusFail",
			status:   StatusFail,
			expected: "❌ FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("CheckStatus = %q, want %q", string(tt.status), tt.expected)
			}
		})
	}
}

func TestCheckStatusString(t *testing.T) {
	status := StatusPass
	result := string(status)
	if result != "✅ OK" {
		t.Errorf("CheckStatus string conversion failed: got %q", result)
	}
}

func TestCheckStatusComparison(t *testing.T) {
	t.Run("equal statuses", func(t *testing.T) {
		if StatusPass != StatusPass {
			t.Error("StatusPass should equal StatusPass")
		}
	})

	t.Run("different statuses", func(t *testing.T) {
		if StatusPass == StatusFail {
			t.Error("StatusPass should not equal StatusFail")
		}
	})
}
