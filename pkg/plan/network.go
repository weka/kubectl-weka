package plan

import (
	"fmt"
	"github.com/weka/kubectl-weka/pkg/device-support"
	"github.com/weka/kubectl-weka/pkg/hostcheck"
	"github.com/weka/kubectl-weka/pkg/printer"
	"github.com/weka/weka-k8s-api/api/v1alpha1"
	"k8s.io/api/core/v1"
	"net"
	"sort"
	"strings"
)

// ValidateNetworkInterfacesWithStats validates network interfaces and collects statistics
// Returns both the validation result and statistics for summary table
func ValidateNetworkInterfacesWithStats(
	spec interface{},
	nodeHostchecks map[string]*hostcheck.HostChecksResult,
	udpMode bool,
	nodesForRole []v1.Node,
) (NetworkValidationResult, *NetworkValidationStats) {

	stats := NewNetworkValidationStats()

	// Extract network config from spec (could be WekaCluster.Spec or WekaClient.Spec)
	var ethDevice string
	var ethDevices []string

	switch s := spec.(type) {
	case *v1alpha1.WekaClusterSpec:
		if s != nil && s.Network.EthDevice != "" {
			ethDevice = s.Network.EthDevice
			ethDevices = s.Network.EthDevices
		}
	case *v1alpha1.WekaClientSpec:
		if s != nil && s.Network.EthDevice != "" {
			ethDevice = s.Network.EthDevice
			ethDevices = s.Network.EthDevices
		}
	}

	// Build list of NICs to validate
	var nicsToValidate []string
	if ethDevice != "" {
		nicsToValidate = append(nicsToValidate, ethDevice)
	}
	nicsToValidate = append(nicsToValidate, ethDevices...)

	// Initialize stats for each interface
	for _, nic := range nicsToValidate {
		stats.InterfaceStats[nic] = &NetworkInterfaceStats{
			InterfaceName: nic,
			Configured:    0,
			Missing:       0,
			Misconfigured: 0,
		}
	}

	// Validate using the existing function
	result := ValidateNetworkInterfaces(spec, nodeHostchecks, udpMode, nodesForRole)

	// Collect statistics from validation results
	collectNetworkValidationStats(stats, result, nodeHostchecks, nicsToValidate, nodesForRole)

	return result, stats
}

// collectNetworkValidationStats collects statistics from validation errors/warnings
func collectNetworkValidationStats(
	stats *NetworkValidationStats,
	result NetworkValidationResult,
	nodeHostchecks map[string]*hostcheck.HostChecksResult,
	nicsToValidate []string,
	nodesForRole []v1.Node,
) {
	// Build error map for easy lookup
	errorsByNodeNIC := make(map[string]map[string][]NetworkValidationError)
	for _, err := range result.Errors {
		if _, ok := errorsByNodeNIC[err.NodeName]; !ok {
			errorsByNodeNIC[err.NodeName] = make(map[string][]NetworkValidationError)
		}
		errorsByNodeNIC[err.NodeName][err.NICName] = append(errorsByNodeNIC[err.NodeName][err.NICName], err)
	}

	// Count stats for each interface on each node
	for _, nic := range nicsToValidate {
		if stat, ok := stats.InterfaceStats[nic]; ok {
			for _, node := range nodesForRole {
				// Check if there are errors for this node/NIC combination
				if nodeErrors, hasNode := errorsByNodeNIC[node.Name]; hasNode {
					if nicErrors, hasNIC := nodeErrors[nic]; hasNIC && len(nicErrors) > 0 {
						// This NIC has errors on this node
						stat.Misconfigured++
					} else {
						// No errors for this NIC on this node - it's configured
						stat.Configured++
					}
				} else {
					// Check if hostcheck data is missing entirely
					if _, hasHostCheck := nodeHostchecks[node.Name]; !hasHostCheck {
						// No hostcheck data means we can't tell if it's missing or misconfigured
						// Treat as missing
						stat.Missing++
					} else {
						// Has hostcheck data but no error entry - check if NIC exists
						if hostCheck := nodeHostchecks[node.Name]; findNICInHostCheck(hostCheck, nic) == nil {
							stat.Missing++
						} else {
							stat.Configured++
						}
					}
				}
			}
		}
	}
}

