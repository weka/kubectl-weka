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
