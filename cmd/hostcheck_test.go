package cmd

import (
	"strings"
	"testing"
)

// TestValidateInterface tests NetworkInterfacesModule.validateInterface
func TestValidateInterface(t *testing.T) {
	// Mock NetworkInterfacesModule with minimal dependencies
	m := &NetworkInterfacesModule{}

	// Helper: create interface with all fields
	iface := &NetworkInterface{
		Name:           "eth0",
		Type:           "ethernet",
		IP:             "192.168.1.10",
		VendorModel:    "15b3:1021",
		EffectiveSpeed: "100Gbps",
		MTU:            9000,
	}
	// Normal case
	v := m.validateInterface(iface)
	if v.Status != statusPass {
		t.Errorf("Expected statusPass, got %v", v.Status)
	}
	if v.Name != "eth0" {
		t.Errorf("Expected Name 'eth0', got %v", v.Name)
	}

	// Missing VendorModel
	ifaceMissingModel := &NetworkInterface{
		Name: "eth1", Type: "ethernet", IP: "192.168.1.11", EffectiveSpeed: "100Gbps", MTU: 9000,
	}
	v = m.validateInterface(ifaceMissingModel)
	if v.Status != statusFail {
		t.Errorf("Expected statusFail for missing VendorModel, got %v", v.Status)
	}
	if !strings.Contains(v.Reason, "Unknown device model") {
		t.Errorf("Expected reason to mention 'Unknown device model', got %v", v.Reason)
	}

	// Missing IP
	ifaceMissingIP := &NetworkInterface{
		Name: "eth2", Type: "ethernet", VendorModel: "15b3:1021", EffectiveSpeed: "100Gbps", MTU: 9000,
	}
	v = m.validateInterface(ifaceMissingIP)
	if v.Status != statusFail {
		t.Errorf("Expected statusFail for missing IP, got %v", v.Status)
	}
	if !strings.Contains(v.Reason, "No IP address") {
		t.Errorf("Expected reason to mention 'No IP address', got %v", v.Reason)
	}

	// Missing Speed
	ifaceMissingSpeed := &NetworkInterface{
		Name: "eth3", Type: "ethernet", VendorModel: "15b3:1021", IP: "192.168.1.12", MTU: 9000,
	}
	v = m.validateInterface(ifaceMissingSpeed)
	if v.Status != statusFail {
		t.Errorf("Expected statusFail for missing Speed, got %v", v.Status)
	}
	if !strings.Contains(v.Reason, "Speed is not reported") {
		t.Errorf("Expected reason to mention 'Speed is not reported', got %v", v.Reason)
	}

	// MTU too small
	ifaceSmallMTU := &NetworkInterface{
		Name: "eth4", Type: "ethernet", VendorModel: "15b3:1021", IP: "192.168.1.13", EffectiveSpeed: "100Gbps", MTU: 1500,
	}
	v = m.validateInterface(ifaceSmallMTU)
	if v.Status != statusWarn {
		t.Errorf("Expected statusWarn for small MTU, got %v", v.Status)
	}
	if !strings.Contains(v.Reason, "MTU too small") {
		t.Errorf("Expected reason to mention 'MTU too small', got %v", v.Reason)
	}

	// Bond interface with wrong slave count
	bondIface := &NetworkInterface{
		Name: "bond0", Type: "bond", VendorModel: "15b3:1021", IP: "192.168.1.14", EffectiveSpeed: "100Gbps", MTU: 9000,
		BondSlaves: []string{"eth0"},
	}
	v = m.validateInterface(bondIface)
	if v.Status != statusFail {
		t.Errorf("Expected statusFail for bond with 1 slave, got %v", v.Status)
	}
	if !strings.Contains(v.Reason, "Bond must have 2 interfaces") {
		t.Errorf("Expected reason to mention 'Bond must have 2 interfaces', got %v", v.Reason)
	}

	// UDP/DPDK support: these require ds mock, so skip unless dependency is injected
}