// printNetworkInterfaceSummaryTable prints a summary table of network interface validation
func printNetworkInterfaceSummaryTable(stats *NetworkValidationStats) string {
	if stats == nil || len(stats.InterfaceStats) == 0 {
		return "No interfaces found"
	}

	p := &printer.TablePrinter{}
	p.SetOptions(printer.PrinterOptions{
		ShowHeader:        true,
		WideOutput:        true,
		HideEmptyColumns:  false,
		IndentByNumSpaces: 5,
		TableStyle:        printer.TableStyleRoundedBox,
	})

	columns := []printer.TableColumn{
		{Name: "INTERFACE"},
		{Name: "CONFIGURED"},
		{Name: "MISSING"},
		{Name: "MISCONFIGURED"},
	}
	var rows []printer.TableRow
	// Sort interfaces by name for consistent output
	var interfaces []string
	for _, stat := range stats.InterfaceStats {
		interfaces = append(interfaces, stat.InterfaceName)
	}
	sort.Strings(interfaces)

	for _, ifName := range interfaces {
		stat := stats.InterfaceStats[ifName]
		row := printer.TableRow{Values: make(map[string]interface{})}
		row.Values["INTERFACE"] = stat.InterfaceName
		row.Values["CONFIGURED"] = fmt.Sprintf("%d", stat.Configured)
		row.Values["MISSING"] = fmt.Sprintf("%d", stat.Missing)
		row.Values["MISCONFIGURED"] = fmt.Sprintf("%d", stat.Misconfigured)
		rows = append(rows, row)
	}
	// Render output
	var sb strings.Builder
	_ = p.Print(columns, rows, &sb)
	return sb.String()

}

// printNetworkValidationTestsDescription prints what network validations are being performed
func printNetworkValidationTestsDescription() {
	fmt.Println("Validating network interfaces against the following criteria:")
	fmt.Println("  • Interface exists on the node")
	fmt.Println("  • Interface has an IP address configured")
	fmt.Println("  • Interface has valid speed/rate information")
	fmt.Println("  • Interface is compatible with WEKA (supports UDP and/or DPDK mode)")
	fmt.Println("  • No duplicate IP addresses across nodes")
	fmt.Println()
}

// validateUDPModeWithoutSpecificInterfaces validates that at least one interface supports UDP mode
// when no specific interfaces are configured. This is used for UDP mode deployments that can
// auto-discover available interfaces.
func validateUDPModeWithoutSpecificInterfaces(
	nodeHostchecks map[string]*hostcheck.HostChecksResult,
	nodesForRole []v1.Node,
) NetworkValidationResult {
	result := NetworkValidationResult{
		Errors:   []NetworkValidationError{},
		Warnings: []NetworkValidationError{},
		Valid:    true,
	}

	if len(nodesForRole) == 0 {
		result.Errors = append(result.Errors, NetworkValidationError{
			Severity: "ERROR",
			Message:  "No nodes available for validation",
		})
		result.Valid = false
		return result
	}

	// Check each node has at least one UDP-capable interface
	for _, node := range nodesForRole {
		hostCheck, exists := nodeHostchecks[node.Name]
		if !exists {
			result.Errors = append(result.Errors, NetworkValidationError{
				NodeName: node.Name,
				Severity: "ERROR",
				Message:  "No hostcheck data available for node (please run 'kubectl weka preflight nodes' first)",
			})
			result.Valid = false
			continue
		}

		// Check if this node has at least one UDP-capable interface
		hasUDPCapableInterface := false
		var availableInterfaces []string

		if hostCheck.NetworkInterfaces != nil {
			for _, iface := range hostCheck.NetworkInterfaces {
				// Skip if interface has no IP address
				if iface.IP == "" {
					continue
				}

				// Skip if interface has no speed
				if iface.MaxSpeed == 0 && iface.MaxRate == "" {
					continue
				}

				availableInterfaces = append(availableInterfaces, iface.Name)

				// Get NIC capabilities
				caps := device_support.GetNICCapabilities(iface.VendorModel)
				if caps != nil && (caps.SupportedByWekaUdp || caps.SupportedByWekaDpdk) {
					hasUDPCapableInterface = true
					break
				}
			}
		}

		if !hasUDPCapableInterface {
			if len(availableInterfaces) == 0 {
				result.Errors = append(result.Errors, NetworkValidationError{
					NodeName: node.Name,
					Severity: "ERROR",
					Message:  "No network interfaces with IP address found on node",
				})
			} else {
				result.Errors = append(result.Errors, NetworkValidationError{
					NodeName: node.Name,
					Severity: "ERROR",
					Message:  fmt.Sprintf("No interfaces support WEKA UDP mode. Available interfaces: %v", availableInterfaces),
				})
			}
			result.Valid = false
		}
	}

	return result
}

