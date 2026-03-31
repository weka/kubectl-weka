package hostcheck

import (
	"encoding/json"
	"fmt"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/kubectl-weka/pkg/types"
	"net"
	"strings"
)

// SourceBasedRoutingModuleData represents source-based routing validation data
type SourceBasedRoutingModuleData struct {
	MultiInterfaceFound bool                        `json:"multi_interface_found"`
	SameSubnetFound     bool                        `json:"same_subnet_found"`
	InterfaceGroups     []SubnetInterfaceGroup      `json:"interface_groups"`
	SourceRoutingFound  bool                        `json:"source_routing_found"`
	SBRRules            []SourceBasedRoutingRule    `json:"sbr_rules"`
	ValidationWarnings  []SourceBasedRoutingWarning `json:"validation_warnings"`
}

// SubnetInterfaceGroup represents a group of interfaces on the same subnet
type SubnetInterfaceGroup struct {
	Subnet      string   `json:"subnet"`
	Interfaces  []string `json:"interfaces"`
	DPDKSupport string   `json:"dpdk_support"` // "single-nic", "nic-per-process", "udp-only", or "mixed"
	HasSBRRules bool     `json:"has_sbr_rules"`
	RequiresSBR bool     `json:"requires_sbr"`
}

// SourceBasedRoutingRule represents a source-based routing rule
type SourceBasedRoutingRule struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Table       string `json:"table,omitempty"`
	Device      string `json:"device,omitempty"`
}

// SourceBasedRoutingWarning represents a warning about source-based routing
type SourceBasedRoutingWarning struct {
	Severity string `json:"severity"` // "error", "warning"
	Message  string `json:"message"`
}

// SourceBasedRoutingModuleResponse implements HostCheckModuleResponse for source-based routing validation
type SourceBasedRoutingModuleResponse struct {
	data       *SourceBasedRoutingModuleData
	status     types.CheckStatus
	moduleName ModuleName
	detail     string
	err        error
}

func (r *SourceBasedRoutingModuleResponse) Status() types.CheckStatus { return r.status }
func (r *SourceBasedRoutingModuleResponse) ModuleName() ModuleName    { return r.moduleName }
func (r *SourceBasedRoutingModuleResponse) Details() string           { return r.detail }
func (r *SourceBasedRoutingModuleResponse) Error() error              { return r.err }
func (r *SourceBasedRoutingModuleResponse) Map() map[string]interface{} {
	m := map[string]interface{}{
		"Status":     r.status,
		"ModuleName": r.moduleName,
		"Details":    r.detail,
		"Error":      r.err,
	}
	if r.data != nil {
		m["MultiInterfaceFound"] = r.data.MultiInterfaceFound
		m["SameSubnetFound"] = r.data.SameSubnetFound
		m["InterfaceGroups"] = r.data.InterfaceGroups
		m["SourceRoutingFound"] = r.data.SourceRoutingFound
		m["SBRRules"] = r.data.SBRRules
		m["ValidationWarnings"] = r.data.ValidationWarnings
	}
	return m
}

// SourceBasedRoutingModule validates source-based routing configuration for multi-interface setups
type SourceBasedRoutingModule struct {
	data *SourceBasedRoutingModuleData
}

func (m *SourceBasedRoutingModule) Name() ModuleName {
	return ModuleNameSourceBasedRouting
}

func (m *SourceBasedRoutingModule) FriendlyName() string {
	return "Source-Based Routing Configuration"
}

func (m *SourceBasedRoutingModule) Description() string {
	return "Source-based routing configuration for multi-interface setups"
}

func (m *SourceBasedRoutingModule) Validate(podOutput string) (HostCheckModuleResponse, error) {
	var hostCheck HostChecksResult
	if err := json.Unmarshal([]byte(podOutput), &hostCheck); err != nil {
		return &SourceBasedRoutingModuleResponse{
			status:     types.StatusFail,
			moduleName: m.Name(),
			detail:     fmt.Sprintf("Failed to parse hostcheck data: %v", err),
			err:        err,
		}, err
	}

	return m.validateSBR(&hostCheck)
}

func (m *SourceBasedRoutingModule) ValidateWithParams(podOutput string, _ map[string]interface{}) (HostCheckModuleResponse, error) {
	return m.Validate(podOutput)
}