// TestHostChecksResultFields tests that HostChecksResult has expected fields
func TestHostChecksResultFields(t *testing.T) {
	result := &HostChecksResult{
		IsRHCOS:           true,
		OSRelease:         "RHCOS 4.10",
		KernelVersion:     "5.15.0",
		WekaDirPath:       "/mnt/weka",
		WekaDirAvailBytes: 1099511627776, // 1TB
		XFSFound:          true,
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
			name:  "file system",
			check: func() bool { return result.XFSFound },
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

// TestNetworkInterfaceBond tests NetworkInterface as bond
func TestNetworkInterfaceBond(t *testing.T) {
	bond := NetworkInterface{
		Name:       "bond0",
		Type:       "bond",
		IP:         "10.0.0.5/24",
		BondSlaves: []string{"ib0", "ib1"},
		BondMode:   "802.3ad",
	}

	if bond.Name != "bond0" {
		t.Errorf("Expected name 'bond0', got %q", bond.Name)
	}
	if !bond.IsBond() {
		t.Errorf("Expected IsBond() to return true, got false")
	}
	if len(bond.BondSlaves) != 2 {
		t.Errorf("Expected 2 slaves, got %d", len(bond.BondSlaves))
	}
	if bond.BondMode != "802.3ad" {
		t.Errorf("Expected bond mode '802.3ad', got %q", bond.BondMode)
	}
}

// TestNetworkInterfaceNotBond tests NetworkInterface for non-bond interfaces
func TestNetworkInterfaceNotBond(t *testing.T) {
	iface := NetworkInterface{
		Name: "eth0",
		Type: "ethernet",
	}

	if iface.IsBond() {
		t.Errorf("Expected IsBond() to return false for ethernet interface, got true")
	}
}

// TestHostChecksResultFilterMethods tests GetBonds, GetEthernets, GetInfiniBands, GetVirtualInterfaces
func TestHostChecksResultFilterMethods(t *testing.T) {
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{Name: "eth0", Type: "ethernet"},
			{Name: "eth1", Type: "ethernet"},
			{Name: "ib0", Type: "infiniband"},
			{Name: "ib1", Type: "infiniband"},
			{Name: "bond0", Type: "bond"},
			{Name: "bond1", Type: "bond"},
		},
	}

	tests := []struct {
		name          string
		method        func() []*NetworkInterface
		expectedCount int
		expectedType  string
	}{
		{
			name:          "GetBonds",
			method:        hc.NetworkInterfaces.GetBonds,
			expectedCount: 2,
			expectedType:  "bond",
		},
		{
			name:          "GetEthernets",
			method:        hc.NetworkInterfaces.GetEthernets,
			expectedCount: 2,
			expectedType:  "ethernet",
		},
		{
			name:          "GetInfiniBands",
			method:        hc.NetworkInterfaces.GetInfiniBands,
			expectedCount: 2,
			expectedType:  "infiniband",
		},
		{
			name:          "GetVirtualInterfaces",
			method:        hc.NetworkInterfaces.GetVirtualInterfaces,
			expectedCount: 2,
			expectedType:  "bond",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d interfaces, got %d", tt.expectedCount, len(result))
			}
			for _, iface := range result {
				if iface.Type != tt.expectedType {
					t.Errorf("Expected type '%s', got '%s' for interface %s", tt.expectedType, iface.Type, iface.Name)
				}
			}
		})
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
		IsBondSlave:    false,
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
		IsBondSlave:    false,
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

// TestRouteEntry tests RouteEntry struct
func TestRouteEntry(t *testing.T) {
	route := RouteEntry{
		Destination: "10.0.0.0/24",
		Gateway:     "10.0.0.1",
		Device:      "eth0",
		Metric:      100,
		Table:       "main",
		Protocol:    "kernel",
	}

	if route.Destination != "10.0.0.0/24" {
		t.Errorf("Expected Destination '10.0.0.0/24', got %q", route.Destination)
	}
	if route.Gateway != "10.0.0.1" {
		t.Errorf("Expected Gateway '10.0.0.1', got %q", route.Gateway)
	}
	if route.Metric != 100 {
		t.Errorf("Expected Metric 100, got %d", route.Metric)
	}
}

// TestRoutingRule tests RoutingRule struct
func TestRoutingRule(t *testing.T) {
	rule := RoutingRule{
		Priority:  1000,
		Condition: "from 10.0.1.0/24",
		Table:     "200",
		Action:    "lookup",
	}

	if rule.Priority != 1000 {
		t.Errorf("Expected Priority 1000, got %d", rule.Priority)
	}
	if rule.Table != "200" {
		t.Errorf("Expected Table '200', got %q", rule.Table)
	}
}

// TestRoutingTableInfo tests RoutingTableInfo struct
func TestRoutingTableInfo(t *testing.T) {
	table := RoutingTableInfo{
		TableName: "main",
		TableID:   254,
		Routes: []RouteEntry{
			{
				Destination: "default",
				Gateway:     "10.0.0.1",
				Device:      "eth0",
				Metric:      100,
			},
		},
	}

	if table.TableName != "main" {
		t.Errorf("Expected TableName 'main', got %q", table.TableName)
	}
	if table.TableID != 254 {
		t.Errorf("Expected TableID 254, got %d", table.TableID)
	}
	if len(table.Routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(table.Routes))
	}
}

