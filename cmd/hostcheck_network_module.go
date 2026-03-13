package cmd

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	ds "github.com/weka/kubectl-weka/pkg/device-support"
)

// NetworkInterfaceValidation represents validation data for a single network interface
type NetworkInterfaceValidation struct {
	Name        string `json:"name"`
	VendorModel string `json:"vendor_model"`
	DeviceModel string `json:"device_model"`
	Type        string `json:"type"`
	IPAddress   string `json:"ip_address"`
	Speed       string `json:"speed"`
	MTU         string `json:"mtu"`
	Supported   string `json:"supported"` // "DPDK", "UDP", or "-"
	Reason      string `json:"reason"`    // Error reason if not supported
}

// NetworkInterfacesModuleData represents the validation result data
type NetworkInterfacesModuleData struct {
	TotalInterfaces int                          `json:"total_interfaces"`
	CandidateCount  int                          `json:"candidate_count"`
	DPDKSupported   int                          `json:"dpdk_supported"`
	UDPSupported    int                          `json:"udp_supported"`
	NotSupported    int                          `json:"not_supported"`
	Interfaces      []NetworkInterfaceValidation `json:"interfaces"`
}

// NetworkInterfacesModule validates network interfaces for Weka support
type NetworkInterfacesModule struct {
	data *NetworkInterfacesModuleData
}

func (m *NetworkInterfacesModule) Name() string {
	return "network_interfaces"
}

func (m *NetworkInterfacesModule) Description() string {
	return "Validates network interfaces for Weka compatibility (DPDK, UDP, or unsupported)"
}

func (m *NetworkInterfacesModule) Validate(podOutput string) (interface{}, error) {
	return m.ValidateWithParams(podOutput, nil)
}

func (m *NetworkInterfacesModule) ValidateWithParams(podOutput string, params map[string]interface{}) (interface{}, error) {
	// Extract HostChecksResult from params
	var hc *HostChecksResult

	// Get hostChecksMap and nodeName from params
	if params != nil {
		// Try to get from direct hostChecksResult param (if provided)
		if result, ok := params["hostChecksResult"]; ok {
			hc, _ = result.(*HostChecksResult)
		}

		// If not found, try to get from hostChecksMap using nodeName
		if hc == nil {
			if hostChecksMap, ok := params["hostChecksMap"]; ok {
				hcMap, _ := hostChecksMap.(map[string]*HostChecksResult)
				if nodeName, ok := params["nodeName"]; ok {
					nodeNameStr, _ := nodeName.(string)
					if nodeNameStr != "" && len(hcMap) > 0 {
						hc = hcMap[nodeNameStr]
					}
				}
			}
		}
	}

	if hc == nil || len(hc.NetworkInterfaces) == 0 {
		return &NetworkInterfacesModuleData{
			TotalInterfaces: 0,
			CandidateCount:  0,
			Interfaces:      []NetworkInterfaceValidation{},
		}, nil
	}

	// Get candidate interfaces for Weka
	candidates := hc.NetworkInterfaces.GetCandidateInterfacesForWeka()

	data := &NetworkInterfacesModuleData{
		TotalInterfaces: len(hc.NetworkInterfaces),
		CandidateCount:  len(candidates),
		Interfaces:      []NetworkInterfaceValidation{},
	}

	// Validate each candidate interface
	for _, iface := range candidates {
		validation := m.validateInterface(iface)
		data.Interfaces = append(data.Interfaces, validation)

		// Count support types
		switch validation.Supported {
		case "DPDK":
			data.DPDKSupported++
		case "UDP":
			data.UDPSupported++
		case "-":
			data.NotSupported++
		}
	}

	// Determine overall status based on support availability
	// Success if at least one interface is supported
	// Warning if some interfaces are not supported but at least one is
	// Error if no interfaces are supported
	if data.DPDKSupported+data.UDPSupported == 0 {
		return data, fmt.Errorf("no network interfaces are supported for Weka")
	}
	m.data = data
	return data, nil
}