func (m *SourceBasedRoutingModule) validateSBR(hostCheck *HostChecksResult) (HostCheckModuleResponse, error) {
	data := &SourceBasedRoutingModuleData{
		InterfaceGroups:    []SubnetInterfaceGroup{},
		ValidationWarnings: []SourceBasedRoutingWarning{},
	}
	m.data = data

	if hostCheck == nil || len(hostCheck.NetworkInterfaces) == 0 {
		return &SourceBasedRoutingModuleResponse{
			status:     types.StatusPass,
			moduleName: m.Name(),
			detail:     "No network interfaces to validate",
			data:       data,
		}, nil
	}

	// Filter WEKA-capable interfaces
	wekaInterfaces := m.filterWekaInterfaces(hostCheck.NetworkInterfaces)
	if len(wekaInterfaces) == 0 {
		return &SourceBasedRoutingModuleResponse{
			status:     types.StatusPass,
			moduleName: m.Name(),
			detail:     "No WEKA-capable interfaces found",
			data:       data,
		}, nil
	}

	// Group interfaces by subnet
	subnetGroups := m.groupInterfacesBySubnet(wekaInterfaces)
	data.InterfaceGroups = subnetGroups

	// Check if we have multiple interfaces on the same subnet
	if len(subnetGroups) == 0 || !m.hasMultipleInterfacesPerSubnet(subnetGroups) {
		data.MultiInterfaceFound = false
		return &SourceBasedRoutingModuleResponse{
			status:     types.StatusPass,
			moduleName: m.Name(),
			detail:     "Single interface per subnet",
			data:       data,
		}, nil
	}

	data.MultiInterfaceFound = true
	data.SameSubnetFound = true

	// Check for source-based routing rules
	m.validateSourceBasedRouting(hostCheck.NetworkInterfaces, data, hostCheck)

	// Build result
	return m.buildResult(data)
}

func (m *SourceBasedRoutingModule) filterWekaInterfaces(interfaces NetworkInterfaces) []*NetworkInterface {
	var wekaInterfaces []*NetworkInterface
	for _, iface := range interfaces {
		if iface.IsBondSlave {
			continue
		}
		if !iface.IsNetworkAdapter() && !iface.IsLogicalInterface() {
			continue
		}
		if iface.IP == "" {
			continue
		}
		wekaInterfaces = append(wekaInterfaces, iface)
	}
	return wekaInterfaces
}

func (m *SourceBasedRoutingModule) groupInterfacesBySubnet(interfaces []*NetworkInterface) []SubnetInterfaceGroup {
	subnetMap := make(map[string][]string)
	dpdkMap := make(map[string]string)

	for _, iface := range interfaces {
		if iface.IP == "" {
			continue
		}

		subnet := m.getSubnet(iface.IP)
		if subnet == "" {
			continue
		}

		subnetMap[subnet] = append(subnetMap[subnet], iface.Name)
		if dpdkMap[subnet] == "" {
			dpdkMap[subnet] = m.getDPDKSupport(iface)
		}
	}

	var groups []SubnetInterfaceGroup
	for subnet, interfaceNames := range subnetMap {
		if len(interfaceNames) > 1 {
			groups = append(groups, SubnetInterfaceGroup{
				Subnet:      subnet,
				Interfaces:  interfaceNames,
				DPDKSupport: dpdkMap[subnet],
				RequiresSBR: m.requiresSBR(dpdkMap[subnet]),
				HasSBRRules: false,
			})
		}
	}

	return groups
}

func (m *SourceBasedRoutingModule) getSubnet(cidr string) string {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}
	return ipNet.String()
}

func (m *SourceBasedRoutingModule) getDPDKSupport(_ *NetworkInterface) string {
	return "single-nic"
}

func (m *SourceBasedRoutingModule) requiresSBR(dpdkSupport string) bool {
	return dpdkSupport == "single-nic" || dpdkSupport == "udp-only"
}

func (m *SourceBasedRoutingModule) hasMultipleInterfacesPerSubnet(groups []SubnetInterfaceGroup) bool {
	for _, group := range groups {
		if len(group.Interfaces) > 1 {
			return true
		}
	}
	return false
}

