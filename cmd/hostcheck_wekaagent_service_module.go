package cmd

import (
	"encoding/json"
)

// WekaAgentServiceModuleModule validates Weka client cleanup
type WekaAgentServiceModuleModule struct{}

// WekaAgentServiceModuleResponse implements HostCheckModuleResponse for Weka client validation
type WekaAgentServiceModuleResponse struct {
	status     checkStatus
	Clean      bool
	Detail     string
	moduleName string
	err        error
}

func (r *WekaAgentServiceModuleResponse) Status() checkStatus { return r.status }
func (r *WekaAgentServiceModuleResponse) ModuleName() string  { return r.moduleName }
func (r *WekaAgentServiceModuleResponse) Details() string     { return r.Detail }
func (r *WekaAgentServiceModuleResponse) Error() error        { return r.err }
func (r *WekaAgentServiceModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":     r.status,
		"Clean":      r.Clean,
		"Detail":     r.Detail,
		"ModuleName": r.moduleName,
		"Error":      r.err,
	}
}

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

func (m *WekaAgentServiceModuleModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &WekaAgentServiceModuleResponse{status: statusFail, moduleName: m.Name(), err: err}, err
	}

	status := statusPass
	detail := "clean (no Weka agent service found)"
	clean := hc.IsWekaAgentClean()
	if !clean {
		status = statusWarn
		detail = "Weka agent service exists - may interfere with installation"
	}

	return &WekaAgentServiceModuleResponse{
		status:     status,
		Clean:      clean,
		Detail:     detail,
		moduleName: m.Name(),
		err:        nil,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for Weka client validation
func (m *WekaAgentServiceModuleModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}
