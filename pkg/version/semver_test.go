package version

import (
	"testing"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input         string
		expectedMajor int
		expectedMinor int
		expectedPatch int
		expectRaw     string
	}{
		{"0.2.1", 0, 2, 1, "0.2.1"},
		{"v0.2.1", 0, 2, 1, "v0.2.1"},
		{"0.3.0", 0, 3, 0, "0.3.0"},
		{"1.0.0", 1, 0, 0, "1.0.0"},
		{"0.2.1-alpha", 0, 2, 1, "0.2.1-alpha"},
		{"0.2.1+build", 0, 2, 1, "0.2.1+build"},
		{"0.2", 0, 2, 0, "0.2"},
		{"v0.2", 0, 2, 0, "v0.2"},
		{"0.2.5-SNAPSHOT-0d2770f", 0, 2, 5, "0.2.5-SNAPSHOT-0d2770f"}, // goreleaser format
		{"0.2.5-SNAPSHOT-abcdef", 0, 2, 5, "0.2.5-SNAPSHOT-abcdef"},   // goreleaser format
		{"dev", 0, 0, 0, "dev"},
		{"", 0, 0, 0, ""},
		{"invalid", 0, 0, 0, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseSemver(tt.input)

			if result.Major != tt.expectedMajor {
				t.Errorf("Major: expected %d, got %d", tt.expectedMajor, result.Major)
			}
			if result.Minor != tt.expectedMinor {
				t.Errorf("Minor: expected %d, got %d", tt.expectedMinor, result.Minor)
			}
			if result.Patch != tt.expectedPatch {
				t.Errorf("Patch: expected %d, got %d", tt.expectedPatch, result.Patch)
			}
			if result.Raw != tt.expectRaw {
				t.Errorf("Raw: expected %s, got %s", tt.expectRaw, result.Raw)
			}
		})
	}
}

func TestHasMinorVersionChange(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
		desc     string
	}{
		{"0.2.1", "0.2.0", false, "same minor, different patch"},
		{"0.2.1", "0.3.0", true, "different minor"},
		{"0.2.1", "1.0.0", true, "different major"},
		{"0.2.1", "0.2.5", false, "same minor, different patch"},
		{"v0.2.1", "v0.2.5", false, "same minor with v prefix"},
		{"v0.2.1", "v0.3.0", true, "different minor with v prefix"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			pv1 := ParseSemver(tt.v1)
			pv2 := ParseSemver(tt.v2)

			result := pv1.HasMinorVersionChange(pv2)
			if result != tt.expected {
				t.Errorf("HasMinorVersionChange(%s, %s): expected %v, got %v",
					tt.v1, tt.v2, tt.expected, result)
			}
		})
	}
}

func TestIsSameMinorVersion(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected bool
		desc     string
	}{
		{"0.2.1", "0.2.0", true, "same minor"},
		{"0.2.1", "0.3.0", false, "different minor"},
		{"0.2.1", "1.0.0", false, "different major"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			pv1 := ParseSemver(tt.v1)
			pv2 := ParseSemver(tt.v2)

			result := pv1.IsSameMinorVersion(pv2)
			if result != tt.expected {
				t.Errorf("IsSameMinorVersion(%s, %s): expected %v, got %v",
					tt.v1, tt.v2, tt.expected, result)
			}
		})
	}
}