func (m *SourceBasedRoutingModule) validateSourceBasedRouting(interfaces NetworkInterfaces, data *SourceBasedRoutingModuleData, hostCheck *HostChecksResult) {
	// If no routing configuration available, skip SBR validation
	if hostCheck.NetworkNamespaceRouting == nil || len(hostCheck.NetworkNamespaceRouting.RoutingRules) == 0 {
		// No SBR rules at all
		for i := range data.InterfaceGroups {
			data.InterfaceGroups[i].HasSBRRules = false
		}
		return
	}

	routing := hostCheck.NetworkNamespaceRouting

	// Map source IPs to routing rules (from RoutingRules)
	sourceToRule := make(map[string][]*RoutingRule)
	for _, rule := range routing.RoutingRules {
		if rule.Condition != "" {
			// Extract source IP from condition like "from 10.0.0.10"
			sourceToRule[rule.Condition] = append(sourceToRule[rule.Condition], rule)
		}
	}

	// Map source IPs to routing tables
	sourceToTable := make(map[string]*RoutingTableInfo)
	for _, table := range routing.RoutingTables {
		if table == nil {
			continue
		}
		for _, route := range table.Routes {
			if route.Source != "" {
				sourceToTable[route.Source] = table
			}
		}
	}

	// Check if each interface in multi-interface subnets has SBR rules
	for i, group := range data.InterfaceGroups {
		var hasSBRRule bool
		var hasSBRTable bool
		allInterfacesHaveSBR := true
		for _, ifaceName := range group.Interfaces {
			hasSBRRule = false
			hasSBRTable = false
			// Find the interface IP
			var interfaceIP string
			for j := range interfaces {
				if interfaces[j].Name == ifaceName && interfaces[j].IP != "" {
					// Extract just the IP from CIDR (e.g., "10.0.0.10/24" -> "10.0.0.10")
					interfaceIP = strings.Split(interfaces[j].IP, "/")[0]
					break
				}
			}

			if interfaceIP == "" {
				continue
			}

			// Check if there's a routing rule for this interface IP
			conditionKey := "from " + interfaceIP
			if rules, ok := sourceToRule[conditionKey]; ok && len(rules) > 0 {
				hasSBRRule = true
				for _, rule := range rules {
					// Look up the routing table for this rule
					if rule.Table != "" {
						if table, ok := sourceToTable[interfaceIP]; ok {
							// Get routes from this table
							for _, route := range table.Routes {
								data.SBRRules = append(data.SBRRules, SourceBasedRoutingRule{
									Source:      interfaceIP,
									Destination: route.Destination,
									Table:       rule.Table,
									Device:      ifaceName,
								})
							}
							hasSBRTable = true
						} else {
							hasSBRTable = false
						}
					}
				}
			} else {
				hasSBRRule = false
			}

			if !hasSBRTable || !hasSBRRule {
				allInterfacesHaveSBR = false
				if !hasSBRTable {
					data.ValidationWarnings = append(data.ValidationWarnings, SourceBasedRoutingWarning{
						Severity: "error",
						Message:  fmt.Sprintf("Interface %s (IP: %s) has no associated routing table", ifaceName, interfaceIP),
					})
				}
				if !hasSBRRule {
					data.ValidationWarnings = append(data.ValidationWarnings, SourceBasedRoutingWarning{
						Severity: "error",
						Message:  fmt.Sprintf("Interface %s (IP: %s) has no routing rules", ifaceName, interfaceIP),
					})
				}

			}
		}

		data.InterfaceGroups[i].HasSBRRules = hasSBRRule
		if allInterfacesHaveSBR {
			data.SourceRoutingFound = true
		}
	}
}

func (m *SourceBasedRoutingModule) buildResult(data *SourceBasedRoutingModuleData) (HostCheckModuleResponse, error) {
	if !data.MultiInterfaceFound || !data.SameSubnetFound {
		return &SourceBasedRoutingModuleResponse{
			status:     types.StatusPass,
			moduleName: m.Name(),
			detail:     "Single interface per subnet",
			data:       data,
		}, nil
	}

	var errorGroupsNeedingSBR []string
	var warningGroupsNeedingSBR []string

	for _, group := range data.InterfaceGroups {
		if !group.HasSBRRules && group.RequiresSBR {
			if group.DPDKSupport == "nic-per-process" {
				warningGroupsNeedingSBR = append(warningGroupsNeedingSBR, fmt.Sprintf("%s (%s)", group.Subnet, strings.Join(group.Interfaces, ",")))
			} else {
				errorGroupsNeedingSBR = append(errorGroupsNeedingSBR, fmt.Sprintf("%s (%s)", group.Subnet, strings.Join(group.Interfaces, ",")))
			}
		}
	}

	if len(errorGroupsNeedingSBR) > 0 {
		data.ValidationWarnings = append(data.ValidationWarnings, SourceBasedRoutingWarning{
			Severity: "error",
			Message:  fmt.Sprintf("Source-based routing required for subnets: %s", strings.Join(errorGroupsNeedingSBR, "; ")),
		})
		return &SourceBasedRoutingModuleResponse{
			status:     types.StatusFail,
			moduleName: m.Name(),
			detail:     fmt.Sprintf("SBR required for %s", strings.Join(errorGroupsNeedingSBR, ", ")),
			data:       data,
		}, nil
	}

	if len(warningGroupsNeedingSBR) > 0 {
		data.ValidationWarnings = append(data.ValidationWarnings, SourceBasedRoutingWarning{
			Severity: "warning",
			Message:  fmt.Sprintf("Source-based routing recommended for subnets: %s", strings.Join(warningGroupsNeedingSBR, "; ")),
		})
		return &SourceBasedRoutingModuleResponse{
			status:     types.StatusWarn,
			moduleName: m.Name(),
			detail:     fmt.Sprintf("SBR recommended for %s", strings.Join(warningGroupsNeedingSBR, ", ")),
			data:       data,
		}, nil
	}

	return &SourceBasedRoutingModuleResponse{
		status:     types.StatusPass,
		moduleName: m.Name(),
		detail:     "Source-based routing properly configured",
		data:       data,
	}, nil
}

