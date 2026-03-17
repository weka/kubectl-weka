package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	ds "github.com/weka/kubectl-weka/pkg/device-support"
)

// NetworkInterfaceValidation represents validation data for a single network interface
type NetworkInterfaceValidation struct {
	Status      CheckStatus `json:"status"`
	Name        string      `json:"name"`
	VendorModel string      `json:"vendor_model"`
	DeviceModel string      `json:"device_model"`
	Type        string      `json:"type"`
	IPAddress   string      `json:"ip_address"`
	Speed       string      `json:"speed"`
	MTU         string      `json:"mtu"`
	Supported   string      `json:"supported"` // "DPDK", "UDP", or "-"
	Reason      string      `json:"reason"`    // Error reason if not supported
}

// String returns a human-readable summary of the validation result
func (nv *NetworkInterfaceValidation) String() string {
	return fmt.Sprintf(
		"Name: %s, Type: %s, IP: %s, Speed: %s, MTU: %s, Supported: %s, Status: %s, Reason: %s",
		nv.Name,
		nv.Type,
		nv.IPAddress,
		nv.Speed,
		nv.MTU,
		nv.Supported,
		nv.Status,
		nv.Reason,
	)
}

func (nv *NetworkInterfaceValidation) setStatus(status CheckStatus) {
	switch status {
	case statusFail:
		nv.Status = statusFail
	case statusWarn:
		if nv.Status != statusFail {
			nv.Status = statusWarn
		}
	case statusPass:
		if nv.Status != statusFail && nv.Status != statusWarn {
			nv.Status = statusPass
		}
	}
}

// NetworkInterfacesModuleData represents the validation result data
type NetworkInterfacesModuleData struct {
	TotalInterfaces int                           `json:"total_interfaces"`
	CandidateCount  int                           `json:"candidate_count"`
	DPDKSupported   int                           `json:"dpdk_supported"`
	UDPSupported    int                           `json:"udp_supported"`
	NotSupported    int                           `json:"not_supported"`
	Warnings        int                           `json:"warnings"`
	Interfaces      []*NetworkInterfaceValidation `json:"interfaces"`
}

// NetworkInterfacesModuleResponse implements HostCheckModuleResult for network validation
// Wraps NetworkInterfacesModuleData
type NetworkInterfacesModuleResponse struct {
	data       *NetworkInterfacesModuleData
	status     CheckStatus
	moduleName ModuleName
	details    string
	err        error
}

func (r *NetworkInterfacesModuleResponse) Status() CheckStatus                { return r.status }
func (r *NetworkInterfacesModuleResponse) ModuleName() ModuleName             { return r.moduleName }
func (r *NetworkInterfacesModuleResponse) Details() string                    { return r.details }
func (r *NetworkInterfacesModuleResponse) Error() error                       { return r.err }
func (r *NetworkInterfacesModuleResponse) Data() *NetworkInterfacesModuleData { return r.data }
func (r *NetworkInterfacesModuleResponse) Map() map[string]interface{} {
	m := map[string]interface{}{
		"Status":     r.status,
		"ModuleName": r.moduleName,
		"Details":    r.details,
		"Error":      r.err,
	}
	if r.data != nil {
		m["TotalInterfaces"] = r.data.TotalInterfaces
		m["CandidateCount"] = r.data.CandidateCount
		m["DPDKSupported"] = r.data.DPDKSupported
		m["UDPSupported"] = r.data.UDPSupported
		m["NotSupported"] = r.data.NotSupported
		m["Warnings"] = r.data.Warnings
		m["Interfaces"] = r.data.Interfaces
	}
	return m
}

// NetworkInterfacesModule validates network interfaces for Weka support
type NetworkInterfacesModule struct {
	data *NetworkInterfacesModuleData
}

func (m *NetworkInterfacesModule) Name() ModuleName {
	return ModuleNameNetworkInterfaces
}

func (m *NetworkInterfacesModule) FriendlyName() string {
	return "Network Interfaces Configuration"
}

func (m *NetworkInterfacesModule) Description() string {
	return "Validates network interfaces for Weka compatibility (DPDK, UDP, or unsupported)"
}

func (m *NetworkInterfacesModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	// Extract HostChecksResult from params
	var hc *HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hc); err != nil {
		return nil, fmt.Errorf("failed to parse hostcheck JSON: %v", err)
	}

	if hc == nil || len(hc.NetworkInterfaces) == 0 {
		data := &NetworkInterfacesModuleData{
			TotalInterfaces: 0,
			CandidateCount:  0,
			Interfaces:      []*NetworkInterfaceValidation{},
		}
		return &NetworkInterfacesModuleResponse{
			data:       data,
			status:     statusFail,
			moduleName: m.Name(),
			details:    "No network interfaces found",
			err:        nil,
		}, nil
	}

	// Get candidate interfaces for Weka
	candidates := hc.NetworkInterfaces.GetCandidateInterfacesForWeka()

	data := &NetworkInterfacesModuleData{
		TotalInterfaces: len(hc.NetworkInterfaces),
		CandidateCount:  len(candidates),
		Interfaces:      []*NetworkInterfaceValidation{},
	}
	m.data = data

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
		if validation.Status == statusWarn {
			data.Warnings++
		}

	}
	// Determine overall status based on support availability
	// Success if at least one interface is supported
	// Warning if some interfaces are not supported but at least one is
	// Error if no interfaces are supported
	status := statusPass
	if data.NotSupported > 0 {
		status = statusFail
	} else if data.Warnings > 0 {
		status = statusWarn
	}

	return &NetworkInterfacesModuleResponse{
		data:       data,
		status:     status,
		moduleName: m.Name(),
		details:    "Network interface validation complete",
		err:        nil,
	}, nil
}