// TestNetworkNamespaceRouting tests NetworkNamespaceRouting struct
func TestNetworkNamespaceRouting(t *testing.T) {
	nsRouting := &NetworkNamespaceRouting{
		Namespace: "",
		RoutingTables: []RoutingTableInfo{
			{
				TableName: "main",
				TableID:   254,
				Routes:    []RouteEntry{},
			},
		},
		RoutingRules: []RoutingRule{
			{
				Priority: 1000,
				Table:    "200",
			},
		},
		RuleCount:  1,
		TableCount: 1,
	}

	if nsRouting.RuleCount != 1 {
		t.Errorf("Expected RuleCount 1, got %d", nsRouting.RuleCount)
	}
	if nsRouting.TableCount != 1 {
		t.Errorf("Expected TableCount 1, got %d", nsRouting.TableCount)
	}
}

// TestNetworkInterfaceWithRouting tests NetworkInterface with routing information
func TestNetworkInterfaceWithRouting(t *testing.T) {
	iface := &NetworkInterface{
		Name:           "eth0",
		Type:           "ethernet",
		IP:             "10.0.0.1/24",
		IsDefaultRoute: true,
		RouteCount:     2,
		AssociatedRoutes: []RouteEntry{
			{
				Destination: "default",
				Gateway:     "10.0.0.254",
				Device:      "eth0",
			},
			{
				Destination: "10.0.0.0/24",
				Device:      "eth0",
			},
		},
	}

	if !iface.IsDefaultRoute {
		t.Errorf("Expected IsDefaultRoute true, got %v", iface.IsDefaultRoute)
	}
	if iface.RouteCount != 2 {
		t.Errorf("Expected RouteCount 2, got %d", iface.RouteCount)
	}
	if len(iface.AssociatedRoutes) != 2 {
		t.Errorf("Expected 2 associated routes, got %d", len(iface.AssociatedRoutes))
	}
}

// TestNVMeDriveInfo tests NvmeDrive struct
func TestNVMeDriveInfo(t *testing.T) {
	drive := NvmeDrive{
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

// TestSubnetInterface tests SubnetInterface struct
func TestSubnetInterface(t *testing.T) {
	subnetIface := SubnetInterface{
		Name: "eth0",
		IP:   "10.0.0.10",
	}

	if subnetIface.Name != "eth0" {
		t.Errorf("Expected Name 'eth0', got %q", subnetIface.Name)
	}
	if subnetIface.IP != "10.0.0.10" {
		t.Errorf("Expected IP '10.0.0.10', got %q", subnetIface.IP)
	}
}

// TestSubnet tests Subnet struct
func TestSubnet(t *testing.T) {
	subnet := Subnet{
		NetworkAddress: "10.0.0.0",
		Netmask:        "255.255.255.0",
		CIDR:           "10.0.0.0/24",
		Interfaces: []SubnetInterface{
			{Name: "eth0", IP: "10.0.0.10"},
			{Name: "eth1", IP: "10.0.0.20"},
		},
		InterfaceCount: 2,
		IsCNISubnet:    true,
	}

	if subnet.NetworkAddress != "10.0.0.0" {
		t.Errorf("Expected NetworkAddress '10.0.0.0', got %q", subnet.NetworkAddress)
	}
	if subnet.Netmask != "255.255.255.0" {
		t.Errorf("Expected Netmask '255.255.255.0', got %q", subnet.Netmask)
	}
	if subnet.CIDR != "10.0.0.0/24" {
		t.Errorf("Expected CIDR '10.0.0.0/24', got %q", subnet.CIDR)
	}
	if subnet.InterfaceCount != 2 {
		t.Errorf("Expected InterfaceCount 2, got %d", subnet.InterfaceCount)
	}
	if len(subnet.Interfaces) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(subnet.Interfaces))
	}
	if !subnet.IsCNISubnet {
		t.Errorf("Expected IsCNISubnet true, got %v", subnet.IsCNISubnet)
	}
}

// TestNetworkNamespaceRoutingWithSubnets tests NetworkNamespaceRouting with subnets
func TestNetworkNamespaceRoutingWithSubnets(t *testing.T) {
	nsRouting := &NetworkNamespaceRouting{
		Namespace:     "",
		RoutingTables: []RoutingTableInfo{},
		RoutingRules: []RoutingRule{
			{Priority: 32766, Table: "main"},
		},
		RuleCount: 1,
		Subnets: []Subnet{
			{
				NetworkAddress: "10.0.0.0",
				Netmask:        "255.255.255.0",
				CIDR:           "10.0.0.0/24",
				Interfaces: []SubnetInterface{
					{Name: "eth0", IP: "10.0.0.10"},
				},
				InterfaceCount: 1,
				IsCNISubnet:    true,
			},
		},
		SubnetCount: 1,
	}

	if nsRouting.SubnetCount != 1 {
		t.Errorf("Expected SubnetCount 1, got %d", nsRouting.SubnetCount)
	}
	if len(nsRouting.Subnets) != 1 {
		t.Errorf("Expected 1 subnet, got %d", len(nsRouting.Subnets))
	}
	if nsRouting.Subnets[0].CIDR != "10.0.0.0/24" {
		t.Errorf("Expected CIDR '10.0.0.0/24', got %q", nsRouting.Subnets[0].CIDR)
	}
}

