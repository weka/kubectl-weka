package device_support

import (
	"testing"
)

func TestIsVersionInRange(t *testing.T) {
	tests := []struct {
		name         string
		checkVersion string
		minVersion   string
		maxVersion   string
		expected     bool
	}{
		// Empty version
		{
			name:         "empty check version",
			checkVersion: "",
			minVersion:   "4.2",
			maxVersion:   "5.0",
			expected:     true, // No version to check = always true
		},

		// Single constraint - minimum
		{
			name:         "meets minimum",
			checkVersion: "4.4.10.200",
			minVersion:   "4.2",
			maxVersion:   "",
			expected:     true,
		},
		{
			name:         "below minimum",
			checkVersion: "4.1.0",
			minVersion:   "4.2",
			maxVersion:   "",
			expected:     false,
		},
		{
			name:         "equal to minimum",
			checkVersion: "4.2.0",
			minVersion:   "4.2",
			maxVersion:   "",
			expected:     true,
		},

		// Single constraint - maximum
		{
			name:         "meets maximum",
			checkVersion: "4.4.10.200",
			minVersion:   "",
			maxVersion:   "5.0",
			expected:     true,
		},
		{
			name:         "exceeds maximum",
			checkVersion: "5.1.0",
			minVersion:   "",
			maxVersion:   "5.0",
			expected:     false,
		},
		{
			name:         "equal to maximum",
			checkVersion: "5.0.0",
			minVersion:   "",
			maxVersion:   "5.0",
			expected:     true,
		},

		// Both constraints
		{
			name:         "within range",
			checkVersion: "4.4.10.200",
			minVersion:   "4.2",
			maxVersion:   "5.0",
			expected:     true,
		},
		{
			name:         "below range",
			checkVersion: "4.1.0",
			minVersion:   "4.2",
			maxVersion:   "5.0",
			expected:     false,
		},
		{
			name:         "above range",
			checkVersion: "5.1.0",
			minVersion:   "4.2",
			maxVersion:   "5.0",
			expected:     false,
		},
		{
			name:         "at minimum boundary",
			checkVersion: "4.2.0",
			minVersion:   "4.2",
			maxVersion:   "5.0",
			expected:     true,
		},
		{
			name:         "at maximum boundary",
			checkVersion: "5.0.0",
			minVersion:   "4.2",
			maxVersion:   "5.0",
			expected:     true,
		},

		// Partial versions
		{
			name:         "partial min version",
			checkVersion: "4.2.3.191",
			minVersion:   "4.2",
			maxVersion:   "",
			expected:     true,
		},
		{
			name:         "partial max version",
			checkVersion: "4.1.0",
			minVersion:   "",
			maxVersion:   "4.2",
			expected:     true,
		},
		{
			name:         "partial versions with dots",
			checkVersion: "4.10.19.200",
			minVersion:   "4.10",
			maxVersion:   "5.0",
			expected:     true,
		},

		// Four-part versions
		{
			name:         "four-part check with two-part min",
			checkVersion: "4.4.10.200",
			minVersion:   "4.4",
			maxVersion:   "",
			expected:     true,
		},
		{
			name:         "four-part check vs four-part min",
			checkVersion: "4.4.10.200",
			minVersion:   "4.4.10.200",
			maxVersion:   "",
			expected:     true,
		},
		{
			name:         "four-part check below four-part min",
			checkVersion: "4.4.10.199",
			minVersion:   "4.4.10.200",
			maxVersion:   "",
			expected:     false,
		},

		// No constraints
		{
			name:         "no constraints",
			checkVersion: "4.4.10.200",
			minVersion:   "",
			maxVersion:   "",
			expected:     true,
		},

		// Major version differences
		{
			name:         "major version less",
			checkVersion: "3.9.9.999",
			minVersion:   "4.0",
			maxVersion:   "",
			expected:     false,
		},
		{
			name:         "major version greater",
			checkVersion: "5.0.0.0",
			minVersion:   "",
			maxVersion:   "4.9",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVersionInRange(tt.checkVersion, tt.minVersion, tt.maxVersion)
			if result != tt.expected {
				t.Errorf("IsVersionInRange(%q, %q, %q) = %v, want %v",
					tt.checkVersion, tt.minVersion, tt.maxVersion, result, tt.expected)
			}
		})
	}
}

func TestNICInfo(t *testing.T) {
	t.Run("create NIC info", func(t *testing.T) {
		nic := NICInfo{
			VendorID:                "8086",
			DeviceID:                "1021",
			Vendor:                  "Intel",
			ShortModel:              "E810",
			Model:                   "Ethernet Adapter E810-CQDA2",
			Speed:                   "100Gb/s",
			NumPorts:                2,
			InterfaceMode:           "Ethernet",
			MinSupportedWekaVersion: "4.2",
			MaxSupportedWekaVersion: "5.0",
		}

		if nic.VendorID != "8086" {
			t.Error("VendorID not set correctly")
		}
		if nic.NumPorts != 2 {
			t.Error("NumPorts not set correctly")
		}
		if nic.Speed != "100Gb/s" {
			t.Error("Speed not set correctly")
		}
	})

	t.Run("NIC with version constraints", func(t *testing.T) {
		nic := NICInfo{
			MinSupportedWekaVersion: "4.4.10",
			MaxSupportedWekaVersion: "5.1.0",
		}

		// Version within range
		if !IsVersionInRange("4.4.10.200", nic.MinSupportedWekaVersion, nic.MaxSupportedWekaVersion) {
			t.Error("Version should be within NIC support range")
		}

		// Version below range
		if IsVersionInRange("4.4.9", nic.MinSupportedWekaVersion, nic.MaxSupportedWekaVersion) {
			t.Error("Version should be outside NIC support range")
		}
	})
}

func TestNVMeInfo(t *testing.T) {
	t.Run("create NVMe info", func(t *testing.T) {
		nvme := NVMeInfo{
			VendorID:                "8086",
			DeviceID:                "0953",
			Vendor:                  "Intel",
			ShortModel:              "DC P3320",
			Model:                   "Intel SSD DC P3320 Series",
			MinSupportedWekaVersion: "4.2",
			MaxSupportedWekaVersion: "4.10",
		}

		if nvme.VendorID != "8086" {
			t.Error("VendorID not set correctly")
		}
		if nvme.Vendor != "Intel" {
			t.Error("Vendor not set correctly")
		}
	})
}

func TestNICCapabilities(t *testing.T) {
	t.Run("high-end NIC capabilities", func(t *testing.T) {
		caps := NICCapabilities{
			SupportedByWekaDpdk:             true,
			SupportedByWekaDpdkSingleNic:    true,
			SupportedByWekaUdp:              true,
			SupportedByWekaForLacpSameCard:  true,
			SupportedByWekaForLacpDiffCards: false,
		}

		if !caps.SupportedByWekaDpdk {
			t.Error("Should support DPDK")
		}
		if !caps.SupportedByWekaDpdkSingleNic {
			t.Error("Should support DPDK single NIC")
		}
	})

	t.Run("legacy NIC capabilities", func(t *testing.T) {
		caps := NICCapabilities{
			SupportedByWekaDpdk:             false,
			SupportedByWekaDpdkSingleNic:    false,
			SupportedByWekaUdp:              true,
			SupportedByWekaForLacpSameCard:  false,
			SupportedByWekaForLacpDiffCards: false,
		}

		if caps.SupportedByWekaDpdk {
			t.Error("Should not support DPDK")
		}
		if !caps.SupportedByWekaUdp {
			t.Error("Should support UDP fallback")
		}
	})
}
