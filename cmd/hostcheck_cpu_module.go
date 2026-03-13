package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
