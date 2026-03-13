package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OSModule validates OS compatibility
type OSModule struct{}

func (m *OSModule) Name() string {
	return "os"
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

func (m *OSModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	// Parse OSRelease to extract NAME and VERSION_ID
	// The OSRelease is a concatenated string from /etc/os-release with newlines converted to spaces
	// e.g., "PRETTY_NAME=\"Ubuntu 22.04.5 LTS\" NAME=\"Ubuntu\" VERSION_ID=\"22.04\" ..."

	name := ""
	versionID := ""
	prettyName := ""

	// Split by space to get individual key=value pairs
	// But we need to be careful with quoted values that might contain spaces
	parts := strings.Fields(hc.OSRelease)
	for _, part := range parts {
		// Each part looks like KEY=VALUE or KEY="VALUE"
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				value := kv[1]

				// Remove surrounding quotes
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

	// Build osDisplay with fallback chain
	osDisplay := ""
	if name != "" && versionID != "" {
		osDisplay = fmt.Sprintf("%s %s", name, versionID)
	} else if name != "" {
		osDisplay = name
	} else if prettyName != "" {
		// Extract just the OS name from PRETTY_NAME (e.g., "Ubuntu 22.04.5 LTS" -> best effort)
		// Remove the distribution name in parentheses if present
		if idx := strings.Index(prettyName, "("); idx > 0 {
			osDisplay = strings.TrimSpace(prettyName[:idx])
		} else {
			osDisplay = prettyName
		}
	} else {
		osDisplay = "Unknown OS"
	}

	return map[string]interface{}{
		"IsRHCOS":   hc.IsRHCOS,
		"OSRelease": osDisplay,
		"Status":    "success",
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for OS validation
func (m *OSModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}
