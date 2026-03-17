package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NVMeDrivesModule validates NVMe drive availability and status
type NVMeDrivesModule struct{}

// NVMeDrivesModuleResponse implements HostCheckModuleResponse for NVMe validation
type NVMeDrivesModuleResponse struct {
	status     CheckStatus
	Detail     string
	Drives     []NvmeDrive
	DriveCount int
	HasDrives  bool
	moduleName ModuleName
	err        error
}

func (r *NVMeDrivesModuleResponse) Status() CheckStatus    { return r.status }
func (r *NVMeDrivesModuleResponse) ModuleName() ModuleName { return r.moduleName }
func (r *NVMeDrivesModuleResponse) Details() string        { return r.Detail }
func (r *NVMeDrivesModuleResponse) Error() error           { return r.err }
func (r *NVMeDrivesModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":     r.status,
		"Detail":     r.Detail,
		"Drives":     r.Drives,
		"DriveCount": r.DriveCount,
		"HasDrives":  r.HasDrives,
		"ModuleName": r.moduleName,
		"Error":      r.err,
	}
}

func (m *NVMeDrivesModule) Name() ModuleName {
	return ModuleNameNVMeDrives
}

func (m *NVMeDrivesModule) FriendlyName() string {
	return "NVMe Drives"
}

func (m *NVMeDrivesModule) Description() string {
	return "NVMe drive discovery and availability check"
}

func (m *NVMeDrivesModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.Detail}}"
}

func (m *NVMeDrivesModule) WarningTemplate() string {
	return "⚠️ WARN: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NVMeDrivesModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}}"
}

func (m *NVMeDrivesModule) SuggestedResolutionTemplate() string {
	return "On node {{.NodeName}}: {{.Resolution}}"
}

func (m *NVMeDrivesModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &NVMeDrivesModuleResponse{status: statusFail, moduleName: m.Name(), err: err}, err
	}

	validDrives := 0
	for _, drive := range hc.NVMeDrives {
		if drive.SerialNumber != "" && !strings.Contains(drive.DeviceName, "p") {
			validDrives++
		}
	}

	detail := ""
	status := statusPass
	if !hc.HasNVMeDrives() {
		detail = "No NVMe drives detected"
		status = statusFail
	} else {
		detail = fmt.Sprintf("%d drive(s) available", validDrives)
	}

	return &NVMeDrivesModuleResponse{
		status:     status,
		Detail:     detail,
		Drives:     hc.NVMeDrives,
		DriveCount: validDrives,
		HasDrives:  hc.HasNVMeDrives(),
		moduleName: m.Name(),
		err:        nil,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for NVMe drives validation
func (m *NVMeDrivesModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}
