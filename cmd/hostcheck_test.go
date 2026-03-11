package cmd

import (
	"testing"
)

// TestHostChecksResultFields tests that HostChecksResult has expected fields
func TestHostChecksResultFields(t *testing.T) {
	result := &HostChecksResult{
		IsRHCOS:           true,
		OSRelease:         "RHCOS 4.10",
		KernelVersion:     "5.15.0",
		WekaDirOK:         true,
		WekaDirPath:       "/mnt/weka",
		WekaDirAvailBytes: 1099511627776, // 1TB
		XFSInstalled:      true,
		WekaClientClean:   true,
		Mellanox:          true,
		PhysicalCores:     32,
		LogicalCores:      64,
		MemoryBytes:       274877906944, // 256GB
		HTEnabled:         true,
		NVMeDriveCount:    4,
		CPUSockets:        2,
	}

	// Test that basic fields can be set and read
	tests := []struct {
		name  string
		check func() bool
	}{
		{
			name:  "OS detection",
			check: func() bool { return result.IsRHCOS && result.OSRelease == "RHCOS 4.10" },
		},
		{
			name:  "kernel version",
			check: func() bool { return result.KernelVersion == "5.15.0" },
		},
		{
			name:  "WEKA directory",
			check: func() bool { return result.WekaDirOK && result.WekaDirPath == "/mnt/weka" },
		},
		{
			name:  "file system",
			check: func() bool { return result.XFSInstalled },
		},
		{
			name:  "client status",
			check: func() bool { return result.WekaClientClean },
		},
		{
			name:  "network detection",
			check: func() bool { return result.Mellanox },
		},
		{
			name:  "CPU info",
			check: func() bool { return result.PhysicalCores == 32 && result.LogicalCores == 64 },
		},
		{
			name:  "memory info",
			check: func() bool { return result.MemoryBytes == 274877906944 },
		},
		{
			name:  "NVMe drives",
			check: func() bool { return result.NVMeDriveCount == 4 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Errorf("HostChecksResult field check failed for %s", tt.name)
			}
		})
	}
}

// TestMellanoxIfaceStructure tests MellanoxIface struct
func TestMellanoxIfaceStructure(t *testing.T) {
	iface := MellanoxIface{
		Name:  "ib0",
		Bond:  "",
		IP:    "10.0.0.1/24",
		Model: "CX-7",
		Speed: "400Gbps",
	}

	if iface.Name != "ib0" {
		t.Errorf("Expected name 'ib0', got %q", iface.Name)
	}
	if iface.Model != "CX-7" {
		t.Errorf("Expected model 'CX-7', got %q", iface.Model)
	}
	if iface.Speed != "400Gbps" {
		t.Errorf("Expected speed '400Gbps', got %q", iface.Speed)
	}
}

// TestBondInfoStructure tests BondInfo struct
func TestBondInfoStructure(t *testing.T) {
	bond := BondInfo{
		Name:   "bond0",
		IP:     "10.0.0.5/24",
		Slaves: []string{"ib0", "ib1"},
		Mode:   "802.3ad",
		Speed:  "800Gbps",
	}

	if bond.Name != "bond0" {
		t.Errorf("Expected name 'bond0', got %q", bond.Name)
	}
	if len(bond.Slaves) != 2 {
		t.Errorf("Expected 2 slaves, got %d", len(bond.Slaves))
	}
	if bond.Mode != "802.3ad" {
		t.Errorf("Expected mode '802.3ad', got %q", bond.Mode)
	}
}

// TestNetworkInterfaceMetrics tests NetworkInterfaceMetrics struct
func TestNetworkInterfaceMetrics(t *testing.T) {
	metrics := &NetworkInterfaceMetrics{
		BytesIn:      1000000000, // 1GB
		BytesOut:     500000000,  // 500MB
		PacketsIn:    1000000,
		PacketsOut:   500000,
		ErrorsIn:     10,
		ErrorsOut:    5,
		DroppedIn:    2,
		DroppedOut:   1,
		CollisionsIn: 0,
		OverrunsIn:   0,
		OverrunsOut:  0,
		CRCErrors:    0,
	}

	if metrics.BytesIn != 1000000000 {
		t.Errorf("Expected BytesIn 1000000000, got %d", metrics.BytesIn)
	}
	if metrics.PacketsIn != 1000000 {
		t.Errorf("Expected PacketsIn 1000000, got %d", metrics.PacketsIn)
	}
	if metrics.ErrorsIn != 10 {
		t.Errorf("Expected ErrorsIn 10, got %d", metrics.ErrorsIn)
	}
}