func (m *NetworkInterfacesModule) ValidateWithParams(podOutput string, params map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}

// validateInterface validates a single network interface
func (m *NetworkInterfacesModule) validateInterface(iface *NetworkInterface) *NetworkInterfaceValidation {
	reasons := []string{}
	validation := &NetworkInterfaceValidation{
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
		validation.setStatus(statusFail)
		reasons = append(reasons, "Unknown device model")
	}

	if validation.IPAddress == "" {
		validation.IPAddress = "No IP"
		validation.setStatus(statusFail)
		reasons = append(reasons, "No IP address")
	}

	if validation.Speed == "" {
		validation.setStatus(statusFail)
		reasons = append(reasons, "Speed is not reported, disconnected?")
	}

	if iface.MTU > 0 {
		validation.MTU = fmt.Sprintf("%d", iface.MTU)
	}
	if iface.MTU < 8000 && iface.IsEthernet() {
		validation.setStatus(statusWarn)
		reasons = append(reasons, "ETH MTU too small")
	}

	if iface.MTU < 4000 && iface.IsInfiniBand() {
		validation.setStatus(statusWarn)
		reasons = append(reasons, "IB subnet MTU too small")
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
	if validation.DeviceModel == "Unknown" {
		validation.setStatus(statusFail)
		reasons = append(reasons, "Unknown device model")
	}

	// Handle bond interface type
	if iface.IsBond() {
		slaves := iface.GetSlaves()
		validation.Type = fmt.Sprintf("bond (%d slaves)", len(slaves))
		if len(slaves) != 2 {
			validation.setStatus(statusFail)
			reasons = append(reasons, "Bond must have 2 interfaces")
		}
		if !iface.IsLACP() {
			validation.setStatus(statusFail)
			reasons = append(reasons, "Bond mode is not LACP")
		}
	}

	// Try UDP first as a bare minimum
	udpSupported, udpErr := iface.IsSupportedByWekaInUdpMode()
	if udpSupported {
		validation.Supported = "UDP"
	} else {
		if udpErr != nil {
			reasons = append(reasons, udpErr.Error())
		} else {
			reasons = append(reasons, "Not supported")
		}
		validation.setStatus(statusFail)
		validation.Reason = strings.Join(reasons, "\n")
		return validation
	}

	// if UDP is supported, check if DPDK is supported for optimal performance
	dpdkSupported, dpdkErr := iface.IsSupportedByWekaDpdk()
	if dpdkSupported {
		validation.Supported = "DPDK"
	} else {
		if dpdkErr != nil {
			reasons = append(reasons, dpdkErr.Error())
		} else {
			reasons = append(reasons, "No DPDK support")
		}
		validation.setStatus(statusWarn)
	}
	validation.Reason = strings.Join(reasons, "\n")
	validation.setStatus(statusPass) // will set only if empty
	return validation
}

func (m *NetworkInterfacesModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Network interfaces validation passed\n" + FormatNetworkInterfacesTable(m.data, nil)
}

func (m *NetworkInterfacesModule) WarningTemplate() string {
	return "⚠️ WARNING: {{.FriendlyName}}: Some network interfaces are not supported for optimal Weka performance\n" + FormatNetworkInterfacesTable(m.data, nil)
}

func (m *NetworkInterfacesModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: {{.Issue}} \n" + FormatNetworkInterfacesTable(m.data, nil)
}

func (m *NetworkInterfacesModule) SuggestedResolutionTemplate() string {
	return "Review network interface compatibility. Refer to Weka documentation for supported network devices."
}

// FormatNetworkInterfacesTable formats the network interfaces data as a pretty table
// This replaces the old printCandidateNetworkInterfacesToOutput function
func FormatNetworkInterfacesTable(data *NetworkInterfacesModuleData, visibleColumns []string) string {
	if data == nil || len(data.Interfaces) == 0 {
		return ""
	}

	// Define all columns
	allColumns := []TableColumn{
		{Name: "Status", VisibleInWide: false},
		{Name: "Name", VisibleInWide: false},
		{Name: "Vendor:Model", VisibleInWide: false},
		{Name: "Device Model", VisibleInWide: false},
		{Name: "Type", VisibleInWide: false},
		{Name: "IP Address/CIDR", VisibleInWide: false},
		{Name: "Speed", VisibleInWide: false},
		{Name: "MTU", VisibleInWide: false},
		{Name: "Supported", VisibleInWide: false},
		{Name: "Reason", VisibleInWide: false},
	}

	// Select columns to show
	var columns []TableColumn
	if len(visibleColumns) > 0 {
		colSet := map[string]struct{}{}
		for _, name := range visibleColumns {
			colSet[name] = struct{}{}
		}
		for _, col := range allColumns {
			if _, ok := colSet[col.Name]; ok {
				columns = append(columns, col)
			}
		}
	} else {
		columns = allColumns
	}

	// Build rows
	var rows []TableRow
	for _, iface := range data.Interfaces {
		row := TableRow{Values: map[string]interface{}{
			"Status":          iface.Status,
			"Name":            iface.Name,
			"Vendor:Model":    iface.VendorModel,
			"Device Model":    iface.DeviceModel,
			"Type":            iface.Type,
			"IP Address/CIDR": iface.IPAddress,
			"Speed":           iface.Speed,
			"MTU":             iface.MTU,
			"Supported":       iface.Supported,
			"Reason":          iface.Reason,
		}}
		rows = append(rows, row)
	}

	// Render using TablePrinter
	printer := &TablePrinter{}
	printer.SetOptions(PrinterOptions{
		ShowHeader: true,
		TableStyle: TableStyleRoundedBox,
	})
	var sb strings.Builder
	_ = printer.Print(columns, rows, &sb)
	return sb.String()
}