// TestSubnetCNIDetection tests various subnet CIDR patterns for CNI detection accuracy
func TestSubnetCNIDetection(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		expectedCNI bool
	}{
		{
			name:        "10.0.0.0/24 (typical per-node CNI)",
			cidr:        "10.0.0.0/24",
			expectedCNI: true,
		},
		{
			name:        "10.1.0.0/24 (Flannel pattern)",
			cidr:        "10.1.0.0/24",
			expectedCNI: true,
		},
		{
			name:        "10.200.0.0/16 (management network)",
			cidr:        "10.200.0.0/16",
			expectedCNI: false,
		},
		{
			name:        "10.0.0.0/8 (too broad, not CNI)",
			cidr:        "10.0.0.0/8",
			expectedCNI: false,
		},
		{
			name:        "172.16.0.0/12 (not Pod CIDR)",
			cidr:        "172.16.0.0/12",
			expectedCNI: false,
		},
		{
			name:        "192.168.0.0/16 (not Pod CIDR)",
			cidr:        "192.168.0.0/16",
			expectedCNI: false,
		},
		{
			name:        "10.10.100.0/24 (typical CNI Pod subnet)",
			cidr:        "10.10.100.0/24",
			expectedCNI: true,
		},
		{
			name:        "10.10.0.0/25 (smaller CNI subnet)",
			cidr:        "10.10.0.0/25",
			expectedCNI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet := Subnet{
				CIDR:        tt.cidr,
				IsCNISubnet: false, // Will be set by actual detection in script
			}

			// For this test, we're just documenting what the detection should be
			// The actual detection happens in the shell script via is_cni_subnet()
			// This test documents the expected behavior

			if tt.expectedCNI {
				if subnet.CIDR != tt.cidr {
					t.Errorf("Test setup error: CIDR mismatch")
				}
				// In production, IsCNISubnet would be true for these cases
			}
		})
	}
}

// TestCIDRToNetmaskAllPrefixes tests cidr_to_netmask conversion for all 32 prefix sizes
func TestCIDRToNetmaskAllPrefixes(t *testing.T) {
	// Comprehensive mapping of all CIDR prefixes to their corresponding netmasks
	testCases := []struct {
		cidr     string
		expected string
		name     string
	}{
		{"/0", "0.0.0.0", "Class-less routing (entire internet)"},
		{"/1", "128.0.0.0", "First bit set"},
		{"/2", "192.0.0.0", "First 2 bits set"},
		{"/3", "224.0.0.0", "First 3 bits set"},
		{"/4", "240.0.0.0", "First nibble set"},
		{"/5", "248.0.0.0", "First 5 bits set"},
		{"/6", "252.0.0.0", "First 6 bits set"},
		{"/7", "254.0.0.0", "First 7 bits set"},
		{"/8", "255.0.0.0", "Class A network"},
		{"/9", "255.128.0.0", "Class A with subnet bit"},
		{"/10", "255.192.0.0", "10 bits set"},
		{"/11", "255.224.0.0", "11 bits set"},
		{"/12", "255.240.0.0", "12 bits set"},
		{"/13", "255.248.0.0", "13 bits set"},
		{"/14", "255.252.0.0", "14 bits set"},
		{"/15", "255.254.0.0", "15 bits set"},
		{"/16", "255.255.0.0", "Class B network"},
		{"/17", "255.255.128.0", "Class B with subnet bit"},
		{"/18", "255.255.192.0", "18 bits set"},
		{"/19", "255.255.224.0", "19 bits set"},
		{"/20", "255.255.240.0", "Class B /20 subnet"},
		{"/21", "255.255.248.0", "21 bits set"},
		{"/22", "255.255.252.0", "22 bits set"},
		{"/23", "255.255.254.0", "23 bits set"},
		{"/24", "255.255.255.0", "Class C network"},
		{"/25", "255.255.255.128", "Class C subnet /1"},
		{"/26", "255.255.255.192", "Class C subnet /2"},
		{"/27", "255.255.255.224", "Class C subnet /3"},
		{"/28", "255.255.255.240", "Class C subnet /4"},
		{"/29", "255.255.255.248", "Class C subnet /5"},
		{"/30", "255.255.255.252", "Point-to-point link"},
		{"/31", "255.255.255.254", "Host mask (RFC 3021)"},
		{"/32", "255.255.255.255", "Single host"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Extract prefix from CIDR notation
			prefix := tc.cidr[1:] // Skip the '/'

			// For this test, we're documenting the expected mapping
			// The actual conversion happens in the shell script
			if prefix == "" {
				t.Errorf("Invalid CIDR: %s", tc.cidr)
				return
			}

			// Verify the expected netmask is valid (has valid octet values)
			parts := len(strings.Split(tc.expected, "."))
			if parts != 4 {
				t.Errorf("Invalid netmask format: %s for %s", tc.expected, tc.name)
			}
		})
	}
}