// TestNetworkInterface tests NetworkInterface struct with all fields
func TestNetworkInterface(t *testing.T) {
	iface := &NetworkInterface{
		Name:           "eth0",
		Type:           "ethernet",
		IP:             "10.0.0.1/24",
		MTU:            1500,
		MAC:            "52:54:00:12:34:56",
		BondMaster:     "",
		BondSlave:      false,
		MaxSpeed:       "10Gbps",
		EffectiveSpeed: "10Gbps",
		PCIAddress:     "0000:01:00.0",
		Model:          "Intel I350",
		Status:         "up",
		Metrics: &NetworkInterfaceMetrics{
			BytesIn:   5000000000,
			BytesOut:  3000000000,
			PacketsIn: 5000000,
		},
	}

	if iface.Name != "eth0" {
		t.Errorf("Expected name 'eth0', got %q", iface.Name)
	}
	if iface.Type != "ethernet" {
		t.Errorf("Expected type 'ethernet', got %q", iface.Type)
	}
	if iface.PCIAddress != "0000:01:00.0" {
		t.Errorf("Expected PCIAddress '0000:01:00.0', got %q", iface.PCIAddress)
	}
	if iface.MaxSpeed != "10Gbps" {
		t.Errorf("Expected MaxSpeed '10Gbps', got %q", iface.MaxSpeed)
	}
	if iface.EffectiveSpeed != "10Gbps" {
		t.Errorf("Expected EffectiveSpeed '10Gbps', got %q", iface.EffectiveSpeed)
	}
	if iface.Metrics == nil {
		t.Errorf("Expected Metrics to be set, got nil")
	}
	if iface.Metrics.BytesIn != 5000000000 {
		t.Errorf("Expected Metrics.BytesIn 5000000000, got %d", iface.Metrics.BytesIn)
	}
}

// TestNetworkInterfaceInfiniBand tests InfiniBand interface
func TestNetworkInterfaceInfiniBand(t *testing.T) {
	iface := &NetworkInterface{
		Name:           "ib0",
		Type:           "infiniband",
		IP:             "192.168.1.10/24",
		MTU:            2048,
		BondSlave:      false,
		MaxSpeed:       "400Gbps",
		EffectiveSpeed: "400Gbps",
		PCIAddress:     "0000:3d:00.0",
		Model:          "Mellanox ConnectX-7",
		Status:         "up",
	}

	if iface.Type != "infiniband" {
		t.Errorf("Expected type 'infiniband', got %q", iface.Type)
	}
	if iface.MaxSpeed != "400Gbps" {
		t.Errorf("Expected MaxSpeed '400Gbps', got %q", iface.MaxSpeed)
	}
	if iface.Model != "Mellanox ConnectX-7" {
		t.Errorf("Expected Model 'Mellanox ConnectX-7', got %q", iface.Model)
	}
}

// TestNVMeDriveInfo tests NVMeDriveInfo struct
func TestNVMeDriveInfo(t *testing.T) {
	drive := NVMeDriveInfo{
		DeviceName:   "nvme0n1",
		DevicePath:   "/dev/nvme0n1",
		SerialNumber: "ABC123XYZ",
		Model:        "Samsung 970 EVO",
		Size:         1099511627776, // 1TB
		Mounted:      true,
		MountPoint:   "/mnt/nvme0n1",
		PCIAddress:   "0000:01:00.0",
	}

	if drive.DeviceName != "nvme0n1" {
		t.Errorf("Expected DeviceName 'nvme0n1', got %q", drive.DeviceName)
	}
	if drive.Size != 1099511627776 {
		t.Errorf("Expected size 1099511627776, got %d", drive.Size)
	}
	if drive.Model != "Samsung 970 EVO" {
		t.Errorf("Expected model 'Samsung 970 EVO', got %q", drive.Model)
	}
	if !drive.Mounted {
		t.Errorf("Expected Mounted=true, got %v", drive.Mounted)
	}
	if drive.PCIAddress != "0000:01:00.0" {
		t.Errorf("Expected PCIAddress '0000:01:00.0', got %q", drive.PCIAddress)
	}
}
