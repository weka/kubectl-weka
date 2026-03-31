package hostcheck

import (
	"encoding/json"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/types"
)

// WekaDirModule validates Weka directory availability
type WekaDirModule struct{}

// WekaDirModuleResponse implements HostCheckModuleResponse for Weka directory validation
type WekaDirModuleResponse struct {
	status     types.CheckStatus
	OK         bool
	Path       string
	Issue      string
	AvailBytes int64
	AvailGB    string
	MinFailGB  int64
	MinWarnGB  int64
	MinGB      int64
	moduleName ModuleName
	err        error
}

func (r *WekaDirModuleResponse) Status() types.CheckStatus { return r.status }
func (r *WekaDirModuleResponse) ModuleName() ModuleName    { return r.moduleName }
func (r *WekaDirModuleResponse) Details() string           { return r.Issue }
func (r *WekaDirModuleResponse) Error() error              { return r.err }
func (r *WekaDirModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":     r.status,
		"OK":         r.OK,
		"Path":       r.Path,
		"Issue":      r.Issue,
		"AvailBytes": r.AvailBytes,
		"AvailGB":    r.AvailGB,
		"MinFailGB":  r.MinFailGB,
		"MinWarnGB":  r.MinWarnGB,
		"MinGB":      r.MinGB,
		"ModuleName": r.moduleName,
		"Error":      r.err,
	}
}

func (m *WekaDirModule) Name() ModuleName {
	return ModuleNameWekaDirectory
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

func (m *WekaDirModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &WekaDirModuleResponse{status: types.StatusFail, moduleName: m.Name(), err: err}, err
	}

	status := types.StatusPass
	issue := ""
	minFailGB := int64(300)
	minWarnGB := int64(500)
	availGB := float64(hc.WekaDirAvailBytes) / (1024 * 1024 * 1024)
	ok := hc.IsWekaDirAtLeast(100)

	if !hc.WekaDirExists {
		status = types.StatusFail
		issue = fmt.Sprintf("Weka directory does not exist: %s", hc.WekaDirPath)
	} else if int64(availGB) < minFailGB {
		status = types.StatusFail
		issue = fmt.Sprintf("Only %.1f GB available, need at least %d GB", availGB, minFailGB)
	} else if int64(availGB) < minWarnGB {
		status = types.StatusWarn
		issue = fmt.Sprintf("Only %.1f GB available, recommended at least %d GB", availGB, minWarnGB)
	}

	return &WekaDirModuleResponse{
		status:     status,
		OK:         ok,
		Path:       hc.WekaDirPath,
		Issue:      issue,
		AvailBytes: hc.WekaDirAvailBytes,
		AvailGB:    fmt.Sprintf("%.1f", availGB),
		MinFailGB:  minFailGB,
		MinWarnGB:  minWarnGB,
		MinGB:      minFailGB,
		moduleName: m.Name(),
		err:        nil,
	}, nil
}

// ValidateWithParams implements HostCheckModule with min GB parameter support
// Params: {"wekaDirMinFailGB": 800} to set minimum GB requirement for failure
func (m *WekaDirModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &WekaDirModuleResponse{status: types.StatusFail, moduleName: m.Name(), err: err}, err
	}

	minFailGB := int64(300)
	minWarnGB := int64(500)
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

	availGB := float64(hc.WekaDirAvailBytes) / (1024 * 1024 * 1024)
	status := types.StatusPass
	issue := ""
	ok := hc.IsWekaDirAtLeast(100)

	if !hc.WekaDirExists {
		status = types.StatusFail
		issue = fmt.Sprintf("Weka directory does not exist: %s", hc.WekaDirPath)
	} else if int64(availGB) < minFailGB {
		status = types.StatusFail
		issue = fmt.Sprintf("Only %.1f GB available, need at least %d GB", availGB, minFailGB)
	} else if int64(availGB) < minWarnGB {
		status = types.StatusWarn
		issue = fmt.Sprintf("Only %.1f GB available, recommended at least %d GB", availGB, minWarnGB)
	}

	return &WekaDirModuleResponse{
		status:     status,
		OK:         ok,
		Path:       hc.WekaDirPath,
		Issue:      issue,
		AvailBytes: hc.WekaDirAvailBytes,
		AvailGB:    fmt.Sprintf("%.1f", availGB),
		MinFailGB:  minFailGB,
		MinWarnGB:  minWarnGB,
		MinGB:      minFailGB,
		moduleName: m.Name(),
		err:        nil,
	}, nil
}
