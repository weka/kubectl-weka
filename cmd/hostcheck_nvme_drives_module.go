package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NVMeDrivesModule validates NVMe drive availability and status
type NVMeDrivesModule struct{}

func (m *NVMeDrivesModule) Name() string {
	return "nvme_drives"
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

func (m *NVMeDrivesModule) Validate(podOutput string) (interface{}, error) {
	var hc HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	// Build detail string
	// Count only valid NVMe drives with serial numbers (exclude partitions and drives without serial)
	validDrives := 0
	for _, drive := range hc.NVMeDrives {
		// Only count drives with serial numbers (not empty), and exclude partitions (device names like nvme0n1p1)
		if drive.SerialNumber != "" && !strings.Contains(drive.DeviceName, "p") {
			validDrives++
		}
	}

	detail := ""
	if !hc.HasNVMeDrives() {
		detail = "No NVMe drives detected"
	} else {
		detail = fmt.Sprintf("%d drive(s) available", validDrives)
	}

	return map[string]interface{}{
		"Status":     "success",
		"Detail":     detail,
		"Drives":     hc.NVMeDrives,
		"DriveCount": validDrives,
		"HasDrives":  hc.HasNVMeDrives(),
	}, nil
}

// ValidateWithParams implements HostCheckModule - params not used for NVMe drives validation
func (m *NVMeDrivesModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	return m.Validate(podOutput)
}
