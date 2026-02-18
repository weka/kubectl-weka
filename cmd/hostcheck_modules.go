package cmd

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// HostCheck Modules - Modular Validation of Host Check Results
// ============================================================================

// OSModule validates OS compatibility
type OSModule struct{}

func (m *OSModule) Name() string {
	return "os"
}

func (m *OSModule) FriendlyName() string {
	return "Operating System"
}

func (m *OSModule) Description() string {
	return "OS detection and validation (RHCOS/CoreOS/Standard Linux)"
}

func (m *OSModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.OSRelease}}"
}

func (m *OSModule) WarningTemplate() string {
	return "" // No warning state for OS module
}

func (m *OSModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}} unsupported: {{.OSRelease}}"
}

func (m *OSModule) SuggestedResolutionTemplate() string {
	return "Please ensure node {{.NodeName}} is running a supported Linux distribution (Ubuntu, RHEL/CentOS, RHCOS, etc.)"
}

func (m *OSModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	return map[string]interface{}{
		"is_rhcos":   hc.IsRHCOS,
		"os_release": hc.OSRelease,
		"status":     "ok",
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for OS validation
func (m *OSModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}

// WekaDirModule validates Weka directory availability
type WekaDirModule struct{}

func (m *WekaDirModule) Name() string {
	return "weka_dir"
}

func (m *WekaDirModule) FriendlyName() string {
	return "Weka Directory"
}

func (m *WekaDirModule) Description() string {
	return "Weka directory existence and available space (>=300GB)"
}

func (m *WekaDirModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Path}} has {{.AvailGB}}GB available"
}

func (m *WekaDirModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Path}} has {{.AvailGB}}GB available (recommended: 300GB)"
}

func (m *WekaDirModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}} check failed: {{.Issue}}"
}

func (m *WekaDirModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}, ensure /opt/k8s-weka has at least 300GB free space: {{.Command}}"
}

func (m *WekaDirModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	status := "ok"
	if !hc.WekaDirOK {
		status = "error"
	}

	return map[string]interface{}{
		"status":      status,
		"ok":          hc.WekaDirOK,
		"path":        hc.WekaDirPath,
		"detail":      hc.WekaDirDetail,
		"avail_bytes": hc.WekaDirAvailBytes,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for Weka directory validation
func (m *WekaDirModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}

// XFSModule validates XFS tools installation
type XFSModule struct{}

func (m *XFSModule) Name() string {
	return "xfs"
}

func (m *XFSModule) FriendlyName() string {
	return "XFS Tools"
}

func (m *XFSModule) Description() string {
	return "XFS tools installation validation"
}

func (m *XFSModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *XFSModule) WarningTemplate() string {
	return "" // No warning state for XFS module
}

func (m *XFSModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}} not found: {{.Detail}}"
}

func (m *XFSModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}, install XFS tools: sudo apt-get install xfsprogs (Ubuntu) or sudo yum install xfsprogs (RHEL/CentOS)"
}

func (m *XFSModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	status := "ok"
	if !hc.XFSInstalled {
		status = "error"
	}

	return map[string]interface{}{
		"status":    status,
		"installed": hc.XFSInstalled,
		"detail":    hc.XFSDetail,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for XFS validation
func (m *XFSModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}

// WekaClientModule validates Weka client cleanup
type WekaClientModule struct{}

func (m *WekaClientModule) Name() string {
	return "weka_client"
}

func (m *WekaClientModule) FriendlyName() string {
	return "Weka Client"
}

func (m *WekaClientModule) Description() string {
	return "Weka client presence and cleanup validation"
}

func (m *WekaClientModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *WekaClientModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Detail}}"
}

func (m *WekaClientModule) ErrorTemplate() string {
	return "⚠️ {{.FriendlyName}}: {{.Detail}}"
}

func (m *WekaClientModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}, clean up old Weka client: sudo apt-get remove weka-client (Ubuntu) or sudo yum remove weka-client (RHEL/CentOS)"
}

