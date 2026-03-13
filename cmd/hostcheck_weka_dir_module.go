package cmd

import (
	"encoding/json"
	"fmt"
)

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
