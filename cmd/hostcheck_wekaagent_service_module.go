package cmd

import (
	"encoding/json"
	"fmt"
)

// WekaAgentServiceModuleModule validates Weka client cleanup
type WekaAgentServiceModuleModule struct{}

func (m *WekaAgentServiceModuleModule) Name() string {
	return "weka_client"
}

func (m *WekaAgentServiceModuleModule) FriendlyName() string {
	return "Weka Client"
}

func (m *WekaAgentServiceModuleModule) Description() string {
	return "Weka client presence and cleanup validation"
}

func (m *WekaAgentServiceModuleModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *WekaAgentServiceModuleModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Detail}}"
}

func (m *WekaAgentServiceModuleModule) ErrorTemplate() string {
	return "⚠️ {{.FriendlyName}}: {{.Detail}}"
}

func (m *WekaAgentServiceModuleModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}, clean up old Weka client: sudo apt-get remove weka-client (Ubuntu) or sudo yum remove weka-client (RHEL/CentOS)"
}

func (m *WekaAgentServiceModuleModule) Validate(podOutput string) (interface{}, error) {
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
func (m *WekaAgentServiceModuleModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}