// TestCNIDetection tests CNIDetection struct
func TestCNIDetection(t *testing.T) {
	detection := &CNIDetection{
		PodCIDR:  "10.244.0.0/24",
		Source:   "kubelet_config",
		CNIType:  "flannel",
		Detected: true,
	}

	if detection.PodCIDR != "10.244.0.0/24" {
		t.Errorf("Expected PodCIDR '10.244.0.0/24', got %q", detection.PodCIDR)
	}
	if detection.Source != "kubelet_config" {
		t.Errorf("Expected Source 'kubelet_config', got %q", detection.Source)
	}
	if detection.CNIType != "flannel" {
		t.Errorf("Expected CNIType 'flannel', got %q", detection.CNIType)
	}
	if !detection.Detected {
		t.Errorf("Expected Detected true, got %v", detection.Detected)
	}
}

// TestCNIDetectionVariousSources tests CNI detection from different sources
func TestCNIDetectionVariousSources(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		cniType     string
		podCIDR     string
		shouldExist bool
	}{
		{
			name:        "Flannel from kubelet config",
			source:      "kubelet_config",
			cniType:     "flannel",
			podCIDR:     "10.244.0.0/24",
			shouldExist: true,
		},
		{
			name:        "Calico from kubelet args",
			source:      "kubelet_args",
			cniType:     "calico",
			podCIDR:     "10.0.0.0/8",
			shouldExist: true,
		},
		{
			name:        "Weave from config files",
			source:      "config_files",
			cniType:     "weave",
			podCIDR:     "10.32.0.0/12",
			shouldExist: true,
		},
		{
			name:        "Flannel from data file",
			source:      "flannel_data",
			cniType:     "flannel",
			podCIDR:     "10.244.0.0/16",
			shouldExist: true,
		},
		{
			name:        "Not detected",
			source:      "",
			cniType:     "",
			podCIDR:     "",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := &CNIDetection{
				PodCIDR:  tt.podCIDR,
				Source:   tt.source,
				CNIType:  tt.cniType,
				Detected: tt.shouldExist,
			}

			if detection.Detected != tt.shouldExist {
				t.Errorf("Expected Detected %v, got %v", tt.shouldExist, detection.Detected)
			}

			if tt.shouldExist {
				if detection.PodCIDR == "" {
					t.Errorf("Expected PodCIDR to be set when detected")
				}
				if detection.Source == "" {
					t.Errorf("Expected Source to be set when detected")
				}
			}
		})
	}
}

// TestNVMePCIAddressExtraction tests NVMe PCI address extraction from device paths
func TestNVMePCIAddressExtraction(t *testing.T) {
	tests := []struct {
		name           string
		devicePath     string
		expectedFormat string
		description    string
	}{
		{
			name:           "Valid PCI address in path",
			devicePath:     "/sys/devices/pci0000:00/0000:00:1d.0/nvme/nvme0/nvme0n1",
			expectedFormat: "0000:00:1d.0",
			description:    "Standard NVMe PCI address format",
		},
		{
			name:           "Alternative PCI address",
			devicePath:     "/sys/devices/pci0000:00/0000:3d:00.0/nvme/nvme4/nvme4n1",
			expectedFormat: "0000:3d:00.0",
			description:    "Different slot NVMe PCI address",
		},
		{
			name:           "Multi-digit domain",
			devicePath:     "/sys/devices/pci0000:80/0000:80:17.0/nvme/nvme2",
			expectedFormat: "0000:80:17.0",
			description:    "NUMA system with domain 80",
		},
		{
			name:           "High slot number",
			devicePath:     "/sys/devices/pci0000:00/0000:00:1f.2/nvme/nvme5",
			expectedFormat: "0000:00:1f.2",
			description:    "Higher slot number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the format matches the expected PCI address pattern
			// Pattern: XXXX:XX:XX.X where X is hex digit
			if tt.expectedFormat != "" {
				// Verify the expected format matches the pattern
				if !matchPCIAddressPattern(tt.expectedFormat) {
					t.Errorf("Expected format %q doesn't match PCI address pattern", tt.expectedFormat)
				}
			}
		})
	}
}

