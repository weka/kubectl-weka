package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OSModule validates OS compatibility

// OsModuleResponse implements HostCheckModuleResponse for OS validation
type OsModuleResponse struct {
	status     CheckStatus
	IsRHCOS    bool
	OSRelease  string
	moduleName ModuleName
	err        error
}

func (r *OsModuleResponse) Status() CheckStatus    { return r.status }
func (r *OsModuleResponse) ModuleName() ModuleName { return r.moduleName }
func (r *OsModuleResponse) Details() string        { return r.OSRelease }
func (r *OsModuleResponse) Error() error           { return r.err }
func (r *OsModuleResponse) Map() map[string]interface{} {
	return map[string]interface{}{
		"Status":     r.status,
		"IsRHCOS":    r.IsRHCOS,
		"OSRelease":  r.OSRelease,
		"ModuleName": r.moduleName,
		"Error":      r.err,
	}
}

// OSModule validates OS compatibility
type OSModule struct{}

func (m *OSModule) Name() ModuleName {
	return ModuleNameOs
}

func (m *OSModule) FriendlyName() string {
	return "Operating System"
}

func (m *OSModule) Description() string {
	return "OS detection and validation (RHCOS/CoreOS/Standard Linux)"
}

func (m *OSModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: {{.OSRelease}}"
}

func (m *OSModule) WarningTemplate() string {
	return "" // No warning state for OS module
}

func (m *OSModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}} unsupported: {{.OSRelease}}"
}

func (m *OSModule) SuggestedResolutionTemplate() string {
	return "Please ensure node {{.NodeName}} is running a supported Linux distribution (Ubuntu, RHEL/CentOS, RHCOS, etc.)"
}

func (m *OSModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return &OsModuleResponse{status: statusFail, moduleName: m.Name(), err: err}, err
	}

	name := ""
	versionID := ""
	prettyName := ""
	parts := strings.Fields(hc.OSRelease)
	for _, part := range parts {
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				value := kv[1]
				value = strings.Trim(value, `"`)
				switch key {
				case "NAME":
					name = value
				case "VERSION_ID":
					versionID = value
				case "PRETTY_NAME":
					prettyName = value
				}
			}
		}
	}
	osDisplay := ""
	if name != "" && versionID != "" {
		osDisplay = fmt.Sprintf("%s %s", name, versionID)
	} else if name != "" {
		osDisplay = name
	} else if prettyName != "" {
		if idx := strings.Index(prettyName, "("); idx > 0 {
			osDisplay = strings.TrimSpace(prettyName[:idx])
		} else {
			osDisplay = prettyName
		}
	} else {
		osDisplay = "Unknown OS"
	}

	// Check if OS is supported by Weka
	// Weka requires Linux-based distributions
	status := statusFail // Default to fail unless we can confirm it's supported

	supportedPatterns := map[string]bool{
		"ubuntu":    true,
		"rhel":      true,
		"centos":    true,
		"rhcos":     true,
		"rocky":     true,
		"almalinux": true,
		"fedora":    true,
		"debian":    true,
		"opensuse":  true,
		"sles":      true,
		"amzn":      true, // Amazon Linux
	}

	// Check against known supported OS names
	lowerName := strings.ToLower(name)
	lowerPretty := strings.ToLower(prettyName)
	lowerRelease := strings.ToLower(hc.OSRelease)

	for pattern := range supportedPatterns {
		if strings.Contains(lowerName, pattern) ||
			strings.Contains(lowerPretty, pattern) ||
			strings.Contains(lowerRelease, pattern) {
			status = statusPass
			break
		}
	}

	// RHCOS detection
	if hc.IsRHCOS {
		status = statusPass
	}

	return &OsModuleResponse{
		status:     status,
		IsRHCOS:    hc.IsRHCOS,
		OSRelease:  osDisplay,
		moduleName: m.Name(),
		err:        nil,
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for OS validation
func (m *OSModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}

// TODO: need to add actual FAR support for OS version...
