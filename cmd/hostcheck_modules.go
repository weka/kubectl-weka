package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
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

	// Parse OSRelease to extract NAME and VERSION_ID
	// The OSRelease is a concatenated string from /etc/os-release with newlines converted to spaces
	// e.g., "PRETTY_NAME=\"Ubuntu 22.04.5 LTS\" NAME=\"Ubuntu\" VERSION_ID=\"22.04\" ..."

	name := ""
	versionID := ""
	prettyName := ""

	// Split by space to get individual key=value pairs
	// But we need to be careful with quoted values that might contain spaces
	parts := strings.Fields(hc.OSRelease)
	for _, part := range parts {
		// Each part looks like KEY=VALUE or KEY="VALUE"
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				value := kv[1]

				// Remove surrounding quotes
				value = strings.Trim(value, `"`)

				switch key {
				case "NAME":
					name = value
				case "VERSION_ID":
					versionID = value
				case "PRETTY_NAME":
					prettyName = value
				}
			}
		}
	}

	// Build osDisplay with fallback chain
	osDisplay := ""
	if name != "" && versionID != "" {
		osDisplay = fmt.Sprintf("%s %s", name, versionID)
	} else if name != "" {
		osDisplay = name
	} else if prettyName != "" {
		// Extract just the OS name from PRETTY_NAME (e.g., "Ubuntu 22.04.5 LTS" -> best effort)
		// Remove the distribution name in parentheses if present
		if idx := strings.Index(prettyName, "("); idx > 0 {
			osDisplay = strings.TrimSpace(prettyName[:idx])
		} else {
			osDisplay = prettyName
		}
	} else {
		osDisplay = "Unknown OS"
	}

	return map[string]interface{}{
		"IsRHCOS":   hc.IsRHCOS,
		"OSRelease": osDisplay,
		"Status":    "success",
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
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *WekaDirModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *WekaDirModule) SuggestedResolutionTemplate() string {
	return "Ensure {{.Path}} has at least {{.MinGB}}GB free space. Use: df -h {{.Path}}"
}

func (m *WekaDirModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	status := "success"
	if !hc.IsWekaDirExists() {
		status = "error"
	}

	availGB := float64(hc.WekaDirAvailBytes) / (1024 * 1024 * 1024)

	return map[string]interface{}{
		"Status":     status,
		"OK":         hc.IsWekaDirAtLeast(100), // default value
		"Path":       hc.WekaDirPath,
		"AvailBytes": hc.WekaDirAvailBytes,
		"AvailGB":    fmt.Sprintf("%.1d", availGB),
	}, nil
}

