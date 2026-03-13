package cmd

import (
	"encoding/json"
	"fmt"
)

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
