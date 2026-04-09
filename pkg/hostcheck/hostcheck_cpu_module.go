package hostcheck

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weka/kubectl-weka/pkg/types"
)

// CPUModule validates CPU and memory resources
type CPUModule struct{}

func (m *CPUModule) Name() ModuleName {
	return ModuleNameCpuMemory
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

// CPUModuleResponse implements HostCheckModuleResult for CPU validation
type CPUModuleResponse struct {
	status          types.CheckStatus
	Detail          string
	Warning         string
	HTEnabled       bool
	PhysicalCores   int
	LogicalCores    int
	MemoryBytes     int64
	FreeMemoryBytes int64
	HugepagesFree   int64
	CPUModel        string
	CPUFamily       string
	CPUArch         string
	CPUSockets      int
	moduleName      ModuleName
	err             error
}

func (r *CPUModuleResponse) Status() types.CheckStatus { return r.status }
func (r *CPUModuleResponse) ModuleName() ModuleName    { return r.moduleName }
func (r *CPUModuleResponse) Details() string           { return r.Detail }
func (r *CPUModuleResponse) Error() error              { return r.err }
func (r *CPUModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":          r.status,
		"Detail":          r.Detail,
		"Warning":         r.Warning,
		"HTEnabled":       r.HTEnabled,
		"PhysicalCores":   r.PhysicalCores,
		"LogicalCores":    r.LogicalCores,
		"MemoryBytes":     r.MemoryBytes,
		"FreeMemoryBytes": r.FreeMemoryBytes,
		"HugepagesFree":   r.HugepagesFree,
		"CPUModel":        r.CPUModel,
		"CPUFamily":       r.CPUFamily,
		"CPUArch":         r.CPUArch,
		"CPUSockets":      r.CPUSockets,
		"ModuleName":      r.moduleName,
		"Error":           r.err,
	}
}

// Validate validates the CPU and memory resources of a node
func (m *CPUModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &CPUModuleResponse{status: types.StatusFail, moduleName: m.Name(), err: err}, err
	}

	status := types.StatusPass
	var warningMsg string
	var errorMsg string

	// Check for dual-socket AMD - not recommended for WEKA
	if hc.CPUSockets == 2 && strings.ToLower(hc.CPUFamily) == "amd" {
		status = types.StatusWarn
		warningMsg = "Dual-socket AMD architecture detected! This architecture does not provide best performance with WEKA"
	}

	// Check for AVX2 support (required)
	if !hc.AVX2Supported {
		status = types.StatusFail
		errorMsg = "CPU does not support AVX2 instructions. WEKA requires AVX2 (Haswell or newer Intel, or appropriate AMD)."
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

	// Add AVX2 support status
	if hc.AVX2Supported {
		detail += " [AVX2: yes]"
	} else {
		detail += " [AVX2: NO]"
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
	if errorMsg != "" {
		displayDetail = detail + "\n     " + errorMsg
	}

	return &CPUModuleResponse{
		status:          status,
		Detail:          displayDetail,
		Warning:         warningMsg,
		HTEnabled:       hc.HTEnabled,
		PhysicalCores:   hc.PhysicalCores,
		LogicalCores:    hc.LogicalCores,
		MemoryBytes:     hc.MemoryBytes,
		FreeMemoryBytes: hc.FreeMemoryBytes,
		HugepagesFree:   hc.HugepagesFree,
		CPUModel:        hc.CPUModel,
		CPUFamily:       hc.CPUFamily,
		CPUArch:         hc.CPUArch,
		CPUSockets:      hc.CPUSockets,
		moduleName:      m.Name(),
		err: func() error {
			if errorMsg != "" {
				return fmt.Errorf("%s", errorMsg)
			} else {
				return nil
			}
		}(),
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for CPU validation
func (m *CPUModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}
