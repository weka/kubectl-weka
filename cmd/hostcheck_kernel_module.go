package cmd

import (
	"encoding/json"
	"fmt"
)

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
