package version

// Version information - set by main package via SetVersion()
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// ParsedVersion represents a parsed semantic version
type ParsedVersion struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// ParseSemver parses a semantic version string (e.g., "v0.2.1", "0.2.1", "0.2.1-alpha")
// Returns a ParsedVersion with the parsed components
// If parsing fails, returns the raw version with zero components
func ParseSemver(versionStr string) ParsedVersion {
	return parseVersion(versionStr)
}

// HasMinorVersionChange checks if two versions have a different minor version
// Returns true if major or minor version differs
func (pv ParsedVersion) HasMinorVersionChange(other ParsedVersion) bool {
	return pv.Major != other.Major || pv.Minor != other.Minor
}

// IsSameMinorVersion checks if two versions have the same major and minor versions
func (pv ParsedVersion) IsSameMinorVersion(other ParsedVersion) bool {
	return !pv.HasMinorVersionChange(other)
}
