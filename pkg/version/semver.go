package version

import (
	"regexp"
	"strconv"
	"strings"
)

// semverRegex matches semantic version strings like v0.2.1, 0.2.1, 0.2.1-alpha, 0.2.1+build
// Also matches goreleaser-style: 0.2.5-SNAPSHOT-abcdef or 0.2.5-0d2770f
// Pattern explanation:
// - ^v? - optional 'v' prefix
// - (\d+) - major version (digits)
// - \.(\d+) - dot + minor version (digits)
// - (?:\.(\d+))? - optional dot + patch version (digits)
// - (?:[-+][a-zA-Z0-9.-]*)? - optional prerelease/build metadata with hyphens
// - $ - end of string
var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)(?:\.(\d+))?(?:[-+][a-zA-Z0-9.-]*)?$`)

// parseVersion parses a semantic version string and returns a ParsedVersion
// Supports formats like: "v0.2.1", "0.2.1", "0.2.1-alpha", "0.2.1+build"
// If parsing fails, returns ParsedVersion with Raw set and zero components
func parseVersion(versionStr string) ParsedVersion {
	parsed := ParsedVersion{
		Raw: versionStr,
	}

	// Handle empty or "dev" version
	if versionStr == "" || versionStr == "dev" {
		return parsed
	}

	// Try regex match
	matches := semverRegex.FindStringSubmatch(versionStr)
	if len(matches) < 3 {
		// No match - return raw version
		return parsed
	}

	// Extract major (matches[1])
	if major, err := strconv.Atoi(matches[1]); err == nil {
		parsed.Major = major
	}

	// Extract minor (matches[2])
	if minor, err := strconv.Atoi(matches[2]); err == nil {
		parsed.Minor = minor
	}

	// Extract patch (matches[3]), only if present
	if len(matches) > 3 && matches[3] != "" {
		if patch, err := strconv.Atoi(matches[3]); err == nil {
			parsed.Patch = patch
		}
	}

	return parsed
}

// String returns the parsed version as a string (without prerelease/build metadata)
func (pv ParsedVersion) String() string {
	if pv.Major == 0 && pv.Minor == 0 && pv.Patch == 0 && pv.Raw != "" {
		// Unparseable version - return raw
		return pv.Raw
	}

	if pv.Patch > 0 {
		return strings.Trim(pv.Raw, "v")
	}

	return ""
}

// IsZero checks if version is unparseable (all zeros and no raw value)
func (pv ParsedVersion) IsZero() bool {
	return pv.Major == 0 && pv.Minor == 0 && pv.Patch == 0
}

// GetMinorVersion returns "major.minor" format for easy comparison
func (pv ParsedVersion) GetMinorVersion() string {
	return strings.Trim(strings.TrimPrefix(pv.Raw, "v"), "-.+")
}