// ValidateNetworkInterfaces validates network interfaces specified in the Weka resource
// Parameters:
//   - spec: Either *WekaClusterSpec or *WekaClientSpec
//   - nodeHostchecks: Map of node name to HostChecksResult from preflight checks
//   - udpMode: Whether UDP mode is desired (if false, DPDK mode is required)
//   - nodesForRole: Nodes that match the nodeSelector for this resource
//
// Returns validation results with errors and warnings
func ValidateNetworkInterfaces(
	spec interface{},
	nodeHostchecks map[string]*hostcheck.HostChecksResult,
	udpMode bool,
	nodesForRole []v1.Node,
) NetworkValidationResult {

	result := NetworkValidationResult{
		Errors:   []NetworkValidationError{},
		Warnings: []NetworkValidationError{},
		Valid:    true,
	}

	// Extract network config from spec (could be WekaCluster.Spec or WekaClient.Spec)
	var ethDevice string
	var ethDevices []string

	switch s := spec.(type) {
	case *v1alpha1.WekaClusterSpec:
		if s != nil && s.Network.EthDevice != "" {
			ethDevice = s.Network.EthDevice
			ethDevices = s.Network.EthDevices
		}
	case *v1alpha1.WekaClientSpec:
		if s != nil && s.Network.EthDevice != "" {
			ethDevice = s.Network.EthDevice
			ethDevices = s.Network.EthDevices
		}
	default:
		result.Errors = append(result.Errors, NetworkValidationError{
			Severity: "ERROR",
			Message:  "Invalid spec type for network validation",
		})
		result.Valid = false
		return result
	}

	// Build list of NICs to validate
	var nicsToValidate []string
	if ethDevice != "" {
		nicsToValidate = append(nicsToValidate, ethDevice)
	}
	nicsToValidate = append(nicsToValidate, ethDevices...)

	if len(nicsToValidate) == 0 {
		// In UDP mode, no specific interfaces required - we'll validate at least one exists
		// In DPDK mode, interfaces are required
		if !udpMode {
			result.Errors = append(result.Errors, NetworkValidationError{
				Severity: "ERROR",
				Message:  "No network interfaces specified (network.ethDevice or network.ethDevices required for DPDK mode)",
			})
			result.Valid = false
			return result
		}
		// For UDP mode without specific interfaces, validate at least one UDP-capable interface exists
		return validateUDPModeWithoutSpecificInterfaces(nodeHostchecks, nodesForRole)
	}

	// Track IP addresses for conflict detection
	ipAddressToNode := make(map[string][]string) // IP -> list of nodes
	nicIPs := make(map[string]string)            // NIC name -> IP

	// Validate on each node
	for _, node := range nodesForRole {
		hostCheck, exists := nodeHostchecks[node.Name]
		if !exists {
			result.Errors = append(result.Errors, NetworkValidationError{
				NodeName: node.Name,
				Severity: "ERROR",
				Message:  "No hostcheck data available for node (please run 'kubectl weka preflight nodes' first)",
			})
			result.Valid = false
			continue
		}

		// Validate each NIC on this node
		for _, nicName := range nicsToValidate {
			err := validateNICOnNode(hostCheck, node.Name, nicName, udpMode, &result)
			if err != nil {
				result.Valid = false
			}

			// Collect IP addresses for conflict detection
			if iface := findNICInHostCheck(hostCheck, nicName); iface != nil && iface.IP != "" {
				ip := extractIPAddress(iface.IP)
				ipAddressToNode[ip] = append(ipAddressToNode[ip], node.Name)
				nicIPs[nicName] = ip
			}
		}
	}

	// Check for IP address conflicts
	for ip, nodes := range ipAddressToNode {
		if len(nodes) > 1 {
			result.Errors = append(result.Errors, NetworkValidationError{
				Severity: "ERROR",
				Message:  fmt.Sprintf("Multiple nodes have same IP address %s: %v", ip, nodes),
			})
			result.Valid = false
		}
	}

	return result
}

