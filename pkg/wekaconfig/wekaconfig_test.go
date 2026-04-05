package wekaconfig

import (
	"testing"
)

func TestIsValidEthDeviceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid names
		{
			name:     "simple eth device",
			input:    "eth0",
			expected: true,
		},
		{
			name:     "device with hyphen",
			input:    "eth-0",
			expected: true,
		},
		{
			name:     "device with underscore",
			input:    "eth_0",
			expected: true,
		},
		{
			name:     "vlan device",
			input:    "eth0.100",
			expected: true,
		},
		{
			name:     "complex name",
			input:    "eno1-vlan.200",
			expected: true,
		},
		{
			name:     "all uppercase",
			input:    "ETH0",
			expected: true,
		},
		{
			name:     "mixed case",
			input:    "EtH0",
			expected: true,
		},
		{
			name:     "long name",
			input:    "ethernet_interface_10",
			expected: true,
		},

		// Invalid names
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "with space",
			input:    "eth 0",
			expected: false,
		},
		{
			name:     "with special char !",
			input:    "eth!0",
			expected: false,
		},
		{
			name:     "with special char @",
			input:    "eth@0",
			expected: false,
		},
		{
			name:     "with special char #",
			input:    "eth#0",
			expected: false,
		},
		{
			name:     "with special char $",
			input:    "eth$0",
			expected: false,
		},
		{
			name:     "with slash",
			input:    "eth/0",
			expected: false,
		},
		{
			name:     "with colon",
			input:    "eth:0",
			expected: false,
		},
		{
			name:     "with comma",
			input:    "eth,0",
			expected: false,
		},
		{
			name:     "with equals",
			input:    "eth=0",
			expected: false,
		},
		{
			name:     "with parenthesis",
			input:    "eth(0)",
			expected: false,
		},
		{
			name:     "with brackets",
			input:    "eth[0]",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEthDeviceName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidEthDeviceName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWekaConfigObjectType(t *testing.T) {
	t.Run("cluster type", func(t *testing.T) {
		if WekaConfigTypeCluster != "cluster" {
			t.Errorf("WekaConfigTypeCluster = %q, want 'cluster'", WekaConfigTypeCluster)
		}
	})

	t.Run("client type", func(t *testing.T) {
		if WekaConfigTypeClient != "client" {
			t.Errorf("WekaConfigTypeClient = %q, want 'client'", WekaConfigTypeClient)
		}
	})

	t.Run("container type", func(t *testing.T) {
		if WekaConfigTypeContainer != "container" {
			t.Errorf("WekaConfigTypeContainer = %q, want 'container'", WekaConfigTypeContainer)
		}
	})
}

func TestWekaConfigObjectTypeComparison(t *testing.T) {
	t.Run("equal types", func(t *testing.T) {
		t1 := WekaConfigTypeCluster
		t2 := WekaConfigObjectType("cluster")
		if t1 != t2 {
			t.Error("Same object types should be equal")
		}
	})

	t.Run("different types", func(t *testing.T) {
		if WekaConfigTypeCluster == WekaConfigTypeClient {
			t.Error("Different object types should not be equal")
		}
	})
}

func TestWekaConfigObjectTypeAsString(t *testing.T) {
	types := []struct {
		objType  WekaConfigObjectType
		expected string
	}{
		{WekaConfigTypeCluster, "cluster"},
		{WekaConfigTypeClient, "client"},
		{WekaConfigTypeContainer, "container"},
	}

	for _, tt := range types {
		t.Run(string(tt.objType), func(t *testing.T) {
			if string(tt.objType) != tt.expected {
				t.Errorf("string(%v) = %q, want %q", tt.objType, string(tt.objType), tt.expected)
			}
		})
	}
}

func TestValidEthDeviceNames(t *testing.T) {
	validNames := []string{
		"eth0",
		"eth1",
		"eno0",
		"eno1",
		"enp0s25",
		"wlan0",
		"bond0",
		"eth0.100",
		"eth0.200",
		"eth_0",
		"eth-0",
		"vlan100",
		"interface_1",
		"INTERFACE_1",
	}

	for _, name := range validNames {
		if !isValidEthDeviceName(name) {
			t.Errorf("isValidEthDeviceName(%q) = false, want true", name)
		}
	}
}

func TestInvalidEthDeviceNames(t *testing.T) {
	invalidNames := []string{
		"",
		"eth 0",
		"eth!0",
		"eth@0",
		"eth#0",
		"eth$0",
		"/eth0",
		"eth:0",
		"eth,0",
		"eth=0",
		"eth(0)",
		"eth[0]",
		"eth{0}",
		"eth<0>",
		"eth/0",
		"eth|0",
		"eth\\0",
		"eth?0",
		"eth&0",
		"eth%0",
		"eth*0",
		"eth+0",
		"eth`0",
		"eth~0",
	}

	for _, name := range invalidNames {
		if isValidEthDeviceName(name) {
			t.Errorf("isValidEthDeviceName(%q) = true, want false", name)
		}
	}
}