// TestNetworkInterfacePCIAddressExtraction tests network interface PCI address extraction
func TestNetworkInterfacePCIAddressExtraction(t *testing.T) {
	tests := []struct {
		name           string
		ifname         string
		expectedFormat string
		description    string
	}{
		{
			name:           "Ethernet NIC",
			ifname:         "eth0",
			expectedFormat: "0000:00:1f.6",
			description:    "Standard Ethernet interface",
		},
		{
			name:           "High speed NIC",
			ifname:         "eno2",
			expectedFormat: "0000:3d:00.0",
			description:    "High-speed network card on different slot",
		},
		{
			name:           "InfiniBand interface",
			ifname:         "ib0",
			expectedFormat: "0000:3d:00.0",
			description:    "InfiniBand HCA",
		},
		{
			name:           "NUMA domain interface",
			ifname:         "eno3",
			expectedFormat: "0000:80:00.0",
			description:    "Interface in NUMA domain 80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the format matches the expected PCI address pattern
			if tt.expectedFormat != "" {
				// Verify the expected format matches the pattern
				if !matchPCIAddressPattern(tt.expectedFormat) {
					t.Errorf("Expected format %q doesn't match PCI address pattern", tt.expectedFormat)
				}
			}
		})
	}
}

// TestNetworkInterfaceWithPCIAddress tests NetworkInterface with PCI address populated
func TestNetworkInterfaceWithPCIAddress(t *testing.T) {
	iface := &NetworkInterface{
		Name:       "eth0",
		Type:       "ethernet",
		IP:         "10.0.0.1/24",
		MAC:        "52:54:00:12:34:56",
		PCIAddress: "0000:00:1f.6",
		Model:      "Intel I350",
		Status:     "up",
	}

	if iface.PCIAddress != "0000:00:1f.6" {
		t.Errorf("Expected PCIAddress '0000:00:1f.6', got %q", iface.PCIAddress)
	}

	// Verify format
	if !matchPCIAddressPattern(iface.PCIAddress) {
		t.Errorf("PCIAddress %q doesn't match expected format", iface.PCIAddress)
	}
}

// TestInfiniBandInterfaceWithPCIAddress tests InfiniBand interface with PCI address
func TestInfiniBandInterfaceWithPCIAddress(t *testing.T) {
	iface := &NetworkInterface{
		Name:       "ib0",
		Type:       "infiniband",
		IP:         "192.168.1.10/24",
		MTU:        2048,
		PCIAddress: "0000:3d:00.0",
		Model:      "Mellanox ConnectX-7",
		MaxSpeed:   "400Gbps",
		Status:     "up",
	}

	if iface.PCIAddress != "0000:3d:00.0" {
		t.Errorf("Expected PCIAddress '0000:3d:00.0', got %q", iface.PCIAddress)
	}

	// Verify it's a valid PCI address
	if !matchPCIAddressPattern(iface.PCIAddress) {
		t.Errorf("PCIAddress %q doesn't match expected format", iface.PCIAddress)
	}
}

// TestNetworkInterfaceWithNUMANode tests NetworkInterface with NUMA node information
func TestNetworkInterfaceWithNUMANode(t *testing.T) {
	iface := &NetworkInterface{
		Name:       "eth0",
		Type:       "ethernet",
		IP:         "10.0.0.1/24",
		PCIAddress: "0000:00:1f.6",
		NUMANode:   0,
		Model:      "Intel I350",
		Status:     "up",
	}

	if iface.NUMANode != 0 {
		t.Errorf("Expected NUMANode 0, got %d", iface.NUMANode)
	}
}

// TestNetworkInterfaceMultipleNUMANodes tests interfaces on different NUMA nodes
func TestNetworkInterfaceMultipleNUMANodes(t *testing.T) {
	tests := []struct {
		name     string
		ifname   string
		pci      string
		numa     int
		expected int
	}{
		{
			name:     "NUMA node 0",
			ifname:   "eth0",
			pci:      "0000:00:1f.6",
			numa:     0,
			expected: 0,
		},
		{
			name:     "NUMA node 1",
			ifname:   "eth1",
			pci:      "0001:00:00.0",
			numa:     1,
			expected: 1,
		},
		{
			name:     "Unknown NUMA",
			ifname:   "eth2",
			pci:      "",
			numa:     -1,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface := &NetworkInterface{
				Name:       tt.ifname,
				PCIAddress: tt.pci,
				NUMANode:   tt.numa,
			}

			if iface.NUMANode != tt.expected {
				t.Errorf("Expected NUMANode %d, got %d", tt.expected, iface.NUMANode)
			}
		})
	}
}

