package cmd

import (
	"testing"
)

// TestSetVersion tests the SetVersion function
func TestSetVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
	}{
		{
			name:    "release version",
			version: "v1.0.0",
			commit:  "abc123def456",
			date:    "2026-03-11T15:30:00Z",
		},
		{
			name:    "development version",
			version: "v1.0.0-5-abc123d",
			commit:  "abc123def456",
			date:    "2026-03-11T15:30:00Z",
		},
		{
			name:    "dirty version",
			version: "v1.0.0-5-abc123d-dirty",
			commit:  "abc123def456",
			date:    "2026-03-11T15:30:00Z",
		},
		{
			name:    "empty values",
			version: "",
			commit:  "",
			date:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original values
			origVersion := Version
			origCommit := Commit
			origDate := Date

			// Set new values
			SetVersion(tt.version, tt.commit, tt.date)

			// Verify they were set correctly
			if Version != tt.version {
				t.Errorf("SetVersion(%q, ...) set Version to %q, expected %q", tt.version, Version, tt.version)
			}
			if Commit != tt.commit {
				t.Errorf("SetVersion(..., %q, ...) set Commit to %q, expected %q", tt.commit, Commit, tt.commit)
			}
			if Date != tt.date {
				t.Errorf("SetVersion(..., ..., %q) set Date to %q, expected %q", tt.date, Date, tt.date)
			}

			// Restore original values
			SetVersion(origVersion, origCommit, origDate)
		})
	}
}
