package wekaversion

import (
	"testing"
)

func TestParseWekaVersion(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		wantMajor   int
		wantMinor   int
		wantPatch   int
		wantBuild   int
		wantRaw     string
		shouldError bool
	}{
		{
			name:        "full quay.io reference with build",
			image:       "quay.io/weka.io/weka-in-container:4.4.10.200",
			wantMajor:   4,
			wantMinor:   4,
			wantPatch:   10,
			wantBuild:   200,
			wantRaw:     "4.4.10.200",
			shouldError: false,
		},
		{
			name:        "three-part version",
			image:       "weka/weka:4.2.5",
			wantMajor:   4,
			wantMinor:   2,
			wantPatch:   5,
			wantBuild:   0,
			wantRaw:     "4.2.5",
			shouldError: false,
		},
		{
			name:        "version with suffix",
			image:       "quay.io/weka.io/weka:5.1.0.461-qa-alpha",
			wantMajor:   5,
			wantMinor:   1,
			wantPatch:   0,
			wantBuild:   461,
			wantRaw:     "5.1.0.461",
			shouldError: false,
		},
		{
			name:        "registry with port",
			image:       "registry.example.com:5000/weka:4.3.0.100",
			wantMajor:   4,
			wantMinor:   3,
			wantPatch:   0,
			wantBuild:   100,
			wantRaw:     "4.3.0.100",
			shouldError: false,
		},
		{
			name:        "no version tag",
			image:       "quay.io/weka.io/weka-in-container",
			shouldError: true,
		},
		{
			name:        "invalid major version",
			image:       "weka:a.2.5",
			shouldError: true,
		},
		{
			name:        "incomplete version",
			image:       "weka:4.2",
			shouldError: true,
		},
		{
			name:        "invalid minor version",
			image:       "weka:4.b.5",
			shouldError: true,
		},
		{
			name:        "invalid patch version",
			image:       "weka:4.2.c",
			shouldError: true,
		},
		{
			name:        "version with rc suffix",
			image:       "weka:4.4.10-rc1",
			wantMajor:   4,
			wantMinor:   4,
			wantPatch:   10,
			wantBuild:   0,
			wantRaw:     "4.4.10",
			shouldError: false,
		},
		{
			name:        "version with dev suffix",
			image:       "weka:5.0.0.500-dev",
			wantMajor:   5,
			wantMinor:   0,
			wantPatch:   0,
			wantBuild:   500,
			wantRaw:     "5.0.0.500",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := ParseWekaVersion(tt.image)

			if (err != nil) != tt.shouldError {
				t.Errorf("ParseWekaVersion(%q) error = %v, wantError %v", tt.image, err, tt.shouldError)
				return
			}

			if tt.shouldError {
				return
			}

			if version.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", version.Major, tt.wantMajor)
			}
			if version.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", version.Minor, tt.wantMinor)
			}
			if version.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", version.Patch, tt.wantPatch)
			}
			if version.Build != tt.wantBuild {
				t.Errorf("Build = %d, want %d", version.Build, tt.wantBuild)
			}
			if version.Raw != tt.wantRaw {
				t.Errorf("Raw = %q, want %q", version.Raw, tt.wantRaw)
			}
		})
	}
}

func TestWekaVersionString(t *testing.T) {
	tests := []struct {
		name     string
		version  WekaVersion
		expected string
	}{
		{
			name:     "with build number",
			version:  WekaVersion{Major: 4, Minor: 4, Patch: 10, Build: 200, Raw: "4.4.10.200"},
			expected: "4.4.10.200",
		},
		{
			name:     "without build number",
			version:  WekaVersion{Major: 4, Minor: 2, Patch: 5, Build: 0, Raw: "4.2.5"},
			expected: "4.2.5",
		},
		{
			name:     "zero patch",
			version:  WekaVersion{Major: 5, Minor: 0, Patch: 0, Build: 461, Raw: "5.0.0.461"},
			expected: "5.0.0.461",
		},
		{
			name:     "all zeros except major",
			version:  WekaVersion{Major: 3, Minor: 0, Patch: 0, Build: 0, Raw: "3.0.0"},
			expected: "3.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestWekaVersionComparison(t *testing.T) {
	v1 := WekaVersion{Major: 4, Minor: 4, Patch: 10, Build: 200, Raw: "4.4.10.200"}
	v2 := WekaVersion{Major: 4, Minor: 4, Patch: 10, Build: 200, Raw: "4.4.10.200"}
	v3 := WekaVersion{Major: 4, Minor: 4, Patch: 10, Build: 201, Raw: "4.4.10.201"}

	t.Run("same versions equal", func(t *testing.T) {
		if v1 != v2 {
			t.Error("Same versions should be equal")
		}
	})

	t.Run("different versions not equal", func(t *testing.T) {
		if v1 == v3 {
			t.Error("Different versions should not be equal")
		}
	})
}

func TestParseWekaVersionEdgeCases(t *testing.T) {
	t.Run("large version numbers", func(t *testing.T) {
		version, err := ParseWekaVersion("weka:99.99.99.9999")
		if err != nil {
			t.Fatalf("ParseWekaVersion failed: %v", err)
		}
		if version.Major != 99 || version.Minor != 99 || version.Patch != 99 || version.Build != 9999 {
			t.Error("Large version numbers not parsed correctly")
		}
	})

	t.Run("multiple colons in registry", func(t *testing.T) {
		version, err := ParseWekaVersion("registry.example.com:5000:8080/weka:4.4.10.200")
		if err != nil {
			t.Fatalf("ParseWekaVersion failed: %v", err)
		}
		if version.Major != 4 || version.Minor != 4 {
			t.Error("Version with multiple colons not parsed correctly")
		}
	})

	t.Run("multiple dashes in suffix", func(t *testing.T) {
		version, err := ParseWekaVersion("weka:4.4.10-rc1-build123")
		if err != nil {
			t.Fatalf("ParseWekaVersion failed: %v", err)
		}
		if version.Major != 4 || version.Minor != 4 || version.Patch != 10 {
			t.Error("Version with multiple dashes not parsed correctly")
		}
		if version.Raw != "4.4.10" {
			t.Errorf("Raw version should stop at dash, got %q", version.Raw)
		}
	})
}
