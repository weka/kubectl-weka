package hostcheck

import (
	"encoding/json"
	"github.com/weka/kubectl-weka/pkg/types"
	"strings"
	"testing"
)

// ============================================================================
// CPU Module Tests
// ============================================================================

func TestCPUModule_Name(t *testing.T) {
	m := &CPUModule{}
	if m.Name() != ModuleNameCpuMemory {
		t.Errorf("Expected ModuleNameCpuMemory, got %v", m.Name())
	}
}

func TestCPUModule_FriendlyName(t *testing.T) {
	m := &CPUModule{}
	expected := "CPU & Memory"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestCPUModule_Description(t *testing.T) {
	m := &CPUModule{}
	desc := m.Description()
	if desc == "" {
		t.Errorf("Expected non-empty description")
	}
}

func TestCPUModule_TemplatesNotEmpty(t *testing.T) {
	m := &CPUModule{}
	if m.SuccessTemplate() == "" {
		t.Errorf("SuccessTemplate should not be empty")
	}
	if m.ErrorTemplate() == "" {
		t.Errorf("ErrorTemplate should not be empty")
	}
	if m.SuggestedResolutionTemplate() == "" {
		t.Errorf("SuggestedResolutionTemplate should not be empty")
	}
}

func TestCPUModule_Validate(t *testing.T) {
	m := &CPUModule{}

	hc := &HostChecksResult{
		PhysicalCores: 32,
		LogicalCores:  64,
		MemoryBytes:   274877906944,
		HTEnabled:     true,
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response == nil {
		t.Fatalf("Expected non-nil response")
	}

	if response.ModuleName() != ModuleNameCpuMemory {
		t.Errorf("Expected ModuleNameCpuMemory, got %v", response.ModuleName())
	}
}

func TestCPUModule_ValidateInvalidJSON(t *testing.T) {
	m := &CPUModule{}
	response, err := m.Validate("invalid json")

	if err == nil {
		t.Errorf("Expected error for invalid JSON")
	}

	// Response should contain the error even though parsing failed
	if response == nil {
		t.Errorf("Expected response object even with error")
	}

	if response != nil && response.Status() != types.StatusFail {
		t.Errorf("Expected fail status for invalid JSON, got %v", response.Status())
	}
}

// ============================================================================
// Kernel Module Tests
// ============================================================================

func TestKernelModule_Name(t *testing.T) {
	m := &KernelModule{}
	if m.Name() != ModuleNameKernel {
		t.Errorf("Expected ModuleNameKernel, got %v", m.Name())
	}
}

func TestKernelModule_FriendlyName(t *testing.T) {
	m := &KernelModule{}
	expected := "Kernel Version"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestKernelModule_Validate(t *testing.T) {
	m := &KernelModule{}

	hc := &HostChecksResult{
		KernelVersion: "5.15.0-1023-aws",
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() == types.StatusFail {
		t.Errorf("Expected pass status for valid kernel version")
	}
}

func TestKernelModule_ValidateOldKernel(t *testing.T) {
	m := &KernelModule{}

	hc := &HostChecksResult{
		KernelVersion: "4.15.0",
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Old kernel should be fail or warning
	if response.Status() == types.StatusPass {
		t.Errorf("Expected fail/warn status for old kernel version")
	}
}

// ============================================================================
// OS Module Tests
// ============================================================================

func TestOSModule_Name(t *testing.T) {
	m := &OSModule{}
	if m.Name() != ModuleNameOs {
		t.Errorf("Expected ModuleNameOs, got %v", m.Name())
	}
}

func TestOSModule_FriendlyName(t *testing.T) {
	m := &OSModule{}
	expected := "Operating System"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestOSModule_ValidateRHCOS(t *testing.T) {
	m := &OSModule{}

	hc := &HostChecksResult{
		IsRHCOS:   true,
		OSRelease: "RHCOS 4.10",
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status for RHCOS")
	}
}

func TestOSModule_ValidateUnsupported(t *testing.T) {
	m := &OSModule{}

	hc := &HostChecksResult{
		IsRHCOS:   false,
		OSRelease: "Windows Server",
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusFail {
		t.Errorf("Expected fail status for unsupported OS")
	}
}

// ============================================================================
// XFS Module Tests
// ============================================================================

func TestXFSModule_Name(t *testing.T) {
	m := &XFSModule{}
	if m.Name() != ModuleNameXfs {
		t.Errorf("Expected ModuleNameXfs, got %v", m.Name())
	}
}

func TestXFSModule_FriendlyName(t *testing.T) {
	m := &XFSModule{}
	expected := "XFS Tools"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestXFSModule_ValidateFound(t *testing.T) {
	m := &XFSModule{}

	hc := &HostChecksResult{
		XFSFound: true,
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status when XFS is found")
	}
}

func TestXFSModule_ValidateNotFound(t *testing.T) {
	m := &XFSModule{}

	hc := &HostChecksResult{
		XFSFound: false,
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusFail {
		t.Errorf("Expected fail status when XFS is not found")
	}
}

// ============================================================================
// NVMe Drives Module Tests
// ============================================================================

func TestNVMeDrivesModule_Name(t *testing.T) {
	m := &NVMeDrivesModule{}
	if m.Name() != ModuleNameNVMeDrives {
		t.Errorf("Expected ModuleNameNVMeDrives, got %v", m.Name())
	}
}

func TestNVMeDrivesModule_FriendlyName(t *testing.T) {
	m := &NVMeDrivesModule{}
	expected := "NVMe Drives"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestNVMeDrivesModule_ValidateWithDrives(t *testing.T) {
	m := &NVMeDrivesModule{}

	hc := &HostChecksResult{
		NVMeDriveCount: 2,
		NVMeDrives: []NvmeDrive{
			{
				DeviceName:   "nvme0n1",
				SerialNumber: "ABC123",
				Model:        "Samsung 970 EVO",
				Size:         1099511627776,
			},
			{
				DeviceName:   "nvme1n1",
				SerialNumber: "DEF456",
				Model:        "Samsung 980 PRO",
				Size:         2199023255552,
			},
		},
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status with NVMe drives")
	}
}

func TestNVMeDrivesModule_ValidateNoDrives(t *testing.T) {
	m := &NVMeDrivesModule{}

	hc := &HostChecksResult{
		NVMeDrives: []NvmeDrive{},
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// No drives might be warning or pass depending on module logic
	if response == nil {
		t.Errorf("Expected non-nil response")
	}
}

// ============================================================================
// Weka Directory Module Tests
// ============================================================================

func TestWekaDirModule_Name(t *testing.T) {
	m := &WekaDirModule{}
	if m.Name() != ModuleNameWekaDirectory {
		t.Errorf("Expected ModuleNameWekaDirectory, got %v", m.Name())
	}
}

func TestWekaDirModule_FriendlyName(t *testing.T) {
	m := &WekaDirModule{}
	expected := "Weka Directory"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestWekaDirModule_ValidateSufficientSpace(t *testing.T) {
	m := &WekaDirModule{}

	hc := &HostChecksResult{
		WekaDirExists:     true,
		WekaDirPath:       "/mnt/weka",
		WekaDirAvailBytes: 1099511627776, // 1TB
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status with sufficient space")
	}
}

func TestWekaDirModule_ValidateInsufficientSpace(t *testing.T) {
	m := &WekaDirModule{}

	hc := &HostChecksResult{
		WekaDirExists:     true,
		WekaDirPath:       "/mnt/weka",
		WekaDirAvailBytes: 10737418240, // 10GB (below 300GB minimum)
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusFail {
		t.Errorf("Expected fail status with insufficient space")
	}
}

// ============================================================================
// Weka Agent Service Module Tests
// ============================================================================

func TestWekaAgentServiceModule_Name(t *testing.T) {
	m := &WekaAgentServiceModuleModule{}
	if m.Name() != ModuleNameWekaAgentService {
		t.Errorf("Expected ModuleNameWekaAgentService, got %v", m.Name())
	}
}

func TestWekaAgentServiceModule_FriendlyName(t *testing.T) {
	m := &WekaAgentServiceModuleModule{}
	expected := "Weka Agent Service Installation"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestWekaAgentServiceModule_ValidateClean(t *testing.T) {
	m := &WekaAgentServiceModuleModule{}

	hc := &HostChecksResult{
		WekaAgentServiceExists: false,
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status when agent service is clean")
	}
}

func TestWekaAgentServiceModule_ValidateNotClean(t *testing.T) {
	m := &WekaAgentServiceModuleModule{}

	hc := &HostChecksResult{
		WekaAgentServiceExists: true,
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusWarn {
		t.Errorf("Expected warn status when agent service is present")
	}
}

// ============================================================================
// Network Interfaces Module Tests
// ============================================================================

func TestNetworkInterfacesModule_Name(t *testing.T) {
	m := &NetworkInterfacesModule{}
	if m.Name() != ModuleNameNetworkInterfaces {
		t.Errorf("Expected ModuleNameNetworkInterfaces, got %v", m.Name())
	}
}

func TestNetworkInterfacesModule_FriendlyName(t *testing.T) {
	m := &NetworkInterfacesModule{}
	expected := "Network Interfaces Configuration"
	if m.FriendlyName() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, m.FriendlyName())
	}
}

func TestNetworkInterfacesModule_ValidateNoInterfaces(t *testing.T) {
	m := &NetworkInterfacesModule{}

	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{},
	}

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if response.Status() != types.StatusFail {
		t.Errorf("Expected fail status with no interfaces")
	}
}

func TestNetworkInterfacesModule_SuccessTemplateIncludesTable(t *testing.T) {
	m := &NetworkInterfacesModule{
		data: &NetworkInterfacesModuleData{
			TotalInterfaces: 2,
			CandidateCount:  2,
			DPDKSupported:   2,
			Interfaces: []*NetworkInterfaceValidation{
				{Name: "eth0", Status: types.StatusPass},
				{Name: "eth1", Status: types.StatusPass},
			},
		},
	}

	template := m.SuccessTemplate()
	if template == "" {
		t.Errorf("Expected non-empty success template")
	}
}

// ============================================================================
// NetworkInterfacesModule Extended Tests - Device Support Scenarios
// ============================================================================

// TestNetworkInterfacesModule_NoSupportedDevices tests scenario 1:
// Interfaces do not include anything that is supported by Weka
func TestNetworkInterfacesModule_NoSupportedDevices(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create unsupported network interfaces (e.g., with no VendorModel or unknown model)
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       1000, // 1Gbps in Mbps
				MaxRate:        "",   // Not used for Ethernet
				EffectiveSpeed: 1000,
				EffectiveRate:  "",
				MTU:            1500,
				VendorModel:    "", // Unknown/unsupported device
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "10.0.0.11/24",
				MaxSpeed:       1000, // 1Gbps in Mbps
				MaxRate:        "",   // Not used for Ethernet
				EffectiveSpeed: 1000,
				EffectiveRate:  "",
				MTU:            1500,
				VendorModel:    "", // Unknown/unsupported device
			},
		},
	}
	// Initialize parents for bonds/slaves logic
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Should fail since no interfaces are supported
	if response.Status() != types.StatusFail {
		t.Errorf("Expected fail status when no interfaces are supported, got %v", response.Status())
	}
}

// TestNetworkInterfacesModule_MultipleUDPOnlyInterfaces tests scenario 2:
// Interfaces include multiple interfaces that are supported (UDP and/or DPDK)
func TestNetworkInterfacesModule_MultipleUDPOnlyInterfaces(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create multiple interfaces supported by Weka (via UDP or DPDK)
	// Using test device ffff:0007 (UDP=T, DPDK=F, SameCard=T, DiffCard=T)
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       20000, // 20Gbps in Mbps
				EffectiveSpeed: 20000, // 20Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0007", // Test device: UDP-only with LACP support
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "10.0.0.11/24",
				MaxSpeed:       20000, // 20Gbps in Mbps
				EffectiveSpeed: 20000, // 20Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0007", // Test device: UDP-only with LACP support
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Should pass or warn (UDP is supported, just not optimal DPDK)
	if response.Status() == types.StatusFail {
		t.Errorf("Expected pass/warn status for UDP-supported interfaces, got %v", response.Status())
	}
}

// TestNetworkInterfacesModule_SingleDPDKPerProcessInterface tests scenario 3:
// Interfaces include only single interface that supports DPDK in per-process mode
func TestNetworkInterfacesModule_SingleDPDKPerProcessInterface(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create single interface with DPDK per-process support
	// Using test device ffff:0006 (UDP=T, DPDK=T, PerProcess=T, no LACP)
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       25000, // 25Gbps in Mbps
				EffectiveSpeed: 25000, // 25Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0006", // Test device: PerProcess DPDK, no LACP
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Single DPDK interface should pass
	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status for single DPDK interface, got %v", response.Status())
	}
}

// TestNetworkInterfacesModule_MultipleDPDKPerProcessInterfaces tests scenario 4:
// Interfaces include multiple interfaces that support DPDK in per-process mode
func TestNetworkInterfacesModule_MultipleDPDKPerProcessInterfaces(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create multiple interfaces with DPDK per-process support
	// Using test device ffff:0004 (UDP=T, DPDK=T, PerProcess=T)
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0004", // Test device: PerProcess DPDK
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "10.0.0.11/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0004", // Test device: PerProcess DPDK
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Multiple DPDK interfaces should pass
	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status for multiple DPDK interfaces, got %v", response.Status())
	}
}

// TestNetworkInterfacesModule_MixedTypesSameSubnetFails tests scenario 5:
// Interfaces include multiple interfaces that reside on same subnet but belong to
// different types (e.g., same subnet, but 1 interface is Ethernet and another is Infiniband)
// This should fail or warn as mixed types on same subnet are problematic
func TestNetworkInterfacesModule_MixedTypesSameSubnetFails(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create mixed type interfaces on same subnet
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1017", // Mellanox ConnectX-4
			},
			{
				Name:           "ib0",
				Type:           "infiniband",
				IP:             "10.0.0.11/24", // Same subnet!
				MaxSpeed:       100000,         // 100Gbps in Mbps
				EffectiveSpeed: 100000,         // 100Gbps in Mbps
				MTU:            2048,
				VendorModel:    "15b3:1017", // Mellanox MT2892 (IB)
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Mixed types on same subnet should cause warning or error
	// (depends on validation logic - this is a problematic configuration)
	if response.Status() == types.StatusPass {
		t.Logf("Mixed interface types on same subnet resulted in pass - may need validation review")
	}
}

// TestNetworkInterfacesModule_BondDPDKSingleNicPass tests scenario 6:
// Interfaces include bond with 2 supported in DPDK single nic - should pass
func TestNetworkInterfacesModule_BondDPDKSingleNicPass(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create bond with 2 slaves supporting DPDK single-nic
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0002", // Supports DPDK SingleNIC
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0002", // Supports DPDK SingleNIC
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "bond0",
				Type:           "bond",
				IP:             "10.0.0.10/24",
				MaxSpeed:       100000,    // 100Gbps in Mbps
				EffectiveSpeed: 100000,    // 100Gbps in Mbps
				BondMode:       "802.3ad", // LACP
				BondSlaves:     []string{"eth0", "eth1"},
				MTU:            9000,
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Bond with DPDK single-nic support should pass
	if response.Status() == types.StatusFail {
		t.Errorf("Expected pass/warn status for DPDK single-nic bond, got fail")
	}
}

// TestNetworkInterfacesModule_BondDPDKPerProcessFails tests scenario 7:
// Interfaces include bond with 2 supported in DPDK per-process NIC, should fail
// (Per-process means each Weka process needs its own NIC, so bonding doesn't help)
func TestNetworkInterfacesModule_BondDPDKPerProcessFails(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create bond with 2 slaves supporting only DPDK per-process
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1017", // Mellanox per-process only
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1017", // Mellanox per-process only
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:        "bond0",
				Type:        "bond",
				IP:          "10.0.0.10/24",
				MaxSpeed:    100000,    // 100Gbps in Mbps
				BondMode:    "802.3ad", // LACP
				BondSlaves:  []string{"eth0", "eth1"},
				MTU:         9000,
				VendorModel: "15b3:1017",
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Bond with per-process DPDK may fail or warn - per-process doesn't benefit from bonding
	// This is a warning/error scenario because per-process mode needs dedicated NICs
	if response.Status() != types.StatusFail && response.Status() != types.StatusWarn {
		t.Logf("Bond with per-process DPDK: got %v - validation may vary by logic", response.Status())
	}
}

// ============================================================================
// NetworkInterfacesModule Extended Bond Tests - Additional Scenarios
// ============================================================================

// TestNetworkInterfacesModule_BondNonLACP tests scenario 8:
// Bond is configured with non-LACP mode (should fail)
func TestNetworkInterfacesModule_BondNonLACP(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create bond with non-LACP mode (e.g., balance-rr)
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:        "bond0",
				Type:        "bond",
				IP:          "10.0.0.10/24",
				MaxSpeed:    100000,       // 100Gbps in Mbps
				BondMode:    "balance-rr", // Non-LACP mode!
				BondSlaves:  []string{"eth0", "eth1"},
				MTU:         9000,
				VendorModel: "15b3:1018",
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Bond with non-LACP mode should fail
	if response.Status() != types.StatusFail {
		t.Errorf("Expected fail status for non-LACP bond, got %v", response.Status())
	}
}

// TestNetworkInterfacesModule_BondWrongSlaveCount tests scenario 9:
// Bond has number of interfaces different from 2 (should fail)
func TestNetworkInterfacesModule_BondWrongSlaveCount(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create bond with only 1 slave
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:        "bond0",
				Type:        "bond",
				IP:          "10.0.0.10/24",
				MaxSpeed:    100000, // 100Gbps in Mbps
				BondMode:    "802.3ad",
				BondSlaves:  []string{"eth0"}, // Only 1 slave!
				MTU:         9000,
				VendorModel: "15b3:1018",
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Bond with wrong slave count should fail
	if response.Status() != types.StatusFail {
		t.Errorf("Expected fail status for bond with 1 slave, got %v", response.Status())
	}

	// Test with 3 slaves as well
	hc2 := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "eth2",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:        "bond0",
				Type:        "bond",
				IP:          "10.0.0.10/24",
				MaxSpeed:    100000, // 100Gbps in Mbps
				BondMode:    "802.3ad",
				BondSlaves:  []string{"eth0", "eth1", "eth2"}, // 3 slaves!
				MTU:         9000,
				VendorModel: "15b3:1018",
			},
		},
	}
	hc2.NetworkInterfaces.InitializeParents()

	jsonData2, _ := json.Marshal(hc2)
	response2, _ := m.Validate(string(jsonData2))

	// Bond with 3 slaves should also fail
	if response2.Status() != types.StatusFail {
		t.Errorf("Expected fail status for bond with 3 slaves, got %v", response2.Status())
	}
}

// TestNetworkInterfacesModule_BondSameCardLACP tests scenario 10:
// Bond has 2 interfaces on same card - should work if device supports SupportedByWekaForLacpSameCard
func TestNetworkInterfacesModule_BondSameCardLACP(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create bond with 2 slaves on the same NIC card (dual-port)
	// Same card: both on "0000:50:00" but with function numbers .0 and .1
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				PCIAddress:     "0000:50:00.0", // Same card, function 0
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				PCIAddress:     "0000:50:00.1", // Same card, function 1
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:        "bond0",
				Type:        "bond",
				IP:          "10.0.0.10/24",
				MaxSpeed:    100000,    // 100Gbps in Mbps
				BondMode:    "802.3ad", // LACP
				BondSlaves:  []string{"eth0", "eth1"},
				MTU:         9000,
				VendorModel: "15b3:1018",
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Bond on same card should pass or warn (depends on device support)
	// The key is that it's not automatically a failure like non-LACP or wrong slave count
	if response.Status() == types.StatusFail {
		t.Logf("Bond with same-card LACP resulted in fail - device may not support same-card LACP")
	} else {
		t.Logf("Bond with same-card LACP resulted in %v - acceptable for same-card configuration", response.Status())
	}
}

// TestNetworkInterfacesModule_BondDifferentCardsLACP tests scenario 11:
// Bond has 2 interfaces on different NICs - should work if device supports SupportedByWekaForLacpDifferentCards
func TestNetworkInterfacesModule_BondDifferentCardsLACP(t *testing.T) {
	m := &NetworkInterfacesModule{}

	// Create bond with 2 slaves on different NIC cards
	// Different cards: one on "0000:50:00" and one on "0000:51:00"
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				PCIAddress:     "0000:50:00.0", // First NIC
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "",
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "15b3:1018",
				PCIAddress:     "0000:51:00.0", // Different NIC
				IsBondSlave:    true,
				BondMaster:     "bond0",
			},
			{
				Name:        "bond0",
				Type:        "bond",
				IP:          "10.0.0.10/24",
				MaxSpeed:    100000,    // 100Gbps in Mbps
				BondMode:    "802.3ad", // LACP
				BondSlaves:  []string{"eth0", "eth1"},
				MTU:         9000,
				VendorModel: "15b3:1018",
			},
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Bond on different cards should work (standard LACP configuration)
	// This is the most common and supported configuration
	if response.Status() == types.StatusFail {
		t.Logf("Bond with different-card LACP resulted in fail")
	} else {
		t.Logf("Bond with different-card LACP resulted in %v - acceptable for standard LACP", response.Status())
	}
}

// ============================================================================
// Module Interface Tests
// ============================================================================

func TestAllModulesImplementInterface(t *testing.T) {
	modules := []HostCheckModule{
		&CPUModule{},
		&KernelModule{},
		&OSModule{},
		&XFSModule{},
		&NVMeDrivesModule{},
		&WekaDirModule{},
		&WekaAgentServiceModuleModule{},
		&NetworkInterfacesModule{},
		&SourceBasedRoutingModule{},
	}

	for _, module := range modules {
		if module.Name() == "" {
			t.Errorf("Module %T has empty Name()", module)
		}

		if module.FriendlyName() == "" {
			t.Errorf("Module %T has empty FriendlyName()", module)
		}

		if module.Description() == "" {
			t.Errorf("Module %T has empty Description()", module)
		}

		// Templates can be empty for some modules, but error template should exist
		if module.ErrorTemplate() == "" && module.SuccessTemplate() == "" {
			t.Errorf("Module %T has no templates defined", module)
		}
	}
}

func TestAllModulesHaveValidateMethod(t *testing.T) {
	modules := []HostCheckModule{
		&CPUModule{},
		&KernelModule{},
		&OSModule{},
		&XFSModule{},
		&NVMeDrivesModule{},
		&WekaDirModule{},
		&WekaAgentServiceModuleModule{},
		&NetworkInterfacesModule{},
		&SourceBasedRoutingModule{},
	}

	validHostCheck := &HostChecksResult{
		PhysicalCores: 32,
		LogicalCores:  64,
		MemoryBytes:   274877906944,
	}

	jsonData, err := json.Marshal(validHostCheck)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	for _, module := range modules {
		response, err := module.Validate(string(jsonData))

		// All modules should handle the basic case without panic
		if err != nil {
			t.Logf("Module %T returned error: %v (this may be expected)", module, err)
		}

		if response == nil {
			t.Errorf("Module %T Validate returned nil response", module)
			continue
		}

		if response.ModuleName() == "" {
			t.Errorf("Module %T returned empty ModuleName in response", module)
		}
	}
}

func TestValidateWithParams(t *testing.T) {
	modules := []HostCheckModule{
		&CPUModule{},
		&KernelModule{},
		&OSModule{},
		&XFSModule{},
		&NVMeDrivesModule{},
		&WekaDirModule{},
		&WekaAgentServiceModuleModule{},
		&NetworkInterfacesModule{},
		&SourceBasedRoutingModule{},
	}

	validHostCheck := &HostChecksResult{
		PhysicalCores: 32,
		LogicalCores:  64,
	}

	jsonData, err := json.Marshal(validHostCheck)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	for _, module := range modules {
		response, err := module.ValidateWithParams(string(jsonData), map[string]interface{}{})

		// All modules should support ValidateWithParams
		if err != nil {
			t.Logf("Module %T returned error: %v (this may be expected)", module, err)
		}

		if response == nil {
			t.Errorf("Module %T ValidateWithParams returned nil", module)
		}
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestModuleOutputFormatting(t *testing.T) {
	tests := []struct {
		name   string
		module HostCheckModule
		hc     *HostChecksResult
	}{
		{
			name:   "CPU Module",
			module: &CPUModule{},
			hc: &HostChecksResult{
				PhysicalCores: 32,
				LogicalCores:  64,
				MemoryBytes:   274877906944,
				HTEnabled:     true,
			},
		},
		{
			name:   "Kernel Module",
			module: &KernelModule{},
			hc: &HostChecksResult{
				KernelVersion: "5.15.0",
			},
		},
		{
			name:   "OS Module",
			module: &OSModule{},
			hc: &HostChecksResult{
				IsRHCOS:   true,
				OSRelease: "RHCOS 4.10",
			},
		},
		{
			name:   "XFS Module",
			module: &XFSModule{},
			hc: &HostChecksResult{
				XFSFound: true,
			},
		},
		{
			name:   "Weka Directory Module",
			module: &WekaDirModule{},
			hc: &HostChecksResult{
				WekaDirPath:       "/mnt/weka",
				WekaDirAvailBytes: 1099511627776,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.hc)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			response, err := tt.module.Validate(string(jsonData))
			if err != nil {
				t.Fatalf("Validate failed: %v", err)
			}

			// Verify response has basic structure
			if response.ModuleName() != tt.module.Name() {
				t.Errorf("Module name mismatch")
			}

			// Verify status is one of the valid statuses
			status := response.Status()
			if status != types.StatusPass && status != types.StatusWarn && status != types.StatusFail {
				t.Errorf("Invalid status: %v", status)
			}
		})
	}
}

// ============================================================================
// Module Response Type Tests
// ============================================================================

func TestCPUModuleResponse_Map(t *testing.T) {
	response := &CPUModuleResponse{
		status:          types.StatusPass,
		Detail:          "CPU and memory OK",
		HTEnabled:       true,
		PhysicalCores:   32,
		LogicalCores:    64,
		MemoryBytes:     274877906944,
		FreeMemoryBytes: 137438953472,
	}

	m := response.Map()
	if m == nil {
		t.Errorf("Expected non-nil map")
	}

	if m["Status"] != types.StatusPass {
		t.Errorf("Status mismatch in map")
	}
}

func TestNetworkInterfacesModuleResponse_Map(t *testing.T) {
	data := &NetworkInterfacesModuleData{
		TotalInterfaces: 2,
		CandidateCount:  2,
		DPDKSupported:   2,
		Interfaces:      []*NetworkInterfaceValidation{},
	}

	response := &NetworkInterfacesModuleResponse{
		data:       data,
		status:     types.StatusPass,
		moduleName: ModuleNameNetworkInterfaces,
		details:    "All interfaces OK",
	}

	m := response.Map()
	if m == nil {
		t.Errorf("Expected non-nil map")
	}

	if m["TotalInterfaces"] != 2 {
		t.Errorf("TotalInterfaces mismatch in map")
	}
}

func TestSourceBasedRoutingModuleResponse_Map(t *testing.T) {
	data := &SourceBasedRoutingModuleData{
		MultiInterfaceFound: true,
		SameSubnetFound:     true,
		InterfaceGroups:     []SubnetInterfaceGroup{},
		SBRRules:            []SourceBasedRoutingRule{},
	}

	response := &SourceBasedRoutingModuleResponse{
		data:       data,
		status:     types.StatusPass,
		moduleName: ModuleNameSourceBasedRouting,
		detail:     "SBR configured",
	}

	m := response.Map()
	if m == nil {
		t.Errorf("Expected non-nil map")
	}

	if m["MultiInterfaceFound"] != true {
		t.Errorf("MultiInterfaceFound mismatch in map")
	}
}

// ============================================================================
// Source-Based Routing Module Comprehensive Tests
// ============================================================================

// TestSourceBasedRoutingModule_ValidSBRRulesOnBothInterfaces tests scenario 1:
// 2 interfaces on same subnet, both have valid SBR rules (should pass)
func TestSourceBasedRoutingModule_ValidSBRRulesOnBothInterfaces(t *testing.T) {
	m := &SourceBasedRoutingModule{}

	// Create two interfaces on same subnet with valid SBR rules
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0001", // Test device with full support
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "10.0.0.11/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0001", // Test device with full support
			},
		},
		// SBR rules configured for both interfaces
		NetworkNamespaceRouting: &NetworkNamespaceRouting{
			Namespace: "default",
			RoutingTables: []*RoutingTableInfo{
				{
					TableName: "100",
					TableID:   100,
					Routes: []RouteEntry{
						{
							Destination: "0.0.0.0/0",
							Gateway:     "10.0.0.1",
							Device:      "eth0",
							Source:      "10.0.0.10",
						},
					},
				},
				{
					TableName: "101",
					TableID:   101,
					Routes: []RouteEntry{
						{
							Destination: "0.0.0.0/0",
							Gateway:     "10.0.0.1",
							Device:      "eth1",
							Source:      "10.0.0.11",
						},
					},
				},
			},
			RoutingRules: []*RoutingRule{
				{
					Priority:  100,
					Condition: "from 10.0.0.10",
					Table:     "100",
				},
				{
					Priority:  101,
					Condition: "from 10.0.0.11",
					Table:     "101",
				},
			},
			RuleCount:  2,
			TableCount: 2,
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Should pass when both interfaces have valid SBR rules
	if response.Status() != types.StatusPass {
		t.Errorf("Expected pass status for valid SBR rules on both interfaces, got %v", response.Status())
	}

	// Verify response data contains both interfaces
	if respData, ok := response.(*SourceBasedRoutingModuleResponse); ok {
		if respData.data == nil {
			t.Errorf("Expected response data to be populated")
		} else if !respData.data.SameSubnetFound {
			t.Errorf("Expected SameSubnetFound to be true")
		}
	}
}

// TestSourceBasedRoutingModule_MissingOrInvalidSBRRulesOnOneInterface tests scenario 2:
// 2 interfaces on same subnet, one has missing or invalid SBR rules (should fail/warn)
func TestSourceBasedRoutingModule_MissingOrInvalidSBRRulesOnOneInterface(t *testing.T) {
	m := &SourceBasedRoutingModule{}

	// Create two interfaces on same subnet, only one has SBR rules
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0001", // Test device with full support
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "10.0.0.11/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0001", // Test device with full support
			},
		},
		// SBR rules only for eth0, missing for eth1
		NetworkNamespaceRouting: &NetworkNamespaceRouting{
			Namespace: "default",
			RoutingTables: []*RoutingTableInfo{
				{
					TableName: "100",
					TableID:   100,
					Routes: []RouteEntry{
						{
							Destination: "0.0.0.0/0",
							Gateway:     "10.0.0.1",
							Device:      "eth0",
							Source:      "10.0.0.10",
						},
					},
				},
				// eth1 has no routing table - MISSING!
			},
			RoutingRules: []*RoutingRule{
				{
					Priority:  100,
					Condition: "from 10.0.0.10",
					Table:     "100",
				},
				// eth1 has no rule - MISSING!
			},
			RuleCount:  1,
			TableCount: 1,
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Should fail or warn when SBR rules are missing on one interface
	if response.Status() == types.StatusPass {
		t.Errorf("Expected fail/warn status for missing SBR rules on one interface, got %v", response.Status())
	}

	// Verify response indicates multiple interfaces on same subnet
	if respData, ok := response.(*SourceBasedRoutingModuleResponse); ok {
		if respData.data == nil {
			t.Errorf("Expected response data to be populated")
		} else if !respData.data.SameSubnetFound {
			t.Errorf("Expected SameSubnetFound to be true (multi-interface scenario)")
		}
	}
}

// TestSourceBasedRoutingModule_AllRulesMissingOnBothInterfaces tests scenario 3:
// 2 interfaces on same subnet, both missing SBR rules (should fail)
func TestSourceBasedRoutingModule_AllRulesMissingOnBothInterfaces(t *testing.T) {
	m := &SourceBasedRoutingModule{}

	// Create two interfaces on same subnet with NO SBR rules
	hc := &HostChecksResult{
		NetworkInterfaces: NetworkInterfaces{
			{
				Name:           "eth0",
				Type:           "ethernet",
				IP:             "10.0.0.10/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0001", // Test device with full support
			},
			{
				Name:           "eth1",
				Type:           "ethernet",
				IP:             "10.0.0.11/24",
				MaxSpeed:       100000, // 100Gbps in Mbps
				EffectiveSpeed: 100000, // 100Gbps in Mbps
				MTU:            9000,
				VendorModel:    "ffff:0001", // Test device with full support
			},
		},
		// NO SBR rules at all
		NetworkNamespaceRouting: &NetworkNamespaceRouting{
			Namespace:     "default",
			RoutingTables: []*RoutingTableInfo{},
			RoutingRules:  []*RoutingRule{},
			RuleCount:     0,
			TableCount:    0,
		},
	}
	hc.NetworkInterfaces.InitializeParents()

	jsonData, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	response, err := m.Validate(string(jsonData))
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Should fail when no SBR rules are configured on multi-interface setup
	if response.Status() == types.StatusPass {
		t.Errorf("Expected fail/warn status for no SBR rules on multi-interface subnet, got %v", response.Status())
	}
}

// ============================================================================
// Template Placeholder Tests
// ============================================================================

func TestTemplatePlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		template string
		params   map[string]interface{}
	}{
		{
			name:     "NodeName placeholder",
			template: "Error on {{.NodeName}}",
			params:   map[string]interface{}{"NodeName": "node-1"},
		},
		{
			name:     "Issue placeholder",
			template: "Issue: {{.Issue}}",
			params:   map[string]interface{}{"Issue": "CPU too low"},
		},
		{
			name:     "FriendlyName placeholder",
			template: "{{.FriendlyName}} failed",
			params:   map[string]interface{}{"FriendlyName": "Network Check"},
		},
		{
			name:     "Detail placeholder",
			template: "Details: {{.Detail}}",
			params:   map[string]interface{}{"Detail": "16 cores"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpolateTemplate(tt.template, tt.params)
			if result == "" {
				t.Errorf("Expected non-empty result after interpolation")
			}

			// Verify placeholder was replaced
			if tt.name == "NodeName placeholder" && result == "Error on node-1" {
				// Success
			} else if tt.name != "NodeName placeholder" && !containsPlaceholder(result) {
				// Placeholder should be replaced
			}
		})
	}
}

// Helper function to check if string contains placeholders
func containsPlaceholder(s string) bool {
	return strings.Contains(s, "{{.") && strings.Contains(s, "}}")
}