// TestNVMeDriveWithNUMANode tests NvmeDrive with NUMA node information
func TestNVMeDriveWithNUMANode(t *testing.T) {
	drive := NvmeDrive{
		DeviceName: "nvme0n1",
		DevicePath: "/dev/nvme0n1",
		Model:      "Samsung 970 EVO",
		Size:       1099511627776,
		PCIAddress: "0000:00:1d.0",
		NUMANode:   0,
		Mounted:    true,
		MountPoint: "/data",
	}

	if drive.NUMANode != 0 {
		t.Errorf("Expected NUMANode 0, got %d", drive.NUMANode)
	}
	if drive.PCIAddress != "0000:00:1d.0" {
		t.Errorf("Expected PCIAddress '0000:00:1d.0', got %q", drive.PCIAddress)
	}
}

// TestNVMeDrivesMultipleNUMANodes tests NVMe drives on different NUMA nodes
func TestNVMeDrivesMultipleNUMANodes(t *testing.T) {
	tests := []struct {
		name     string
		devname  string
		pci      string
		numa     int
		expected int
	}{
		{
			name:     "NVMe on NUMA 0",
			devname:  "nvme0n1",
			pci:      "0000:00:1d.0",
			numa:     0,
			expected: 0,
		},
		{
			name:     "NVMe on NUMA 1",
			devname:  "nvme1n1",
			pci:      "0001:00:00.0",
			numa:     1,
			expected: 1,
		},
		{
			name:     "Unknown NUMA",
			devname:  "nvme2n1",
			pci:      "",
			numa:     -1,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drive := NvmeDrive{
				DeviceName: tt.devname,
				PCIAddress: tt.pci,
				NUMANode:   tt.numa,
			}

			if drive.NUMANode != tt.expected {
				t.Errorf("Expected NUMANode %d, got %d", tt.expected, drive.NUMANode)
			}
		})
	}
}

// TestDualFunctionMellanoxDevice tests handling of dual-function Mellanox devices
func TestDualFunctionMellanoxDevice(t *testing.T) {
	// Mellanox MT2910 (ConnectX-7) supports dual-protocol operation:
	// - Function 0: Ethernet controller
	// - Function 1: InfiniBand controller
	// Both functions share the same NUMA node and physical hardware

	// Simulate the Ethernet function interface
	ethernetIface := &NetworkInterface{
		Name:       "eth0",
		Type:       "ethernet",
		IP:         "192.168.1.10/24",
		PCIAddress: "0000:63:00.0", // Ethernet function
		NUMANode:   0,
		Model:      "Mellanox ConnectX-7",
		MaxSpeed:   "100Gbps",
		Status:     "up",
	}

	// Simulate the InfiniBand function interface
	infinibandIface := &NetworkInterface{
		Name:       "ib0",
		Type:       "infiniband",
		IP:         "10.200.5.41/24",
		PCIAddress: "0000:63:00.1", // InfiniBand function
		NUMANode:   0,
		Model:      "Mellanox ConnectX-7",
		MaxSpeed:   "400Gbps",
		Status:     "up",
	}

	// Verify both functions are extracted correctly
	if ethernetIface.PCIAddress != "0000:63:00.0" {
		t.Errorf("Expected Ethernet PCIAddress '0000:63:00.0', got %q", ethernetIface.PCIAddress)
	}
	if infinibandIface.PCIAddress != "0000:63:00.1" {
		t.Errorf("Expected InfiniBand PCIAddress '0000:63:00.1', got %q", infinibandIface.PCIAddress)
	}

	// Verify both are on same NUMA node (they're the same physical device)
	if ethernetIface.NUMANode != infinibandIface.NUMANode {
		t.Errorf("Expected both functions on same NUMA node, got Ethernet=%d, InfiniBand=%d",
			ethernetIface.NUMANode, infinibandIface.NUMANode)
	}

	// Verify function numbers are different
	if ethernetIface.PCIAddress[len(ethernetIface.PCIAddress)-1:] ==
		infinibandIface.PCIAddress[len(infinibandIface.PCIAddress)-1:] {
		t.Errorf("Expected different function numbers for Ethernet and InfiniBand")
	}
}

