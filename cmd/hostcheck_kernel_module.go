package cmd

import (
	"encoding/json"
)

type KernelModule struct{}

// KernelModuleResponse implements HostCheckModuleResponse for kernel validation
type KernelModuleResponse struct {
	status        checkStatus
	KernelVersion string
	moduleName    string
	err           error
}

func (r *KernelModuleResponse) Status() checkStatus { return r.status }
func (r *KernelModuleResponse) ModuleName() string  { return r.moduleName }
func (r *KernelModuleResponse) Details() string     { return r.KernelVersion }
func (r *KernelModuleResponse) Error() error        { return r.err }
func (r *KernelModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":        r.status,
		"KernelVersion": r.KernelVersion,
		"ModuleName":    r.moduleName,
		"Error":         r.err,
	}
}

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

func (m *KernelModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &KernelModuleResponse{status: statusFail, moduleName: m.Name(), err: err}, err
	}

	status := statusPass
	if hc.KernelVersion <= "5.10" {
		status = statusWarn
	}

	return &KernelModuleResponse{
		status:        status,
		KernelVersion: hc.KernelVersion,
		moduleName:    m.Name(),
		err:           nil,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for kernel validation
func (m *KernelModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}
