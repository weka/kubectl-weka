package cmd

import (
	"testing"
)

// TestCollectorResultStructure tests CollectorResult structure and fields
func TestCollectorResultStructure(t *testing.T) {
	tests := []struct {
		name          string
		filesCreated  int
		warningsCount int
		status        CollectorStatus
	}{
		{
			name:          "successful collection",
			filesCreated:  5,
			warningsCount: 0,
			status:        StatusSuccess,
		},
		{
			name:          "partial success",
			filesCreated:  3,
			warningsCount: 2,
			status:        StatusPartial,
		},
		{
			name:          "failed collection",
			filesCreated:  0,
			warningsCount: 0,
			status:        StatusFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result CollectorResult

			// Build collector result
			for i := 0; i < tt.filesCreated; i++ {
				result.FilesCreated = append(result.FilesCreated, "file.txt")
			}
			for i := 0; i < tt.warningsCount; i++ {
				result.Warnings = append(result.Warnings, "warning message")
			}
			result.Status = tt.status

			// Validate structure is correct
			if len(result.FilesCreated) != tt.filesCreated {
				t.Errorf("Expected %d files, got %d", tt.filesCreated, len(result.FilesCreated))
			}
			if len(result.Warnings) != tt.warningsCount {
				t.Errorf("Expected %d warnings, got %d", tt.warningsCount, len(result.Warnings))
			}
			if result.Status != tt.status {
				t.Errorf("Expected status %v, got %v", tt.status, result.Status)
			}
		})
	}
}

// TestSecretReference tests SecretReference structure
func TestSecretReference(t *testing.T) {
	ref := SecretReference{
		Name:      "my-secret",
		Namespace: "default",
	}

	if ref.Name != "my-secret" {
		t.Errorf("Expected name 'my-secret', got %q", ref.Name)
	}
	if ref.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got %q", ref.Namespace)
	}
}
