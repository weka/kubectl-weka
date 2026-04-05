package airgapped

import (
	"context"
	"testing"
)

func TestNewDownloadOptions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		outputFile        string
		wekaVersion       string
		operatorVersion   string
		csiVersion        string
		operatorHelmPath  string
		csiHelmPath       string
		archs             string
		expectedArchCount int
	}{
		{
			name:              "basic options",
			outputFile:        "bundle.tar.gz",
			wekaVersion:       "5.3.0",
			operatorVersion:   "1.2.0",
			csiVersion:        "2.1.0",
			operatorHelmPath:  "",
			csiHelmPath:       "",
			archs:             "amd64,arm64",
			expectedArchCount: 2,
		},
		{
			name:              "single architecture",
			outputFile:        "",
			wekaVersion:       "5.3.0",
			operatorVersion:   "",
			csiVersion:        "",
			operatorHelmPath:  "",
			csiHelmPath:       "",
			archs:             "amd64",
			expectedArchCount: 1,
		},
		{
			name:              "with whitespace in archs",
			outputFile:        "",
			wekaVersion:       "5.3.0",
			operatorVersion:   "",
			csiVersion:        "",
			operatorHelmPath:  "",
			csiHelmPath:       "",
			archs:             "amd64 , arm64",
			expectedArchCount: 2,
		},
		{
			name:              "empty archs",
			outputFile:        "",
			wekaVersion:       "5.3.0",
			operatorVersion:   "",
			csiVersion:        "",
			operatorHelmPath:  "",
			csiHelmPath:       "",
			archs:             "",
			expectedArchCount: 1, // SplitAndTrim("", ",") returns [""]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewDownloadOptions(
				ctx,
				tt.outputFile,
				tt.wekaVersion,
				tt.operatorVersion,
				tt.csiVersion,
				tt.operatorHelmPath,
				tt.csiHelmPath,
				tt.archs,
			)

			if opts == nil {
				t.Fatal("NewDownloadOptions returned nil")
			}

			if opts.Ctx != ctx {
				t.Error("Context not properly set")
			}

			if opts.WekaVersion != tt.wekaVersion {
				t.Errorf("WekaVersion = %q, want %q", opts.WekaVersion, tt.wekaVersion)
			}

			if opts.OperatorVersion != tt.operatorVersion {
				t.Errorf("OperatorVersion = %q, want %q", opts.OperatorVersion, tt.operatorVersion)
			}

			if opts.CSIVersion != tt.csiVersion {
				t.Errorf("CSIVersion = %q, want %q", opts.CSIVersion, tt.csiVersion)
			}

			if len(opts.Archs) != tt.expectedArchCount {
				t.Errorf("Archs length = %d, want %d", len(opts.Archs), tt.expectedArchCount)
			}

			// Check output filename is generated when not provided
			if tt.outputFile == "" && opts.OutputFile == "" {
				t.Error("OutputFile should be auto-generated when not provided")
			}
		})
	}
}

func TestDownloadOptionsValidate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		wekaVersion string
		opVersion   string
		opHelmPath  string
		csiVersion  string
		csiHelmPath string
		shouldError bool
	}{
		{
			name:        "valid with weka version",
			wekaVersion: "5.3.0",
			shouldError: false,
		},
		{
			name:        "valid with operator version",
			opVersion:   "1.2.0",
			shouldError: false,
		},
		{
			name:        "valid with operator helm path",
			opHelmPath:  "/path/to/operator",
			shouldError: false,
		},
		{
			name:        "valid with csi version",
			csiVersion:  "2.1.0",
			shouldError: false,
		},
		{
			name:        "valid with csi helm path",
			csiHelmPath: "/path/to/csi",
			shouldError: false,
		},
		{
			name:        "invalid - no components",
			wekaVersion: "",
			opVersion:   "",
			opHelmPath:  "",
			csiVersion:  "",
			csiHelmPath: "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewDownloadOptions(
				ctx,
				"",
				tt.wekaVersion,
				tt.opVersion,
				tt.csiVersion,
				tt.opHelmPath,
				tt.csiHelmPath,
				"amd64",
			)

			err := opts.Validate()
			if (err != nil) != tt.shouldError {
				t.Errorf("Validate() error = %v, shouldError %v", err, tt.shouldError)
			}
		})
	}
}

func TestUploadOptions(t *testing.T) {
	t.Run("create upload options", func(t *testing.T) {
		opts := &UploadOptions{
			BundleFile:  "/path/to/bundle.tar.gz",
			RegistryURL: "registry.example.com",
		}

		if opts.BundleFile != "/path/to/bundle.tar.gz" {
			t.Errorf("BundleFile not set correctly")
		}

		if opts.RegistryURL != "registry.example.com" {
			t.Errorf("RegistryURL not set correctly")
		}
	})
}

func TestDownloadOptionsOutputFilename(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		wekaVersion      string
		operatorVersion  string
		csiVersion       string
		expectedContains []string
	}{
		{
			name:             "with all versions",
			wekaVersion:      "5.3.0",
			operatorVersion:  "1.2.0",
			csiVersion:       "2.1.0",
			expectedContains: []string{"weka-5.3.0", "operator-1.2.0", "csi-2.1.0", "offline-bundle.tar.gz"},
		},
		{
			name:             "with weka and operator",
			wekaVersion:      "5.3.0",
			operatorVersion:  "1.2.0",
			csiVersion:       "",
			expectedContains: []string{"weka-5.3.0", "operator-1.2.0", "offline-bundle.tar.gz"},
		},
		{
			name:             "with weka only",
			wekaVersion:      "5.3.0",
			operatorVersion:  "",
			csiVersion:       "",
			expectedContains: []string{"weka-5.3.0", "offline-bundle.tar.gz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewDownloadOptions(
				ctx,
				"",
				tt.wekaVersion,
				tt.operatorVersion,
				tt.csiVersion,
				"",
				"",
				"amd64",
			)

			filename := opts.OutputFile
			if filename == "" {
				t.Error("OutputFile should be generated")
				return
			}

			for _, expected := range tt.expectedContains {
				if !contains(filename, expected) {
					t.Errorf("OutputFile %q should contain %q", filename, expected)
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr ||
		len(s) >= len(substr) && (s[len(s)-len(substr):] == substr ||
			hasPrefixOrContains(s, substr)))
}

func hasPrefixOrContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (
	// Simple contains check - not perfect but works for test
	containsSimple(s, substr))
}

func containsSimple(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