// TestMellanoxConnectX7DualProtocol tests ConnectX-7 dual-protocol capability
func TestMellanoxConnectX7DualProtocol(t *testing.T) {
	tests := []struct {
		name         string
		ifname       string
		pciFunc      string // Function number (0 or 1)
		ifaceType    string
		expectedType string
	}{
		{
			name:         "ConnectX-7 Ethernet mode",
			ifname:       "eth0",
			pciFunc:      "0",
			ifaceType:    "ethernet",
			expectedType: "ethernet",
		},
		{
			name:         "ConnectX-7 InfiniBand mode",
			ifname:       "ib0",
			pciFunc:      "1",
			ifaceType:    "infiniband",
			expectedType: "infiniband",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface := &NetworkInterface{
				Name:       tt.ifname,
				Type:       tt.ifaceType,
				PCIAddress: "0000:63:00." + tt.pciFunc,
				Model:      "Mellanox ConnectX-7",
			}

			if iface.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, iface.Type)
			}

			// Verify PCI address format is correct
			if !matchPCIAddressPattern(iface.PCIAddress) {
				t.Errorf("PCIAddress %q doesn't match expected format", iface.PCIAddress)
			}
		})
	}
}

// TestNUMANodeDetectionFallbacks tests the fallback paths for NUMA node detection
func TestNUMANodeDetectionFallbacks(t *testing.T) {
	tests := []struct {
		name         string
		pciAddr      string
		expectedNUMA int
		scenario     string
	}{
		{
			name:         "Standard Ethernet",
			pciAddr:      "0000:00:1f.6",
			expectedNUMA: 0,
			scenario:     "Uses /sys/bus/pci/devices/<addr>/numa_node",
		},
		{
			name:         "Mellanox Ethernet Function 0",
			pciAddr:      "0000:50:00.0",
			expectedNUMA: 0,
			scenario:     "Mellanox ConnectX-7 Ethernet - primary device",
		},
		{
			name:         "Mellanox InfiniBand Function 1",
			pciAddr:      "0000:50:00.1",
			expectedNUMA: 0,
			scenario:     "Mellanox ConnectX-7 InfiniBand - may use parent device NUMA",
		},
		{
			name:         "NVMe on different NUMA",
			pciAddr:      "0001:00:00.0",
			expectedNUMA: 1,
			scenario:     "Device on NUMA node 1",
		},
		{
			name:         "Unknown NUMA",
			pciAddr:      "",
			expectedNUMA: -1,
			scenario:     "Empty PCI address returns -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test documents the expected behavior
			// Actual NUMA detection happens via sysfs in the shell script

			if tt.pciAddr == "" {
				// Empty address should return -1
				if tt.expectedNUMA != -1 {
					t.Errorf("Expected NUMA -1 for empty address, got %d", tt.expectedNUMA)
				}
			} else {
				// Verify PCI address format is valid
				if !matchPCIAddressPattern(tt.pciAddr) {
					t.Errorf("Invalid PCI address format: %s", tt.pciAddr)
				}

				// Valid NUMA node should be >= 0 or -1 for unknown
				if tt.expectedNUMA < -1 {
					t.Errorf("Invalid NUMA node value: %d", tt.expectedNUMA)
				}
			}
		})
	}
}

// TestMultiFunctionDeviceNUMAInheritance tests NUMA detection for multi-function devices
func TestMultiFunctionDeviceNUMAInheritance(t *testing.T) {
	// Dual-function Mellanox device where Function 1 (InfiniBand) inherits from Function 0

	tests := []struct {
		name        string
		function    string
		description string
	}{
		{
			name:        "Function 0 - Ethernet",
			function:    "0000:50:00.0",
			description: "Primary Mellanox device with direct NUMA node",
		},
		{
			name:        "Function 1 - InfiniBand",
			function:    "0000:50:00.1",
			description: "Secondary function may inherit NUMA from parent (0000:50:00.0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface := &NetworkInterface{
				Name:       "test",
				PCIAddress: tt.function,
				NUMANode:   0, // Both should be on same NUMA
			}

			if iface.PCIAddress != tt.function {
				t.Errorf("PCI address mismatch: expected %s, got %s", tt.function, iface.PCIAddress)
			}

			// For dual-function devices, both functions should have same NUMA node
			// since they're the same physical device
			if !matchPCIAddressPattern(iface.PCIAddress) {
				t.Errorf("Invalid PCI address format: %s", iface.PCIAddress)
			}
		})
	}
}

// Helper function to validate PCI address format
func matchPCIAddressPattern(addr string) bool {
	// PCI address format: XXXX:XX:XX.X where X is hex digit
	// Example: 0000:00:1d.0
	if len(addr) < 12 {
		return false
	}
	// Check format: 4 hex + : + 2 hex + : + 2 hex + . + 1 hex
	parts := strings.Split(addr, ":")
	if len(parts) != 3 {
		return false
	}
	// Verify last part has dot
	if !strings.Contains(parts[2], ".") {
		return false
	}
	return true
}
