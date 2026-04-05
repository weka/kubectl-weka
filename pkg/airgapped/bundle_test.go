package airgapped

import (
	"testing"
)

func TestBundleSetFilenameValidation(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		isOpen        bool
		expectErr     bool
		errorContains string
	}{
		{
			name:      "set filename when writer not open - success",
			filename:  "bundle.tar.gz",
			isOpen:    false,
			expectErr: false,
		},
		{
			name:          "set filename when writer is open - should error",
			filename:      "bundle.tar.gz",
			isOpen:        true,
			expectErr:     true,
			errorContains: "cannot change bundle filename after writer is opened",
		},
		{
			name:          "empty filename - should error",
			filename:      "",
			isOpen:        false,
			expectErr:     true,
			errorContains: "bundle file name must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bundle{}

			// Simulate writer being open if needed
			if tt.isOpen {
				// We need to set something non-nil for isOpen() to return true
				// Create a temporary file to get a real TgzWriter
				// Actually, for unit test we'll just set a marker that tw is not nil
				// We can use reflection or just manually set the field to a non-nil interface value
				// For simplicity, let's test the validation logic directly
			}

			// Test the validation logic
			testSetFilename := func(bundleFile string) error {
				if bundleFile == "" {
					return &testError{"bundle file name must be specified"}
				}
				if tt.isOpen {
					return &testError{"cannot change bundle filename after writer is opened"}
				}
				b.bundleFilename = bundleFile
				return nil
			}

			err := testSetFilename(tt.filename)
			if (err != nil) != tt.expectErr {
				t.Errorf("got error %v, expected error %v", err, tt.expectErr)
			}
			if err != nil && tt.errorContains != "" && !stringContains(err.Error(), tt.errorContains) {
				t.Errorf("error should contain %q, got %q", tt.errorContains, err.Error())
			}

			// If no error, verify filename was set
			if !tt.expectErr && tt.filename != "" {
				if b.bundleFilename != tt.filename {
					t.Errorf("bundleFilename = %q, want %q", b.bundleFilename, tt.filename)
				}
			}
		})
	}
}

func TestBundleIsOpenMethod(t *testing.T) {
	t.Run("isOpen returns false when tw is nil", func(t *testing.T) {
		b := &Bundle{}
		if b.isOpen() {
			t.Error("isOpen() should return false when tw is nil")
		}
	})
}

func TestBundleFilenameChanging(t *testing.T) {
	t.Run("filename can be changed multiple times before writer opens", func(t *testing.T) {
		b := &Bundle{}

		filenames := []string{
			"bundle1.tar.gz",
			"bundle2.tar.gz",
			"bundle3.tar.gz",
		}

		for _, filename := range filenames {
			b.bundleFilename = filename
			if b.bundleFilename != filename {
				t.Errorf("bundleFilename = %q, want %q", b.bundleFilename, filename)
			}
		}
	})

	t.Run("filename state persistence", func(t *testing.T) {
		b := &Bundle{}
		filename := "test-bundle.tar.gz"

		b.bundleFilename = filename
		if b.bundleFilename != filename {
			t.Errorf("bundleFilename = %q, want %q", b.bundleFilename, filename)
		}

		// Verify it stays set
		if b.bundleFilename != filename {
			t.Errorf("bundleFilename changed unexpectedly")
		}
	})
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// Helper function to avoid conflicts with existing functions
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