// validateNICOnNode validates a single NIC on a specific node
func validateNICOnNode(
	hostCheck *hostcheck.HostChecksResult,
	nodeName string,
	nicName string,
	udpMode bool,
	result *NetworkValidationResult,
) error {

	// Find the NIC in hostcheck results
	iface := findNICInHostCheck(hostCheck, nicName)
	if iface == nil {
		result.Errors = append(result.Errors, NetworkValidationError{
			NodeName: nodeName,
			NICName:  nicName,
			Severity: "ERROR",
			Message:  fmt.Sprintf("NIC not found on node"),
		})
		return fmt.Errorf("NIC %s not found on node %s", nicName, nodeName)
	}

	// Rule 9: Check if NIC has IP address
	if iface.IP == "" {
		result.Errors = append(result.Errors, NetworkValidationError{
			NodeName: nodeName,
			NICName:  nicName,
			Severity: "ERROR",
			Message:  "NIC must have an IP address configured",
		})
		return fmt.Errorf("NIC %s on node %s has no IP address", nicName, nodeName)
	}

	// Rule 8: Check if NIC has speed 0
	if iface.MaxSpeed == 0 && iface.MaxRate == "" {
		result.Errors = append(result.Errors, NetworkValidationError{
			NodeName: nodeName,
			NICName:  nicName,
			Severity: "ERROR",
			Message:  "NIC speed/rate is 0 - interface may not be properly detected",
		})
		return fmt.Errorf("NIC %s on node %s reports speed 0", nicName, nodeName)
	}

	// For bond interfaces
	if iface.IsBond() {
		return validateBond(hostCheck, nodeName, iface, udpMode, result)
	}

	// For regular interfaces, check WEKA support
	return validateNICSupport(nodeName, iface, udpMode, result)
}

// validateBond validates that a bond supports either UDP or DPDK mode
func validateBond(
	hostCheck *hostcheck.HostChecksResult,
	nodeName string,
	bondIface *hostcheck.NetworkInterface,
	udpMode bool,
	result *NetworkValidationResult,
) error {

	if len(bondIface.BondSlaves) == 0 {
		result.Errors = append(result.Errors, NetworkValidationError{
			NodeName: nodeName,
			NICName:  bondIface.Name,
			Severity: "ERROR",
			Message:  "Bond has no slaves configured",
		})
		return fmt.Errorf("bond %s has no slaves", bondIface.Name)
	}

	// Get capabilities of first slave (assuming homogeneous bond)
	slaveName := bondIface.BondSlaves[0]
	slave := findNICInHostCheck(hostCheck, slaveName)
	if slave == nil {
		result.Errors = append(result.Errors, NetworkValidationError{
			NodeName: nodeName,
			NICName:  bondIface.Name,
			Severity: "ERROR",
			Message:  fmt.Sprintf("Bond slave %s not found", slaveName),
		})
		return fmt.Errorf("bond slave %s not found", slaveName)
	}

	// Validate the slave's support for the desired mode
	return validateNICSupport(nodeName, slave, udpMode, result)
}

