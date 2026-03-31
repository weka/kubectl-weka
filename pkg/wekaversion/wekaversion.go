package wekaversion

import (
	"fmt"
	"strconv"
	"strings"
)

// WekaVersion represents a parsed WEKA version
type WekaVersion struct {
	Major int
	Minor int
	Patch int
	Build int
	Raw   string
}

func (v WekaVersion) String() string {
	if v.Build > 0 {
		return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseWekaVersion extracts version from WEKA container image
// Supports formats like:
//   - quay.io/weka.io/weka-in-container:4.4.10.200
//   - weka/weka:4.2.5
//   - registry.example.com/weka:4.3.0.100
//   - quay.io/weka.io/weka:5.1.0.461-qa-alpha
func ParseWekaVersion(image string) (*WekaVersion, error) {
	// Extract version from image tag (everything after the last ':')
	// Format: <registry>/<image>:<version>
	colonIndex := strings.LastIndex(image, ":")
	if colonIndex == -1 {
		return nil, fmt.Errorf("image does not contain version tag: %s", image)
	}

	versionStr := image[colonIndex+1:]

	// Remove any suffix after a dash (e.g., "-qa-alpha", "-rc1", "-dev")
	// This allows us to parse "5.1.0.461-qa-alpha" as "5.1.0.461"
	if dashIndex := strings.Index(versionStr, "-"); dashIndex != -1 {
		versionStr = versionStr[:dashIndex]
	}

	// Parse version components (e.g., "4.4.10.200" or "4.2.5")
	versionParts := strings.Split(versionStr, ".")
	if len(versionParts) < 3 {
		return nil, fmt.Errorf("invalid version format: %s (expected at least major.minor.patch)", versionStr)
	}

	version := &WekaVersion{Raw: versionStr}

	// Parse major version
	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version '%s': %w", versionParts[0], err)
	}
	version.Major = major

	// Parse minor version
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version '%s': %w", versionParts[1], err)
	}
	version.Minor = minor

	// Parse patch version
	patch, err := strconv.Atoi(versionParts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version '%s': %w", versionParts[2], err)
	}
	version.Patch = patch

	// Parse build version (optional)
	if len(versionParts) >= 4 {
		build, err := strconv.Atoi(versionParts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid build version '%s': %w", versionParts[3], err)
		}
		version.Build = build
	}

	return version, nil
}