func (m *WekaClientModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	status := "ok"
	if !hc.WekaClientClean {
		status = "warning"
	}

	return map[string]interface{}{
		"status": status,
		"clean":  hc.WekaClientClean,
		"detail": hc.WekaClientDetail,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for Weka client validation
func (m *WekaClientModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}

// NetworkModule validates network configuration (Mellanox, bonds, LACP)
type NetworkModule struct{}

func (m *NetworkModule) Name() string {
	return "network"
}

func (m *NetworkModule) FriendlyName() string {
	return "Network Configuration"
}

func (m *NetworkModule) Description() string {
	return "Network interface validation (Mellanox NICs, bond configuration, LACP)"
}

func (m *NetworkModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *NetworkModule) WarningTemplate() string {
	return "⚠️  WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NetworkModule) ErrorTemplate() string {
	return "⚠️  {{.FriendlyName}}: {{.Issue}}"
}

func (m *NetworkModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}, verify network configuration: check Mellanox NIC presence and bond setup with: ethtool {{.Interface}}"
}

func (m *NetworkModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	status := "ok"
	if !hc.Mellanox {
		status = "warning"
	}
	if !hc.BondLACPOk {
		status = "warning"
	}

	return map[string]interface{}{
		"status":           status,
		"mellanox":         hc.Mellanox,
		"mellanox_detail":  hc.MellanoxDetail,
		"mlx_ifaces":       hc.MlxIfaces,
		"mlx_bonds":        hc.MlxBonds,
		"bond_lacp_ok":     hc.BondLACPOk,
		"bond_lacp_detail": hc.BondLACPDetail,
	}, nil
}

// ValidateWithParams implements HostCheckModule with ethDevice parameter support
// Params: {"ethDevice": "bond0"} to validate a specific interface
func (m *NetworkModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	// If no ethDevice parameter provided, fall back to generic validation
	ethDevice, hasEthDevice := params["ethDevice"].(string)
	if !hasEthDevice || ethDevice == "" {
		return m.Validate(podOutput)
	}

	// Validate specific ethDevice
	status := "ok"
	found := false
	hasLACP := false
	var issues []string

	// Check if ethDevice is a regular Mellanox interface
	for _, iface := range hc.MlxIfaces {
		if iface.Name == ethDevice {
			found = true
			break
		}
	}

	// Check if ethDevice is a bond
	if !found {
		for _, bond := range hc.MlxBonds {
			if bond.Name == ethDevice {
				found = true
				// Validate LACP if this is a bond
				if !hc.BondLACPOk {
					status = "error"
					issues = append(issues, fmt.Sprintf("Bond not using LACP: %s", hc.BondLACPDetail))
				} else {
					hasLACP = true
				}
				break
			}
		}
	}

	if !found {
		status = "error"
		issues = append(issues, fmt.Sprintf("Interface '%s' not found", ethDevice))
	}

	return map[string]interface{}{
		"status":           status,
		"ethDevice":        ethDevice,
		"found":            found,
		"has_lacp":         hasLACP,
		"issues":           issues,
		"mlx_ifaces":       hc.MlxIfaces,
		"mlx_bonds":        hc.MlxBonds,
		"bond_lacp_ok":     hc.BondLACPOk,
		"bond_lacp_detail": hc.BondLACPDetail,
	}, nil
}

// CPUModule validates CPU and memory resources
type CPUModule struct{}

func (m *CPUModule) Name() string {
	return "cpu_memory"
}

func (m *CPUModule) FriendlyName() string {
	return "CPU & Memory"
}

func (m *CPUModule) Description() string {
	return "CPU hyperthreading, core count, and memory availability"
}

func (m *CPUModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *CPUModule) WarningTemplate() string {
	return "" // No warning state for CPU module
}

func (m *CPUModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *CPUModule) SuggestedResolutionTemplate() string {
	return "Node {{.NodeName}} has insufficient resources. Check current allocation with: free -h && lscpu"
}

func (m *CPUModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	return map[string]interface{}{
		"status":            "ok",
		"ht_enabled":        hc.HTEnabled,
		"physical_cores":    hc.PhysicalCores,
		"logical_cores":     hc.LogicalCores,
		"memory_bytes":      hc.MemoryBytes,
		"free_memory_bytes": hc.FreeMemoryBytes,
		"hugepages_free":    hc.HugepagesFree,
		"cpu_model":         hc.CPUModel,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for CPU validation
func (m *CPUModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}

type KernelModule struct{}

func (m *KernelModule) Name() string {
	return "kernel"
}

func (m *KernelModule) FriendlyName() string {
	return "Kernel Version"
}

func (m *KernelModule) Description() string {
	return "Kernel version validation (recommended >=5.10)"
}

func (m *KernelModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.KernelVersion}}"
}

func (m *KernelModule) WarningTemplate() string {
	return "⚠️  WARN: {{.FriendlyName}}: {{.KernelVersion}} (recommended >=5.10)"
}

func (m *KernelModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.KernelVersion}} (recommended >=5.10)"
}

func (m *KernelModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}, consider upgrading kernel to version 5.10 or later for optimal performance and compatibility"
}

func (m *KernelModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	status := "ok"
	if hc.KernelVersion <= "5.10" {
		status = "warning"
	}

	return map[string]interface{}{
		"status":         status,
		"kernel_version": hc.KernelVersion,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for kernel validation
func (m *KernelModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}

// NVMeDrivesModule validates NVMe drive availability and status
type NVMeDrivesModule struct{}

func (m *NVMeDrivesModule) Name() string {
	return "nvme_drives"
}

func (m *NVMeDrivesModule) FriendlyName() string {
	return "NVMe Drives"
}

func (m *NVMeDrivesModule) Description() string {
	return "NVMe drive discovery and availability check"
}

func (m *NVMeDrivesModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *NVMeDrivesModule) WarningTemplate() string {
	return "⚠️  WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NVMeDrivesModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NVMeDrivesModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}: {{.Resolution}}"
}

func (m *NVMeDrivesModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	return map[string]interface{}{
		"status":       "ok",
		"drives":       hc.NVMeDrives,
		"drive_count":  hc.NVMeDriveCount,
		"drive_detail": hc.NVMeDriveDetail,
		"detail":       hc.NVMeDriveDetail,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for NVMe drives validation
func (m *NVMeDrivesModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}
