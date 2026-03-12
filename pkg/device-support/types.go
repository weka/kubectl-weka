package device_support

import (
	"fmt"
	"strings"
)

// NICInfo contains comprehensive information about a network interface controller
type NICInfo struct {
	VendorID                string // e.g., "8086", "15b3", "1af4"
	DeviceID                string // e.g., "1021", "0955"
	Vendor                  string // e.g., "Intel", "Mellanox", "RedHat"
	ShortModel              string // e.g., "ConnectX-7", "E810", "vmxnet3"
	Model                   string // Full model as appears in lspci, e.g., "MT2910 Family [ConnectX-7]"
	Speed                   string // Port speed (e.g., "100Gb/s", "10Gb/s", "1Gb/s")
	NumPorts                int    // Number of ports on this NIC
	InterfaceMode           string // "Ethernet", "InfiniBand", or "Converged" (Ethernet + InfiniBand)
	MinSupportedWekaVersion string // Minimum Weka version this device is supported in (e.g., "4.2" or "4.2.3.191")
	MaxSupportedWekaVersion string // Maximum Weka version this device is supported in (e.g., "4.10" or "4.10.19.200")
}

// NVMeInfo contains comprehensive information about an NVMe drive
type NVMeInfo struct {
	VendorID                string // e.g., "8086", "144d" (Samsung)
	DeviceID                string // e.g., "0953"
	Vendor                  string // e.g., "Intel", "Samsung", "Kioxia"
	ShortModel              string // e.g., "DC P3320", "SM961"
	Model                   string // Full model as appears in lspci
	MinSupportedWekaVersion string // Minimum Weka version this device is supported in
	MaxSupportedWekaVersion string // Maximum Weka version this device is supported in
}

// NICCapabilities defines what Weka can do with a particular NIC
type NICCapabilities struct {
	SupportedByWekaDpdk             bool // Can be used with Weka DPDK (may need dedicated NIC per process)
	SupportedByWekaDpdkSingleNic    bool // Can share single NIC between multiple Weka processes
	SupportedByWekaUdp              bool // Can be used with Weka UDP mode (fallback for older/unsupported NICs)
	SupportedByWekaForLacpSameCard  bool // LACP supported on 2 ports of same NIC
	SupportedByWekaForLacpDiffCards bool // LACP supported across different cards (currently false for all)
}

// Version checking utilities

// isVersionInRange checks if a given version is within the min and max constraints.
// Versions can be in formats like "4.2.3.191" or partial like "4.2" or "4.10.19"
// Partial versions match any version that starts with that prefix.
// If minVersion or maxVersion is empty string, that constraint is ignored.
func IsVersionInRange(checkVersion, minVersion, maxVersion string) bool {
	if checkVersion == "" {
		return true // Can't validate if no version specified
	}

	// Check minimum version constraint
	if minVersion != "" {
		if !versionGreaterOrEqual(checkVersion, minVersion) {
			return false
		}
	}

	// Check maximum version constraint
	if maxVersion != "" {
		if !versionLessOrEqual(checkVersion, maxVersion) {
			return false
		}
	}

	return true
}

// versionGreaterOrEqual checks if actualVersion >= constraintVersion
// Handles partial versions (e.g., "4.2" matches "4.2.3.191")
func versionGreaterOrEqual(actualVersion, constraintVersion string) bool {
	actual := normalizeVersion(actualVersion)
	constraint := normalizeVersion(constraintVersion)

	return compareVersions(actual, constraint) >= 0
}

// versionLessOrEqual checks if actualVersion <= constraintVersion
// Handles partial versions (e.g., "4.2" matches "4.2.3.191")
func versionLessOrEqual(actualVersion, constraintVersion string) bool {
	actual := normalizeVersion(actualVersion)
	constraint := normalizeVersion(constraintVersion)

	return compareVersions(actual, constraint) <= 0
}

// normalizeVersion splits a version string into parts and pads with zeros
// e.g., "4.2" becomes [4, 2, 0, 0], "4.2.3.191" becomes [4, 2, 3, 191]
func normalizeVersion(version string) []int {
	parts := strings.Split(version, ".")
	normalized := make([]int, 4) // Always 4 parts for comparison

	for i, part := range parts {
		if i >= 4 {
			break // Only compare up to 4 parts
		}
		var num int
		_, _ = fmt.Sscanf(part, "%d", &num)
		normalized[i] = num
	}

	return normalized
}

// compareVersions compares two normalized versions
// Returns: < 0 if v1 < v2, 0 if v1 == v2, > 0 if v1 > v2
func compareVersions(v1, v2 []int) int {
	for i := 0; i < len(v1) && i < len(v2); i++ {
		if v1[i] < v2[i] {
			return -1
		}
		if v1[i] > v2[i] {
			return 1
		}
	}
	return 0
}
