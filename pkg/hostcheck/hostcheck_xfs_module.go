package hostcheck

import (
	"encoding/json"
	"github.com/weka/kubectl-weka/pkg/types"
)

// XFSModule validates XFS tools installation
type XFSModule struct{}

// XFSModuleResponse implements HostCheckModuleResponse for XFS validation
type XFSModuleResponse struct {
	status     types.CheckStatus
	Found      bool
	Detail     string
	moduleName ModuleName
	err        error
}

func (r *XFSModuleResponse) Status() types.CheckStatus { return r.status }
func (r *XFSModuleResponse) ModuleName() ModuleName    { return r.moduleName }
func (r *XFSModuleResponse) Details() string           { return r.Detail }
func (r *XFSModuleResponse) Error() error              { return r.err }
func (r *XFSModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":     r.status,
		"Found":      r.Found,
		"Detail":     r.Detail,
		"ModuleName": r.moduleName,
		"Error":      r.err,
	}
}

func (m *XFSModule) Name() ModuleName {
	return ModuleNameXfs
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

func (m *XFSModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &XFSModuleResponse{status: types.StatusFail, moduleName: m.Name(), err: err}, err
	}

	status := types.StatusPass
	detail := "found"
	found := hc.HasXFS()
	if !found {
		status = types.StatusFail
		detail = "not found"
	}

	return &XFSModuleResponse{
		status:     status,
		Found:      found,
		Detail:     detail,
		moduleName: m.Name(),
		err:        nil,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for XFS validation
func (m *XFSModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}