// ValidateWithParams implements HostCheckModule with min GB parameter support
// Params: {"wekaDirMinFailGB": 800} to set minimum GB requirement for failure
func (m *WekaDirModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	// Get minimum GB requirement from params
	minFailGB := int64(300) // default 300GB
	minWarnGB := int64(500) // default 500GB

	if val, ok := params["wekaDirMinFailGB"].(float64); ok {
		minFailGB = int64(val)
	} else if val, ok := params["wekaDirMinFailGB"].(int64); ok {
		minFailGB = val
	} else if val, ok := params["wekaDirMinFailGB"].(int); ok {
		minFailGB = int64(val)
	}

	if val, ok := params["wekaDirMinWarnGB"].(float64); ok {
		minWarnGB = int64(val)
	} else if val, ok := params["wekaDirMinWarnGB"].(int64); ok {
		minWarnGB = val
	} else if val, ok := params["wekaDirMinWarnGB"].(int); ok {
		minWarnGB = int64(val)
	}

	availGB := hc.WekaDirAvailBytes / (1024 * 1024 * 1024)

	status := "success"
	issue := ""

	if !hc.WekaDirExists {
		status = "error"
		issue = fmt.Sprintf("Weka directory does not exist: %s", hc.WekaDirPath)
	} else if availGB < minFailGB {
		status = "error"
		issue = fmt.Sprintf("Only %.1f GB available, need at least %d GB", float64(availGB), minFailGB)
	} else if availGB < minWarnGB {
		status = "warning"
		issue = fmt.Sprintf("Only %.1f GB available, recommended at least %d GB", float64(availGB), minWarnGB)
	}

	return map[string]interface{}{
		"Status":     status,
		"OK":         status == "success",
		"Path":       hc.WekaDirPath,
		"Issue":      issue,
		"AvailBytes": hc.WekaDirAvailBytes,
		"AvailGB":    fmt.Sprintf("%.1f", float64(availGB)),
		"MinFailGB":  minFailGB,
		"MinWarnGB":  minWarnGB,
		"MinGB":      minFailGB, // For template use
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

	status := "success"
	detail := "found"
	if !hc.HasXFS() {
		status = "error"
		detail = "not found"
	}

	return map[string]interface{}{
		"Status": status,
		"Found":  hc.HasXFS(),
		"Detail": detail,
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

	status := "success"
	detail := "clean (no Weka agent service found)"

	if !hc.IsWekaAgentClean() {
		status = "warning"
		detail = "Weka agent service exists - may interfere with installation"
	}

	return map[string]interface{}{
		"Status": status,
		"Clean":  hc.IsWekaAgentClean(),
		"Detail": detail,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for Weka client validation
func (m *WekaClientModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
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
	return "⚠️ WARN: {{.FriendlyName}}: {{.Detail}}"
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

	// Determine status based on CPU configuration
	status := "success"
	var warningMsg string

	// Check for dual-socket AMD - not recommended for WEKA
	if hc.CPUSockets == 2 && strings.ToLower(hc.CPUFamily) == "amd" {
		status = "warning"
		warningMsg = "Dual-socket AMD architecture detected! This architecture does not provide best performance with WEKA"
	}

	// Format detail string with both physical cores and logical cores for clarity
	var detail string
	if hc.PhysicalCores == hc.LogicalCores {
		// HT is off or single-threaded
		detail = fmt.Sprintf("%d cores, %.0f GB RAM", hc.PhysicalCores, float64(hc.MemoryBytes)/(1024*1024*1024))
	} else {
		// HT is on - show physical cores and threads
		detail = fmt.Sprintf("%d cores (%d threads), %.0f GB RAM", hc.PhysicalCores, hc.LogicalCores, float64(hc.MemoryBytes)/(1024*1024*1024))
	}

	// Add CPU family and socket info
	if hc.CPUFamily != "" {
		detail += fmt.Sprintf(" [%s", hc.CPUFamily)
		if hc.CPUSockets > 0 {
			detail += fmt.Sprintf(" %d-socket", hc.CPUSockets)
		}
		if hc.CPUArch != "" {
			detail += fmt.Sprintf(" %s", hc.CPUArch)
		}
		detail += "]"
	}

	// Add warning message to detail if present
	displayDetail := detail
	if warningMsg != "" {
		displayDetail = detail + "\n     " + warningMsg
	}

	return map[string]interface{}{
		"Status":          status,
		"Detail":          displayDetail,
		"Warning":         warningMsg,
		"HTEnabled":       hc.HTEnabled,
		"PhysicalCores":   hc.PhysicalCores,
		"LogicalCores":    hc.LogicalCores,
		"MemoryBytes":     hc.MemoryBytes,
		"FreeMemoryBytes": hc.FreeMemoryBytes,
		"HugepagesFree":   hc.HugepagesFree,
		"CPUModel":        hc.CPUModel,
		"CPUFamily":       hc.CPUFamily,
		"CPUArch":         hc.CPUArch,
		"CPUSockets":      hc.CPUSockets,
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
	return "⚠️ WARN: {{.FriendlyName}}: {{.KernelVersion}} (recommended >=5.10)"
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

	status := "success"
	if hc.KernelVersion <= "5.10" {
		status = "warning"
	}

	return map[string]interface{}{
		"Status":        status,
		"KernelVersion": hc.KernelVersion,
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
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
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

	// Build detail string
	// Count only valid NVMe drives with serial numbers (exclude partitions and drives without serial)
	validDrives := 0
	for _, drive := range hc.NVMeDrives {
		// Only count drives with serial numbers (not empty), and exclude partitions (device names like nvme0n1p1)
		if drive.SerialNumber != "" && !strings.Contains(drive.DeviceName, "p") {
			validDrives++
		}
	}

	detail := ""
	if !hc.HasNVMeDrives() {
		detail = "No NVMe drives detected"
	} else {
		detail = fmt.Sprintf("%d drive(s) available", validDrives)
	}

	return map[string]interface{}{
		"Status":     "success",
		"Detail":     detail,
		"Drives":     hc.NVMeDrives,
		"DriveCount": validDrives,
		"HasDrives":  hc.HasNVMeDrives(),
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for NVMe drives validation
func (m *NVMeDrivesModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}