// validateNICSupport checks if a NIC supports the desired operation mode
func validateNICSupport(
	nodeName string,
	iface *hostcheck.NetworkInterface,
	udpMode bool,
	result *NetworkValidationResult,
) error {

	// Get NIC capabilities
	caps := device_support.GetNICCapabilities(iface.VendorModel)
	if caps == nil {
		result.Errors = append(result.Errors, NetworkValidationError{
			NodeName: nodeName,
			NICName:  iface.Name,
			Severity: "ERROR",
			Message:  fmt.Sprintf("NIC model %s has unknown WEKA support (vendor:model=%s)", iface.Model, iface.VendorModel),
		})
		return fmt.Errorf("unknown NIC capability: %s", iface.VendorModel)
	}

	if udpMode {
		// Rule: UDP mode requires either UDP support or DPDK support
		if !caps.SupportedByWekaUdp && !caps.SupportedByWekaDpdk {
			result.Errors = append(result.Errors, NetworkValidationError{
				NodeName: nodeName,
				NICName:  iface.Name,
				Severity: "ERROR",
				Message:  fmt.Sprintf("NIC %s does not support WEKA in UDP or DPDK mode", iface.Model),
			})
			return fmt.Errorf("NIC not supported in UDP mode: %s", iface.VendorModel)
		}
	} else {
		// Rule: DPDK mode required
		if !caps.SupportedByWekaDpdk {
			result.Errors = append(result.Errors, NetworkValidationError{
				NodeName: nodeName,
				NICName:  iface.Name,
				Severity: "ERROR",
				Message:  fmt.Sprintf("NIC %s does not support WEKA in DPDK mode", iface.Model),
			})
			return fmt.Errorf("NIC not supported in DPDK mode: %s", iface.VendorModel)
		}

		// Rule 5: If NIC supports DPDK per-process, warn
		if caps.SupportedByWekaDpdk && !caps.SupportedByWekaDpdkSingleNic {
			result.Warnings = append(result.Warnings, NetworkValidationError{
				NodeName: nodeName,
				NICName:  iface.Name,
				Severity: "WARN",
				Message:  fmt.Sprintf("NIC %s requires per-process DPDK allocation - validation not yet supported", iface.Model),
			})
		}
	}

	return nil
}

// findNICInHostCheck finds a NIC by name in hostcheck results
func findNICInHostCheck(hostCheck *hostcheck.HostChecksResult, nicName string) *hostcheck.NetworkInterface {
	if hostCheck == nil || hostCheck.NetworkInterfaces == nil {
		return nil
	}

	for _, iface := range hostCheck.NetworkInterfaces {
		if iface.Name == nicName {
			return iface
		}
	}

	return nil
}

// extractIPAddress extracts just the IP part from CIDR notation
func extractIPAddress(cidr string) string {
	if idx := strings.Index(cidr, "/"); idx > 0 {
		return cidr[:idx]
	}
	return cidr
}

// isValidIP checks if a string is a valid IP address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// formatValidationErrors formats validation errors for display
func formatValidationErrors(errors []NetworkValidationError) string {
	if len(errors) == 0 {
		return ""
	}

	var output strings.Builder
	for _, err := range errors {
		output.WriteString(fmt.Sprintf("  ❌ %s\n", err.String()))
	}
	return output.String()
}

// formatValidationWarnings formats validation warnings for display
func formatValidationWarnings(warnings []NetworkValidationError) string {
	if len(warnings) == 0 {
		return ""
	}

	var output strings.Builder
	for _, warn := range warnings {
		output.WriteString(fmt.Sprintf("  ⚠️  %s\n", warn.String()))
	}
	return output.String()
}
