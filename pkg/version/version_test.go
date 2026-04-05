package version

import (
	"testing"
)

func TestVersionVariables(t *testing.T) {
	// Test that version variables are accessible and have some value
	t.Run("version is set", func(t *testing.T) {
		if Version == "" {
			t.Error("Version should not be empty")
		}
	})

	t.Run("default version", func(t *testing.T) {
		if Version != "dev" {
			t.Logf("Version is %q (expected 'dev' before release)", Version)
		}
	})

	t.Run("commit and date are set", func(t *testing.T) {
		if Commit == "" {
			t.Logf("Commit is empty (expected value or 'none')")
		}
		if Date == "" {
			t.Logf("Date is empty (expected value or 'unknown')")
		}
	})
}