func (m *SourceBasedRoutingModule) SuccessTemplate() string {
	return "✅ OK:  {{.FriendlyName}}: Source-based routing is properly configured\n" + FormatSourceBasedRoutingTable(m.data)
}

func (m *SourceBasedRoutingModule) WarningTemplate() string {
	return "⚠️ WARNING: {{.FriendlyName}}: Source-based routing is recommended for multi-interface setup\n" + FormatSourceBasedRoutingTable(m.data)
}

func (m *SourceBasedRoutingModule) ErrorTemplate() string {
	return "❌ ERROR: {{.FriendlyName}}: Source-based routing is required but not configured for multi-interface setup\n" + FormatSourceBasedRoutingTable(m.data)
}

func (m *SourceBasedRoutingModule) SuggestedResolutionTemplate() string {
	return "Configure source-based routing (ip rules/ip routes) for Weka to properly handle multi-interface deployments. Use 'ip rule add' and 'ip route add' commands for each interface."
}

// FormatSourceBasedRoutingTable formats the source-based routing interface data as a pretty table
func FormatSourceBasedRoutingTable(data *SourceBasedRoutingModuleData) string {
	if data == nil || len(data.InterfaceGroups) == 0 {
		return "📋 Summary: No multi-interface subnets detected. Single interface per subnet configuration."
	}

	// Define columns for the interface groups table
	columns := []printer.TableColumn{
		{Name: "Subnet", VisibleInWide: false},
		{Name: "Interfaces", VisibleInWide: false},
		{Name: "DPDK Support", VisibleInWide: false},
		{Name: "SBR Configured", VisibleInWide: false},
		{Name: "SBR Required", VisibleInWide: false},
		{Name: "Status", VisibleInWide: false},
	}

	// Build rows from interface groups
	var rows []printer.TableRow
	configuredCount := 0
	requiredCount := 0
	configuredButNotRequired := 0

	for _, group := range data.InterfaceGroups {
		status := "✓ Pass"
		if group.RequiresSBR && !group.HasSBRRules {
			status = "✗ Fail"
		} else if group.HasSBRRules {
			configuredCount++
			if !group.RequiresSBR {
				configuredButNotRequired++
			}
		}
		if group.RequiresSBR {
			requiredCount++
		}

		sbrConfigured := "No"
		if group.HasSBRRules {
			sbrConfigured = "Yes"
		}

		sbrRequired := "No"
		if group.RequiresSBR {
			sbrRequired = "Yes"
		}

		row := printer.TableRow{Values: map[string]interface{}{
			"Subnet":         group.Subnet,
			"Interfaces":     strings.Join(group.Interfaces, ", "),
			"DPDK Support":   group.DPDKSupport,
			"SBR Configured": sbrConfigured,
			"SBR Required":   sbrRequired,
			"Status":         status,
		}}
		rows = append(rows, row)
	}

	// Render table
	p := &printer.TablePrinter{}
	p.SetOptions(printer.PrinterOptions{
		ShowHeader: true,
		TableStyle: printer.TableStyleRoundedBox,
	})
	var sb strings.Builder
	_ = p.Print(columns, rows, &sb)
	tableStr := sb.String()

	// Add summary information
	summaryStr := "📋 Summary:\n"
	summaryStr += fmt.Sprintf("   • Total multi-interface subnets: %d\n", len(data.InterfaceGroups))
	summaryStr += fmt.Sprintf("   • SBR required: %d subnet(s)\n", requiredCount)
	summaryStr += fmt.Sprintf("   • SBR configured: %d subnet(s)\n", configuredCount)
	if len(data.ValidationWarnings) > 0 {
		summaryStr += fmt.Sprintf("   • Warnings: %d\n", len(data.ValidationWarnings))
		for _, warn := range data.ValidationWarnings {
			summaryStr += fmt.Sprintf("      - [%s] %s\n", warn.Severity, warn.Message)
		}
	}
	if len(data.SBRRules) > 0 {
		summaryStr += fmt.Sprintf("   • Total SBR rules configured: %d\n", len(data.SBRRules))
	}

	return tableStr + summaryStr
}
