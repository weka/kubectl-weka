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

func (m *OSModule) Description() string {
	return "OS detection and validation (RHCOS/CoreOS/Standard Linux)"
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

func (m *WekaDirModule) Description() string {
	return "Weka directory existence and available space (>=300GB)"
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

func (m *XFSModule) Description() string {
	return "XFS tools installation validation"
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

func (m *WekaClientModule) Description() string {
	return "Weka client presence and cleanup validation"
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

func (m *NetworkModule) Description() string {
	return "Network interface validation (Mellanox NICs, bond configuration, LACP)"
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

func (m *CPUModule) Description() string {
	return "CPU hyperthreading, core count, and memory availability"
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

// ============================================================================
// HostCheck Module Factory
// ============================================================================

// CreateHostCheckRegistry creates a pre-configured registry with all standard modules
// This is part of the public API for the modular hostcheck system
// and will be used by plan_clients and other future commands.
// nolint:unused
func CreateHostCheckRegistry() *HostCheckRegistry {
	registry := NewHostCheckRegistry()

	// Register all standard modules
	_ = registry.Register(&OSModule{})
	_ = registry.Register(&WekaDirModule{})
	_ = registry.Register(&XFSModule{})
	_ = registry.Register(&WekaClientModule{})
	_ = registry.Register(&NetworkModule{})
	_ = registry.Register(&CPUModule{})

	return registry
}