// validateInterface validates a single network interface
func (m *NetworkInterfacesModule) validateInterface(iface *NetworkInterface) NetworkInterfaceValidation {
	validation := NetworkInterfaceValidation{
		Name:        iface.Name,
		VendorModel: iface.VendorModel,
		Type:        iface.Type,
		IPAddress:   iface.IP,
		Speed:       iface.EffectiveSpeed,
		MTU:         "Unknown",
		Supported:   "-",
		Reason:      "",
	}

	// Set defaults
	if validation.VendorModel == "" {
		validation.VendorModel = "Unknown"
	}

	if validation.IPAddress == "" {
		validation.IPAddress = "No IP"
	}

	if validation.Speed == "" {
		validation.Speed = iface.MaxSpeed
	}

	if validation.Speed == "" {
		validation.Speed = "Unknown"
	}

	if iface.MTU > 0 {
		validation.MTU = fmt.Sprintf("%d", iface.MTU)
	}

	// Get device model name
	if iface.VendorModel != "" {
		devInfo := ds.GetNICInfo(iface.VendorModel)
		if devInfo != nil && devInfo.Model != "" {
			validation.DeviceModel = devInfo.Model
		} else {
			validation.DeviceModel = "Unknown"
		}
	} else {
		validation.DeviceModel = "Unknown"
	}

	// Handle bond interface type
	if iface.IsBond() {
		slaves := iface.GetSlaves()
		validation.Type = fmt.Sprintf("bond (%d slaves)", len(slaves))
	}

	// Determine supported modes and reason
	dpdkSupported, dpdkErr := iface.IsSupportedByWekaDpdk()
	if dpdkSupported {
		validation.Supported = "DPDK"
		return validation
	}

	// Try UDP as fallback
	udpSupported, udpErr := iface.IsSupportedByWekaInUdpMode()
	if udpSupported {
		validation.Supported = "UDP"
		return validation
	}

	// Not supported - set reason from error
	if dpdkErr != nil {
		validation.Reason = dpdkErr.Error()
	} else if udpErr != nil {
		validation.Reason = udpErr.Error()
	} else {
		validation.Reason = "Not supported"
	}

	return validation
}

func (m *NetworkInterfacesModule) SuccessTemplate() string {
	return "✅ OK: Network interfaces validation passed\n" + indentText(FormatNetworkInterfacesTable(m.data), 5)
}

func (m *NetworkInterfacesModule) WarningTemplate() string {
	return "⚠️ WARNING: Some network interfaces are not supported for optimal Weka performance\n" + indentText(FormatNetworkInterfacesTable(m.data), 5)
}

func (m *NetworkInterfacesModule) ErrorTemplate() string {
	return "❌ ERROR: {{.Issue}} \n" + indentText(FormatNetworkInterfacesTable(m.data), 5)
}

func (m *NetworkInterfacesModule) SuggestedResolutionTemplate() string {
	return "Review network interface compatibility. Refer to Weka documentation for supported network devices."
}

// FormatNetworkInterfacesTable formats the network interfaces data as a pretty table
// This replaces the old printCandidateNetworkInterfacesToOutput function
func FormatNetworkInterfacesTable(data *NetworkInterfacesModuleData) string {
	if data == nil || len(data.Interfaces) == 0 {
		return ""
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)

	// Add header
	t.AppendHeader(table.Row{
		"Name",
		"Vendor:Model",
		"Device Model",
		"Type",
		"IP Address/CIDR",
		"Speed",
		"MTU",
		"Supported",
		"Reason",
	})

	// Add rows for each interface
	for _, iface := range data.Interfaces {
		t.AppendRow(table.Row{
			iface.Name,
			iface.VendorModel,
			iface.DeviceModel,
			iface.Type,
			iface.IPAddress,
			iface.Speed,
			iface.MTU,
			iface.Supported,
			iface.Reason,
		})
	}

	// Render and indent
	return t.Render()
}
